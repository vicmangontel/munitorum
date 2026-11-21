package app_test

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vicmangontel/munitorum/internal/app"
	"github.com/vicmangontel/munitorum/internal/domain"
	"github.com/vicmangontel/munitorum/internal/store"
)

// ledgerEnv is a completed coto on a temporary database: cash holding 7,500.00
// and bank holding nothing, opened on the date the test chose, today being 13
// November 2026.
type ledgerEnv struct {
	svc        *app.LedgerService
	cats       *app.CategoryService
	setup      *app.SetupService
	dir        *app.DirectoryService
	db         *sql.DB
	events     *bytes.Buffer
	cash, bank int64
}

func newLedgerEnv(t *testing.T, opening string) *ledgerEnv {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := store.Migrate(db, store.Migrations()); err != nil {
		t.Fatal(err)
	}
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	setup := app.NewSetupService(store.NewSetupRepository(db), fixedClock{today}, quiet)
	in := validInput("2")
	in.OpeningDate = opening
	settings, err := setup.CompleteSetup(context.Background(), in)
	if err != nil {
		t.Fatalf("arrange: complete setup: %v", err)
	}
	events := &bytes.Buffer{}
	repo := store.NewLedgerRepository(db)
	return &ledgerEnv{
		svc:    app.NewLedgerService(repo, fixedClock{today}, slog.New(slog.NewJSONHandler(events, nil))),
		cats:   app.NewCategoryService(repo),
		setup:  setup,
		dir:    app.NewDirectoryService(store.NewVendorRepository(db), fixedClock{today}, quiet),
		db:     db,
		events: events,
		cash:   settings.Accounts[0].ID,
		bank:   settings.Accounts[1].ID,
	}
}

func (e *ledgerEnv) category(t *testing.T, name string) int64 {
	t.Helper()
	var id int64
	if err := e.db.QueryRow(`SELECT id FROM categories WHERE name = ?`, name).Scan(&id); err != nil {
		t.Fatalf("arrange: category %q: %v", name, err)
	}
	return id
}

// expense is a valid expense of 178.50 from cash on 10 November.
func (e *ledgerEnv) expense(t *testing.T) app.ExpenseInput {
	t.Helper()
	return app.ExpenseInput{
		Date: "10/11/2026", AccountID: e.cash, CategoryID: e.category(t, "Insumos"),
		Amount: "178.50", Description: "Garrafones de agua",
	}
}

// income is a valid income of 200.00 into cash on 10 November.
func (e *ledgerEnv) income(t *testing.T) app.IncomeInput {
	t.Helper()
	return app.IncomeInput{
		Date: "10/11/2026", AccountID: e.cash, CategoryID: e.category(t, "Otros ingresos"),
		Amount: "200.00", Description: "Renta del salón",
	}
}

// spend records an expense the test needs as given state.
func (e *ledgerEnv) spend(t *testing.T, date, amount string, account int64) app.Movement {
	t.Helper()
	in := e.expense(t)
	in.Date, in.Amount, in.AccountID = date, amount, account
	m, err := e.svc.RecordExpense(context.Background(), in)
	if err != nil {
		t.Fatalf("arrange: record expense: %v", err)
	}
	return m
}

func (e *ledgerEnv) earn(t *testing.T, date, amount string, account int64) app.Movement {
	t.Helper()
	in := e.income(t)
	in.Date, in.Amount, in.AccountID = date, amount, account
	m, err := e.svc.RecordIncome(context.Background(), in)
	if err != nil {
		t.Fatalf("arrange: record income: %v", err)
	}
	return m
}

func (e *ledgerEnv) balance(t *testing.T, asOf time.Time, account int64) domain.Money {
	t.Helper()
	b, err := e.svc.AccountBalances(context.Background(), asOf)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range b.Accounts {
		if a.Account.ID == account {
			return a.Balance
		}
	}
	t.Fatalf("no balance for account %d", account)
	return 0
}

var november = domain.Month{Year: 2026, Month: time.November}

func TestRecordExpense_OfCashLeavesTheCashBalanceAtItsOpeningMinusTheAmount(t *testing.T) {
	// Arrange
	e := newLedgerEnv(t, "01/11/2026")

	// Act
	_, err := e.svc.RecordExpense(context.Background(), e.expense(t))

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	balances, err := e.svc.AccountBalances(context.Background(), today)
	if err != nil {
		t.Fatal(err)
	}
	if balances.Accounts[0].Balance != 732150 || balances.Accounts[1].Balance != 0 || balances.Total != 732150 {
		t.Fatalf("balances = %+v, want cash 7,321.50, bank 0 and total 7,321.50", balances)
	}
}

