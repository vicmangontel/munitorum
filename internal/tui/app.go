// Package tui is the Bubble Tea user interface. The root model owns layout,
// focus, overlays, and the key-hint bar; each view lives in its own package.
package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/vicmangontel/munitorum/internal/tui/text"
)

// Model is the root model. Phase 0 replaces this placeholder with the shell.
type Model struct{}

// New returns the root model.
func New() Model { return Model{} }

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m Model) View() string {
	return text.AppName + "\n\n" + text.Skeleton + "\n\n" + text.QuitHint + "\n"
}
