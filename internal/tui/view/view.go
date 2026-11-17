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

// Content is interactive content hosted by an overlay or a full-screen flow.
// It is held by pointer and changes in place, so the host can keep a single
// reference while Bubble Tea copies its own model.
type Content interface {
	// Update handles a message and returns the command to run, if any.
	Update(msg tea.Msg) tea.Cmd
	View() string
	// KeyHints lists the keys the content accepts, for the hint bar.
	KeyHints() []Hint
}

// Detailer is implemented by a view that fills the detail panel for its
// selected row. An empty string shows the "no selection" text.
type Detailer interface {
	Detail() string
}

// KeyCapturer is implemented by a view that is reading typed text, such as a
// search box. While CapturesKeys is true the root sends every key to the view,
// so that q, ?, tab and the digits are text and not shell commands.
type KeyCapturer interface {
	CapturesKeys() bool
}

// EscapeHandler is implemented by overlay content with steps of its own, such
// as a picker opened inside a form. While HandlesEscape is true the root sends
// esc to the content instead of closing the overlay.
type EscapeHandler interface {
	HandlesEscape() bool
}

// OpenOverlayMsg asks the root to show Content in an overlay. A view returns it
// from a command.
type OpenOverlayMsg struct {
	Title   string
	Content Content
}

// CloseOverlayMsg asks the root to close the overlay.
type CloseOverlayMsg struct{}

// SizeMsg tells every view how much room its panel's content has, in cells,
// after the border and the title line. The root sends it whenever the terminal
// is resized.
type SizeMsg struct{ Width, Height int }
