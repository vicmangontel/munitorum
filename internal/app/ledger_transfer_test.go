package app_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/vicmangontel/munitorum/internal/app"
	"github.com/vicmangontel/munitorum/internal/domain"
)

// transfer is a valid transfer of 600.00 from cash to the bank on 10 November.
func (e *ledgerEnv) transfer() app.TransferInput {
	return app.TransferInput{
		Date: "10/11/2026", FromAccountID: e.cash, ToAccountID: e.bank, Amount: "600.00",
		Description: "Transferencia Efectivo → Banco (NU)",
	}
}

// describeCount is the wording the tests give the service, like the TUI's.
func describeCount(counted, system domain.Money) string {
	return fmt.Sprintf("Arqueo: contado %s, sistema %s", counted.Format(), system.Format())
}

// cashCount is a valid cash count of cash on 12 November.
func (e *ledgerEnv) cashCount(counted string) app.CashCountInput {
	return app.CashCountInput{Date: "12/11/2026", AccountID: e.cash, Counted: counted, Describe: describeCount}
}

func TestRecordTransfer_MovesMoneyBetweenAccountsKeepsTheTotalAndIsOneRow(t *testing.T) {
	// Arrange: cash 7,500.00, bank 0
	e := newLedgerEnv(t, "01/11/2026")

	// Act
	got, err := e.svc.RecordTransfer(context.Background(), e.transfer())

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if cash, bank := e.balance(t, today, e.cash), e.balance(t, today, e.bank); cash != 690000 || bank != 60000 {
		t.Fatalf("cash = %d, bank = %d; want 6,900.00 and 600.00", cash, bank)
	}
	balances, err := e.svc.AccountBalances(context.Background(), today)
	if err != nil || balances.Total != 750000 {
		t.Fatalf("total = %d (%v), want it unchanged at 7,500.00", balances.Total, err)
	}
	if n := count(t, e.db, "movements"); n != 1 {
		t.Fatalf("movements has %d rows, want one", n)
	}
	if got.Kind != domain.MovementTransfer || got.ToAccountID == nil || *got.ToAccountID != e.bank || got.CategoryID != nil ||
		got.Amount != 60000 || got.Receipt != app.ReceiptNone || got.ID == 0 {
		t.Fatalf("created = %+v", got)
	}
	ledger, err := e.svc.ListMovements(context.Background(), app.LedgerFilter{Month: november})
	if err != nil || len(ledger.Rows) != 1 || ledger.Rows[0].Account != "Efectivo" || ledger.Rows[0].ToAccount != "Banco (NU)" {
		t.Fatalf("rows = %+v (%v), want one transfer row Efectivo to Banco (NU)", ledger.Rows, err)
	}
}

func TestRecordTransfer_RejectsTheSameAccountOnBothSides(t *testing.T) {
	// Arrange
	e := newLedgerEnv(t, "01/11/2026")
	in := e.transfer()
	in.ToAccountID = e.cash

	// Act
	_, err := e.svc.RecordTransfer(context.Background(), in)

	// Assert
	got := invalidFields(t, err)
	if len(got) != 1 || got[app.FieldToAccount] != app.CodeAccountsSame {
		t.Fatalf("fields = %v, want only %s: %s", got, app.FieldToAccount, app.CodeAccountsSame)
	}
	if n := count(t, e.db, "movements"); n != 0 {
		t.Fatalf("movements has %d rows, want 0", n)
	}
}

