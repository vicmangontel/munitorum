package app_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vicmangontel/munitorum/internal/app"
	"github.com/vicmangontel/munitorum/internal/domain"
	"github.com/vicmangontel/munitorum/internal/store"
)

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

// today is the date the tests run on.
var today = time.Date(2026, time.November, 13, 9, 30, 0, 0, time.Local)

// newSetup returns a service over a migrated temporary database, the database
// for direct inspection, and the buffer receiving the wide events.
func newSetup(t *testing.T) (*app.SetupService, *sql.DB, *bytes.Buffer) {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := store.Migrate(db, store.Migrations()); err != nil {
		t.Fatal(err)
	}
	events := &bytes.Buffer{}
	log := slog.New(slog.NewJSONHandler(events, nil))
	return app.NewSetupService(store.NewSetupRepository(db), fixedClock{today}, log), db, events
}

// validInput is a complete, valid setup of count blank houses opened on 01/11/2026.
func validInput(count string) app.SetupInput {
	n, _ := domain.ParseHouseCount(count)
	return app.SetupInput{
		CommunityName: "Coto Los Pinos",
		HouseCount:    count,
		HousePrefix:   "CASA ",
		FeeAmount:     "150.00",
		Accounts: [2]app.SetupAccount{
			{Name: "Efectivo", OpeningBalance: "7,500.00"},
			{Name: "Banco (NU)", OpeningBalance: "0"},
		},
		OpeningDate: "01/11/2026",
		Houses:      make([]app.SetupHouse, n),
	}
}

func invalidFields(t *testing.T, err error) map[string]string {
	t.Helper()
	var ve app.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("error = %v, want app.ValidationError", err)
	}
	return ve.Fields
}

func count(t *testing.T, db *sql.DB, table string) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestGetSetupState_IsNotCompletedOnAFreshDatabaseAndCompletedAfterSetup(t *testing.T) {
	// Arrange
	svc, _, _ := newSetup(t)
	ctx := context.Background()

	// Act
	before, err := svc.GetSetupState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CompleteSetup(ctx, validInput("18")); err != nil {
		t.Fatal(err)
	}
	after, err := svc.GetSetupState(ctx)

	// Assert
	if err != nil || before.Completed || !after.Completed {
		t.Fatalf("state before = %v, after = %v, err = %v; want false then true", before, after, err)
	}
}

func TestGetSettings_ReturnsNotFoundBeforeSetup(t *testing.T) {
	// Arrange
	svc, _, _ := newSetup(t)

	// Act
	_, err := svc.GetSettings(context.Background())

	// Assert
	if !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
}

func TestCompleteSetup_WritesSettingsAccountsHousesAndInitialFeeRate(t *testing.T) {
	// Arrange
	svc, db, _ := newSetup(t)
	in := validInput("18")
	in.CommunityName = "  Coto Los Pinos  "
	in.Houses[2].Arrears = "150.00"
	in.Houses[0].CoveredThrough = "12/2026"
	in.Houses[0].Credit = "50"

	// Act
	got, err := svc.CompleteSetup(context.Background(), in)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if got.CommunityName != "Coto Los Pinos" || got.HouseCount != 18 || got.HousePrefix != "CASA " {
		t.Fatalf("settings = %+v", got)
	}
	if len(got.Accounts) != 2 ||
		got.Accounts[0].Kind != app.KindCash || got.Accounts[0].Name != "Efectivo" || got.Accounts[0].OpeningBalance != 750000 ||
		got.Accounts[1].Kind != app.KindBank || got.Accounts[1].Name != "Banco (NU)" || got.Accounts[1].OpeningBalance != 0 {
		t.Fatalf("accounts = %+v, want cash then bank in sort order", got.Accounts)
	}
	if y, m, d := got.OpeningDate.Date(); y != 2026 || m != time.November || d != 1 {
		t.Fatalf("opening date = %v", got.OpeningDate)
	}
	for table, want := range map[string]int{"settings": 1, "accounts": 2, "houses": 18, "fee_rates": 1} {
		if n := count(t, db, table); n != want {
			t.Errorf("%s has %d rows, want %d", table, n, want)
		}
	}
	var month, policy string
	var cents int64
	if err := db.QueryRow(`SELECT effective_month, amount_cents, policy FROM fee_rates`).Scan(&month, &cents, &policy); err != nil {
		t.Fatal(err)
	}
	if month != "2026-11" || cents != 15000 || policy != "initial" {
		t.Fatalf("fee rate = %s %d %s, want 2026-11 15000 initial", month, cents, policy)
	}
	var covered sql.NullString
	var credit, arrears int64
	if err := db.QueryRow(`SELECT opening_covered_through, opening_credit_cents, opening_arrears_cents FROM houses WHERE number = 1`).
		Scan(&covered, &credit, &arrears); err != nil {
		t.Fatal(err)
	}
	if covered.String != "2026-12" || credit != 5000 || arrears != 0 {
		t.Fatalf("house 1 opening = %v %d %d", covered, credit, arrears)
	}
	if err := db.QueryRow(`SELECT opening_covered_through, opening_credit_cents, opening_arrears_cents FROM houses WHERE number = 3`).
		Scan(&covered, &credit, &arrears); err != nil {
		t.Fatal(err)
	}
	if covered.Valid || arrears != 15000 {
		t.Fatalf("house 3 opening = %v %d %d, want NULL coverage and 15000 arrears", covered, credit, arrears)
	}
}

