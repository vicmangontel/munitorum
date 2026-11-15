package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/vicmangontel/munitorum/internal/domain"
)

// RenameAccountInput is the account to rename and the typed new name.
type RenameAccountInput struct {
	AccountID int64
	Name      string
}

// OpeningBalanceInput is the account and the typed new opening balance.
type OpeningBalanceInput struct {
	AccountID int64
	Amount    string
}

// UpdateCommunityName changes the coto name (setup rule 1) and returns the
// settings. It returns ErrNotFound before setup has completed.
func (s *SetupService) UpdateCommunityName(ctx context.Context, name string) (settings Settings, err error) {
	started := time.Now()
	defer func() { s.emit(ctx, "update_community_name", started, err, nil) }()

	name, ok := trimmedName(name, maxCommunityName)
	if !ok {
		return Settings{}, ValidationError{Fields: map[string]string{FieldCommunityName: CodeCommunityNameLength}}
	}
	if err := s.repo.UpdateCommunityName(ctx, name, s.clock.Now()); err != nil {
		return Settings{}, fmt.Errorf("update community name: %w", err)
	}
	return s.GetSettings(ctx)
}

// RenameAccount renames an account (setup rule 5). The name must differ from the
// other account's name in any case; the account's own name may change case.
// It returns ErrNotFound for an unknown account or before setup has completed.
func (s *SetupService) RenameAccount(ctx context.Context, in RenameAccountInput) (account Account, err error) {
	started := time.Now()
	fields := []any{"account_id", in.AccountID}
	defer func() { s.emit(ctx, "rename_account", started, err, fields) }()

	settings, i, err := s.findAccount(ctx, in.AccountID)
	if err != nil {
		return Account{}, fmt.Errorf("rename account %d: %w", in.AccountID, err)
	}
	name, ok := trimmedName(in.Name, maxAccountName)
	if !ok {
		return Account{}, ValidationError{Fields: map[string]string{FieldAccountName(i): CodeAccountNameLength}}
	}
	for _, other := range settings.Accounts {
		// Full Unicode case folding; the column's NOCASE collation only folds ASCII.
		if other.ID != in.AccountID && strings.EqualFold(other.Name, name) {
			return Account{}, ValidationError{Fields: map[string]string{FieldAccountName(i): CodeAccountNameDuplicate}}
		}
	}
	if err := s.repo.RenameAccount(ctx, in.AccountID, name, s.clock.Now()); err != nil {
		return Account{}, fmt.Errorf("rename account %d: %w", in.AccountID, err)
	}
	return s.reloadAccount(ctx, in.AccountID)
}

// ValidateOpeningBalance applies setup rule 6 and writes nothing, so a form can
// show its errors before the confirmation that UpdateOpeningBalance needs.
func (s *SetupService) ValidateOpeningBalance(ctx context.Context, in OpeningBalanceInput) error {
	_, err := s.checkOpeningBalance(ctx, in)
	return err
}

// UpdateOpeningBalance changes an account's opening balance (setup rule 6). It
// returns ErrNotFound for an unknown account or before setup has completed.
func (s *SetupService) UpdateOpeningBalance(ctx context.Context, in OpeningBalanceInput) (account Account, err error) {
	started := time.Now()
	fields := []any{"account_id", in.AccountID}
	defer func() { s.emit(ctx, "update_opening_balance", started, err, fields) }()

	change, err := s.checkOpeningBalance(ctx, in)
	if err != nil {
		return Account{}, err
	}
	fields = append(fields, "old_balance_cents", int64(change.old), "new_balance_cents", int64(change.new))
	if err := s.repo.UpdateOpeningBalance(ctx, in.AccountID, change.new, s.clock.Now()); err != nil {
		return Account{}, fmt.Errorf("update opening balance of account %d: %w", in.AccountID, err)
	}
	return s.reloadAccount(ctx, in.AccountID)
}

// balanceChange is a validated opening balance edit.
type balanceChange struct{ old, new domain.Money }

func (s *SetupService) checkOpeningBalance(ctx context.Context, in OpeningBalanceInput) (balanceChange, error) {
	settings, i, err := s.findAccount(ctx, in.AccountID)
	if err != nil {
		return balanceChange{}, fmt.Errorf("update opening balance of account %d: %w", in.AccountID, err)
	}
	amount, code := parseBalance(in.Amount)
	if code != "" {
		return balanceChange{}, ValidationError{Fields: map[string]string{FieldOpeningBalance(i): code}}
	}
	return balanceChange{old: settings.Accounts[i].OpeningBalance, new: amount}, nil
}

// findAccount returns the settings and the position of the account in them,
// which is the index used in validation field keys.
func (s *SetupService) findAccount(ctx context.Context, id int64) (Settings, int, error) {
	settings, err := s.GetSettings(ctx)
	if err != nil {
		return Settings{}, 0, err
	}
	for i, a := range settings.Accounts {
		if a.ID == id {
			return settings, i, nil
		}
	}
	return Settings{}, 0, ErrNotFound
}

func (s *SetupService) reloadAccount(ctx context.Context, id int64) (Account, error) {
	settings, i, err := s.findAccount(ctx, id)
	if err != nil {
		return Account{}, fmt.Errorf("reload account %d: %w", id, err)
	}
	return settings.Accounts[i], nil
}
