package ledger_test

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vicmangontel/munitorum/internal/app"
	"github.com/vicmangontel/munitorum/internal/domain"
	"github.com/vicmangontel/munitorum/internal/tui/text"
)

// fillTransfer opens the transfer form, which starts on the date, and types an
// amount: the date, the source and the destination come before it.
func fillTransfer(model tea.Model, amount string) tea.Model {
	model = press(model, "t")
	model = press(model, tabs(3)...)
	return press(model, amount)
}

// cashCount records a cash count of cash on the test day, as the Arqueo would.
func (e *env) cashCount(t *testing.T, counted string) {
	t.Helper()
	_, err := e.ledger.RecordCashCount(context.Background(), app.CashCountInput{
		Date: domain.FormatInputDate(today), AccountID: e.cash.ID, Counted: counted,
		Describe: func(c, s domain.Money) string { return text.CashCountDescription(c.Format(), s.Format()) },
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestTransferForm_MovesMoneyBetweenAccountsAsOneRowWithTheGeneratedDescription(t *testing.T) {
	// Arrange
	e := newEnv(t)
	model := fillTransfer(e.shell(t), "600.00")

	// Act: the description is left blank
	model = press(model, "enter")

	// Assert: one row, the amount unsigned, the destination in the party cell
	screen := model.View()
	mustNotContain(t, screen, "Nueva transferencia")
	if n := len(e.movements(t)); n != 1 {
		t.Fatalf("%d movements, want one transfer", n)
	}
	row := lineWith(rightOf(screen), "Transferencia Efectivo")
	mustContain(t, row, "Transferencia", "→ Banco", "Efectivo", "$600.00", "$7,500.00")
	mustNotContain(t, row, "+$600.00", "−$600.00")
	mustContain(t, rightOf(screen), text.LedgerTotals("$0.00", "$0.00", "$0.00", "$0.00"))

	// Assert: the summary panel moved the money and kept the total
	summary := leftOf(press(model, "1").View())
	mustContain(t, lineWith(summary, "Efectivo"), "$6,900.00")
	mustContain(t, lineWith(summary, "Banco (NU)"), "$600.00")
	mustContain(t, lineWith(summary, "Total"), "$7,500.00")
}

func TestTransferForm_KeepsADescriptionTheTreasurerTyped(t *testing.T) {
	// Arrange
	e := newEnv(t)
	model := fillTransfer(e.shell(t), "50.00")

	// Act
	model = press(model, "tab", "Fondo de caja chica", "enter")

	// Assert
	mustContain(t, rightOf(model.View()), "Fondo de caja chica")
	mustNotContain(t, rightOf(model.View()), "Transferencia Efectivo")
}

func TestTransferForm_ShowsEveryInvalidFieldAtOnceAndWritesNothing(t *testing.T) {
	// Arrange: the destination is set to the source, the date is not a date, no amount
	e := newEnv(t)
	model := press(e.shell(t), "t", "ctrl+u", "ayer", "tab", "tab", "left")

	// Act
	model = press(model, "enter")

	// Assert
	mustContain(t, model.View(), "Escriba la fecha como DD/MM/AAAA.", "Elija una cuenta de destino distinta", "Escriba un importe válido")
	if n := len(e.movements(t)); n != 0 {
		t.Fatalf("%d movements written by a form with errors, want 0", n)
	}
}

func TestTransferForm_WarnsWithTheResultingBalanceWhenTheSourceWouldGoNegative(t *testing.T) {
	// Arrange: cash holds 7,500.00
	e := newEnv(t)
	model := fillTransfer(e.shell(t), "8,000.00")

	// Act
	model = press(model, "enter")

	// Assert: the warning names the source and the resulting balance, No selected
	mustContain(t, model.View(), "Esta transferencia deja la cuenta Efectivo en −$500.00", "¿Continuar?", "[ No ]")
}

func TestTransferForm_AnswerNoToTheWarningKeepsTheFormAndWritesNothing(t *testing.T) {
	// Arrange
	e := newEnv(t)
	model := press(fillTransfer(e.shell(t), "8,000.00"), "enter")

	// Act
	model = press(model, "enter")

	// Assert
	mustContain(t, model.View(), "Nueva transferencia")
	if n := len(e.movements(t)); n != 0 {
		t.Fatalf("%d movements written after answering No, want 0", n)
	}
}

func TestTransferForm_AnswerSiRecordsTheTransferThatLeavesTheSourceNegative(t *testing.T) {
	// Arrange
	e := newEnv(t)
	model := press(fillTransfer(e.shell(t), "8,000.00"), "enter")

	// Act
	model = press(model, "right", "enter")

	// Assert
	screen := model.View()
	mustNotContain(t, screen, "Nueva transferencia", "¿Continuar?")
	mustContain(t, lineWith(leftOf(screen), "Efectivo"), "−$500.00")
	mustContain(t, lineWith(leftOf(screen), "Banco (NU)"), "$8,000.00")
}

func TestArqueo_OpensOnCashAndTodayShowingTheSystemBalanceAndNoDifference(t *testing.T) {
	// Arrange: cash holds 7,500.00
	e := newEnv(t)

	// Act
	model := press(e.shell(t), "a")

	// Assert
	mustContain(t, model.View(), "Arqueo", "Fecha (DD/MM/AAAA)  13/11/2026", "‹ Efectivo ›", "Saldo del sistema: $7,500.00")
	mustNotContain(t, model.View(), "Diferencia")
}

func TestArqueo_ShowsTheDifferenceAsTheAmountIsTyped(t *testing.T) {
	cases := []struct {
		name    string
		typed   []string
		want    string
		wantNot string
	}{
		{"one digit", []string{"7"}, "Diferencia: −$7,493.00", ""},
		{"the full amount, one key at a time", []string{"7", "4", "7", "6", ".", "5", "0"}, "Diferencia: −$23.50", ""},
		{"an amount over the system balance", []string{"7", "5", "1", "0"}, "Diferencia: +$10.00", ""},
		{"an amount that matches", []string{"7", "5", "0", "0"}, text.CountBalanced, "Diferencia"},
		{"an amount that is erased again", []string{"7", "backspace"}, "Saldo del sistema", "Diferencia"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange: the overlay opens on the date; the amount is two fields on
			e := newEnv(t)
			model := press(e.shell(t), "a", "tab", "tab")

			// Act
			model = press(model, c.typed...)

			// Assert
			mustContain(t, model.View(), c.want)
			if c.wantNot != "" {
				mustNotContain(t, model.View(), c.wantNot)
			}
		})
	}
}

func TestArqueo_ReadsTheSystemBalanceOfTheChosenDateAndAccount(t *testing.T) {
	cases := []struct {
		name string
		keys []string
		want string
	}{
		{"yesterday, before today's expense", []string{"ctrl+u", "12/11/2026"}, "Saldo del sistema: $7,500.00"},
		{"the bank, today", []string{"tab", "right"}, "Saldo del sistema: $0.00"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange: an expense today of 100.00 from cash, so cash holds 7,400.00 now
			e := newEnv(t)
			if _, err := e.ledger.RecordExpense(context.Background(), app.ExpenseInput{
				Date: domain.FormatInputDate(today), AccountID: e.cash.ID, CategoryID: e.category(t, "Insumos"),
				Amount: "100.00", Description: "Escobas",
			}); err != nil {
				t.Fatal(err)
			}
			model := press(e.shell(t), "a")

			// Act
			model = press(model, c.keys...)

			// Assert
			mustContain(t, model.View(), c.want)
		})
	}
}

func TestArqueo_WithADifferenceWritesOneAdjustmentShownAsAjusteAndMovesTheSummary(t *testing.T) {
	// Arrange: counted 7,476.50 against 7,500.00
	e := newEnv(t)
	model := press(e.shell(t), "a", "tab", "tab", "7,476.50")

	// Act
	model = press(model, "enter")

	// Assert
	screen := model.View()
	mustNotContain(t, screen, "Saldo del sistema")
	rows := e.movements(t)
	if len(rows) != 1 || rows[0].Movement.Kind != domain.MovementAdjustment || rows[0].Movement.Amount != -2350 ||
		rows[0].Movement.Description != "Arqueo: contado $7,476.50, sistema $7,500.00" {
		t.Fatalf("rows = %+v, want one −23.50 adjustment worded by the TUI", rows)
	}
	row := lineWith(rightOf(screen), "Arqueo: contado")
	mustContain(t, row, "Ajuste", "Efectivo", "−$23.50", "$7,476.50")
	mustContain(t, rightOf(screen), text.LedgerTotals("$0.00", "$0.00", "−$23.50", "−$23.50"))
	mustContain(t, lineWith(leftOf(screen), "Efectivo"), "$7,476.50")
}

func TestArqueo_ShowsAPositiveAdjustmentWithAPlusSign(t *testing.T) {
	// Arrange
	e := newEnv(t)
	model := press(e.shell(t), "a", "tab", "tab", "7,510.00", "enter")

	// Assert
	mustContain(t, lineWith(rightOf(model.View()), "Arqueo: contado"), "Ajuste", "+$10.00", "$7,510.00")
}

func TestArqueo_ThatMatchesWritesNothingAndKeepsShowingCuadra(t *testing.T) {
	// Arrange
	e := newEnv(t)
	model := press(e.shell(t), "a", "tab", "tab", "7,500.00")

	// Act
	model = press(model, "enter")

	// Assert
	mustContain(t, model.View(), "Arqueo", text.CountBalanced)
	if n := len(e.movements(t)); n != 0 {
		t.Fatalf("%d movements written by a matching count, want 0", n)
	}
}

func TestArqueo_RejectsANegativeCountedAmountAndWritesNothing(t *testing.T) {
	// Arrange
	e := newEnv(t)
	model := press(e.shell(t), "a", "tab", "tab", "-5")

	// Act
	model = press(model, "enter")

	// Assert
	mustContain(t, model.View(), "El monto contado no puede ser negativo.")
	mustNotContain(t, model.View(), "Diferencia")
	if n := len(e.movements(t)); n != 0 {
		t.Fatalf("%d movements written, want 0", n)
	}
}

func TestArqueo_EscapeCancelsWithoutWriting(t *testing.T) {
	// Arrange
	e := newEnv(t)
	model := press(e.shell(t), "a", "tab", "tab", "100.00")

	// Act
	model = press(model, "esc")

	// Assert
	mustNotContain(t, model.View(), "Saldo del sistema")
	if n := len(e.movements(t)); n != 0 {
		t.Fatalf("%d movements written after esc, want 0", n)
	}
}

func TestLedger_ShowsAnAdjustmentRecordedElsewhereAsAjuste(t *testing.T) {
	// Arrange
	e := newEnv(t)
	e.cashCount(t, "7,400.00")

	// Act
	screen := e.shell(t).View()

	// Assert
	mustContain(t, lineWith(rightOf(screen), "Arqueo: contado $7,400.00"), "Ajuste", "−$100.00", "$7,400.00")
}

func TestLedger_ListsTheTransferAndArqueoKeysInTheHintBar(t *testing.T) {
	// Arrange
	e := newEnv(t)

	// Act
	screen := e.shell(t).View()

	// Assert
	mustContain(t, screen, "t transferencia", "a arqueo")
}