func TestCompleteSetup_CreatesHouseCodesFromPrefixAndCount(t *testing.T) {
	cases := []struct {
		name, prefix, count string
		first, last         string
	}{
		{"CASA prefix, 18 houses", "CASA ", "18", "CASA 01", "CASA 18"},
		{"dash prefix, 18 houses", "C-", "18", "C-01", "C-18"},
		{"120 houses pad to three digits", "CASA ", "120", "CASA 001", "CASA 120"},
		{"empty prefix", "", "18", "01", "18"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange
			svc, db, _ := newSetup(t)
			in := validInput(c.count)
			in.HousePrefix = c.prefix

			// Act
			_, err := svc.CompleteSetup(context.Background(), in)

			// Assert
			if err != nil {
				t.Fatal(err)
			}
			var first, last string
			if err := db.QueryRow(`SELECT code FROM houses ORDER BY number LIMIT 1`).Scan(&first); err != nil {
				t.Fatal(err)
			}
			if err := db.QueryRow(`SELECT code FROM houses ORDER BY number DESC LIMIT 1`).Scan(&last); err != nil {
				t.Fatal(err)
			}
			if first != c.first || last != c.last {
				t.Fatalf("codes %q … %q, want %q … %q", first, last, c.first, c.last)
			}
		})
	}
}

func TestValidateSetup_ReportsEveryInvalidFieldAtOnce(t *testing.T) {
	// Arrange
	svc, _, _ := newSetup(t)
	in := app.SetupInput{
		CommunityName: "   ",
		HouseCount:    "0",
		HousePrefix:   "ABCDEFGHIJK", // 11 characters
		FeeAmount:     "0",
		Accounts: [2]app.SetupAccount{
			{Name: "", OpeningBalance: "-1"},
			{Name: strings.Repeat("x", 31), OpeningBalance: "abc"},
		},
		OpeningDate: "13/11/2027",
	}

	// Act
	_, err := svc.ValidateSetup(in)

	// Assert
	want := map[string]string{
		app.FieldCommunityName:     app.CodeCommunityNameLength,
		app.FieldHouseCount:        app.CodeHouseCountRange,
		app.FieldHousePrefix:       app.CodeHousePrefixLength,
		app.FieldFeeAmount:         app.CodeFeeNotPositive,
		app.FieldAccountName(0):    app.CodeAccountNameLength,
		app.FieldAccountName(1):    app.CodeAccountNameLength,
		app.FieldOpeningBalance(0): app.CodeBalanceNegative,
		app.FieldOpeningBalance(1): app.CodeAmountInvalid,
		app.FieldOpeningDate:       app.CodeDateFuture,
	}
	got := invalidFields(t, err)
	if len(got) != len(want) {
		t.Fatalf("invalid fields = %v, want %v", got, want)
	}
	for k, code := range want {
		if got[k] != code {
			t.Errorf("field %s = %q, want %q", k, got[k], code)
		}
	}
}

