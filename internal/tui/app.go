// Package tui is the Bubble Tea user interface. The root model owns layout,
// focus, overlays, and the key-hint bar; each view lives in its own package.
package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/vicmangontel/munitorum/internal/app"
	directoryview "github.com/vicmangontel/munitorum/internal/tui/directory"
	settingsview "github.com/vicmangontel/munitorum/internal/tui/settings"
	"github.com/vicmangontel/munitorum/internal/tui/text"
	"github.com/vicmangontel/munitorum/internal/tui/theme"
	"github.com/vicmangontel/munitorum/internal/tui/view"
)

// The shell needs at least this much terminal.
const (
	minWidth  = 80
	minHeight = 24
)

// panel is a focusable area of the shell. The summary panel is not focusable.
type panel int

const (
	panelViews panel = iota
	panelMain
	panelDetail
	panelCount
)

// Model is the root model.
type Model struct {
	views         [len(text.ViewNames)]view.View
	selected      int
	focus         panel
	width, height int
	overlay       *overlay      // nil when no overlay is open
	settings      *app.Settings // nil until the coto exists
	svc           *app.SetupService
}

// New returns the root model with one placeholder view per entry of the view
// list. Phases 1 to 8 replace the placeholders with real views.
func New() Model {
	m := Model{}
	for i, name := range text.ViewNames {
		m.views[i] = view.NewPlaceholder(name)
	}
	return m
}

func (m Model) Init() tea.Cmd {
	cmds := make([]tea.Cmd, len(m.views))
	for i, v := range m.views {
		cmds[i] = v.Init()
	}
	return tea.Batch(cmds...)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m.broadcast(m.sizeMsg())
	case tea.KeyMsg:
		return m.updateKey(msg)
	case view.OpenOverlayMsg:
		return m.openOverlay(msg.Title, msg.Content), nil
	case view.CloseOverlayMsg:
		m.overlay = nil
		return m, nil
	case settingsview.SavedMsg:
		// The summary panel follows every Ajustes save, even one that finishes
		// after the overlay was closed; an open overlay shows it too.
		m.settings = &msg.Settings
		if m.overlay != nil && m.overlay.content != nil {
			return m, m.overlay.content.Update(msg)
		}
		return m, nil
	}
	// Results of commands belong to the views, which ignore what is not theirs,
	// and to the open overlay's content.
	return m.broadcast(msg)
}

// broadcast delivers a non-key message to every view and to the overlay content.
func (m Model) broadcast(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	for i := range m.views {
		var cmd tea.Cmd
		m.views[i], cmd = m.views[i].Update(msg)
		cmds = append(cmds, cmd)
	}
	if m.overlay != nil && m.overlay.content != nil {
		cmds = append(cmds, m.overlay.content.Update(msg))
	}
	return m, tea.Batch(cmds...)
}

// sizeMsg is the room the main panel's content has: the panel without its
// border and title line, as layout.go draws it.
func (m Model) sizeMsg() view.SizeMsg {
	frameW, frameH := theme.PanelFocused.GetFrameSize()
	return view.SizeMsg{
		Width:  max(m.width-leftWidth-frameW, 1),
		Height: max((m.height-1)*3/5-frameH-1, 1),
	}
}

func (m Model) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if key == "ctrl+c" {
		return m, tea.Quit
	}

	if m.overlay != nil {
		switch {
		case key == "esc" && handlesEscape(m.overlay.content):
			return m, m.overlay.content.Update(msg)
		case key == "esc", key == "?" && m.overlay.content == nil:
			m.overlay = nil
		case m.overlay.content != nil:
			return m, m.overlay.content.Update(msg)
		}
		return m, nil
	}

	if m.focus == panelMain {
		if c, ok := m.views[m.selected].(view.KeyCapturer); ok && c.CapturesKeys() {
			v, cmd := m.views[m.selected].Update(msg)
			m.views[m.selected] = v
			return m, cmd
		}
	}

	switch key {
	case "q":
		return m, tea.Quit
	case "?":
		m.overlay = m.helpOverlay()
		return m, nil
	case "f2":
		if m.settings != nil {
			m = m.openOverlay(text.SettingsTitle, settingsview.New(m.svc, *m.settings))
		}
		return m, nil
	case "tab":
		m.focus = (m.focus + 1) % panelCount
		return m, nil
	case "shift+tab":
		m.focus = (m.focus + panelCount - 1) % panelCount
		return m, nil
	case "1", "2", "3", "4", "5", "6", "7", "8":
		m.selected = int(key[0] - '1')
		m.focus = panelMain
		return m, nil
	}

	switch m.focus {
	case panelViews:
		switch key {
		case "up":
			m.selected = max(m.selected-1, 0)
		case "down":
			m.selected = min(m.selected+1, len(m.views)-1)
		case "enter":
			m.focus = panelMain
		}
	case panelMain:
		v, cmd := m.views[m.selected].Update(msg)
		m.views[m.selected] = v
		return m, cmd
	}
	return m, nil
}

// WithSettings returns the shell showing the coto: its name as the summary title
// and its accounts in the summary panel.
func (m Model) WithSettings(s app.Settings) Model {
	m.settings = &s
	return m
}

// WithService returns the shell calling svc for Ajustes.
func (m Model) WithService(svc *app.SetupService) Model {
	m.svc = svc
	return m
}

// openOverlay shows content in a centred overlay; esc closes it.
func (m Model) openOverlay(title string, content view.Content) Model {
	m.overlay = &overlay{title: title, content: content}
	return m
}

// handlesEscape reports whether overlay content wants esc itself.
func handlesEscape(c view.Content) bool {
	h, ok := c.(view.EscapeHandler)
	return ok && h.HandlesEscape()
}

// WithDirectory returns the shell showing the Directorio view over dir.
func (m Model) WithDirectory(dir *app.DirectoryService) Model {
	v := directoryview.New(dir)
	m.views[text.ViewDirectory] = v
	if m.width > 0 {
		m.views[text.ViewDirectory], _ = v.Update(m.sizeMsg())
	}
	return m
}