func TestRecordTransfer_ReportsEveryInvalidFieldTogether(t *testing.T) {
	// Arrange
	e := newLedgerEnv(t, "01/11/2026")
	in := app.TransferInput{
		Date: "mañana", FromAccountID: 999, ToAccountID: 998, Amount: "x", Description: "", Notes: strings.Repeat("n", 501),
	}

	// Act
	_, err := e.svc.RecordTransfer(context.Background(), in)

	// Assert
	got := invalidFields(t, err)
	want := map[string]string{
		app.FieldDate: app.CodeDateInvalid, app.FieldAmount: app.CodeAmountInvalid,
		app.FieldAccount: app.CodeAccountInvalid, app.FieldToAccount: app.CodeAccountInvalid,
		app.FieldDescription: app.CodeDescriptionLength, app.FieldNotes: app.CodeNotesLength,
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

func TestRecordTransfer_ChecksTheDateAndAmountLikeAnyMovement(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*app.TransferInput)
		field  string
		code   string
	}{
		{"before the opening date", func(in *app.TransferInput) { in.Date = "31/10/2026" }, app.FieldDate, app.CodeDateBeforeOpening},
		{"after today", func(in *app.TransferInput) { in.Date = "14/11/2026" }, app.FieldDate, app.CodeDateFuture},
		{"zero amount", func(in *app.TransferInput) { in.Amount = "0" }, app.FieldAmount, app.CodeAmountNotPositive},
		{"negative amount", func(in *app.TransferInput) { in.Amount = "-5" }, app.FieldAmount, app.CodeAmountNotPositive},
		{"blank description", func(in *app.TransferInput) { in.Description = "  " }, app.FieldDescription, app.CodeDescriptionLength},
		{"description of 121 characters", func(in *app.TransferInput) { in.Description = strings.Repeat("d", 121) }, app.FieldDescription, app.CodeDescriptionLength},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange
			e := newLedgerEnv(t, "01/11/2026")
			in := e.transfer()
			c.mutate(&in)

			// Act
			_, err := e.svc.RecordTransfer(context.Background(), in)

			// Assert
			if got := invalidFields(t, err); len(got) != 1 || got[c.field] != c.code {
				t.Fatalf("fields = %v, want only %s: %s", got, c.field, c.code)
			}
		})
	}
}

func TestRecordTransfer_ThatLeavesTheSourceNegativeWarnsAndWritesNothing(t *testing.T) {
	// Arrange: cash holds 7,500.00
	e := newLedgerEnv(t, "01/11/2026")
	in := e.transfer()
	in.Amount = "8,000.00"

	// Act
	_, err := e.svc.RecordTransfer(context.Background(), in)

	// Assert
	var warning app.NegativeBalanceWarning
	if !errors.As(err, &warning) || warning.Account.ID != e.cash || warning.Resulting != -50000 {
		t.Fatalf("error = %v, want a warning leaving cash at −500.00", err)
	}
	if n := count(t, e.db, "movements"); n != 0 {
		t.Fatalf("movements has %d rows after a declined warning, want 0", n)
	}
}

func TestRecordTransfer_WithConfirmNegativeWritesTheTransferThatLeavesTheSourceNegative(t *testing.T) {
	// Arrange: cash holds 7,500.00
	e := newLedgerEnv(t, "01/11/2026")
	in := e.transfer()
	in.Amount, in.ConfirmNegative = "8,000.00", true

	// Act
	_, err := e.svc.RecordTransfer(context.Background(), in)

	// Assert
	if err != nil {
		t.Fatalf("a confirmed transfer was refused: %v", err)
	}
	if cash, bank := e.balance(t, today, e.cash), e.balance(t, today, e.bank); cash != -50000 || bank != 800000 {
		t.Fatalf("cash = %d, bank = %d; want −500.00 and 8,000.00", cash, bank)
	}
}

func TestRecordTransfer_OfTheWholeSourceBalanceDoesNotWarn(t *testing.T) {
	// Arrange
	e := newLedgerEnv(t, "01/11/2026")
	in := e.transfer()
	in.Amount = "7,500.00"

	// Act
	_, err := e.svc.RecordTransfer(context.Background(), in)

	// Assert
	if err != nil {
		t.Fatalf("a transfer that leaves exactly zero was refused: %v", err)
	}
}

func TestRecordTransfer_WarnsFromTheBalanceOfEveryDateNotOnlyThatOfTheTransfer(t *testing.T) {
	// Arrange: a later expense already took cash to 7,000.00; an earlier transfer of 7,200.00 leaves −200.00 today
	e := newLedgerEnv(t, "01/11/2026")
	e.spend(t, "12/11/2026", "500.00", e.cash)
	in := e.transfer()
	in.Date, in.Amount = "05/11/2026", "7,200.00"

	// Act
	_, err := e.svc.RecordTransfer(context.Background(), in)

	// Assert
	var warning app.NegativeBalanceWarning
	if !errors.As(err, &warning) || warning.Resulting != -20000 {
		t.Fatalf("error = %v, want a warning leaving cash at −200.00", err)
	}
}

func TestRecordTransfer_IntoAnAccountBelowZeroDoesNotWarn(t *testing.T) {
	// Arrange: the bank is overdrawn by 100.00; only the source is checked
	e := newLedgerEnv(t, "01/11/2026")
	e.overdraw(t, e.bank, "100.00")
	in := e.transfer()
	in.Amount = "10.00"

	// Act
	_, err := e.svc.RecordTransfer(context.Background(), in)

	// Assert
	if err != nil {
		t.Fatalf("a transfer into an overdrawn account was refused: %v", err)
	}
	if got := e.balance(t, today, e.bank); got != -9000 {
		t.Fatalf("bank = %d, want −90.00", got)
	}
}

