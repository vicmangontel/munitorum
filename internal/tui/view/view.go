// Package view defines what the root model needs from each view. Views live in
// their own packages from the phase that builds them; until then the root uses
// Placeholder.
package view

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/vicmangontel/munitorum/internal/tui/text"
)

// Hint is one entry of the key-hint bar and of the help overlay.
type Hint struct{ Key, Desc string }

// View is one entry of the view list, shown in the main panel.
type View interface {
	Init() tea.Cmd
	Update(msg tea.Msg) (View, tea.Cmd)
	// View renders the main panel content. The root clips it to the panel.
	View() string
	// KeyHints lists the keys valid in the main panel, for the hint bar and help.
	KeyHints() []Hint
	Title() string
}

// Placeholder is a view with no behaviour yet.
type Placeholder struct{ title string }

// NewPlaceholder returns a placeholder view with the given title.
func NewPlaceholder(title string) Placeholder { return Placeholder{title: title} }

func (p Placeholder) Init() tea.Cmd                  { return nil }
func (p Placeholder) Update(tea.Msg) (View, tea.Cmd) { return p, nil }
func (p Placeholder) View() string                   { return text.PlaceholderBody }
func (p Placeholder) KeyHints() []Hint               { return nil }
func (p Placeholder) Title() string                  { return p.title }
