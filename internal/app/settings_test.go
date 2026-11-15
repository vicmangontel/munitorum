package app_test

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/vicmangontel/munitorum/internal/app"
)

// setupDone returns a service over a completed coto (cash "Efectivo" with
// $7,500.00, bank "Banco (NU)" with $0.00) and its settings. The wide events of
// the setup run are dropped from the returned buffer.
func setupDone(t *testing.T) (*app.SetupService, *sql.DB, *bytes.Buffer, app.Settings) {
	t.Helper()
	svc, db, events := newSetup(t)
	settings, err := svc.CompleteSetup(context.Background(), validInput("18"))
	if err != nil {
		t.Fatal(err)
	}
	events.Reset()
	return svc, db, events, settings
}

func TestUpdateCommunityName_ChecksTheNameLength(t *testing.T) {
	cases := []struct {
		name, input, want string // want is the stored name; empty when rejected
	}{
		{"one character", "X", "X"},
		{"60 characters", strings.Repeat("ñ", 60), strings.Repeat("ñ", 60)},
		{"trimmed", "  Los Álamos  ", "Los Álamos"},
		{"blank", "   ", ""},
		{"61 characters", strings.Repeat("ñ", 61), ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange
			svc, _, _, _ := setupDone(t)
			ctx := context.Background()

			// Act
			_, err := svc.UpdateCommunityName(ctx, c.input)

			// Assert
			stored, getErr := svc.GetSettings(ctx)
			if getErr != nil {
				t.Fatal(getErr)
			}
			if c.want == "" {
				if got := invalidFields(t, err); len(got) != 1 || got[app.FieldCommunityName] != app.CodeCommunityNameLength {
					t.Fatalf("invalid fields = %v, want only community_name_length", got)
				}
				if stored.CommunityName != "Coto Los Pinos" {
					t.Fatalf("stored name = %q, want it unchanged", stored.CommunityName)
				}
				return
			}
			if err != nil || stored.CommunityName != c.want {
				t.Fatalf("stored name = %q, error = %v; want %q", stored.CommunityName, err, c.want)
			}
		})
	}
}

func TestUpdateCommunityName_ReturnsTheUpdatedSettings(t *testing.T) {
	// Arrange
	svc, _, _, _ := setupDone(t)

	// Act
	got, err := svc.UpdateCommunityName(context.Background(), "Los Álamos")

	// Assert
	if err != nil || got.CommunityName != "Los Álamos" || len(got.Accounts) != 2 {
		t.Fatalf("settings = %+v, error = %v", got, err)
	}
}

func TestUpdateCommunityName_ReturnsNotFoundBeforeSetup(t *testing.T) {
	// Arrange
	svc, _, _ := newSetup(t)

	// Act
	_, err := svc.UpdateCommunityName(context.Background(), "Los Álamos")

	// Assert
	if !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
}

func TestRenameAccount_ChecksLengthAndUniquenessAgainstTheOtherAccount(t *testing.T) {
	cases := []struct {
		name    string
		account int // index of the account being renamed
		input   string
		want    string // stored name; empty when rejected
		code    string // expected code when rejected
	}{
		{"one character", 0, "E", "E", ""},
		{"30 characters", 0, strings.Repeat("a", 30), strings.Repeat("a", 30), ""},
		{"trimmed", 1, "  Banco Nuevo ", "Banco Nuevo", ""},
		{"own name in another case", 0, "EFECTIVO", "EFECTIVO", ""},
		{"blank", 0, "  ", "", app.CodeAccountNameLength},
		{"31 characters", 0, strings.Repeat("a", 31), "", app.CodeAccountNameLength},
		{"the other account's name", 1, "Efectivo", "", app.CodeAccountNameDuplicate},
		{"the other account's name in another case", 1, "efectivo", "", app.CodeAccountNameDuplicate},
		{"the other account's name after trimming", 0, "  banco (nu) ", "", app.CodeAccountNameDuplicate},
		{"a different letter, not a case variant", 1, "EFECTIVÓ", "EFECTIVÓ", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange
			svc, _, _, settings := setupDone(t)
			id := settings.Accounts[c.account].ID
			before := settings.Accounts[c.account].Name

			// Act
			got, err := svc.RenameAccount(context.Background(), app.RenameAccountInput{AccountID: id, Name: c.input})

			// Assert
			stored, getErr := svc.GetSettings(context.Background())
			if getErr != nil {
				t.Fatal(getErr)
			}
			if c.want == "" {
				fields := invalidFields(t, err)
				if key := app.FieldAccountName(c.account); len(fields) != 1 || fields[key] != c.code {
					t.Fatalf("invalid fields = %v, want only %s = %s", fields, key, c.code)
				}
				if stored.Accounts[c.account].Name != before {
					t.Fatalf("stored name = %q, want it unchanged (%q)", stored.Accounts[c.account].Name, before)
				}
				return
			}
			if err != nil || got.Name != c.want || stored.Accounts[c.account].Name != c.want {
				t.Fatalf("returned %q, stored %q, error = %v; want %q", got.Name, stored.Accounts[c.account].Name, err, c.want)
			}
		})
	}
}