func TestRecordTransfer_FromAnAccountAlreadyBelowZeroWarnsAgain(t *testing.T) {
	// Arrange: the bank was overdrawn to −100.00
	e := newLedgerEnv(t, "01/11/2026")
	e.overdraw(t, e.bank, "100.00")
	in := e.transfer()
	in.FromAccountID, in.ToAccountID, in.Amount = e.bank, e.cash, "10.00"

	// Act
	_, err := e.svc.RecordTransfer(context.Background(), in)

	// Assert
	var warning app.NegativeBalanceWarning
	if !errors.As(err, &warning) || warning.Account.ID != e.bank || warning.Resulting != -11000 {
		t.Fatalf("error = %v, want a warning leaving the bank at −110.00", err)
	}
}

func TestRecordCashCount_ShortCountWritesOneAdjustmentForTheDifference(t *testing.T) {
	// Arrange: cash 7,500.00 − 3,176.50 = 4,323.50 on 12 November
	e := newLedgerEnv(t, "01/11/2026")
	e.spend(t, "10/11/2026", "3,176.50", e.cash)

	// Act
	got, err := e.svc.RecordCashCount(context.Background(), e.cashCount("4,300.00"))

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if got.Difference != -2350 || got.Movement == nil {
		t.Fatalf("result = %+v, want a −23.50 difference with a movement", got)
	}
	m := got.Movement
	if m.Kind != domain.MovementAdjustment || m.Amount != -2350 || m.AccountID != e.cash || m.ID == 0 ||
		m.CategoryID != nil || m.ToAccountID != nil || m.Receipt != app.ReceiptNone ||
		m.Description != "Arqueo: contado $4,300.00, sistema $4,323.50" || m.Date != time.Date(2026, time.November, 12, 0, 0, 0, 0, time.Local) {
		t.Fatalf("movement = %+v", m)
	}
	if n := count(t, e.db, "movements"); n != 2 {
		t.Fatalf("movements has %d rows, want the expense and one adjustment", n)
	}
	if cash := e.balance(t, today, e.cash); cash != 430000 {
		t.Fatalf("cash = %d, want the counted 4,300.00", cash)
	}
}

func TestRecordCashCount_OverCountWritesAPositiveAdjustment(t *testing.T) {
	// Arrange
	e := newLedgerEnv(t, "01/11/2026")

	// Act
	got, err := e.svc.RecordCashCount(context.Background(), e.cashCount("7,510.00"))

	// Assert
	if err != nil || got.Difference != 1000 || got.Movement == nil || got.Movement.Amount != 1000 {
		t.Fatalf("result = %+v (%v), want a +10.00 adjustment", got, err)
	}
}

func TestRecordCashCount_MatchingTheSystemBalanceWritesNothing(t *testing.T) {
	// Arrange
	e := newLedgerEnv(t, "01/11/2026")

	// Act
	got, err := e.svc.RecordCashCount(context.Background(), e.cashCount("7,500.00"))

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if got.Difference != 0 || got.Movement != nil {
		t.Fatalf("result = %+v, want no difference and no movement", got)
	}
	if n := count(t, e.db, "movements"); n != 0 {
		t.Fatalf("movements has %d rows, want 0", n)
	}
}

func TestRecordCashCount_SystemBalanceIncludesTheWholeOfTheDateAndNothingAfterIt(t *testing.T) {
	// Arrange: an expense on the counted day and one the day after
	e := newLedgerEnv(t, "01/11/2026")
	e.spend(t, "12/11/2026", "100.00", e.cash)
	e.spend(t, "13/11/2026", "50.00", e.cash)

	// Act: 7,400.00 is the balance at the end of the 12th
	got, err := e.svc.RecordCashCount(context.Background(), e.cashCount("7,400.00"))

	// Assert
	if err != nil || got.Difference != 0 || got.Movement != nil {
		t.Fatalf("result = %+v (%v), want the count to match", got, err)
	}
}

