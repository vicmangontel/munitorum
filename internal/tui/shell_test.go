package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vicmangontel/munitorum/internal/tui/view"
)

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
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

// sized returns a shell that has already received a terminal size.
func sized(w, h int) Model {
	next, _ := New().Update(tea.WindowSizeMsg{Width: w, Height: h})
	return next.(Model)
}

func TestTab_CyclesFocusThroughViewsMainAndDetail(t *testing.T) {
	cases := []struct {
		name string
		keys []string
		want panel
	}{
		{"starts on the view list", nil, panelViews},
		{"tab moves to main", []string{"tab"}, panelMain},
		{"tab twice moves to detail", []string{"tab", "tab"}, panelDetail},
		{"tab three times wraps to the list", []string{"tab", "tab", "tab"}, panelViews},
		{"shift+tab from the list wraps to detail", []string{"shift+tab"}, panelDetail},
		{"shift+tab twice reaches main", []string{"shift+tab", "shift+tab"}, panelMain},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange
			m := sized(80, 24)
			for _, k := range c.keys[:max(len(c.keys)-1, 0)] {
				next, _ := m.Update(keyMsg(k))
				m = next.(Model)
			}

			// Act
			if len(c.keys) > 0 {
				next, _ := m.Update(keyMsg(c.keys[len(c.keys)-1]))
				m = next.(Model)
			}

			// Assert
			if m.focus != c.want {
				t.Fatalf("focus = %d, want %d", m.focus, c.want)
			}
		})
	}
}

func TestNumberKeys_JumpToViewAndFocusMain(t *testing.T) {
	names := []string{"Resumen", "Casas", "Movimientos", "Cooperaciones", "Obras", "Recordatorios", "Directorio", "Pendientes"}
	for i, name := range names {
		t.Run(name, func(t *testing.T) {
			// Arrange
			m := sized(80, 24)

			// Act
			next, _ := m.Update(keyMsg(string(rune('1' + i))))
			m = next.(Model)

			// Assert
			if m.selected != i || m.focus != panelMain {
				t.Fatalf("selected = %d, focus = %d; want %d, main", m.selected, m.focus, i)
			}
			if !strings.Contains(m.View(), "> "+string(rune('1'+i))+" "+name) {
				t.Fatalf("view list does not mark %q as selected:\n%s", name, m.View())
			}
		})
	}
}

func TestArrowKeys_MoveSelectionInTheViewListWithoutWrapping(t *testing.T) {
	cases := []struct {
		name  string
		start int
		key   string
		want  int
	}{
		{"down moves forward", 0, "down", 1},
		{"up moves back", 3, "up", 2},
		{"up stops at the first view", 0, "up", 0},
		{"down stops at the last view", 7, "down", 7},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange
			m := sized(80, 24)
			m.selected = c.start

			// Act
			next, _ := m.Update(keyMsg(c.key))
			m = next.(Model)

			// Assert
			if m.selected != c.want {
				t.Fatalf("selected = %d, want %d", m.selected, c.want)
			}
		})
	}
}

func TestEnter_OnTheViewListFocusesMain(t *testing.T) {
	// Arrange
	m := sized(80, 24)

	// Act
	next, _ := m.Update(keyMsg("enter"))
	m = next.(Model)

	// Assert
	if m.focus != panelMain {
		t.Fatalf("focus = %d, want main", m.focus)
	}
}

func TestHelp_QuestionMarkClosesAnOpenOverlay(t *testing.T) {
	// Arrange
	next, _ := sized(80, 24).Update(keyMsg("?"))
	opened := next.(Model)

	// Act
	next, _ = opened.Update(keyMsg("?"))
	closed := next.(Model)

	// Assert
	if closed.overlay != nil {
		t.Fatal("overlay still open")
	}
}

func TestHelp_NamesTheViewInTheMainPanel(t *testing.T) {
	// Arrange
	next, _ := sized(80, 24).Update(keyMsg("3"))
	m := next.(Model)

	// Act
	next, _ = m.Update(keyMsg("?"))
	m = next.(Model)

	// Assert
	if !strings.Contains(m.View(), "Teclas de Movimientos") {
		t.Fatalf("help does not name the focused view:\n%s", m.View())
	}
}

func TestOverlay_SwallowsOtherKeysAndOnlyCtrlCQuits(t *testing.T) {
	// Arrange
	next, _ := sized(80, 24).Update(keyMsg("?"))
	opened := next.(Model)

	cases := []struct {
		key      string
		wantQuit bool
	}{
		{"3", false},
		{"tab", false},
		{"q", false},
		{"ctrl+c", true},
	}
	for _, c := range cases {
		t.Run(c.key, func(t *testing.T) {
			// Act
			next, cmd := opened.Update(keyMsg(c.key))
			got := next.(Model)

			// Assert
			quit := cmd != nil && isQuit(cmd())
			if quit != c.wantQuit {
				t.Fatalf("quit = %v, want %v", quit, c.wantQuit)
			}
			if !c.wantQuit && (got.overlay == nil || got.selected != 0 || got.focus != panelViews) {
				t.Fatalf("overlay state changed by %q: overlay=%v selected=%d focus=%d", c.key, got.overlay != nil, got.selected, got.focus)
			}
		})
	}
}