func TestRecordExpense_StoresTheMovementTrimmedWithItsOptionalFieldsNull(t *testing.T) {
	// Arrange
	e := newLedgerEnv(t, "01/11/2026")
	in := e.expense(t)
	in.Description = "  Garrafones  "

	// Act
	got, err := e.svc.RecordExpense(context.Background(), in)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	var date, kind, description, created string
	var amount int64
	var to, vendor, notes, link sql.NullString
	var receipt string
	err = e.db.QueryRow(`SELECT date, kind, amount_cents, description, to_account_id, vendor_id, notes, receipt_link, manual_receipt, created_at
		FROM movements WHERE id = ?`, got.ID).Scan(&date, &kind, &amount, &description, &to, &vendor, &notes, &link, &receipt, &created)
	if err != nil {
		t.Fatal(err)
	}
	if date != "2026-11-10" || kind != "expense" || amount != 17850 || description != "Garrafones" ||
		to.Valid || vendor.Valid || notes.Valid || link.Valid || receipt != "none" || created != "2026-11-13 09:30:00" {
		t.Fatalf("stored %v %v %v %q %v %v %v %v %v %v", date, kind, amount, description, to, vendor, notes, link, receipt, created)
	}
}

func TestRecordExpense_ChecksTheDateAgainstTheOpeningDateAndToday(t *testing.T) {
	cases := []struct {
		name string
		date string
		want string // validation code, empty when the date is accepted
	}{
		{"the day before the opening date", "31/10/2026", app.CodeDateBeforeOpening},
		{"the opening date", "01/11/2026", ""},
		{"today", "13/11/2026", ""},
		{"tomorrow", "14/11/2026", app.CodeDateFuture},
		{"not a date", "ayer", app.CodeDateInvalid},
		{"a day the month does not have", "31/11/2026", app.CodeDateInvalid},
		{"empty", "", app.CodeDateInvalid},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange
			e := newLedgerEnv(t, "01/11/2026")
			in := e.expense(t)
			in.Date = c.date

			// Act
			_, err := e.svc.RecordExpense(context.Background(), in)

			// Assert
			if c.want == "" {
				if err != nil {
					t.Fatalf("date %q rejected: %v", c.date, err)
				}
				return
			}
			if got := invalidFields(t, err)[app.FieldDate]; got != c.want {
				t.Fatalf("code for date %q = %q, want %q", c.date, got, c.want)
			}
		})
	}
}

func TestRecordExpense_ChecksTheAmountIsPositiveMoney(t *testing.T) {
	cases := []struct {
		name   string
		amount string
		want   string
	}{
		{"zero", "0", app.CodeAmountNotPositive},
		{"negative", "-5.00", app.CodeAmountNotPositive},
		{"not a number", "mucho", app.CodeAmountInvalid},
		{"three decimals", "1.005", app.CodeAmountInvalid},
		{"empty", "", app.CodeAmountInvalid},
		{"one centavo", "0.01", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange
			e := newLedgerEnv(t, "01/11/2026")
			in := e.expense(t)
			in.Amount = c.amount

			// Act
			_, err := e.svc.RecordExpense(context.Background(), in)

			// Assert
			if c.want == "" {
				if err != nil {
					t.Fatalf("amount %q rejected: %v", c.amount, err)
				}
				return
			}
			if got := invalidFields(t, err)[app.FieldAmount]; got != c.want {
				t.Fatalf("code for amount %q = %q, want %q", c.amount, got, c.want)
			}
		})
	}
}

func TestRecordExpense_ChecksTheLengthOfTheTextFields(t *testing.T) {
	cases := []struct {
		name  string
		field string
		set   func(*app.ExpenseInput, string)
		len   int
		ok    bool
	}{
		{"description of 1", app.FieldDescription, func(in *app.ExpenseInput, s string) { in.Description = s }, 1, true},
		{"description of 120", app.FieldDescription, func(in *app.ExpenseInput, s string) { in.Description = s }, 120, true},
		{"description of 121", app.FieldDescription, func(in *app.ExpenseInput, s string) { in.Description = s }, 121, false},
		{"empty description", app.FieldDescription, func(in *app.ExpenseInput, s string) { in.Description = s }, 0, false},
		{"notes of 500", app.FieldNotes, func(in *app.ExpenseInput, s string) { in.Notes = s }, 500, true},
		{"notes of 501", app.FieldNotes, func(in *app.ExpenseInput, s string) { in.Notes = s }, 501, false},
		{"receipt link of 500", app.FieldReceiptLink, func(in *app.ExpenseInput, s string) { in.ReceiptLink = s }, 500, true},
		{"receipt link of 501", app.FieldReceiptLink, func(in *app.ExpenseInput, s string) { in.ReceiptLink = s }, 501, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange: accented characters count once each
			e := newLedgerEnv(t, "01/11/2026")
			in := e.expense(t)
			c.set(&in, strings.Repeat("é", c.len))

			// Act
			_, err := e.svc.RecordExpense(context.Background(), in)

			// Assert
			if c.ok {
				if err != nil {
					t.Fatalf("%d characters rejected: %v", c.len, err)
				}
				return
			}
			if _, bad := invalidFields(t, err)[c.field]; !bad {
				t.Fatalf("%d characters in %s were accepted", c.len, c.field)
			}
		})
	}
}