func TestRecordCashCount_CountsOneAccountOnly(t *testing.T) {
	// Arrange: the bank holds 600.00 after a transfer
	e := newLedgerEnv(t, "01/11/2026")
	if _, err := e.svc.RecordTransfer(context.Background(), e.transfer()); err != nil {
		t.Fatal(err)
	}
	in := e.cashCount("600.00")
	in.AccountID = e.bank

	// Act
	got, err := e.svc.RecordCashCount(context.Background(), in)

	// Assert
	if err != nil || got.Difference != 0 || got.Movement != nil {
		t.Fatalf("result = %+v (%v), want the bank count to match", got, err)
	}
}

func TestRecordCashCount_CheckedCountedAmount(t *testing.T) {
	cases := []struct {
		name    string
		counted string
		code    string // empty: accepted
	}{
		{"negative", "-1.00", app.CodeAmountNegative},
		{"unreadable", "abc", app.CodeAmountInvalid},
		{"empty", "", app.CodeAmountInvalid},
		{"zero is a valid count", "0", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange
			e := newLedgerEnv(t, "01/11/2026")

			// Act
			got, err := e.svc.RecordCashCount(context.Background(), e.cashCount(c.counted))

			// Assert
			if c.code == "" {
				if err != nil || got.Difference != -750000 || got.Movement == nil {
					t.Fatalf("result = %+v (%v), want a −7,500.00 adjustment", got, err)
				}
				return
			}
			if fields := invalidFields(t, err); len(fields) != 1 || fields[app.FieldAmount] != c.code {
				t.Fatalf("fields = %v, want only amount: %s", fields, c.code)
			}
			if n := count(t, e.db, "movements"); n != 0 {
				t.Fatalf("movements has %d rows after a rejected count, want 0", n)
			}
		})
	}
}

