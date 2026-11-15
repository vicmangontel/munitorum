package settings

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vicmangontel/munitorum/internal/app"
	"github.com/vicmangontel/munitorum/internal/domain"
	"github.com/vicmangontel/munitorum/internal/tui/confirm"
)

// fakeRepo is an in-memory SetupRepository holding a completed coto.
type fakeRepo struct{ s app.Settings }

func (r *fakeRepo) SetupCompleted(context.Context) (bool, error) { return true, nil }

func (r *fakeRepo) LoadSettings(context.Context) (app.Settings, error) {
	s := r.s
	s.Accounts = append([]app.Account(nil), r.s.Accounts...)
	return s, nil
}

func (r *fakeRepo) SaveSetup(context.Context, app.NewSetup) error { return nil }

func (r *fakeRepo) UpdateCommunityName(_ context.Context, name string, _ time.Time) error {
	r.s.CommunityName = name
	return nil
}

func (r *fakeRepo) RenameAccount(_ context.Context, id int64, name string, _ time.Time) error {
	r.s.Accounts[id-1].Name = name
	return nil
}

func (r *fakeRepo) UpdateOpeningBalance(_ context.Context, id int64, balance domain.Money, _ time.Time) error {
	r.s.Accounts[id-1].OpeningBalance = balance
	return nil
}

type fixedClock struct{}

func (fixedClock) Now() time.Time { return time.Date(2026, time.November, 13, 9, 0, 0, 0, time.Local) }