func TestRecordExpense_ReportsEveryInvalidFieldTogether(t *testing.T) {
	// Arrange
	e := newLedgerEnv(t, "01/11/2026")
	unknown := int64(999)
	in := app.ExpenseInput{
		Date: "mañana", AccountID: 999, CategoryID: 999, VendorID: &unknown,
		Amount: "x", Description: "", Notes: strings.Repeat("n", 501), ReceiptLink: strings.Repeat("r", 501),
	}

	// Act
	_, err := e.svc.RecordExpense(context.Background(), in)

	// Assert
	got := invalidFields(t, err)
	want := map[string]string{
		app.FieldDate: app.CodeDateInvalid, app.FieldAmount: app.CodeAmountInvalid,
		app.FieldAccount: app.CodeAccountInvalid, app.FieldCategory: app.CodeCategoryInvalid,
		app.FieldVendor: app.CodeVendorInvalid, app.FieldDescription: app.CodeDescriptionLength,
		app.FieldNotes: app.CodeNotesLength, app.FieldReceiptLink: app.CodeReceiptLinkLength,
	}
	if len(got) != len(want) {
		t.Fatalf("fields = %v, want %v", got, want)
	}
	for field, code := range want {
		if got[field] != code {
			t.Errorf("code for %s = %q, want %q", field, got[field], code)
		}
	}
	if n := count(t, e.db, "movements"); n != 0 {
		t.Fatalf("movements has %d rows after a rejected input, want 0", n)
	}
}

func TestRecordIncome_ReportsEveryInvalidFieldTogether(t *testing.T) {
	// Arrange
	e := newLedgerEnv(t, "01/11/2026")
	in := app.IncomeInput{Date: "", AccountID: 0, CategoryID: 0, Amount: "0", Description: " "}

	// Act
	_, err := e.svc.RecordIncome(context.Background(), in)

	// Assert
	got := invalidFields(t, err)
	want := map[string]string{
		app.FieldDate: app.CodeDateInvalid, app.FieldAmount: app.CodeAmountNotPositive,
		app.FieldAccount: app.CodeAccountInvalid, app.FieldCategory: app.CodeCategoryInvalid,
		app.FieldDescription: app.CodeDescriptionLength,
	}
	if len(got) != len(want) {
		t.Fatalf("fields = %v, want %v", got, want)
	}
	for field, code := range want {
		if got[field] != code {
			t.Errorf("code for %s = %q, want %q", field, got[field], code)
		}
	}
}

func TestRecordIncome_RejectsTheCategoriesOtherModulesOwn(t *testing.T) {
	cases := []struct {
		category string
		want     string
	}{
		{"Cuota", app.CodeCategoryReserved},
		{"Extraordinaria", app.CodeCategoryReserved},
		{"Insumos", app.CodeCategoryInvalid}, // an expense category
		{"Otros ingresos", ""},
	}
	for _, c := range cases {
		t.Run(c.category, func(t *testing.T) {
			// Arrange
			e := newLedgerEnv(t, "01/11/2026")
			in := e.income(t)
			in.CategoryID = e.category(t, c.category)

			// Act
			_, err := e.svc.RecordIncome(context.Background(), in)

			// Assert
			if c.want == "" {
				if err != nil {
					t.Fatalf("category %q rejected: %v", c.category, err)
				}
				return
			}
			if got := invalidFields(t, err)[app.FieldCategory]; got != c.want {
				t.Fatalf("code for %q = %q, want %q", c.category, got, c.want)
			}
			if n := count(t, e.db, "movements"); n != 0 {
				t.Fatalf("movements has %d rows after a rejected category, want 0", n)
			}
		})
	}
}

func TestRecordExpense_RejectsAnExpenseCategoryThatWasDeactivated(t *testing.T) {
	// Arrange
	e := newLedgerEnv(t, "01/11/2026")
	in := e.expense(t)
	if _, err := e.db.Exec(`UPDATE categories SET active = 0 WHERE id = ?`, in.CategoryID); err != nil {
		t.Fatal(err)
	}

	// Act
	_, err := e.svc.RecordExpense(context.Background(), in)

	// Assert
	if got := invalidFields(t, err)[app.FieldCategory]; got != app.CodeCategoryInvalid {
		t.Fatalf("code = %q, want %q", got, app.CodeCategoryInvalid)
	}
}