func TestValidateSetup_ChecksNamesDatesAndFeeBoundaries(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*app.SetupInput)
		field  string // empty when the input must be valid
		code   string
	}{
		{"community name of 60 characters", func(in *app.SetupInput) { in.CommunityName = strings.Repeat("ñ", 60) }, "", ""},
		{"community name of one character", func(in *app.SetupInput) { in.CommunityName = "X" }, "", ""},
		{"community name of 61 characters", func(in *app.SetupInput) { in.CommunityName = strings.Repeat("ñ", 61) }, app.FieldCommunityName, app.CodeCommunityNameLength},
		{"account name of 30 characters", func(in *app.SetupInput) { in.Accounts[0].Name = strings.Repeat("a", 30) }, "", ""},
		{"account name of one character", func(in *app.SetupInput) { in.Accounts[0].Name = "E" }, "", ""},
		{"account name of 31 characters", func(in *app.SetupInput) { in.Accounts[0].Name = strings.Repeat("a", 31) }, app.FieldAccountName(0), app.CodeAccountNameLength},
		{"account names equal in another case", func(in *app.SetupInput) { in.Accounts[1].Name = "EFECTIVO" }, app.FieldAccountName(1), app.CodeAccountNameDuplicate},
		{"account names equal after trimming", func(in *app.SetupInput) { in.Accounts[1].Name = "  Efectivo " }, app.FieldAccountName(1), app.CodeAccountNameDuplicate},
		{"account names equal beyond ASCII", func(in *app.SetupInput) { in.Accounts[0].Name, in.Accounts[1].Name = "Ñandú", "ñANDÚ" }, app.FieldAccountName(1), app.CodeAccountNameDuplicate},
		{"prefix of 10 characters kept as typed", func(in *app.SetupInput) { in.HousePrefix = "ABCDEFGHI-" }, "", ""},
		{"opening date today", func(in *app.SetupInput) { in.OpeningDate = "13/11/2026" }, "", ""},
		{"opening date tomorrow", func(in *app.SetupInput) { in.OpeningDate = "14/11/2026" }, app.FieldOpeningDate, app.CodeDateFuture},
		{"opening date unreadable", func(in *app.SetupInput) { in.OpeningDate = "2026-11-01" }, app.FieldOpeningDate, app.CodeDateInvalid},
		{"fee of one centavo", func(in *app.SetupInput) { in.FeeAmount = "0.01" }, "", ""},
		{"fee unreadable", func(in *app.SetupInput) { in.FeeAmount = "ciento" }, app.FieldFeeAmount, app.CodeAmountInvalid},
		{"zero opening balance", func(in *app.SetupInput) { in.Accounts[1].OpeningBalance = "0.00" }, "", ""},
		{"house count 200", func(in *app.SetupInput) { in.HouseCount = "200"; in.Houses = make([]app.SetupHouse, 200) }, "", ""},
		{"house count 201", func(in *app.SetupInput) { in.HouseCount = "201" }, app.FieldHouseCount, app.CodeHouseCountRange},
		{"house rows fewer than the count", func(in *app.SetupInput) { in.Houses = in.Houses[:3] }, app.FieldHouses, app.CodeHousesMismatch},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange
			svc, _, _ := newSetup(t)
			in := validInput("18")
			c.mutate(&in)

			// Act
			_, err := svc.ValidateSetup(in)

			// Assert
			if c.field == "" {
				if err != nil {
					t.Fatalf("error = %v, want valid", err)
				}
				return
			}
			if got := invalidFields(t, err); len(got) != 1 || got[c.field] != c.code {
				t.Fatalf("invalid fields = %v, want only %s = %s", got, c.field, c.code)
			}
		})
	}
}

