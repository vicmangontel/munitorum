package tui

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
	"github.com/vicmangontel/munitorum/internal/tui/view"
)

// settleAll delivers msgs to model, running every command that comes back (a
// batch in order) and feeding its message in, as the Bubble Tea runtime does.
func settleAll(model tea.Model, msgs ...tea.Msg) tea.Model {
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

// newLedgerCoto returns the shell of a real, migrated coto (cash 7,500.00, bank
// 0) wired to the ledger, with the ledger service and the cash account id.
func newLedgerCoto(t *testing.T) (tea.Model, *app.LedgerService, int64) {
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
	setup := app.NewSetupService(store.NewSetupRepository(db), testClock{}, log)
	repo := store.NewLedgerRepository(db)
	ledger := app.NewLedgerService(repo, testClock{}, log)
	cats := app.NewCategoryService(repo)
	dir := app.NewDirectoryService(store.NewVendorRepository(db), testClock{}, log)
	settings, err := setup.CompleteSetup(context.Background(), app.SetupInput{
		CommunityName: "Coto Los Pinos", HouseCount: "2", HousePrefix: "CASA ", FeeAmount: "150",
		Accounts:    [2]app.SetupAccount{{Name: "Efectivo", OpeningBalance: "7,500"}, {Name: "Banco (NU)", OpeningBalance: "0"}},
		OpeningDate: "01/01/2020", Houses: make([]app.SetupHouse, 2),
	})
	if err != nil {
		t.Fatal(err)
	}
	root := NewShell(setup, settings).WithDirectory(dir).WithLedger(ledger, cats, dir, testClock{})
	var model tea.Model = root
	model = settleAll(model, tea.WindowSizeMsg{Width: 100, Height: 30})
	model = settleAll(model, root.Init()())
	return model, ledger, settings.Accounts[0].ID
}

// recordExpense records an expense of amount from account today.
func recordExpense(t *testing.T, ledger *app.LedgerService, account int64, amount string) {
	t.Helper()
	// Category 4 is Insumos, the first expense category of the migration seed.
	_, err := ledger.RecordExpense(context.Background(), app.ExpenseInput{
		Date: domain.FormatInputDate(testToday), AccountID: account, CategoryID: 4, Amount: amount, Description: "Garrafones",
	})
	if err != nil {
		t.Fatalf("arrange: record expense: %v", err)
	}
}

func summaryOf(screen string) string {
	var left []string
	for _, line := range strings.Split(screen, "\n") {
		r := []rune(line)
		left = append(left, string(r[:min(leftWidth, len(r))]))
	}
	return strings.Join(left, "\n")
}

func TestSummaryPanel_ShowsTheDerivedBalancesOnceTheLedgerHasBeenRead(t *testing.T) {
	// Arrange
	model, _, _ := newLedgerCoto(t)

	// Act
	summary := summaryOf(model.View())

	// Assert
	for _, want := range []string{"Efectivo", "$7,500.00", "Banco (NU)", "$0.00", "Total"} {
		if !strings.Contains(summary, want) {
			t.Errorf("summary panel does not show %q:\n%s", want, summary)
		}
	}
}

func TestSummaryPanel_ReloadsTheBalancesWhenAMovementWasSaved(t *testing.T) {
	// Arrange
	model, ledger, cash := newLedgerCoto(t)
	recordExpense(t, ledger, cash, "178.50")

	// Act
	model = settleAll(model, view.ChangedMsg{})

	// Assert
	summary := summaryOf(model.View())
	if !strings.Contains(summary, "$7,321.50") {
		t.Fatalf("summary panel does not show the cash balance after the expense:\n%s", summary)
	}
}

func TestAjustes_ChangingAnOpeningBalanceMovesTheDerivedBalancesOfTheSummaryPanel(t *testing.T) {
	// Arrange: an expense of 178.50, then the cash opening balance is edited to 8,000.00
	model, ledger, cash := newLedgerCoto(t)
	recordExpense(t, ledger, cash, "178.50")
	model = settleAll(model, view.ChangedMsg{})

	// Act
	keys := append([]tea.KeyMsg{f2()}, typeKeys(3, "8,000")...) // row 4: the cash balance
	keys = append(keys, keyMsg("right"), keyMsg("enter"), keyMsg("esc"))
	for _, k := range keys {
		model = settleAll(model, k)
	}

	// Assert
	summary := summaryOf(model.View())
	if !strings.Contains(summary, "$7,821.50") {
		t.Fatalf("summary panel does not show 8,000.00 − 178.50:\n%s", summary)
	}
}

// testToday is the date these tests run on.
var testToday = time.Date(2026, time.November, 13, 9, 30, 0, 0, time.Local)

type testClock struct{}

func (testClock) Now() time.Time { return testToday }