// newModel returns the overlay over a coto "Coto Los Pinos" with cash "Efectivo"
// ($7,500.00) and bank "Banco (NU)" ($1,200.50), and the repository behind it.
func newModel() (*Model, *fakeRepo) {
	repo := &fakeRepo{s: app.Settings{
		CommunityName: "Coto Los Pinos",
		Accounts: []app.Account{
			{ID: 1, Name: "Efectivo", Kind: app.KindCash, OpeningBalance: 750000},
			{ID: 2, Name: "Banco (NU)", Kind: app.KindBank, OpeningBalance: 120050},
		},
	}}
	svc := app.NewSetupService(repo, fixedClock{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	s, _ := repo.LoadSettings(context.Background())
	return New(svc, s), repo
}

func press(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	case "clear":
		return tea.KeyMsg{Type: tea.KeyCtrlU}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

// send delivers keys one by one, running the commands the model returns and
// feeding their messages back. It returns the SavedMsg messages the host would
// receive.
func send(m *Model, keys ...string) []SavedMsg {
	var saved []SavedMsg
	var deliver func(tea.Msg)
	deliver = func(msg tea.Msg) {
		cmd := m.Update(msg)
		if cmd == nil {
			return
		}
		switch next := cmd().(type) {
		case nil:
		case SavedMsg:
			saved = append(saved, next)
			deliver(next) // the host passes it back to the open overlay
		default:
			deliver(next)
		}
	}
	for _, k := range keys {
		deliver(press(k))
	}
	return saved
}

// edit selects row index and replaces its text with typed, then presses enter.
func edit(m *Model, index int, typed string) []SavedMsg {
	keys := []string{"e", "clear", typed, "enter"}
	for range index {
		keys = append([]string{"down"}, keys...)
	}
	return send(m, keys...)
}

// Row order: coto name, cash name, bank name, cash balance, bank balance.
const (
	rowCommunity = iota
	rowCashName
	rowBankName
	rowCashBalance
	rowBankBalance
)

func TestSettings_ListsTheCotoNameAccountNamesAndOpeningBalances(t *testing.T) {
	// Arrange
	m, _ := newModel()

	// Act
	out := m.View()

	// Assert
	for _, want := range []string{"Nombre del coto", "Coto Los Pinos", "Cuenta de efectivo", "Efectivo",
		"Cuenta de banco", "Banco (NU)", "Saldo inicial de Efectivo", "$7,500.00", "Saldo inicial de Banco (NU)", "$1,200.50"} {
		if !strings.Contains(out, want) {
			t.Errorf("view does not show %q:\n%s", want, out)
		}
	}
}

func TestSettings_SavedCotoNameShowsInTheListAndIsReportedToTheHost(t *testing.T) {
	// Arrange
	m, repo := newModel()

	// Act
	saved := edit(m, rowCommunity, "Los Álamos")

	// Assert
	if len(saved) != 1 || saved[0].Settings.CommunityName != "Los Álamos" {
		t.Fatalf("saved = %+v, want one SavedMsg with the new name", saved)
	}
	if repo.s.CommunityName != "Los Álamos" || !strings.Contains(m.View(), "Los Álamos") {
		t.Fatalf("stored name %q; view:\n%s", repo.s.CommunityName, m.View())
	}
}

func TestSettings_SavedAccountRenameShowsInTheListAndInTheBalanceLabel(t *testing.T) {
	// Arrange
	m, repo := newModel()

	// Act
	saved := edit(m, rowBankName, "Banco Azul")

	// Assert
	if len(saved) != 1 || saved[0].Settings.Accounts[1].Name != "Banco Azul" {
		t.Fatalf("saved = %+v, want one SavedMsg with the renamed account", saved)
	}
	out := m.View()
	if repo.s.Accounts[1].Name != "Banco Azul" || !strings.Contains(out, "Saldo inicial de Banco Azul") {
		t.Fatalf("stored name %q; view:\n%s", repo.s.Accounts[1].Name, out)
	}
}

func TestSettings_RenamingToTheOtherAccountsNameShowsTheErrorAndWritesNothing(t *testing.T) {
	// Arrange
	m, repo := newModel()

	// Act
	saved := edit(m, rowBankName, "EFECTIVO")

	// Assert
	out := m.View()
	if len(saved) != 0 || repo.s.Accounts[1].Name != "Banco (NU)" {
		t.Fatalf("saved = %+v, stored name %q; want nothing written", saved, repo.s.Accounts[1].Name)
	}
	if !strings.Contains(out, "Las dos cuentas deben tener nombres distintos.") {
		t.Fatalf("view does not show the duplicate-name error:\n%s", out)
	}
}

func TestSettings_BlankCotoNameShowsTheErrorAndKeepsTheFormOpen(t *testing.T) {
	// Arrange
	m, repo := newModel()

	// Act
	saved := edit(m, rowCommunity, "   ")

	// Assert
	out := m.View()
	if len(saved) != 0 || repo.s.CommunityName != "Coto Los Pinos" {
		t.Fatalf("saved = %+v, stored name %q; want nothing written", saved, repo.s.CommunityName)
	}
	if !strings.Contains(out, "El nombre es obligatorio y admite hasta 60 caracteres.") {
		t.Fatalf("view does not show the name error:\n%s", out)
	}
}

func TestSettings_InvalidOpeningBalanceShowsTheErrorBeforeAnyConfirmation(t *testing.T) {
	cases := []struct{ name, typed, want string }{
		{"negative", "-1", "El saldo no puede ser negativo."},
		{"unreadable", "mucho", "Escriba un importe válido, por ejemplo 1,234.50."},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange
			m, repo := newModel()

			// Act
			saved := edit(m, rowCashBalance, c.typed)

			// Assert
			out := m.View()
			if len(saved) != 0 || repo.s.Accounts[0].OpeningBalance != 750000 {
				t.Fatalf("saved = %+v, stored balance %d; want nothing written", saved, repo.s.Accounts[0].OpeningBalance)
			}
			if !strings.Contains(out, c.want) || strings.Contains(out, "¿Continuar?") {
				t.Fatalf("want the error and no confirmation:\n%s", out)
			}
		})
	}
}

func TestSettings_ValidOpeningBalanceAsksForConfirmationBeforeWriting(t *testing.T) {
	// Arrange
	m, repo := newModel()

	// Act
	saved := edit(m, rowCashBalance, "8,000")

	// Assert
	out := m.View()
	if len(saved) != 0 || repo.s.Accounts[0].OpeningBalance != 750000 {
		t.Fatalf("saved = %+v, stored balance %d; want nothing written yet", saved, repo.s.Accounts[0].OpeningBalance)
	}
	for _, want := range []string{"Efectivo", "$7,500.00", "8,000", "modifica todos los saldos", "[ No ]"} {
		if !strings.Contains(out, want) {
			t.Errorf("confirmation does not show %q:\n%s", want, out)
		}
	}
}

func TestSettings_DecliningTheOpeningBalanceConfirmationWritesNothing(t *testing.T) {
	// Arrange
	m, repo := newModel()
	edit(m, rowCashBalance, "8,000")

	// Act
	saved := send(m, "enter") // No is the default answer

	// Assert
	out := m.View()
	if len(saved) != 0 || repo.s.Accounts[0].OpeningBalance != 750000 {
		t.Fatalf("saved = %+v, stored balance %d; want nothing written", saved, repo.s.Accounts[0].OpeningBalance)
	}
	if !strings.Contains(out, "$7,500.00") || strings.Contains(out, "¿Continuar?") {
		t.Fatalf("want the list with the old balance:\n%s", out)
	}
}

func TestSettings_AcceptingTheOpeningBalanceConfirmationWritesAndReportsTheChange(t *testing.T) {
	// Arrange
	m, repo := newModel()
	edit(m, rowCashBalance, "8,000")

	// Act
	saved := send(m, "right", "enter") // move to Sí, accept

	// Assert
	if len(saved) != 1 || saved[0].Settings.Accounts[0].OpeningBalance != 800000 {
		t.Fatalf("saved = %+v, want one SavedMsg with $8,000.00", saved)
	}
	if repo.s.Accounts[0].OpeningBalance != 800000 || !strings.Contains(m.View(), "$8,000.00") {
		t.Fatalf("stored balance %d; view:\n%s", repo.s.Accounts[0].OpeningBalance, m.View())
	}
}

// writePending returns the overlay with a new cash balance of $8,000.00 accepted
// on the confirmation and its write started but not yet finished, and the
// command that performs the write.
func writePending() (*Model, tea.Cmd) {
	m, _ := newModel()
	edit(m, rowCashBalance, "8,000")
	m.Update(press("right"))
	answer := m.Update(press("enter"))
	write := m.Update(answer())
	return m, write
}

func TestSettings_ResultsThatArriveAfterTheSaveAreIgnored(t *testing.T) {
	cases := []struct {
		name string
		msg  tea.Msg
	}{
		{"a late validation without errors", validatedMsg{}},
		{"a late validation error", validatedMsg{err: app.ValidationError{Fields: map[string]string{app.FieldOpeningBalance(0): app.CodeBalanceNegative}}}},
		{"a late write failure", failedMsg{err: errors.New("disk full")}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange
			m, _ := newModel()
			edit(m, rowCashBalance, "8,000")
			send(m, "right", "enter")

			// Act
			cmd := m.Update(c.msg)

			// Assert
			out := m.View()
			if cmd != nil || !strings.Contains(out, "$8,000.00") || strings.Contains(out, "¿Continuar?") || strings.Contains(out, "negativo") || strings.Contains(out, "disk full") {
				t.Fatalf("cmd = %v; want the saved list untouched:\n%s", cmd, out)
			}
		})
	}
}

