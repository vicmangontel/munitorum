// Package tui is the Bubble Tea user interface. The root model owns layout,
// focus, overlays, and the key-hint bar; each view lives in its own package.
package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/vicmangontel/munitorum/internal/app"
	"github.com/vicmangontel/munitorum/internal/tui/text"
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
		return m, nil
	case tea.KeyMsg:
		return m.updateKey(msg)
	}
	return m, nil
}

func (m Model) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if key == "ctrl+c" {
		return m, tea.Quit
	}

	if m.overlay != nil {
		switch {
		case key == "esc", key == "?" && m.overlay.content == nil:
			m.overlay = nil
		case m.overlay.content != nil:
			return m, m.overlay.content.Update(msg)
		}
		return m, nil
	}

	switch key {
	case "q":
		return m, tea.Quit
	case "?":
		m.overlay = m.helpOverlay()
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

// openOverlay shows content in a centred overlay; esc closes it.
func (m Model) openOverlay(title string, content view.Content) Model {
	m.overlay = &overlay{title: title, content: content}
	return m
}