func TestValidateSetup_ChecksEachHouseRowAgainstTheOpeningRules(t *testing.T) {
	cases := []struct {
		name  string
		house app.SetupHouse
		field string // row 1 field, empty when valid
		code  string
	}{
		{"covered through the opening month", app.SetupHouse{CoveredThrough: "11/2026"}, "", ""},
		{"covered 36 months past the opening month", app.SetupHouse{CoveredThrough: "11/2029"}, "", ""},
		{"covered before the opening month", app.SetupHouse{CoveredThrough: "10/2026"}, domain.OpeningCoveredThrough, domain.CodeCoveredBeforeOpening},
		{"covered beyond 36 months", app.SetupHouse{CoveredThrough: "12/2029"}, domain.OpeningCoveredThrough, domain.CodeCoveredTooFar},
		{"covered month unreadable", app.SetupHouse{CoveredThrough: "2026-11"}, domain.OpeningCoveredThrough, app.CodeMonthInvalid},
		{"credit below the cuota", app.SetupHouse{Credit: "149.99"}, "", ""},
		{"credit equal to the cuota", app.SetupHouse{Credit: "150"}, domain.OpeningCredit, domain.CodeCreditOutOfRange},
		{"credit unreadable", app.SetupHouse{Credit: "mucho"}, domain.OpeningCredit, app.CodeAmountInvalid},
		{"arrears alone", app.SetupHouse{Arrears: "300"}, "", ""},
		{"arrears unreadable", app.SetupHouse{Arrears: "1.234"}, domain.OpeningArrears, app.CodeAmountInvalid},
		{"arrears with coverage", app.SetupHouse{CoveredThrough: "12/2026", Arrears: "10"}, domain.OpeningArrears, domain.CodeArrearsWithCoverage},
		{"arrears with credit", app.SetupHouse{Credit: "10", Arrears: "10"}, domain.OpeningArrears, domain.CodeArrearsWithCredit},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange
			svc, _, _ := newSetup(t)
			in := validInput("18")
			in.Houses[1] = c.house

			// Act
			_, err := svc.ValidateSetup(in)

			// Assert
			if c.field == "" {
				if err != nil {
					t.Fatalf("error = %v, want valid", err)
				}
				return
			}
			key := app.FieldHouse(1, c.field)
			if got := invalidFields(t, err); len(got) != 1 || got[key] != c.code {
				t.Fatalf("invalid fields = %v, want only %s = %s", got, key, c.code)
			}
		})
	}
}

func TestValidateSetup_ReturnsTheSummaryAndWritesNothing(t *testing.T) {
	// Arrange
	svc, db, _ := newSetup(t)
	in := validInput("18")
	in.Houses[0] = app.SetupHouse{CoveredThrough: "12/2026", Credit: "20"}
	in.Houses[1] = app.SetupHouse{CoveredThrough: "11/2026", Credit: "30.50"}
	in.Houses[2] = app.SetupHouse{Arrears: "150"}
	in.Houses[3] = app.SetupHouse{Arrears: "75.25"}
	in.Accounts[1].OpeningBalance = "1,000.50"

	// Act
	got, err := svc.ValidateSetup(in)

	// Assert
	want := app.SetupSummary{
		HouseCount:      18,
		FeeAmount:       15000,
		OpeningBalances: [2]domain.Money{750000, 100050},
		TotalBalance:    850050,
		HousesCovered:   2,
		TotalCredit:     5050,
		TotalArrears:    22525,
	}
	if err != nil || got != want {
		t.Fatalf("summary = %+v, %v; want %+v", got, err, want)
	}
	if n := count(t, db, "settings"); n != 0 {
		t.Fatalf("settings has %d rows after ValidateSetup, want 0", n)
	}
}

func TestCompleteSetup_LeavesNoRowsWhenTheWriteFailsMidway(t *testing.T) {
	// Arrange
	svc, db, _ := newSetup(t)
	if _, err := db.Exec(`CREATE TRIGGER fail_fee_rates BEFORE INSERT ON fee_rates
		BEGIN SELECT RAISE(ABORT, 'forced failure'); END`); err != nil {
		t.Fatal(err)
	}

	// Act
	_, err := svc.CompleteSetup(context.Background(), validInput("18"))

	// Assert
	if err == nil || !strings.Contains(err.Error(), "forced failure") {
		t.Fatalf("error = %v, want the forced failure", err)
	}
	for _, table := range []string{"settings", "accounts", "houses", "fee_rates"} {
		if n := count(t, db, table); n != 0 {
			t.Errorf("%s has %d rows after the failed write, want 0", table, n)
		}
	}
}