func TestRecordExpense_AcceptsALiveVendorAndRejectsADeletedOne(t *testing.T) {
	// Arrange
	e := newLedgerEnv(t, "01/11/2026")
	live := create(t, e.dir, service("Basura"))
	gone := create(t, e.dir, service("CFE"))
	if err := e.dir.DeleteVendor(context.Background(), gone.ID); err != nil {
		t.Fatal(err)
	}
	withLive, withDeleted := e.expense(t), e.expense(t)
	withLive.VendorID, withDeleted.VendorID = &live.ID, &gone.ID

	// Act
	saved, okErr := e.svc.RecordExpense(context.Background(), withLive)
	_, deletedErr := e.svc.RecordExpense(context.Background(), withDeleted)

	// Assert
	if okErr != nil || saved.VendorID == nil || *saved.VendorID != live.ID {
		t.Fatalf("live vendor: movement %+v, error %v", saved, okErr)
	}
	if got := invalidFields(t, deletedErr)[app.FieldVendor]; got != app.CodeVendorInvalid {
		t.Fatalf("code for a deleted vendor = %q, want %q", got, app.CodeVendorInvalid)
	}
}

func TestRecordIncome_StoresAPendingManualReceiptOnlyWhenAsked(t *testing.T) {
	cases := []struct {
		name    string
		pending bool
		want    string
	}{
		{"flag set", true, "pending"},
		{"flag not set", false, "none"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange
			e := newLedgerEnv(t, "01/11/2026")
			in := e.income(t)
			in.ManualReceiptPending = c.pending

			// Act
			got, err := e.svc.RecordIncome(context.Background(), in)

			// Assert
			if err != nil {
				t.Fatal(err)
			}
			var stored string
			if err := e.db.QueryRow(`SELECT manual_receipt FROM movements WHERE id = ?`, got.ID).Scan(&stored); err != nil {
				t.Fatal(err)
			}
			if stored != c.want {
				t.Fatalf("manual_receipt = %q, want %q", stored, c.want)
			}
		})
	}
}

func TestRecordExpense_ThatLeavesCashNegativeWarnsAndWritesNothingUntilConfirmed(t *testing.T) {
	// Arrange: cash holds 7,500.00
	e := newLedgerEnv(t, "01/11/2026")
	in := e.expense(t)
	in.Amount = "8,000.00"

	// Act
	_, err := e.svc.RecordExpense(context.Background(), in)

	// Assert
	var warning app.NegativeBalanceWarning
	if !errors.As(err, &warning) {
		t.Fatalf("error = %v, want a NegativeBalanceWarning", err)
	}
	if warning.Account.ID != e.cash || warning.Resulting != -50000 {
		t.Fatalf("warning = %+v, want cash left at −500.00", warning)
	}
	if n := count(t, e.db, "movements"); n != 0 {
		t.Fatalf("movements has %d rows after a declined warning, want 0", n)
	}
}

func TestRecordExpense_WithConfirmNegativeWritesTheExpenseThatLeavesCashNegative(t *testing.T) {
	// Arrange
	e := newLedgerEnv(t, "01/11/2026")
	in := e.expense(t)
	in.Amount, in.ConfirmNegative = "8,000.00", true

	// Act
	_, err := e.svc.RecordExpense(context.Background(), in)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if got := e.balance(t, today, e.cash); got != -50000 {
		t.Fatalf("cash = %d, want −500.00", got)
	}
}

func TestRecordExpense_OfTheWholeBalanceDoesNotWarn(t *testing.T) {
	// Arrange
	e := newLedgerEnv(t, "01/11/2026")
	in := e.expense(t)
	in.Amount = "7,500.00"

	// Act
	_, err := e.svc.RecordExpense(context.Background(), in)

	// Assert
	if err != nil {
		t.Fatalf("an expense that leaves exactly zero was refused: %v", err)
	}
}

func TestRecordExpense_WarnsFromTheBalanceOfEveryDateNotOnlyThatOfTheExpense(t *testing.T) {
	// Arrange: a later expense already took cash to 500.00; an earlier-dated one of 600.00 would sink it
	e := newLedgerEnv(t, "01/11/2026")
	e.spend(t, "12/11/2026", "7,000.00", e.cash)
	in := e.expense(t)
	in.Date, in.Amount = "02/11/2026", "600.00"

	// Act
	_, err := e.svc.RecordExpense(context.Background(), in)

	// Assert
	var warning app.NegativeBalanceWarning
	if !errors.As(err, &warning) || warning.Resulting != -10000 {
		t.Fatalf("error = %v, want a warning leaving cash at −100.00", err)
	}
}

// overdraw records an expense of amount from account with ConfirmNegative, as
// given state that leaves the account below zero.
func (e *ledgerEnv) overdraw(t *testing.T, account int64, amount string) {
	t.Helper()
	in := e.expense(t)
	in.AccountID, in.Amount, in.ConfirmNegative = account, amount, true
	if _, err := e.svc.RecordExpense(context.Background(), in); err != nil {
		t.Fatalf("arrange: overdraw: %v", err)
	}
}