func TestRenameAccount_ReturnsNotFound(t *testing.T) {
	cases := []struct {
		name        string
		beforeSetup bool
		accountID   int64
	}{
		{"for an unknown account", false, 999},
		{"before setup", true, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange
			svc, _, _, _ := setupDone(t)
			if c.beforeSetup {
				svc, _, _ = newSetup(t)
			}

			// Act
			_, err := svc.RenameAccount(context.Background(), app.RenameAccountInput{AccountID: c.accountID, Name: "Caja"})

			// Assert
			if !errors.Is(err, app.ErrNotFound) {
				t.Fatalf("error = %v, want ErrNotFound", err)
			}
		})
	}
}

func TestUpdateOpeningBalance_ChecksTheAmount(t *testing.T) {
	cases := []struct {
		name, input string
		want        int64 // stored cents
		code        string
	}{
		{"zero", "0", 0, ""},
		{"a formatted amount", "$1,234.50", 123450, ""},
		{"one centavo", "0.01", 1, ""},
		{"negative", "-0.01", 0, app.CodeBalanceNegative},
		{"unreadable", "mucho", 0, app.CodeAmountInvalid},
		{"three decimals", "1.234", 0, app.CodeAmountInvalid},
		{"empty", "", 0, app.CodeAmountInvalid},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange
			svc, _, _, settings := setupDone(t)
			cash := settings.Accounts[0]

			// Act
			got, err := svc.UpdateOpeningBalance(context.Background(), app.OpeningBalanceInput{AccountID: cash.ID, Amount: c.input})

			// Assert
			stored, getErr := svc.GetSettings(context.Background())
			if getErr != nil {
				t.Fatal(getErr)
			}
			if c.code != "" {
				fields := invalidFields(t, err)
				if key := app.FieldOpeningBalance(0); len(fields) != 1 || fields[key] != c.code {
					t.Fatalf("invalid fields = %v, want only %s = %s", fields, key, c.code)
				}
				if stored.Accounts[0].OpeningBalance != cash.OpeningBalance {
					t.Fatalf("stored balance = %d, want it unchanged", stored.Accounts[0].OpeningBalance)
				}
				return
			}
			if err != nil || int64(got.OpeningBalance) != c.want || int64(stored.Accounts[0].OpeningBalance) != c.want {
				t.Fatalf("returned %d, stored %d, error = %v; want %d", got.OpeningBalance, stored.Accounts[0].OpeningBalance, err, c.want)
			}
		})
	}
}

