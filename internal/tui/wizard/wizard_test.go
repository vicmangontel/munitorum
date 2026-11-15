package wizard

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
)

// fakeRepo is an in-memory SetupRepository.
type fakeRepo struct {
	saved   *app.NewSetup
	saveErr error
}

func (r *fakeRepo) SetupCompleted(context.Context) (bool, error) { return r.saved != nil, nil }

func (r *fakeRepo) LoadSettings(context.Context) (app.Settings, error) {
	if r.saved == nil {
		return app.Settings{}, app.ErrNotFound
	}
	s := r.saved
	st := app.Settings{CommunityName: s.CommunityName, HousePrefix: s.HousePrefix, HouseCount: s.HouseCount, OpeningDate: s.OpeningDate}
	for i, a := range s.Accounts {
		st.Accounts = append(st.Accounts, app.Account{ID: int64(i + 1), Name: a.Name, Kind: a.Kind, OpeningBalance: a.OpeningBalance, SortOrder: i})
	}
	return st, nil
}

func (r *fakeRepo) SaveSetup(_ context.Context, s app.NewSetup) error {
	if r.saveErr != nil {
		return r.saveErr
	}
	r.saved = &s
	return nil
}

// The wizard never edits settings; these satisfy app.SetupRepository.
func (r *fakeRepo) UpdateCommunityName(context.Context, string, time.Time) error  { return nil }
func (r *fakeRepo) RenameAccount(context.Context, int64, string, time.Time) error { return nil }
func (r *fakeRepo) UpdateOpeningBalance(context.Context, int64, domain.Money, time.Time) error {
	return nil
}

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

var today = time.Date(2026, time.November, 13, 9, 0, 0, 0, time.Local)

