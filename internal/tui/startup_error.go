package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/vicmangontel/munitorum/internal/tui/text"
	"github.com/vicmangontel/munitorum/internal/tui/theme"
)

// StartupError is the screen shown when the database cannot be opened or
// migrated. It is deliberately not the shell and not the setup wizard: the
// existing file must never be treated as a new coto. Any key exits.
type StartupError struct {
	path string
	err  error
}

// NewStartupError returns the error screen for the database at path.
func NewStartupError(path string, err error) StartupError {
	return StartupError{path: path, err: err}
}

func (StartupError) Init() tea.Cmd { return nil }

func (e StartupError) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(tea.KeyMsg); ok {
		return e, tea.Quit
	}
	return e, nil
}

func (e StartupError) View() string {
	return theme.Error.Bold(true).Render(text.DBOpenFailed) + "\n\n" +
		theme.Label.Render(text.DBPathLabel) + " " + e.path + "\n" +
		theme.Label.Render(text.DBErrorLabel) + " " + e.err.Error() + "\n\n" +
		theme.KeyHint.Render(text.AnyKeyToExit) + "\n"
}