func isQuit(msg tea.Msg) bool {
	_, ok := msg.(tea.QuitMsg)
	return ok
}

func TestOverlay_IsDrawnOverTheShellNotInsteadOfIt(t *testing.T) {
	// Arrange
	next, _ := sized(100, 30).Update(keyMsg("?"))
	m := next.(Model)

	// Act
	out := m.View()

	// Assert
	lines := strings.Split(out, "\n")
	if len(lines) != 30 {
		t.Fatalf("screen has %d lines, want 30", len(lines))
	}
	for i, l := range lines {
		if w := lipgloss.Width(l); w > 100 {
			t.Fatalf("line %d is %d cells wide, want at most 100", i, w)
		}
	}
	for _, want := range []string{"Ayuda", "Vistas", "Efectivo", "salir"} {
		if !strings.Contains(out, want) {
			t.Errorf("shell around the overlay lost %q:\n%s", want, out)
		}
	}
}

func TestView_ShowsTooSmallMessageBelowMinimumSize(t *testing.T) {
	cases := []struct {
		name string
		w, h int
		want bool
	}{
		{"narrower than 80", 79, 24, true},
		{"shorter than 24", 80, 23, true},
		{"exactly the minimum", 80, 24, false},
		{"larger", 120, 40, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange
			m := sized(c.w, c.h)

			// Act
			out := m.View()

			// Assert
			got := strings.Contains(out, "Terminal muy pequeña")
			if got != c.want {
				t.Fatalf("too-small message shown = %v, want %v:\n%s", got, c.want, out)
			}
			if !c.want && !strings.Contains(out, "Vistas") {
				t.Fatalf("shell not rendered:\n%s", out)
			}
		})
	}
}

func TestView_ReturnsToShellWhenTheTerminalGrowsBack(t *testing.T) {
	// Arrange
	next, _ := sized(60, 20).Update(tea.WindowSizeMsg{Width: 90, Height: 30})
	m := next.(Model)

	// Act
	out := m.View()

	// Assert
	if strings.Contains(out, "Terminal muy pequeña") || !strings.Contains(out, "Vistas") {
		t.Fatalf("shell did not return:\n%s", out)
	}
}

func TestView_FillsTheTerminalExactly(t *testing.T) {
	cases := []struct{ w, h int }{{80, 24}, {100, 30}, {120, 40}, {200, 50}}
	for _, c := range cases {
		t.Run(fmt.Sprintf("%dx%d", c.w, c.h), func(t *testing.T) {
			// Arrange
			m := sized(c.w, c.h)

			// Act
			lines := strings.Split(m.View(), "\n")

			// Assert
			if len(lines) != c.h {
				t.Fatalf("%d lines, want %d", len(lines), c.h)
			}
			for i, l := range lines[:len(lines)-1] {
				if w := lipgloss.Width(l); w != c.w {
					t.Fatalf("line %d is %d cells wide, want %d", i, w, c.w)
				}
			}
		})
	}
}

func TestView_ShowsViewListSummaryAndDetailPlaceholders(t *testing.T) {
	// Arrange
	m := sized(80, 24)

	// Act
	out := m.View()

	// Assert
	for _, want := range []string{
		"Resumen", "Casas", "Movimientos", "Cooperaciones", "Obras", "Recordatorios", "Directorio", "Pendientes",
		"Coto", "Efectivo", "Banco", "Total", "Disponible", "$0.00", "Detalle", "Sin selección",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("shell missing %q:\n%s", want, out)
		}
	}
}

func TestHintBar_ShowsKeysOfTheFocusedPanel(t *testing.T) {
	cases := []struct {
		name string
		keys []string
		want string
		not  string
	}{
		{"list focus shows navigation", nil, "↑/↓ mover", ""},
		{"main focus drops list navigation", []string{"tab"}, "tab panel", "↑/↓ mover"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange
			m := sized(80, 24)
			for _, k := range c.keys {
				next, _ := m.Update(keyMsg(k))
				m = next.(Model)
			}

			// Act
			bar := lastLine(m.View())

			// Assert
			if !strings.Contains(bar, c.want) || !strings.Contains(bar, "? ayuda") || !strings.Contains(bar, "q salir") {
				t.Fatalf("hint bar = %q, want it to contain %q, '? ayuda' and 'q salir'", bar, c.want)
			}
			if c.not != "" && strings.Contains(bar, c.not) {
				t.Fatalf("hint bar = %q, must not contain %q", bar, c.not)
			}
		})
	}
}