func newWizard(t *testing.T) (*Model, *fakeRepo) {
	t.Helper()
	repo := &fakeRepo{}
	svc := app.NewSetupService(repo, fixedClock{today}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	m := New(svc, today)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	return m, repo
}

func keyMsg(s string) tea.KeyMsg {
	switch s {
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "shift+tab":
		return tea.KeyMsg{Type: tea.KeyShiftTab}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "clear":
		return tea.KeyMsg{Type: tea.KeyCtrlU}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

// send delivers keys one by one, running the commands the wizard returns and
// feeding their messages back. It returns the messages the wizard cannot
// handle itself: DoneMsg and the quit message.
func send(m *Model, keys ...string) []tea.Msg {
	var out []tea.Msg
	var deliver func(tea.Msg)
	deliver = func(msg tea.Msg) {
		_, cmd := m.Update(msg)
		if cmd == nil {
			return
		}
		switch next := cmd().(type) {
		case nil:
		case DoneMsg, tea.QuitMsg:
			out = append(out, next)
		default:
			deliver(next)
		}
	}
	for _, k := range keys {
		deliver(keyMsg(k))
	}
	return out
}

// typeText replaces the focused field's text.
func typeText(m *Model, s string) { send(m, "clear", s) }

func atStep(t *testing.T, m *Model, want int) {
	t.Helper()
	if m.step != want {
		t.Fatalf("step = %d, want %d\n%s", m.step, want, m.View())
	}
}

// toStep6 answers steps 1 to 5 validly and lands on the house rows.
func toStep6(t *testing.T, m *Model) {
	t.Helper()
	typeText(m, "Coto Los Pinos")
	send(m, "enter") // 1 -> 2, defaults: 18 houses, CASA
	send(m, "enter") // 2 -> 3
	typeText(m, "150.00")
	send(m, "enter") // 3 -> 4, defaults for both accounts
	send(m, "enter") // 4 -> 5
	send(m, "tab")
	typeText(m, "7,500.00")
	send(m, "enter") // 5 -> 6
	atStep(t, m, stepHouseRows)
}

func TestWizard_StartsOnStepOneWithTheDefaults(t *testing.T) {
	// Arrange / Act
	m, _ := newWizard(t)
	view := m.View()

	// Assert
	atStep(t, m, stepCommunity)
	if !strings.Contains(view, "paso 1 de 7") || !strings.Contains(view, "Coto") {
		t.Fatalf("view does not show step 1 of 7:\n%s", view)
	}
	send(m, "enter")
	atStep(t, m, stepCommunity) // empty name blocks
	typeText(m, "X")
	send(m, "enter")
	for _, want := range []string{"18", "CASA ", "CASA 01 … CASA 18"} {
		if !strings.Contains(m.View(), want) {
			t.Errorf("step 2 does not show %q:\n%s", want, m.View())
		}
	}
	send(m, "enter", "enter")
	send(m, "enter") // cuota empty: blocked
	atStep(t, m, stepFee)
	typeText(m, "1")
	send(m, "enter")
	for _, want := range []string{"Efectivo", "Banco (NU)"} {
		if !strings.Contains(m.View(), want) {
			t.Errorf("step 4 does not show %q:\n%s", want, m.View())
		}
	}
	send(m, "enter")
	if !strings.Contains(m.View(), "13/11/2026") {
		t.Errorf("step 5 does not prefill today:\n%s", m.View())
	}
}

func TestWizard_ShowsEveryInvalidFieldOfTheStepAtOnce(t *testing.T) {
	// Arrange
	m, _ := newWizard(t)
	typeText(m, "X")
	send(m, "enter")
	typeText(m, "0")
	send(m, "tab")
	typeText(m, "ABCDEFGHIJK")

	// Act
	send(m, "enter")

	// Assert
	atStep(t, m, stepHouses)
	view := m.View()
	for _, want := range []string{"Escriba un número entero de 1 a 200.", "El prefijo admite hasta 10 caracteres."} {
		if !strings.Contains(view, want) {
			t.Errorf("view does not show %q:\n%s", want, view)
		}
	}
}

func TestWizard_PutsTheCursorOnTheFirstInvalidField(t *testing.T) {
	// Arrange
	m, _ := newWizard(t)
	typeText(m, "X")
	send(m, "enter")
	send(m, "tab") // focus the prefix
	typeText(m, "ABCDEFGHIJK")
	send(m, "tab") // back to the count
	typeText(m, "0")

	// Act: both fields invalid; focus must return to the first
	send(m, "tab", "enter")

	// Assert
	if m.houses.OnFirstField() == false {
		t.Fatalf("focus is not on the first invalid field")
	}
}

func TestWizard_BackKeepsEnteredValues(t *testing.T) {
	// Arrange
	m, _ := newWizard(t)
	typeText(m, "Coto Los Pinos")
	send(m, "enter")
	typeText(m, "24")

	// Act
	send(m, "shift+tab") // first field: back
	atStep(t, m, stepCommunity)
	send(m, "enter")

	// Assert
	atStep(t, m, stepHouses)
	if got := m.houses.Value(app.FieldHouseCount); got != "24" {
		t.Fatalf("count = %q, want 24", got)
	}
	if got := m.community.Value(app.FieldCommunityName); got != "Coto Los Pinos" {
		t.Fatalf("name = %q", got)
	}
}

func TestWizard_LeftKeyMovesTheCursorUnlessItIsBeforeTheFirstCharacter(t *testing.T) {
	// Arrange
	m, _ := newWizard(t)
	typeText(m, "X")
	send(m, "enter") // step 2, cursor after "18"

	// Act
	send(m, "left")
	atStepAfterFirstLeft := m.step
	send(m, "left", "left")

	// Assert
	if atStepAfterFirstLeft != stepHouses {
		t.Fatalf("← with the cursor inside the field left the step")
	}
	atStep(t, m, stepCommunity)
}

func TestWizard_EscThenYesQuitsWritingNothing(t *testing.T) {
	// Arrange
	m, repo := newWizard(t)
	toStep6(t, m)

	// Act
	out := send(m, "esc", "left", "enter")

	// Assert
	if len(out) != 1 {
		t.Fatalf("messages = %v, want one quit", out)
	}
	if _, quit := out[0].(tea.QuitMsg); !quit {
		t.Fatalf("message = %T, want tea.QuitMsg", out[0])
	}
	if repo.saved != nil {
		t.Fatal("the wizard wrote a coto")
	}
}

func TestWizard_EscThenEnterKeepsNoAsTheDefaultAndStays(t *testing.T) {
	// Arrange
	m, _ := newWizard(t)
	typeText(m, "X")

	// Act
	out := send(m, "esc")
	prompt := m.View()
	out = append(out, send(m, "enter")...)

	// Assert
	if !strings.Contains(prompt, "¿Cancelar la configuración?") {
		t.Fatalf("no confirmation shown:\n%s", prompt)
	}
	if len(out) != 0 || m.cancel != nil {
		t.Fatalf("messages = %v, cancel open = %v; want the wizard to stay", out, m.cancel != nil)
	}
	if m.community.Value(app.FieldCommunityName) != "X" {
		t.Fatal("entered value lost")
	}
}

func TestWizard_StepSixRowErrorsKeepTheWizardOnTheRow(t *testing.T) {
	// Arrange
	m, _ := newWizard(t)
	toStep6(t, m)
	send(m, "down", "down") // Casa 03
	send(m, "12/2026")      // covered through
	send(m, "tab", "tab")
	send(m, "150")

	// Act
	send(m, "enter")

	// Assert
	atStep(t, m, stepHouseRows)
	view := m.View()
	if !strings.Contains(view, "Una casa con adeudo no puede tener meses cubiertos.") {
		t.Fatalf("row error not shown:\n%s", view)
	}
	if m.rows.row != 2 || m.rows.col != colArrears {
		t.Fatalf("cursor on row %d col %d, want row 2 on the arrears cell", m.rows.row, m.rows.col)
	}
}

func TestWizard_ChangingTheHouseCountKeepsRowsOfHousesThatStillExist(t *testing.T) {
	// Arrange
	m, _ := newWizard(t)
	toStep6(t, m)
	send(m, "down", "down", "tab", "tab")
	send(m, "300") // Casa 03 arrears
	send(m, "shift+tab", "shift+tab", "up", "up")
	send(m, "shift+tab") // first cell of first row: back to step 5
	atStep(t, m, stepOpening)
	for i := 0; i < stepCount && m.step != stepHouses; i++ {
		send(m, "shift+tab")
	}
	atStep(t, m, stepHouses)
	typeText(m, "20")

	// Act
	for i := 0; i < stepCount && m.step != stepHouseRows; i++ {
		send(m, "enter")
	}

	// Assert
	atStep(t, m, stepHouseRows)
	if got := m.rows.vals[2][colArrears]; got != "300" {
		t.Fatalf("Casa 03 arrears = %q, want 300", got)
	}
	if len(m.rows.vals) != 20 {
		t.Fatalf("%d rows, want 20", len(m.rows.vals))
	}
}

func TestWizard_SummaryShowsTotalsAndConfirmWritesTheCoto(t *testing.T) {
	// Arrange
	m, repo := newWizard(t)
	toStep6(t, m)
	send(m, "12/2026", "tab", "50") // Casa 01 covered, credit
	send(m, "down", "tab", "100")   // Casa 02 arrears
	send(m, "enter")
	atStep(t, m, stepSummary)
	summary := m.View()

	// Act
	out := send(m, "enter")

	// Assert
	for _, want := range []string{"18", "$150.00", "$7,500.00", "$0.00", "$50.00", "$100.00", "Casas con meses cubiertos"} {
		if !strings.Contains(summary, want) {
			t.Errorf("summary does not show %q:\n%s", want, summary)
		}
	}
	if len(out) != 1 {
		t.Fatalf("messages = %v, want one DoneMsg", out)
	}
	done, ok := out[0].(DoneMsg)
	if !ok || done.Settings.CommunityName != "Coto Los Pinos" || len(done.Settings.Accounts) != 2 {
		t.Fatalf("message = %#v, want DoneMsg with the coto", out[0])
	}
	if repo.saved == nil || len(repo.saved.Houses) != 18 {
		t.Fatalf("saved = %+v, want 18 houses", repo.saved)
	}
}

func TestWizard_SummaryBackReturnsToTheRows(t *testing.T) {
	// Arrange
	m, _ := newWizard(t)
	toStep6(t, m)
	send(m, "enter")
	atStep(t, m, stepSummary)

	// Act
	send(m, "left")

	// Assert
	atStep(t, m, stepHouseRows)
}

func TestWizard_ASaveFailureIsShownOnTheSummary(t *testing.T) {
	// Arrange
	m, repo := newWizard(t)
	toStep6(t, m)
	send(m, "enter")
	repo.saveErr = errors.New("disk full")

	// Act
	out := send(m, "enter")

	// Assert
	atStep(t, m, stepSummary)
	if len(out) != 0 || !strings.Contains(m.View(), "disk full") {
		t.Fatalf("messages = %v; view:\n%s", out, m.View())
	}
}
