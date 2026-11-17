package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vicmangontel/munitorum/internal/tui/view"
)

// probe is a view that uses every optional part of the view contract.
type probe struct {
	detail    string
	capturing bool
	keys      []string
	sizes     []view.SizeMsg
}

func (p *probe) Init() tea.Cmd { return nil }
func (p *probe) Update(msg tea.Msg) (view.View, tea.Cmd) {
	switch m := msg.(type) {
	case tea.KeyMsg:
		p.keys = append(p.keys, m.String())
	case view.SizeMsg:
		p.sizes = append(p.sizes, m)
	}
	return p, nil
}
func (p *probe) View() string          { return "vista de prueba" }
func (p *probe) KeyHints() []view.Hint { return nil }
func (p *probe) Title() string         { return "Prueba" }
func (p *probe) Detail() string        { return p.detail }
func (p *probe) CapturesKeys() bool    { return p.capturing }

// nested is overlay content with an inner step that wants esc.
type nested struct {
	inner bool
	keys  []string
}

func (n *nested) Update(msg tea.Msg) tea.Cmd {
	if k, ok := msg.(tea.KeyMsg); ok {
		n.keys = append(n.keys, k.String())
		if k.String() == "esc" {
			n.inner = false
		}
	}
	return nil
}
func (n *nested) View() string          { return "contenido anidado" }
func (n *nested) KeyHints() []view.Hint { return nil }
func (n *nested) HandlesEscape() bool   { return n.inner }

func TestDetailer_FillsTheDetailPanelAndAnEmptyDetailShowsNoSelection(t *testing.T) {
	cases := []struct {
		name   string
		detail string
		want   string
		not    string
	}{
		{"detail text", "Nombre: Basura", "Nombre: Basura", "Sin selección"},
		{"empty detail", "", "Sin selección", "Nombre: Basura"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange
			m := sized(100, 30)
			m.views[0] = &probe{detail: c.detail}

			// Act
			out := m.View()

			// Assert
			if !strings.Contains(out, c.want) || strings.Contains(out, c.not) {
				t.Fatalf("detail panel should show %q and not %q:\n%s", c.want, c.not, out)
			}
		})
	}
}

func TestOpenOverlayMsg_ShowsTheContent(t *testing.T) {
	// Arrange
	m := sized(100, 30)

	// Act
	opened, _ := m.Update(view.OpenOverlayMsg{Title: "Nuevo", Content: &recorder{}})

	// Assert
	shown := opened.(Model).View()
	if !strings.Contains(shown, "Nuevo") || !strings.Contains(shown, "contenido interactivo") {
		t.Fatalf("overlay not shown:\n%s", shown)
	}
}

func TestCloseOverlayMsg_RemovesTheOverlay(t *testing.T) {
	// Arrange
	m := sized(100, 30).openOverlay("Nuevo", &recorder{})

	// Act
	closed, _ := m.Update(view.CloseOverlayMsg{})

	// Assert
	if closed.(Model).overlay != nil {
		t.Fatal("CloseOverlayMsg left the overlay open")
	}
}

func TestEscapeHandler_ReceivesEscWhileItHandlesIt(t *testing.T) {
	// Arrange
	content := &nested{inner: true}
	m := sized(100, 30).openOverlay("Anidado", content)

	// Act
	next, _ := m.Update(keyMsg("esc"))

	// Assert
	if next.(Model).overlay == nil || strings.Join(content.keys, ",") != "esc" {
		t.Fatalf("overlay open = %v, content keys = %v; want esc delivered and the overlay kept", next.(Model).overlay != nil, content.keys)
	}
}

func TestEscapeHandler_TheOverlayClosesOnEscOnceNoInnerStepRemains(t *testing.T) {
	// Arrange
	content := &nested{inner: false}
	m := sized(100, 30).openOverlay("Anidado", content)

	// Act
	next, _ := m.Update(keyMsg("esc"))

	// Assert
	if next.(Model).overlay != nil || len(content.keys) != 0 {
		t.Fatalf("overlay open = %v, content keys = %v; want it closed without delivering esc", next.(Model).overlay != nil, content.keys)
	}
}

func TestKeyCapturer_ReceivesEveryKeyWhileCapturingIncludingShellKeys(t *testing.T) {
	// Arrange
	p := &probe{capturing: true}
	m := sized(100, 30)
	m.views[0] = p
	m.focus = panelMain

	// Act
	var cmds []tea.Cmd
	for _, k := range []string{"q", "?", "1", "2", "tab"} {
		next, cmd := m.Update(keyMsg(k))
		m = next.(Model)
		cmds = append(cmds, cmd)
	}

	// Assert
	if got := strings.Join(p.keys, ","); got != "q,?,1,2,tab" {
		t.Fatalf("view received %q, want every key", got)
	}
	for _, cmd := range cmds {
		if cmd != nil && isQuit(cmd()) {
			t.Fatal("q quit while the view was capturing keys")
		}
	}
	if m.overlay != nil || m.selected != 0 || m.focus != panelMain {
		t.Fatalf("the shell acted on a captured key: overlay=%v selected=%d focus=%d", m.overlay, m.selected, m.focus)
	}
}

func TestKeyCapturer_CtrlCStillQuitsWhileCapturing(t *testing.T) {
	// Arrange
	m := sized(100, 30)
	m.views[0] = &probe{capturing: true}
	m.focus = panelMain

	// Act
	_, cmd := m.Update(keyMsg("ctrl+c"))

	// Assert
	if cmd == nil || !isQuit(cmd()) {
		t.Fatal("ctrl+c did not quit")
	}
}

func TestKeyCapturer_ShellKeysWorkAgainWhenTheViewStopsCapturing(t *testing.T) {
	// Arrange
	p := &probe{capturing: false}
	m := sized(100, 30)
	m.views[0] = p
	m.focus = panelMain

	// Act
	next, _ := m.Update(keyMsg("?"))

	// Assert
	if next.(Model).overlay == nil || len(p.keys) != 0 {
		t.Fatalf("? should open help and not reach the view; view keys = %v", p.keys)
	}
}

func TestSizeMsg_ReachesEveryViewWithTheRoomOfTheMainPanelsContent(t *testing.T) {
	cases := []struct {
		name         string
		w, h         int
		wantW, wantH int
	}{
		{"minimum terminal", 80, 24, 50, 10},
		{"large terminal", 120, 40, 90, 20},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange
			p := &probe{}
			m := New()
			m.views[3] = p

			// Act
			m.Update(tea.WindowSizeMsg{Width: c.w, Height: c.h})

			// Assert
			if len(p.sizes) != 1 || p.sizes[0] != (view.SizeMsg{Width: c.wantW, Height: c.wantH}) {
				t.Fatalf("sizes = %v, want one %dx%d", p.sizes, c.wantW, c.wantH)
			}
		})
	}
}

func TestNonKeyMessages_ReachEveryViewNotOnlyTheSelectedOne(t *testing.T) {
	// Arrange
	p := &probe{}
	m := sized(100, 30)
	m.views[5] = p // not the selected view

	// Act
	m.Update(view.SizeMsg{Width: 1, Height: 1})

	// Assert
	if len(p.sizes) != 1 {
		t.Fatalf("view received %d messages, want 1", len(p.sizes))
	}
}