func lastLine(s string) string {
	lines := strings.Split(s, "\n")
	return lines[len(lines)-1]
}

// echoView is a real view that shows the last key it received, standing in for
// the views later phases build.
type echoView struct{ last string }

func (e echoView) Init() tea.Cmd { return nil }
func (e echoView) Update(msg tea.Msg) (view.View, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		e.last = k.String()
	}
	return e, nil
}
func (e echoView) View() string { return "got:" + e.last }
func (e echoView) KeyHints() []view.Hint {
	return []view.Hint{{Key: "x", Desc: "equis"}}
}
func (e echoView) Title() string { return "Eco" }

func TestMainFocus_DeliversKeysToTheViewAndListFocusDoesNot(t *testing.T) {
	cases := []struct {
		name      string
		focusMain bool
		want      string
		not       string
	}{
		{"main focus delivers", true, "got:x", ""},
		{"list focus keeps keys", false, "got:", "got:x"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange
			m := sized(80, 24)
			m.views[0] = echoView{}
			if c.focusMain {
				m.focus = panelMain
			}

			// Act
			next, _ := m.Update(keyMsg("x"))
			out := next.(Model).View()

			// Assert
			if !strings.Contains(out, c.want) || (c.not != "" && strings.Contains(out, c.not)) {
				t.Fatalf("main panel should show %q and not %q:\n%s", c.want, c.not, out)
			}
		})
	}
}

func TestLongViewContent_IsClippedToThePanel(t *testing.T) {
	// Arrange
	m := sized(80, 24)
	m.views[0] = tallView{}

	// Act
	lines := strings.Split(m.View(), "\n")

	// Assert
	if len(lines) != 24 {
		t.Fatalf("%d lines, want 24", len(lines))
	}
	for i, l := range lines[:23] {
		if w := lipgloss.Width(l); w != 80 {
			t.Fatalf("line %d is %d cells wide, want 80", i, w)
		}
	}
}

type tallView struct{ echoView }

func (tallView) View() string {
	return strings.Repeat("fila muy larga "+strings.Repeat("x", 200)+"\n", 100)
}
func (t tallView) Update(msg tea.Msg) (view.View, tea.Cmd) { return t, nil }

func TestView_IsEmptyBeforeTheFirstSizeMessage(t *testing.T) {
	// Arrange
	m := New()

	// Act
	out := m.View()

	// Assert
	if out != "" {
		t.Fatalf("view before a size is known = %q, want empty", out)
	}
}

func TestHelp_ListsGlobalKeysAndTheViewsKeys(t *testing.T) {
	// Arrange
	m := sized(80, 24)

	// Act
	next, _ := m.Update(keyMsg("?"))
	out := next.(Model).View()

	// Assert
	for _, want := range []string{"Ayuda", "Teclas generales", "siguiente panel", "Teclas de Resumen", "no tiene teclas propias"} {
		if !strings.Contains(out, want) {
			t.Errorf("help overlay missing %q:\n%s", want, out)
		}
	}
}

func TestHelp_EscClosesTheOverlay(t *testing.T) {
	// Arrange
	m := sized(80, 24)
	m.overlay = m.helpOverlay()

	// Act
	next, _ := m.Update(keyMsg("esc"))
	closed := next.(Model)

	// Assert
	if closed.overlay != nil || strings.Contains(closed.View(), "Teclas generales") {
		t.Errorf("esc did not close the overlay:\n%s", closed.View())
	}
}

func TestHintBar_ShowsTheFocusedViewsKeys(t *testing.T) {
	// Arrange
	m := sized(80, 24)
	m.views[0] = echoView{}
	m.focus = panelMain

	// Act
	bar := lastLine(m.View())

	// Assert
	if !strings.Contains(bar, "x equis") {
		t.Errorf("hint bar lacks the view's key: %q", bar)
	}
}

func TestHelp_ListsTheFocusedViewsKeys(t *testing.T) {
	// Arrange
	m := sized(80, 24)
	m.views[0] = echoView{}
	m.focus = panelMain

	// Act
	next, _ := m.Update(keyMsg("?"))
	help := next.(Model).View()

	// Assert
	if !strings.Contains(help, "Teclas de Eco") || !strings.Contains(help, "equis") {
		t.Errorf("help lacks the view's keys:\n%s", help)
	}
}

func TestHintBar_ShowsOnlyCloseWhileAnOverlayIsOpen(t *testing.T) {
	// Arrange
	m := sized(80, 24)
	m.views[0] = echoView{}
	m.focus = panelMain
	m.overlay = m.helpOverlay()

	// Act
	bar := strings.TrimSpace(lastLine(m.View()))

	// Assert
	if bar != "esc cerrar" {
		t.Fatalf("hint bar = %q, want only %q (the overlay swallows every other key)", bar, "esc cerrar")
	}
}