func TestRecordIncome_ToAnAccountThatStaysBelowZeroDoesNotWarnAndIsWritten(t *testing.T) {
	// Arrange: the bank holds nothing and was overdrawn by 100.00; 50.00 still leaves it at −50.00
	e := newLedgerEnv(t, "01/11/2026")
	e.overdraw(t, e.bank, "100.00")
	in := e.income(t)
	in.AccountID, in.Amount = e.bank, "50.00"

	// Act
	_, err := e.svc.RecordIncome(context.Background(), in)

	// Assert
	if err != nil {
		t.Fatalf("an income into an overdrawn account was refused: %v", err)
	}
	if n := count(t, e.db, "movements"); n != 2 {
		t.Fatalf("movements has %d rows, want the overdraft and the income", n)
	}
	if got := e.balance(t, today, e.bank); got != -5000 {
		t.Fatalf("bank = %d, want −50.00", got)
	}
}

func TestRecordExpense_FromAnAccountAlreadyBelowZeroWarnsAndWritesNothingUnlessConfirmed(t *testing.T) {
	// Arrange: cash was overdrawn to −500.00 with confirmation
	e := newLedgerEnv(t, "01/11/2026")
	e.overdraw(t, e.cash, "8,000.00")
	in := e.expense(t)
	in.Amount = "10.00"

	// Act
	_, err := e.svc.RecordExpense(context.Background(), in)

	// Assert
	var warning app.NegativeBalanceWarning
	if !errors.As(err, &warning) || warning.Resulting != -51000 {
		t.Fatalf("error = %v, want a warning leaving cash at −510.00", err)
	}
	if n := count(t, e.db, "movements"); n != 1 {
		t.Fatalf("movements has %d rows, want only the confirmed overdraft", n)
	}
}

func TestRecordExpense_FromCashDoesNotWarnWhenOnlyTheBankIsBelowZero(t *testing.T) {
	// Arrange: the warning is about the account the expense touches
	e := newLedgerEnv(t, "01/11/2026")
	e.overdraw(t, e.bank, "100.00")
	in := e.expense(t)

	// Act
	_, err := e.svc.RecordExpense(context.Background(), in)

	// Assert
	if err != nil {
		t.Fatalf("an expense from cash that leaves cash positive was refused: %v", err)
	}
}

func TestAccountBalances_CountOnlyMovementsOnOrBeforeTheDate(t *testing.T) {
	// Arrange
	e := newLedgerEnv(t, "01/11/2026")
	e.spend(t, "10/11/2026", "178.50", e.cash)

	cases := []struct {
		name string
		asOf time.Time
		want domain.Money
	}{
		{"the day before", time.Date(2026, time.November, 9, 0, 0, 0, 0, time.Local), 750000},
		{"the day itself", time.Date(2026, time.November, 10, 0, 0, 0, 0, time.Local), 732150},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Act
			got := e.balance(t, c.asOf, e.cash)

			// Assert
			if got != c.want {
				t.Fatalf("cash = %d, want %d", got, c.want)
			}
		})
	}
}

func TestAccountBalances_IgnoreDeletedMovements(t *testing.T) {
	// Arrange
	e := newLedgerEnv(t, "01/11/2026")
	m := e.spend(t, "10/11/2026", "178.50", e.cash)
	if _, err := e.db.Exec(`UPDATE movements SET deleted_at = '2026-11-13 10:00:00' WHERE id = ?`, m.ID); err != nil {
		t.Fatal(err)
	}

	// Act
	balance := e.balance(t, today, e.cash)
	ledger, err := e.svc.ListMovements(context.Background(), app.LedgerFilter{Month: november})

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if balance != 750000 || len(ledger.Rows) != 0 || ledger.ClosingBalance != 750000 {
		t.Fatalf("cash = %d, rows = %d, closing = %d; want the deleted movement ignored", balance, len(ledger.Rows), ledger.ClosingBalance)
	}
}

func TestAccountBalances_FollowAChangeOfAnOpeningBalance(t *testing.T) {
	// Arrange
	e := newLedgerEnv(t, "01/11/2026")
	e.spend(t, "10/11/2026", "178.50", e.cash)
	if _, err := e.setup.UpdateOpeningBalance(context.Background(), app.OpeningBalanceInput{AccountID: e.cash, Amount: "8,000.00"}); err != nil {
		t.Fatal(err)
	}

	// Act
	balance := e.balance(t, today, e.cash)
	ledger, err := e.svc.ListMovements(context.Background(), app.LedgerFilter{Month: november})

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if balance != 782150 || ledger.OpeningBalance != 800000 || ledger.Rows[0].RunningBalance != 782150 {
		t.Fatalf("cash = %d, opening = %d, row balance = %d; want 7,821.50, 8,000.00, 7,821.50",
			balance, ledger.OpeningBalance, ledger.Rows[0].RunningBalance)
	}
}

