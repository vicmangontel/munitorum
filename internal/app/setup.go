package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/vicmangontel/munitorum/internal/domain"
)

// AccountKind is the kind of an account. Setup creates exactly one of each.
type AccountKind string

const (
	KindCash AccountKind = "cash"
	KindBank AccountKind = "bank"
)

// Settings is the coto's configuration and its accounts in sort order.
type Settings struct {
	CommunityName    string
	HousePrefix      string
	HouseCount       int
	OpeningDate      time.Time
	SetupCompletedAt time.Time
	Accounts         []Account
}

// Account is a cash or bank account with its opening balance.
type Account struct {
	ID             int64
	Name           string
	Kind           AccountKind
	OpeningBalance domain.Money
	SortOrder      int
}

// SetupState tells startup which screen to open.
type SetupState struct{ Completed bool }

// NewSetup is a validated setup ready to be written. Houses are in number order.
type NewSetup struct {
	CommunityName string
	HousePrefix   string
	HouseCount    int
	OpeningDate   time.Time
	Fee           domain.Money
	Accounts      [2]NewAccount // cash, then bank
	Houses        []NewHouse
	Now           time.Time // timestamp for every row written
}

// NewAccount is an account to create.
type NewAccount struct {
	Name           string
	Kind           AccountKind
	OpeningBalance domain.Money
}

// NewHouse is a house to create.
type NewHouse struct {
	Number  int
	Code    string
	Opening domain.HouseOpening
}

// SetupRepository is what SetupService needs from storage.
type SetupRepository interface {
	// SetupCompleted reports whether the settings row exists.
	SetupCompleted(ctx context.Context) (bool, error)
	// LoadSettings returns ErrNotFound when setup has not completed.
	LoadSettings(ctx context.Context) (Settings, error)
	// SaveSetup writes settings, both accounts, every house, and the initial fee
	// rate in one transaction. It returns ErrConflict when setup already completed.
	SaveSetup(ctx context.Context, s NewSetup) error
	// UpdateCommunityName, RenameAccount and UpdateOpeningBalance each change one
	// row and return ErrNotFound when it does not exist. now stamps updated_at.
	UpdateCommunityName(ctx context.Context, name string, now time.Time) error
	RenameAccount(ctx context.Context, id int64, name string, now time.Time) error
	UpdateOpeningBalance(ctx context.Context, id int64, balance domain.Money, now time.Time) error
}

// SetupService creates the coto and reads its settings.
type SetupService struct {
	repo  SetupRepository
	clock Clock
	log   *slog.Logger
}

// NewSetupService returns a SetupService.
func NewSetupService(repo SetupRepository, clock Clock, log *slog.Logger) *SetupService {
	return &SetupService{repo: repo, clock: clock, log: log}
}

// emit writes the wide event of one operation: its action, outcome, duration
// and the fields learned on the way. It never logs names or other free text.
func (s *SetupService) emit(ctx context.Context, action string, started time.Time, err error, fields []any) {
	outcome, level := "ok", slog.LevelInfo
	var ve ValidationError
	switch {
	case errors.As(err, &ve):
		outcome, level = "validation", slog.LevelWarn
		fields = append(fields, "invalid_field_count", len(ve.Fields))
	case err != nil:
		outcome, level = "error", slog.LevelError
		fields = append(fields, "error", err.Error())
	}
	fields = append(fields, "action", action, "outcome", outcome, "duration_ms", time.Since(started).Milliseconds())
	s.log.Log(ctx, level, action, fields...)
}

// GetSetupState reports whether setup has completed.
func (s *SetupService) GetSetupState(ctx context.Context) (SetupState, error) {
	done, err := s.repo.SetupCompleted(ctx)
	if err != nil {
		return SetupState{}, fmt.Errorf("get setup state: %w", err)
	}
	return SetupState{Completed: done}, nil
}

// GetSettings returns the settings and accounts, or ErrNotFound before setup.
func (s *SetupService) GetSettings(ctx context.Context) (Settings, error) {
	settings, err := s.repo.LoadSettings(ctx)
	if err != nil {
		return Settings{}, fmt.Errorf("get settings: %w", err)
	}
	return settings, nil
}

// ValidateSetup applies the setup rules without writing anything. It returns a
// ValidationError listing every invalid field, or the summary of the setup.
func (s *SetupService) ValidateSetup(in SetupInput) (SetupSummary, error) {
	p, err := parseSetup(in, s.clock.Now())
	if err != nil {
		return SetupSummary{}, err
	}
	return p.summary(), nil
}

// CompleteSetup validates in and writes the coto in one transaction. It returns
// ErrConflict when setup already completed. It emits one wide event.
func (s *SetupService) CompleteSetup(ctx context.Context, in SetupInput) (settings Settings, err error) {
	started := time.Now()
	now := s.clock.Now()
	var fields []any
	defer func() { s.emit(ctx, "complete_setup", started, err, fields) }()

	p, err := parseSetup(in, now)
	if err != nil {
		return Settings{}, err
	}
	fields = append(fields,
		"house_count", p.count,
		"fee_amount_cents", int64(p.fee),
		"opening_balance_cash_cents", int64(p.balances[0]),
		"opening_balance_bank_cents", int64(p.balances[1]),
	)

	houses := make([]NewHouse, p.count)
	for i := range houses {
		houses[i] = NewHouse{
			Number:  i + 1,
			Code:    domain.HouseCode(p.prefix, i+1, p.count),
			Opening: p.houses[i],
		}
	}
	err = s.repo.SaveSetup(ctx, NewSetup{
		CommunityName: p.name,
		HousePrefix:   p.prefix,
		HouseCount:    p.count,
		OpeningDate:   p.openingDate,
		Fee:           p.fee,
		Accounts: [2]NewAccount{
			{Name: p.accountNames[0], Kind: KindCash, OpeningBalance: p.balances[0]},
			{Name: p.accountNames[1], Kind: KindBank, OpeningBalance: p.balances[1]},
		},
		Houses: houses,
		Now:    now,
	})
	if err != nil {
		return Settings{}, fmt.Errorf("complete setup: %w", err)
	}
	return s.GetSettings(ctx)
}