func TestUpdateOpeningBalance_ChangesOnlyTheNamedAccount(t *testing.T) {
	// Arrange
	svc, _, _, settings := setupDone(t)
	bank := settings.Accounts[1]

	// Act
	_, err := svc.UpdateOpeningBalance(context.Background(), app.OpeningBalanceInput{AccountID: bank.ID, Amount: "2,000"})

	// Assert
	stored, getErr := svc.GetSettings(context.Background())
	if err != nil || getErr != nil {
		t.Fatal(err, getErr)
	}
	if stored.Accounts[0].OpeningBalance != 750000 || stored.Accounts[1].OpeningBalance != 200000 {
		t.Fatalf("balances = %d, %d; want 750000, 200000", stored.Accounts[0].OpeningBalance, stored.Accounts[1].OpeningBalance)
	}
}

func TestUpdateOpeningBalance_ReturnsNotFound(t *testing.T) {
	cases := []struct {
		name        string
		beforeSetup bool
	}{
		{"for an unknown account", false},
		{"before setup", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange
			svc, _, _, _ := setupDone(t)
			if c.beforeSetup {
				svc, _, _ = newSetup(t)
			}

			// Act
			_, err := svc.UpdateOpeningBalance(context.Background(), app.OpeningBalanceInput{AccountID: 999, Amount: "10"})

			// Assert
			if !errors.Is(err, app.ErrNotFound) {
				t.Fatalf("error = %v, want ErrNotFound", err)
			}
		})
	}
}

func TestValidateOpeningBalance_ReportsErrorsAndWritesNothing(t *testing.T) {
	cases := []struct {
		name     string
		unknown  bool // validate an account that does not exist
		amount   string
		code     string // expected validation code; empty when there is none
		notFound bool
	}{
		{"a valid amount", false, "9,999", "", false},
		{"a negative amount", false, "-5", app.CodeBalanceNegative, false},
		{"an unknown account", true, "5", "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange
			svc, _, _, settings := setupDone(t)
			cash := settings.Accounts[0]
			ctx := context.Background()
			id := cash.ID
			if c.unknown {
				id = 999
			}

			// Act
			err := svc.ValidateOpeningBalance(ctx, app.OpeningBalanceInput{AccountID: id, Amount: c.amount})

			// Assert
			stored, getErr := svc.GetSettings(ctx)
			if getErr != nil {
				t.Fatal(getErr)
			}
			if stored.Accounts[0].OpeningBalance != cash.OpeningBalance {
				t.Fatalf("stored balance = %d, want it unchanged (%d)", stored.Accounts[0].OpeningBalance, cash.OpeningBalance)
			}
			switch {
			case c.notFound:
				if !errors.Is(err, app.ErrNotFound) {
					t.Fatalf("error = %v, want ErrNotFound", err)
				}
			case c.code != "":
				if got := invalidFields(t, err); got[app.FieldOpeningBalance(0)] != c.code {
					t.Fatalf("invalid fields = %v, want %s", got, c.code)
				}
			default:
				if err != nil {
					t.Fatalf("error = %v, want nil", err)
				}
			}
		})
	}
}

// operation runs one update operation; id is the cash account.
type operation func(ctx context.Context, svc *app.SetupService, id int64) error

func updateName(ctx context.Context, svc *app.SetupService, _ int64) error {
	_, err := svc.UpdateCommunityName(ctx, "Los Álamos")
	return err
}

func renameAccount(ctx context.Context, svc *app.SetupService, id int64) error {
	_, err := svc.RenameAccount(ctx, app.RenameAccountInput{AccountID: id, Name: "Caja chica"})
	return err
}

func updateBalance(ctx context.Context, svc *app.SetupService, id int64) error {
	_, err := svc.UpdateOpeningBalance(ctx, app.OpeningBalanceInput{AccountID: id, Amount: "8,000"})
	return err
}

func TestUpdateOperations_StampUpdatedAtWithTheClock(t *testing.T) {
	cases := []struct {
		name      string
		run       operation
		table     string
		byAccount bool // the row is the account, not the settings singleton
	}{
		{"update community name", updateName, "settings", false},
		{"rename account", renameAccount, "accounts", true},
		{"update opening balance", updateBalance, "accounts", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange
			svc, db, _, settings := setupDone(t)
			id := settings.Accounts[0].ID
			rowID := int64(1)
			if c.byAccount {
				rowID = id
			}

			// Act
			err := c.run(context.Background(), svc, id)

			// Assert
			if err != nil {
				t.Fatal(err)
			}
			var updatedAt string
			if err := db.QueryRow(`SELECT updated_at FROM `+c.table+` WHERE id = ?`, rowID).Scan(&updatedAt); err != nil {
				t.Fatal(err)
			}
			if updatedAt != "2026-11-13 09:30:00" {
				t.Fatalf("updated_at = %q, want the clock time", updatedAt)
			}
		})
	}
}