func TestListMovements_ShowsTheRunningBalanceOfTheTotalWithOpeningAndClosing(t *testing.T) {
	// Arrange
	e := newLedgerEnv(t, "01/11/2026")
	e.spend(t, "10/11/2026", "178.50", e.cash)
	e.earn(t, "11/11/2026", "200.00", e.bank)

	// Act
	ledger, err := e.svc.ListMovements(context.Background(), app.LedgerFilter{Month: november})

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if len(ledger.Rows) != 2 || ledger.OpeningBalance != 750000 ||
		ledger.Rows[0].RunningBalance != 732150 || ledger.Rows[1].RunningBalance != 752150 || ledger.ClosingBalance != 752150 {
		t.Fatalf("ledger = %+v, want opening 7,500.00, running 7,321.50 then 7,521.50, closing 7,521.50", ledger)
	}
}

func TestListMovements_ContinuesTheRunningBalanceAcrossAMonthBoundary(t *testing.T) {
	// Arrange
	e := newLedgerEnv(t, "01/09/2026")
	e.spend(t, "20/10/2026", "100.00", e.cash)
	e.spend(t, "02/11/2026", "50.00", e.cash)

	// Act
	ledger, err := e.svc.ListMovements(context.Background(), app.LedgerFilter{Month: november})

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if ledger.OpeningBalance != 740000 || len(ledger.Rows) != 1 || ledger.Rows[0].RunningBalance != 735000 {
		t.Fatalf("opening = %d, rows = %d; want October's closing 7,400.00 and a row at 7,350.00", ledger.OpeningBalance, len(ledger.Rows))
	}
}

func TestListMovements_OfOneAccountShowsItsOwnOpeningAndRunningBalance(t *testing.T) {
	// Arrange
	e := newLedgerEnv(t, "01/11/2026")
	e.spend(t, "10/11/2026", "178.50", e.cash)
	e.earn(t, "11/11/2026", "200.00", e.bank)

	// Act
	ledger, err := e.svc.ListMovements(context.Background(), app.LedgerFilter{Month: november, AccountID: e.bank})

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if len(ledger.Rows) != 1 || ledger.OpeningBalance != 0 || ledger.Rows[0].RunningBalance != 20000 || ledger.Rows[0].Account != "Banco (NU)" {
		t.Fatalf("ledger of the bank = %+v, want one row, opening 0, running 200.00", ledger)
	}
}

func TestListMovements_OfAnUnknownAccountIsNotFound(t *testing.T) {
	// Arrange
	e := newLedgerEnv(t, "01/11/2026")

	// Act
	_, err := e.svc.ListMovements(context.Background(), app.LedgerFilter{Month: november, AccountID: 999})

	// Assert
	if !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
}

func TestListMovements_OrdersByDateThenIdWhateverTheOrderOfEntry(t *testing.T) {
	// Arrange
	e := newLedgerEnv(t, "01/11/2026")
	late := e.spend(t, "12/11/2026", "10.00", e.cash)
	first := e.spend(t, "05/11/2026", "20.00", e.cash)
	second := e.spend(t, "05/11/2026", "30.00", e.cash)

	// Act
	ledger, err := e.svc.ListMovements(context.Background(), app.LedgerFilter{Month: november})

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	var ids []int64
	for _, r := range ledger.Rows {
		ids = append(ids, r.Movement.ID)
	}
	if len(ids) != 3 || ids[0] != first.ID || ids[1] != second.ID || ids[2] != late.ID {
		t.Fatalf("order = %v, want %v", ids, []int64{first.ID, second.ID, late.ID})
	}
}

func TestListMovements_TotalsEqualTheSumOfTheRows(t *testing.T) {
	// Arrange
	e := newLedgerEnv(t, "01/11/2026")
	e.earn(t, "09/11/2026", "45.10", e.bank)
	e.spend(t, "10/11/2026", "178.50", e.cash)
	e.spend(t, "11/11/2026", "20.25", e.bank)
	e.earn(t, "11/11/2026", "200.00", e.cash)

	// Act
	ledger, err := e.svc.ListMovements(context.Background(), app.LedgerFilter{Month: november})

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	var income, expense domain.Money
	for _, r := range ledger.Rows {
		switch r.Movement.Kind {
		case domain.MovementIncome:
			income += r.Movement.Amount
		case domain.MovementExpense:
			expense += r.Movement.Amount
		}
	}
	got := ledger.Totals
	if got.Income != income || got.Expense != expense || got.Adjustments != 0 || got.Net != income-expense ||
		income != 24510 || expense != 19875 {
		t.Fatalf("totals = %+v, rows sum to income %d and expense %d", got, income, expense)
	}
}

