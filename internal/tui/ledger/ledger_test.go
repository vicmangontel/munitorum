package ledger_test

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vicmangontel/munitorum/internal/app"
	"github.com/vicmangontel/munitorum/internal/domain"
	"github.com/vicmangontel/munitorum/internal/store"
	"github.com/vicmangontel/munitorum/internal/tui"
	"github.com/vicmangontel/munitorum/internal/tui/text"
)

// today is the date these tests run on.
var today = time.Date(2026, time.November, 13, 9, 30, 0, 0, time.Local)

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

// settle delivers msgs to model one by one, running every command that comes
// back (a batch in order) and feeding its message in, as the Bubble Tea runtime
// does.
func settle(model tea.Model, msgs ...tea.Msg) tea.Model {
	var deliver func(tea.Msg)
	deliver = func(msg tea.Msg) {
		if msg == nil {
			return
		}
		if batch, ok := msg.(tea.BatchMsg); ok {
			for _, cmd := range batch {
				if cmd != nil {
					deliver(cmd())
				}
			}
			return
		}
		var cmd tea.Cmd
		model, cmd = model.Update(msg)
		if cmd != nil {
			deliver(cmd())
		}
	}
	for _, m := range msgs {
		deliver(m)
	}
	return model
}

// key turns "enter", "tab", "right", ... into the key message the runtime sends;
// anything else is typed text.
func key(s string) tea.KeyMsg {
	named := map[string]tea.KeyType{
		"enter": tea.KeyEnter, "tab": tea.KeyTab, "shift+tab": tea.KeyShiftTab, "esc": tea.KeyEsc,
		"up": tea.KeyUp, "down": tea.KeyDown, "left": tea.KeyLeft, "right": tea.KeyRight,
		"backspace": tea.KeyBackspace, "ctrl+u": tea.KeyCtrlU,
	}
	if t, ok := named[s]; ok {
		return tea.KeyMsg{Type: t}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

// press delivers the keys, in order.
func press(model tea.Model, keys ...string) tea.Model {
	for _, k := range keys {
		model = settle(model, key(k))
	}
	return model
}

// tabs returns n tab keys.
func tabs(n int) []string { return strings.Split(strings.TrimSpace(strings.Repeat("tab ", n)), " ") }

// env is a completed coto on a real, migrated database: cash "Efectivo" holding
// 7,500.00 and bank "Banco (NU)" holding nothing, opened long ago.
type env struct {
	setup  *app.SetupService
	ledger *app.LedgerService
	cats   *app.CategoryService
	dir    *app.DirectoryService
	cash   app.Account
}

func newEnv(t *testing.T) *env {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := store.Migrate(db, store.Migrations()); err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	e := &env{setup: app.NewSetupService(store.NewSetupRepository(db), fixedClock{today}, log)}
	repo := store.NewLedgerRepository(db)
	e.ledger = app.NewLedgerService(repo, fixedClock{today}, log)
	e.cats = app.NewCategoryService(repo)
	e.dir = app.NewDirectoryService(store.NewVendorRepository(db), fixedClock{today}, log)
	settings, err := e.setup.CompleteSetup(context.Background(), app.SetupInput{
		CommunityName: "Coto Los Pinos", HouseCount: "2", HousePrefix: "CASA ", FeeAmount: "150",
		Accounts:    [2]app.SetupAccount{{Name: "Efectivo", OpeningBalance: "7,500.00"}, {Name: "Banco (NU)", OpeningBalance: "0"}},
		OpeningDate: "01/01/2020", Houses: make([]app.SetupHouse, 2),
	})
	if err != nil {
		t.Fatal(err)
	}
	e.cash = settings.Accounts[0]
	return e
}

// shell returns the shell of the coto showing Movimientos, main panel focused,
// with the ledger loaded.
func (e *env) shell(t *testing.T) tea.Model {
	t.Helper()
	settings, err := e.setup.GetSettings(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	root := tui.NewShell(e.setup, settings).WithDirectory(e.dir).WithLedger(e.ledger, e.cats, e.dir, fixedClock{today})
	var model tea.Model = root
	model = settle(model, tea.WindowSizeMsg{Width: 140, Height: 40})
	model = settle(model, root.Init()())
	return press(model, "3")
}

func (e *env) category(t *testing.T, name string) int64 {
	t.Helper()
	for _, dir := range []domain.MovementKind{domain.MovementIncome, domain.MovementExpense} {
		rows, err := e.cats.ListCategories(context.Background(), app.ListCategoriesInput{Direction: dir})
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range rows {
			if c.Name == name {
				return c.ID
			}
		}
	}
	t.Fatalf("no category %q", name)
	return 0
}

// movements returns the rows of the current month, read through the service.
func (e *env) movements(t *testing.T) []app.LedgerRow {
	t.Helper()
	l, err := e.ledger.ListMovements(context.Background(), app.LedgerFilter{Month: domain.Of(today)})
	if err != nil {
		t.Fatal(err)
	}
	return l.Rows
}

// leftOf and rightOf split a screen at the left column, whose first rows are the
// summary panel.
func leftOf(screen string) string  { return split(screen, true) }
func rightOf(screen string) string { return split(screen, false) }

func split(screen string, left bool) string {
	const leftWidth = 28
	var out []string
	for _, line := range strings.Split(screen, "\n") {
		r := []rune(line)
		cut := min(leftWidth, len(r))
		if left {
			out = append(out, string(r[:cut]))
		} else {
			out = append(out, string(r[cut:]))
		}
	}
	return strings.Join(out, "\n")
}

func lineWith(screen, needle string) string {
	for _, line := range strings.Split(screen, "\n") {
		if strings.Contains(line, needle) {
			return line
		}
	}
	return ""
}

func mustContain(t *testing.T, out string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(out, w) {
			t.Errorf("screen does not show %q:\n%s", w, out)
		}
	}
}

func mustNotContain(t *testing.T, out string, nots ...string) {
	t.Helper()
	for _, n := range nots {
		if strings.Contains(out, n) {
			t.Errorf("screen shows %q:\n%s", n, out)
		}
	}
}

// fillExpense types an amount and a description into the expense form, which
// opens on the date: the vendor field sits between the category and the amount.
func fillExpense(model tea.Model, amount, description string) tea.Model {
	model = press(model, "g")
	model = press(model, tabs(4)...) // date, account, category, vendor, amount
	model = press(model, amount, "tab", description)
	return model
}

func TestExpenseFromCash_ShowsCashAtItsNewBalanceInTheListAndInTheSummaryPanel(t *testing.T) {
	// Arrange
	e := newEnv(t)
	model := e.shell(t)

	// Act
	model = fillExpense(model, "178.50", "Garrafones de agua")
	model = press(model, "enter")

	// Assert: the list row, in the main panel
	screen := model.View()
	mustNotContain(t, screen, "Nuevo gasto")
	row := lineWith(rightOf(screen), "Garrafones")
	mustContain(t, row, "Insumos", "Efectivo", "−$178.50", "$7,321.50")
	mustContain(t, rightOf(screen), text.LedgerTotals("$0.00", "$178.50", "$0.00", "−$178.50"))

	// Assert: the summary panel, with the list out of the way
	summary := leftOf(press(model, "1").View())
	mustContain(t, summary, "Efectivo", "$7,321.50", "Banco (NU)", "$0.00", "Total")
	if cashRow := lineWith(summary, "Efectivo"); !strings.Contains(cashRow, "$7,321.50") {
		t.Fatalf("the cash row of the summary panel is %q, want it at $7,321.50", cashRow)
	}
}

func TestExpenseForm_ShowsEveryInvalidFieldAtOnceAndWritesNothing(t *testing.T) {
	cases := []struct {
		name string
		key  string
		wait []string // fields to leave empty: amount and description
		want []string
	}{
		{"expense", "g", nil, []string{"Escriba la fecha como DD/MM/AAAA.", "Escriba un importe válido", "La descripción es obligatoria"}},
		{"income", "i", nil, []string{"Escriba la fecha como DD/MM/AAAA.", "Escriba un importe válido", "La descripción es obligatoria"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange: the date is cleared and replaced by text that is not a date
			e := newEnv(t)
			model := press(e.shell(t), c.key, "ctrl+u", "ayer")

			// Act
			model = press(model, "enter")

			// Assert
			mustContain(t, model.View(), c.want...)
			if n := len(e.movements(t)); n != 0 {
				t.Fatalf("%d movements written by a form with errors, want 0", n)
			}
		})
	}
}

func TestExpenseForm_ShowsTheErrorsOfTheServiceForAnAmountThatIsNotPositive(t *testing.T) {
	// Arrange
	e := newEnv(t)
	model := fillExpense(e.shell(t), "0", "Nada")

	// Act
	model = press(model, "enter")

	// Assert
	mustContain(t, model.View(), "El monto debe ser mayor que cero.")
}

func TestIncomeForm_DoesNotOfferTheCategoriesOfOtherModules(t *testing.T) {
	// Arrange
	e := newEnv(t)

	// Act
	screen := press(e.shell(t), "i").View()

	// Assert
	mustContain(t, screen, "Nuevo ingreso", "‹ Otros ingresos ›")
	mustNotContain(t, screen, "Cuota", "Extraordinaria")
}

func TestIncomeForm_SavesAnIncomeWithAPendingManualReceipt(t *testing.T) {
	// Arrange: date, account, category, amount, description, notes, link, manual receipt
	e := newEnv(t)
	model := press(e.shell(t), "i")
	model = press(model, tabs(3)...)
	model = press(model, "200.00", "tab", "Renta del salón")
	model = press(model, tabs(3)...)

	// Act
	model = press(model, "right", "enter")

	// Assert
	screen := rightOf(model.View())
	row := lineWith(screen, "Renta del salón")
	mustContain(t, row, "Otros ingresos", "+$200.00", "$7,700.00", text.ReceiptPendingMark)
}

func TestExpenseForm_RecordsTheVendorPickedFromTheDirectory(t *testing.T) {
	// Arrange
	e := newEnv(t)
	if _, err := e.dir.CreateVendor(context.Background(), app.VendorInput{Kind: domain.VendorService, Name: "Basura"}); err != nil {
		t.Fatal(err)
	}
	model := press(e.shell(t), "g")
	model = press(model, tabs(3)...) // the vendor field
	model = press(model, "enter", "enter")

	// Act
	model = press(model, "tab", "10.00", "tab", "Recolección")
	model = press(model, "enter")

	// Assert
	mustContain(t, lineWith(rightOf(model.View()), "Recolección"), "Basura")
}

func TestNegativeBalanceWarning_ShowsTheResultingBalanceAndAnswerNoWritesNothing(t *testing.T) {
	// Arrange: cash holds 7,500.00
	e := newEnv(t)
	model := fillExpense(e.shell(t), "8,000.00", "Reparación grande")

	// Act
	model = press(model, "enter")

	// Assert: the warning names the account and the resulting balance, with No selected
	warning := model.View()
	mustContain(t, warning, "Efectivo", "−$500.00", "¿Continuar?", "[ No ]")

	// Act: answer No
	model = press(model, "enter")

	// Assert: back on the form, nothing written
	mustContain(t, model.View(), "Nuevo gasto", "Reparación grande")
	if n := len(e.movements(t)); n != 0 {
		t.Fatalf("%d movements written after answering No, want 0", n)
	}
}

func TestNegativeBalanceWarning_EscapeAnswersNoAndKeepsTheForm(t *testing.T) {
	// Arrange
	e := newEnv(t)
	model := fillExpense(e.shell(t), "8,000.00", "Reparación grande")
	model = press(model, "enter")

	// Act
	model = press(model, "esc")

	// Assert
	mustContain(t, model.View(), "Nuevo gasto")
	mustNotContain(t, model.View(), "¿Continuar?")
	if n := len(e.movements(t)); n != 0 {
		t.Fatalf("%d movements written after esc, want 0", n)
	}
}

func TestNegativeBalanceWarning_AnswerSiRecordsTheExpenseAndShowsTheNegativeBalance(t *testing.T) {
	// Arrange
	e := newEnv(t)
	model := fillExpense(e.shell(t), "8,000.00", "Reparación grande")
	model = press(model, "enter")

	// Act
	model = press(model, "right", "enter")

	// Assert
	screen := model.View()
	mustNotContain(t, screen, "Nuevo gasto", "¿Continuar?")
	mustContain(t, lineWith(rightOf(screen), "Reparación grande"), "−$8,000.00", "−$500.00")
	mustContain(t, lineWith(leftOf(screen), "Efectivo"), "−$500.00")
}

func TestMonthKeys_ChangeTheMonthShownAndLoadItsMovements(t *testing.T) {
	// Arrange: an expense on the first day of last month, none this month
	e := newEnv(t)
	last := domain.Of(today).Add(-1)
	_, err := e.ledger.RecordExpense(context.Background(), app.ExpenseInput{
		Date: domain.FormatInputDate(time.Date(last.Year, last.Month, 1, 0, 0, 0, 0, time.Local)), AccountID: e.cash.ID,
		CategoryID: e.category(t, "Insumos"), Amount: "100.00", Description: "Gasto del mes pasado",
	})
	if err != nil {
		t.Fatal(err)
	}
	model := e.shell(t)
	thisMonth := text.MonthYear(domain.Of(today).Year, domain.Of(today).Month)
	lastMonth := text.MonthYear(last.Year, last.Month)
	mustContain(t, model.View(), thisMonth)
	mustNotContain(t, model.View(), "Gasto del mes pasado")

	// Act
	model = press(model, "[")

	// Assert: last month shows its row, opening at the coto's opening and closing 100.00 lower
	screen := model.View()
	mustContain(t, screen, lastMonth, "Gasto del mes pasado", "Saldo inicial $7,500.00", "Saldo final $7,400.00")

	// Act
	model = press(model, "]")

	// Assert
	mustContain(t, model.View(), thisMonth, "Saldo inicial $7,400.00")
	mustNotContain(t, model.View(), "Gasto del mes pasado")
}

func TestLedger_StartsOnTheLastRowOfTheMonth(t *testing.T) {
	// Arrange
	e := newEnv(t)
	for _, desc := range []string{"Primero", "Segundo", "Tercero"} {
		_, err := e.ledger.RecordExpense(context.Background(), app.ExpenseInput{
			Date: domain.FormatInputDate(today), AccountID: e.cash.ID,
			CategoryID: e.category(t, "Insumos"), Amount: "1.00", Description: desc,
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	// Act
	screen := e.shell(t).View()

	// Assert
	if !strings.Contains(lineWith(screen, "Tercero"), "> ") || strings.Contains(lineWith(screen, "Primero"), "> ") {
		t.Fatalf("the cursor is not on the last row:\n%s", screen)
	}
}

func TestLedger_ListsItsKeysInTheHintBar(t *testing.T) {
	// Arrange
	e := newEnv(t)

	// Act
	screen := e.shell(t).View()

	// Assert
	mustContain(t, screen, "i ingreso", "g gasto", "[ ] mes")
}

func TestLedger_AnEmptyMonthShowsTheHintToRecordOne(t *testing.T) {
	// Arrange
	e := newEnv(t)

	// Act
	screen := e.shell(t).View()

	// Assert
	mustContain(t, screen, "No hay movimientos este mes")
}

func TestVendorDeletedInDirectorio_ShowsTheDeletedSuffixOnItsMovementsWithoutReopeningTheView(t *testing.T) {
	// Arrange: an expense paid to Basura, with Movimientos already loaded
	e := newEnv(t)
	basura, err := e.dir.CreateVendor(context.Background(), app.VendorInput{Kind: domain.VendorService, Name: "Basura"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.ledger.RecordExpense(context.Background(), app.ExpenseInput{
		Date: domain.FormatInputDate(today), AccountID: e.cash.ID, VendorID: &basura.ID,
		CategoryID: e.category(t, "Servicios"), Amount: "450.00", Description: "Recolección",
	}); err != nil {
		t.Fatal(err)
	}
	model := e.shell(t)
	mustNotContain(t, model.View(), "(eliminado)")

	// Act: delete the vendor in Directorio (Sí), then go back to Movimientos
	model = press(model, "7", "d", "right", "enter", "3")

	// Assert: the party cell is cut to its column, so the start of the suffix shows
	mustContain(t, lineWith(rightOf(model.View()), "Recolección"), "Basura (elimi")
}

// expenseToBasura records an expense of 450.00 paid to a new service vendor
// Basura and returns the vendor.
func (e *env) expenseToBasura(t *testing.T) app.Vendor {
	t.Helper()
	basura, err := e.dir.CreateVendor(context.Background(), app.VendorInput{Kind: domain.VendorService, Name: "Basura"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.ledger.RecordExpense(context.Background(), app.ExpenseInput{
		Date: domain.FormatInputDate(today), AccountID: e.cash.ID, VendorID: &basura.ID,
		CategoryID: e.category(t, "Servicios"), Amount: "450.00", Description: "Recolección",
	}); err != nil {
		t.Fatal(err)
	}
	return basura
}

func TestVendorRestoredInDirectorio_RemovesTheDeletedSuffixFromItsMovementsWithoutReopeningTheView(t *testing.T) {
	// Arrange: the vendor is already deleted when Movimientos loads
	e := newEnv(t)
	basura := e.expenseToBasura(t)
	if err := e.dir.DeleteVendor(context.Background(), basura.ID); err != nil {
		t.Fatal(err)
	}
	model := e.shell(t)
	mustContain(t, lineWith(rightOf(model.View()), "Recolección"), "Basura (elimi")

	// Act: show deleted vendors in Directorio, restore the selected one, go back
	model = press(model, "7", ".", "r", "3")

	// Assert
	row := lineWith(rightOf(model.View()), "Recolección")
	mustContain(t, row, "Basura")
	mustNotContain(t, row, "(elimi")
}

func TestVendorRenamedInDirectorio_ShowsTheNewNameOnItsMovementsWithoutReopeningTheView(t *testing.T) {
	// Arrange
	e := newEnv(t)
	e.expenseToBasura(t)
	model := e.shell(t)

	// Act: edit the selected vendor (kind, then name), replace the name, save, go back
	model = press(model, "7", "e", "tab", "ctrl+u", "Basura Plus", "enter", "3")

	// Assert
	mustContain(t, lineWith(rightOf(model.View()), "Recolección"), "Basura Plus")
}

func TestVendorCreatedInDirectorio_ReloadsMovimientosWithoutReopeningTheView(t *testing.T) {
	// Arrange: Movimientos loaded with one row; a vendor-less expense is added behind the view's back
	e := newEnv(t)
	e.expenseToBasura(t)
	model := e.shell(t)
	if _, err := e.ledger.RecordExpense(context.Background(), app.ExpenseInput{
		Date: domain.FormatInputDate(today), AccountID: e.cash.ID,
		CategoryID: e.category(t, "Insumos"), Amount: "10.00", Description: "Escobas",
	}); err != nil {
		t.Fatal(err)
	}
	mustNotContain(t, model.View(), "Escobas")

	// Act: create a vendor in Directorio (kind Persona, name), save, go back
	model = press(model, "7", "n", "tab", "Pedro", "enter", "3")

	// Assert: the view reloaded because the save sent the change message
	mustContain(t, model.View(), "Escobas")
}