func TestUpdateOperations_EmitOneWideEventWithoutNames(t *testing.T) {
	cases := []struct {
		name   string
		run    operation
		action string
		want   func(id int64) map[string]any // fields beyond the common ones
	}{
		{"update community name", updateName, "update_community_name", func(int64) map[string]any { return nil }},
		{"rename account", renameAccount, "rename_account", func(id int64) map[string]any {
			return map[string]any{"account_id": float64(id)}
		}},
		{"update opening balance", updateBalance, "update_opening_balance", func(id int64) map[string]any {
			return map[string]any{"account_id": float64(id), "old_balance_cents": float64(750000), "new_balance_cents": float64(800000)}
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange
			svc, _, events, settings := setupDone(t)
			id := settings.Accounts[0].ID

			// Act
			err := c.run(context.Background(), svc, id)

			// Assert
			if err != nil {
				t.Fatal(err)
			}
			got := readEvents(t, events)
			if len(got) != 1 {
				t.Fatalf("%d events, want one: %v", len(got), got)
			}
			e := got[0]
			if e["action"] != c.action || e["outcome"] != "ok" || e["level"] != "INFO" {
				t.Errorf("event = %v, want %s ok at INFO", e, c.action)
			}
			if _, ok := e["duration_ms"]; !ok {
				t.Errorf("event has no duration_ms: %v", e)
			}
			for k, v := range c.want(id) {
				if e[k] != v {
					t.Errorf("event %s = %v, want %v: %v", k, e[k], v, e)
				}
			}
			raw := events.String()
			for _, name := range []string{"Álamos", "Caja", "Pinos", "Efectivo"} {
				if strings.Contains(raw, name) {
					t.Fatalf("events contain %q: %s", name, raw)
				}
			}
		})
	}
}

func TestUpdateOperations_EmitAValidationEventAtWarnAndAnErrorEventWhenNotFound(t *testing.T) {
	cases := []struct {
		name    string
		run     operation
		outcome string
		level   string
		invalid float64 // invalid_field_count; 0 when the event carries none
	}{
		{"an empty community name", func(ctx context.Context, svc *app.SetupService, _ int64) error {
			_, err := svc.UpdateCommunityName(ctx, "")
			return err
		}, "validation", "WARN", 1},
		{"the other account's name", func(ctx context.Context, svc *app.SetupService, id int64) error {
			_, err := svc.RenameAccount(ctx, app.RenameAccountInput{AccountID: id, Name: "Banco (NU)"})
			return err
		}, "validation", "WARN", 1},
		{"an unknown account", func(ctx context.Context, svc *app.SetupService, _ int64) error {
			_, err := svc.UpdateOpeningBalance(ctx, app.OpeningBalanceInput{AccountID: 999, Amount: "1"})
			return err
		}, "error", "ERROR", 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange
			svc, _, events, settings := setupDone(t)

			// Act
			err := c.run(context.Background(), svc, settings.Accounts[0].ID)

			// Assert
			if err == nil {
				t.Fatal("want an error")
			}
			got := readEvents(t, events)
			if len(got) != 1 {
				t.Fatalf("%d events, want one: %v", len(got), got)
			}
			e := got[0]
			if e["outcome"] != c.outcome || e["level"] != c.level {
				t.Errorf("event = %v, want outcome %s at %s", e, c.outcome, c.level)
			}
			if c.invalid != 0 && e["invalid_field_count"] != c.invalid {
				t.Errorf("event = %v, want invalid_field_count %v", e, c.invalid)
			}
		})
	}
}