func TestListMovements_NamesTheCategoryAccountAndVendorOfEachRow(t *testing.T) {
	// Arrange
	e := newLedgerEnv(t, "01/11/2026")
	acme := create(t, e.dir, company("Portones Acme"))
	juan := create(t, e.dir, personOf("Juan Gómez", acme))
	in := e.expense(t)
	in.VendorID = &juan.ID
	if _, err := e.svc.RecordExpense(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	if err := e.dir.DeleteVendor(context.Background(), juan.ID); err != nil {
		t.Fatal(err)
	}

	// Act
	ledger, err := e.svc.ListMovements(context.Background(), app.LedgerFilter{Month: november})

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	row := ledger.Rows[0]
	if row.Category != "Insumos" || row.Account != "Efectivo" || row.Vendor != "Juan Gómez (Portones Acme)" || !row.VendorDeleted {
		t.Fatalf("row = %+v, want Insumos, Efectivo and the deleted vendor Juan Gómez (Portones Acme)", row)
	}
}

func TestListMovements_OfAMonthWithoutMovementsIsEmptyAndClosesAtTheOpening(t *testing.T) {
	// Arrange
	e := newLedgerEnv(t, "01/11/2026")

	// Act
	ledger, err := e.svc.ListMovements(context.Background(), app.LedgerFilter{Month: november})

	// Assert
	if err != nil || len(ledger.Rows) != 0 || ledger.OpeningBalance != 750000 || ledger.ClosingBalance != 750000 {
		t.Fatalf("ledger = %+v, err = %v; want no rows and 7,500.00 on both ends", ledger, err)
	}
}

func TestListCategories_FiltersByDirectionAndHidesInactiveOnes(t *testing.T) {
	// Arrange
	e := newLedgerEnv(t, "01/11/2026")
	if _, err := e.db.Exec(`UPDATE categories SET active = 0 WHERE name = 'Servicios'`); err != nil {
		t.Fatal(err)
	}
	names := func(rows []app.Category) string {
		var out []string
		for _, r := range rows {
			out = append(out, r.Name)
		}
		return strings.Join(out, ",")
	}
	cases := []struct {
		name string
		in   app.ListCategoriesInput
		want string
	}{
		{"every active category", app.ListCategoriesInput{}, "Cuota,Extraordinaria,Otros ingresos,Insumos,Mtto preventivo,Reparaciones"},
		{"income only", app.ListCategoriesInput{Direction: domain.MovementIncome}, "Cuota,Extraordinaria,Otros ingresos"},
		{"expense only", app.ListCategoriesInput{Direction: domain.MovementExpense}, "Insumos,Mtto preventivo,Reparaciones"},
		{"expense with inactive", app.ListCategoriesInput{Direction: domain.MovementExpense, IncludeInactive: true}, "Insumos,Servicios,Mtto preventivo,Reparaciones"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Act
			rows, err := e.cats.ListCategories(context.Background(), c.in)

			// Assert
			if err != nil {
				t.Fatal(err)
			}
			if got := names(rows); got != c.want {
				t.Fatalf("categories = %s, want %s", got, c.want)
			}
		})
	}
}

func TestListCategories_MarksTheSystemCategories(t *testing.T) {
	// Arrange
	e := newLedgerEnv(t, "01/11/2026")

	// Act
	rows, err := e.cats.ListCategories(context.Background(), app.ListCategoriesInput{Direction: domain.MovementIncome})

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 || rows[0].SystemKey != "fee" || rows[1].SystemKey != "assessment" || rows[2].SystemKey != "" {
		t.Fatalf("income categories = %+v, want fee, assessment and none", rows)
	}
}

func TestRecordIncome_EmitsOneWideEventWithIdsAndAmountAndNoDescription(t *testing.T) {
	// Arrange
	e := newLedgerEnv(t, "01/11/2026")
	in := e.income(t)
	in.Description = "Secreto de la descripción"
	in.Notes = "Secreto de las notas"

	// Act
	got, err := e.svc.RecordIncome(context.Background(), in)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	events := readEvents(t, e.events)
	if len(events) != 1 {
		t.Fatalf("%d events, want one: %v", len(events), events)
	}
	ev := events[0]
	if ev["action"] != "record_income" || ev["outcome"] != "ok" || ev["level"] != "INFO" ||
		ev["movement_id"] != float64(got.ID) || ev["account_id"] != float64(e.cash) ||
		ev["category_id"] != float64(in.CategoryID) || ev["amount_cents"] != float64(20000) {
		t.Fatalf("event = %v", ev)
	}
	if _, ok := ev["duration_ms"].(float64); !ok {
		t.Errorf("duration_ms missing: %v", ev)
	}
	if strings.Contains(e.events.String(), "Secreto") {
		t.Fatalf("the event carries free text: %s", e.events.String())
	}
}

