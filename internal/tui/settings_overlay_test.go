package tui

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vicmangontel/munitorum/internal/app"
	"github.com/vicmangontel/munitorum/internal/store"
	settingsview "github.com/vicmangontel/munitorum/internal/tui/settings"
)

// drive delivers keys to model one by one, running the commands it returns and
// feeding their messages back, as the Bubble Tea runtime does.
func drive(model tea.Model, keys ...tea.KeyMsg) tea.Model {
	var deliver func(tea.Msg)
	deliver = func(msg tea.Msg) {
		var cmd tea.Cmd
		model, cmd = model.Update(msg)
		if cmd == nil {
			return
		}
		if next := cmd(); next != nil {
			deliver(next)
		}
	}
	for _, k := range keys {
		deliver(k)
	}
	return model
}

// typeKeys returns the keys that select the row, open the editor, replace its
// text with typed, and submit.
func typeKeys(downs int, typed string) []tea.KeyMsg {
	var keys []tea.KeyMsg
	for range downs {
		keys = append(keys, keyMsg("down"))
	}
	return append(keys, keyMsg("e"), tea.KeyMsg{Type: tea.KeyCtrlU}, keyMsg(typed), keyMsg("enter"))
}

func f2() tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyF2} }

func TestAjustes_F2OpensTheOverlayOnlyWhenTheCotoExists(t *testing.T) {
	cases := []struct {
		name     string
		m        Model
		wantOpen bool
	}{
		{"the coto exists", sized(100, 30).WithSettings(settings()), true},
		{"the coto does not exist yet", sized(100, 30), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange: the model is in the case

			// Act
			next, _ := c.m.Update(f2())

			// Assert
			out := next.View()
			opened := next.(Model).overlay != nil
			if opened != c.wantOpen {
				t.Fatalf("overlay open = %v, want %v:\n%s", opened, c.wantOpen, out)
			}
			if c.wantOpen && (!strings.Contains(out, "Ajustes") || !strings.Contains(out, "Nombre del coto")) {
				t.Fatalf("F2 did not show Ajustes:\n%s", out)
			}
		})
	}
}

func TestAjustes_F2IsListedInTheKeyHintBar(t *testing.T) {
	// Arrange
	m := sized(100, 30)

	// Act
	bar := m.View()

	// Assert
	if !strings.Contains(bar, "F2 ajustes") {
		t.Errorf("key-hint bar does not list F2:\n%s", bar)
	}
}

func TestAjustes_F2IsListedInTheHelpOverlay(t *testing.T) {
	// Arrange
	m := sized(100, 30)

	// Act
	next, _ := m.Update(keyMsg("?"))

	// Assert
	help := next.View()
	if !strings.Contains(help, "F2") || !strings.Contains(help, "abrir Ajustes") {
		t.Errorf("help overlay does not list F2:\n%s", help)
	}
}

func TestAjustes_SavedChangeReloadsTheSummaryPanel(t *testing.T) {
	// Arrange
	m := sized(100, 30).WithSettings(settings())
	changed := settings()
	changed.CommunityName = "Los Álamos"
	changed.Accounts[0].Name = "Caja"
	changed.Accounts[0].OpeningBalance = 800000

	// Act
	next, _ := m.Update(settingsview.SavedMsg{Settings: changed})

	// Assert
	out := next.View()
	for _, want := range []string{"Los Álamos", "Caja", "$8,000.00", "$9,200.50"} {
		if !strings.Contains(out, want) {
			t.Errorf("summary panel does not show %q:\n%s", want, out)
		}
	}
}

// newCoto returns the root of a real, migrated database holding a completed
// coto "Coto Los Pinos": cash "Efectivo" $7,500.00, bank "Banco (NU)" $0.00.
func newCoto(t *testing.T) tea.Model {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := store.Migrate(db, store.Migrations()); err != nil {
		t.Fatal(err)
	}
	svc := app.NewSetupService(store.NewSetupRepository(db), app.SystemClock{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	created, err := svc.CompleteSetup(context.Background(), app.SetupInput{
		CommunityName: "Coto Los Pinos",
		HouseCount:    "2",
		HousePrefix:   "CASA ",
		FeeAmount:     "150",
		Accounts:      [2]app.SetupAccount{{Name: "Efectivo", OpeningBalance: "7,500"}, {Name: "Banco (NU)", OpeningBalance: "0"}},
		OpeningDate:   "01/01/2020",
		Houses:        make([]app.SetupHouse, 2),
	})
	if err != nil {
		t.Fatal(err)
	}
	model, _ := NewShell(svc, created).Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	return model
}

func TestAjustes_SavedAccountRenameShowsInTheSummaryPanel(t *testing.T) {
	// Arrange
	model := newCoto(t)

	// Act
	model = drive(model, append([]tea.KeyMsg{f2()}, typeKeys(2, "Banco Azul")...)...) // row 3: the bank name

	// Assert
	out := model.View()
	if !strings.Contains(out, "Banco Azul") || strings.Contains(out, "Banco (NU)") {
		t.Fatalf("summary panel does not show the renamed account:\n%s", out)
	}
}

func TestAjustes_DecliningTheOpeningBalanceConfirmationLeavesTheSummaryPanelUnchanged(t *testing.T) {
	// Arrange
	model := newCoto(t)

	// Act
	model = drive(model, append([]tea.KeyMsg{f2()}, append(typeKeys(3, "8,000"), keyMsg("enter"))...)...) // row 4: cash balance; No

	// Assert
	out := model.View()
	if strings.Contains(out, "$8,000.00") || !strings.Contains(out, "$7,500.00") {
		t.Fatalf("summary panel changed after declining:\n%s", out)
	}
}

func TestAjustes_AcceptingTheOpeningBalanceConfirmationChangesTheSummaryPanel(t *testing.T) {
	// Arrange
	model := newCoto(t)

	// Act: accept, then close Ajustes so only the summary panel can show the balance
	model = drive(model, append([]tea.KeyMsg{f2()}, append(typeKeys(3, "8,000"), keyMsg("right"), keyMsg("enter"), keyMsg("esc"))...)...)

	// Assert
	out := model.View()
	if !strings.Contains(out, "$8,000.00") {
		t.Fatalf("summary panel does not show the new balance:\n%s", out)
	}
}

func TestAjustes_ASaveFinishingAfterAjustesWasClosedStillReloadsTheSummaryPanel(t *testing.T) {
	// Arrange: the write is started, then esc closes Ajustes before it returns
	model := drive(newCoto(t), append([]tea.KeyMsg{f2()}, append(typeKeys(3, "8,000"), keyMsg("right"))...)...)
	model, answer := model.Update(keyMsg("enter"))
	model, write := model.Update(answer())
	model, _ = model.Update(keyMsg("esc"))

	// Act
	model, _ = model.Update(write())

	// Assert
	out := model.View()
	if !strings.Contains(out, "$8,000.00") {
		t.Fatalf("summary panel does not show the saved balance:\n%s", out)
	}
}