func TestCompleteSetup_RejectsACompletedDatabase(t *testing.T) {
	// Arrange
	svc, db, _ := newSetup(t)
	ctx := context.Background()
	if _, err := svc.CompleteSetup(ctx, validInput("18")); err != nil {
		t.Fatal(err)
	}
	second := validInput("5")
	second.CommunityName = "Otro coto"

	// Act
	_, err := svc.CompleteSetup(ctx, second)

	// Assert
	if !errors.Is(err, app.ErrConflict) {
		t.Fatalf("error = %v, want ErrConflict", err)
	}
	if n := count(t, db, "houses"); n != 18 {
		t.Fatalf("houses has %d rows, want the original 18", n)
	}
}

func TestCompleteSetup_RejectsInvalidInputWithoutWriting(t *testing.T) {
	// Arrange
	svc, db, _ := newSetup(t)
	in := validInput("18")
	in.OpeningDate = "14/11/2026"
	in.Houses[0] = app.SetupHouse{CoveredThrough: "11/2026", Arrears: "1"}

	// Act
	_, err := svc.CompleteSetup(context.Background(), in)

	// Assert
	got := invalidFields(t, err)
	if got[app.FieldOpeningDate] != app.CodeDateFuture {
		t.Fatalf("invalid fields = %v, want the future opening date", got)
	}
	if n := count(t, db, "settings"); n != 0 {
		t.Fatalf("settings has %d rows, want 0", n)
	}
}

func readEvents(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var events []map[string]any
	for _, line := range bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		var e map[string]any
		if err := json.Unmarshal(line, &e); err != nil {
			t.Fatalf("log line is not JSON: %v\n%s", err, line)
		}
		events = append(events, e)
	}
	return events
}

func TestCompleteSetup_EmitsOneWideEventWithAmountsAndNoNames(t *testing.T) {
	// Arrange
	svc, _, events := newSetup(t)

	// Act
	_, err := svc.CompleteSetup(context.Background(), validInput("18"))

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	got := readEvents(t, events)
	if len(got) != 1 {
		t.Fatalf("%d events, want one: %v", len(got), got)
	}
	e := got[0]
	if e["action"] != "complete_setup" || e["outcome"] != "ok" || e["level"] != "INFO" {
		t.Fatalf("event = %v", e)
	}
	if e["house_count"] != float64(18) || e["fee_amount_cents"] != float64(15000) ||
		e["opening_balance_cash_cents"] != float64(750000) || e["opening_balance_bank_cents"] != float64(0) {
		t.Fatalf("event amounts = %v", e)
	}
	if _, ok := e["duration_ms"]; !ok {
		t.Fatalf("event has no duration_ms: %v", e)
	}
	raw := events.String()
	for _, name := range []string{"Coto Los Pinos", "Efectivo", "Banco", "CASA"} {
		if strings.Contains(raw, name) {
			t.Fatalf("event contains %q: %s", name, raw)
		}
	}
}

func TestCompleteSetup_EmitsAValidationEventAtWarnAndAnErrorEventOnConflict(t *testing.T) {
	// Arrange
	svc, _, events := newSetup(t)
	ctx := context.Background()
	bad := validInput("18")
	bad.CommunityName = ""

	// Act
	_, errValidation := svc.CompleteSetup(ctx, bad)
	_, errOK := svc.CompleteSetup(ctx, validInput("18"))
	_, errConflict := svc.CompleteSetup(ctx, validInput("18"))

	// Assert
	if errValidation == nil || errOK != nil || errConflict == nil {
		t.Fatalf("errors = %v, %v, %v", errValidation, errOK, errConflict)
	}
	got := readEvents(t, events)
	if len(got) != 3 {
		t.Fatalf("%d events, want three: %v", len(got), got)
	}
	wantOutcome := []string{"validation", "ok", "error"}
	wantLevel := []string{"WARN", "INFO", "ERROR"}
	for i, e := range got {
		if e["outcome"] != wantOutcome[i] || e["level"] != wantLevel[i] {
			t.Errorf("event %d = %v, want outcome %s at %s", i, e, wantOutcome[i], wantLevel[i])
		}
	}
	if got[0]["invalid_field_count"] != float64(1) {
		t.Errorf("validation event = %v, want invalid_field_count 1", got[0])
	}
}

func TestValidationCodes_AreUnique(t *testing.T) {
	// Arrange
	seen := map[string]bool{}

	// Act / Assert
	for _, c := range app.Codes {
		if seen[c] {
			t.Fatalf("code %q is listed twice", c)
		}
		seen[c] = true
	}
}