func TestRecordCashCount_ReportsEveryInvalidFieldTogether(t *testing.T) {
	// Arrange
	e := newLedgerEnv(t, "01/11/2026")
	in := app.CashCountInput{Date: "14/11/2026", AccountID: 999, Counted: "-5", Notes: strings.Repeat("n", 501), Describe: describeCount}

	// Act
	_, err := e.svc.RecordCashCount(context.Background(), in)

	// Assert
	got := invalidFields(t, err)
	want := map[string]string{
		app.FieldDate: app.CodeDateFuture, app.FieldAccount: app.CodeAccountInvalid,
		app.FieldAmount: app.CodeAmountNegative, app.FieldNotes: app.CodeNotesLength,
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

func TestRecordCashCount_RejectsADateBeforeTheOpeningDate(t *testing.T) {
	// Arrange
	e := newLedgerEnv(t, "01/11/2026")
	in := e.cashCount("7,500.00")
	in.Date = "31/10/2026"

	// Act
	_, err := e.svc.RecordCashCount(context.Background(), in)

	// Assert
	if got := invalidFields(t, err); len(got) != 1 || got[app.FieldDate] != app.CodeDateBeforeOpening {
		t.Fatalf("fields = %v, want only date: %s", got, app.CodeDateBeforeOpening)
	}
}

func TestRecordCashCount_WithoutADescribeFunctionFailsWithoutWriting(t *testing.T) {
	// Arrange
	e := newLedgerEnv(t, "01/11/2026")
	in := e.cashCount("100.00")
	in.Describe = nil

	// Act
	got, err := e.svc.RecordCashCount(context.Background(), in)

	// Assert
	if err == nil {
		t.Fatal("want an error for a missing Describe")
	}
	if got.Movement != nil || count(t, e.db, "movements") != 0 {
		t.Fatalf("result = %+v, movements = %d; want nothing written", got, count(t, e.db, "movements"))
	}
	events := readEvents(t, e.events)
	if len(events) != 1 || events[0]["outcome"] != "error" || events[0]["action"] != "record_cash_count" {
		t.Fatalf("events = %v, want one error event", events)
	}
}

func TestRecordTransfer_EmitsOneWideEventWithBothAccountsAndNoDescription(t *testing.T) {
	// Arrange
	e := newLedgerEnv(t, "01/11/2026")
	in := e.transfer()
	in.Description, in.Notes = "Secreto de la descripción", "Secreto de las notas"

	// Act
	got, err := e.svc.RecordTransfer(context.Background(), in)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	events := readEvents(t, e.events)
	if len(events) != 1 {
		t.Fatalf("%d events, want one: %v", len(events), events)
	}
	ev := events[0]
	if ev["action"] != "record_transfer" || ev["outcome"] != "ok" || ev["level"] != "INFO" ||
		ev["movement_id"] != float64(got.ID) || ev["account_id"] != float64(e.cash) ||
		ev["to_account_id"] != float64(e.bank) || ev["amount_cents"] != float64(60000) {
		t.Fatalf("event = %v", ev)
	}
	if strings.Contains(e.events.String(), "Secreto") {
		t.Fatalf("the event carries free text: %s", e.events.String())
	}
}

func TestRecordTransfer_EmitsOneWarningEventWhenTheNegativeBalanceIsDeclined(t *testing.T) {
	// Arrange
	e := newLedgerEnv(t, "01/11/2026")
	in := e.transfer()
	in.Amount = "8,000.00"

	// Act
	_, err := e.svc.RecordTransfer(context.Background(), in)

	// Assert
	if err == nil {
		t.Fatal("want a warning")
	}
	events := readEvents(t, e.events)
	if len(events) != 1 {
		t.Fatalf("%d events, want one: %v", len(events), events)
	}
	ev := events[0]
	if ev["action"] != "record_transfer" || ev["outcome"] != "validation" || ev["level"] != "WARN" ||
		ev["warning"] != "negative_balance" || ev["to_account_id"] != float64(e.bank) {
		t.Fatalf("event = %v", ev)
	}
	if _, has := ev["movement_id"]; has {
		t.Errorf("a declined warning carries a movement_id: %v", ev)
	}
}

func TestRecordCashCount_EmitsTheDifferenceAndTheMovementIdWhenWrittenAndNoNotes(t *testing.T) {
	// Arrange
	e := newLedgerEnv(t, "01/11/2026")
	in := e.cashCount("7,476.50")
	in.Notes = "Secreto de las notas"

	// Act
	got, err := e.svc.RecordCashCount(context.Background(), in)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	events := readEvents(t, e.events)
	if len(events) != 1 {
		t.Fatalf("%d events, want one: %v", len(events), events)
	}
	ev := events[0]
	if ev["action"] != "record_cash_count" || ev["outcome"] != "ok" || ev["account_id"] != float64(e.cash) ||
		ev["difference_cents"] != float64(-2350) || ev["movement_id"] != float64(got.Movement.ID) {
		t.Fatalf("event = %v", ev)
	}
	if strings.Contains(e.events.String(), "Secreto") || strings.Contains(e.events.String(), "Arqueo") {
		t.Fatalf("the event carries free text: %s", e.events.String())
	}
}

func TestRecordCashCount_EmitsNoMovementIdWhenTheCountMatches(t *testing.T) {
	// Arrange
	e := newLedgerEnv(t, "01/11/2026")

	// Act
	if _, err := e.svc.RecordCashCount(context.Background(), e.cashCount("7,500.00")); err != nil {
		t.Fatal(err)
	}

	// Assert
	events := readEvents(t, e.events)
	if len(events) != 1 || events[0]["difference_cents"] != float64(0) {
		t.Fatalf("events = %v, want one event with difference_cents 0", events)
	}
	if _, has := events[0]["movement_id"]; has {
		t.Errorf("a matching count carries a movement_id: %v", events[0])
	}
}

func TestRecordCashCount_EmitsAValidationEventWithTheNumberOfInvalidFields(t *testing.T) {
	// Arrange
	e := newLedgerEnv(t, "01/11/2026")

	// Act
	_, err := e.svc.RecordCashCount(context.Background(), e.cashCount("-1"))

	// Assert
	if err == nil {
		t.Fatal("want a validation error")
	}
	events := readEvents(t, e.events)
	if len(events) != 1 || events[0]["outcome"] != "validation" || events[0]["invalid_field_count"] != float64(1) {
		t.Fatalf("events = %v, want one validation event with 1 invalid field", events)
	}
	if _, has := events[0]["difference_cents"]; has {
		t.Errorf("a rejected count carries a difference: %v", events[0])
	}
}

func TestRecordCashCount_AcceptsNotesOfExactlyTheLimitAndStoresThemTrimmed(t *testing.T) {
	// Arrange: 500 characters, padded with spaces that are not counted
	e := newLedgerEnv(t, "01/11/2026")
	in := e.cashCount("7,400.00")
	in.Notes = "  " + strings.Repeat("n", 500) + "  "

	// Act
	got, err := e.svc.RecordCashCount(context.Background(), in)

	// Assert
	if err != nil {
		t.Fatalf("notes of 500 characters were refused: %v", err)
	}
	if got.Movement == nil || got.Movement.Notes != strings.Repeat("n", 500) {
		t.Fatalf("movement = %+v, want the 500-character notes trimmed", got.Movement)
	}
}