func TestRecordExpense_EmitsTheVendorIdAndOneWarningEventWhenTheNegativeBalanceIsDeclined(t *testing.T) {
	// Arrange
	e := newLedgerEnv(t, "01/11/2026")
	vendor := create(t, e.dir, service("Basura"))
	in := e.expense(t)
	in.VendorID, in.Amount = &vendor.ID, "8,000.00"

	// Act
	_, err := e.svc.RecordExpense(context.Background(), in)

	// Assert
	if err == nil {
		t.Fatal("want a warning")
	}
	events := readEvents(t, e.events)
	if len(events) != 1 {
		t.Fatalf("%d events, want one: %v", len(events), events)
	}
	ev := events[0]
	if ev["action"] != "record_expense" || ev["outcome"] != "validation" || ev["level"] != "WARN" ||
		ev["warning"] != "negative_balance" || ev["vendor_id"] != float64(vendor.ID) ||
		ev["amount_cents"] != float64(800000) {
		t.Fatalf("event = %v", ev)
	}
	if _, has := ev["movement_id"]; has {
		t.Errorf("a declined warning carries a movement_id: %v", ev)
	}
}

func TestRecordExpense_EmitsAValidationEventWithTheNumberOfInvalidFields(t *testing.T) {
	// Arrange
	e := newLedgerEnv(t, "01/11/2026")
	in := e.expense(t)
	in.Date, in.Description = "", ""

	// Act
	_, err := e.svc.RecordExpense(context.Background(), in)

	// Assert
	if err == nil {
		t.Fatal("want a validation error")
	}
	events := readEvents(t, e.events)
	if len(events) != 1 || events[0]["outcome"] != "validation" || events[0]["invalid_field_count"] != float64(2) {
		t.Fatalf("events = %v, want one validation event with 2 invalid fields", events)
	}
}

func TestRecordExpense_BeforeSetupFailsWithNotFoundAndLogsAnError(t *testing.T) {
	// Arrange: a migrated database where setup never ran
	db, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := store.Migrate(db, store.Migrations()); err != nil {
		t.Fatal(err)
	}
	events := &bytes.Buffer{}
	svc := app.NewLedgerService(store.NewLedgerRepository(db), fixedClock{today}, slog.New(slog.NewJSONHandler(events, nil)))

	// Act
	_, err = svc.RecordExpense(context.Background(), app.ExpenseInput{})

	// Assert
	if !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
	if got := readEvents(t, events); len(got) != 1 || got[0]["outcome"] != "error" || got[0]["level"] != "ERROR" {
		t.Fatalf("events = %v, want one error event", got)
	}
}

// unorderedCategories is a CategoryRepository that returns its rows in the
// order it holds them, which the interface allows.
type unorderedCategories []app.Category

func (u unorderedCategories) Categories(context.Context) ([]app.Category, error) { return u, nil }

func TestListCategories_OrdersBySortOrderThenIdWhateverTheRepositoryReturns(t *testing.T) {
	// Arrange
	svc := app.NewCategoryService(unorderedCategories{
		{ID: 7, Name: "Reparaciones", Active: true, SortOrder: 13},
		{ID: 5, Name: "Insumos B", Active: true, SortOrder: 10},
		{ID: 1, Name: "Cuota", Active: true, SortOrder: 1},
		{ID: 4, Name: "Insumos A", Active: true, SortOrder: 10},
	})

	// Act
	rows, err := svc.ListCategories(context.Background(), app.ListCategoriesInput{})

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, r := range rows {
		names = append(names, r.Name)
	}
	if got := strings.Join(names, ","); got != "Cuota,Insumos A,Insumos B,Reparaciones" {
		t.Fatalf("order = %s, want Cuota,Insumos A,Insumos B,Reparaciones", got)
	}
}

func TestListMovements_ShowsATransferInTheListOfBothAccountsAndKeepsTheTotalUnchanged(t *testing.T) {
	// Arrange: transfers are recorded by a later step, so the row is inserted directly
	e := newLedgerEnv(t, "01/11/2026")
	if _, err := e.db.Exec(`INSERT INTO movements (date, kind, account_id, to_account_id, amount_cents, description, created_at, updated_at)
		VALUES ('2026-11-12', 'transfer', ?, ?, 60000, 'Transferencia', '2026-11-13 09:30:00', '2026-11-13 09:30:00')`, e.cash, e.bank); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name    string
		account int64
		want    domain.Money
	}{
		{"the total", 0, 750000},
		{"the source", e.cash, 690000},
		{"the destination", e.bank, 60000},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Act
			ledger, err := e.svc.ListMovements(context.Background(), app.LedgerFilter{Month: november, AccountID: c.account})

			// Assert
			if err != nil {
				t.Fatal(err)
			}
			if len(ledger.Rows) != 1 || ledger.Rows[0].RunningBalance != c.want || ledger.Rows[0].ToAccount != "Banco (NU)" {
				t.Fatalf("rows = %+v, want one transfer row to the bank with running balance %d", ledger.Rows, c.want)
			}
		})
	}
	if bank, cash := e.balance(t, today, e.bank), e.balance(t, today, e.cash); bank != 60000 || cash != 690000 {
		t.Fatalf("balances cash = %d, bank = %d; want 6,900.00 and 600.00", cash, bank)
	}
}