func TestSettings_KeysAreIgnoredWhileAWriteIsPending(t *testing.T) {
	// Arrange
	m, _ := writePending()

	// Act
	cmd := m.Update(press("enter"))

	// Assert
	if cmd != nil {
		t.Fatal("enter started a second operation while the write was pending")
	}
}

func TestSettings_AValidationResultWhileAWriteIsPendingDoesNotReopenTheConfirmation(t *testing.T) {
	// Arrange
	m, _ := writePending()

	// Act
	m.Update(validatedMsg{})

	// Assert
	if out := m.View(); strings.Contains(out, "¿Continuar?") {
		t.Fatalf("a stale validation reopened the confirmation:\n%s", out)
	}
}

func TestSettings_ASecondAnswerToTheConfirmationDoesNotWriteAgain(t *testing.T) {
	// Arrange
	m, _ := writePending()

	// Act
	cmd := m.Update(confirm.ResultMsg{Yes: true})

	// Assert
	if cmd != nil {
		t.Fatal("a second answer started a second write")
	}
}

func TestSettings_KeyHintsFollowTheState(t *testing.T) {
	cases := []struct {
		name string
		keys []string
		want string
	}{
		{"list", nil, "↑/↓ mover, e editar"},
		{"editing a value", []string{"e"}, "enter guardar"},
		{"confirming a balance", []string{"down", "down", "down", "e", "clear", "9", "enter"}, "←/→ elegir, enter aceptar"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange
			m, _ := newModel()
			send(m, c.keys...)

			// Act
			var parts []string
			for _, h := range m.KeyHints() {
				parts = append(parts, h.Key+" "+h.Desc)
			}

			// Assert
			if got := strings.Join(parts, ", "); got != c.want {
				t.Fatalf("hints = %q, want %q", got, c.want)
			}
		})
	}
}
