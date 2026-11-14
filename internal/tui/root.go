package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vicmangontel/munitorum/internal/app"
	"github.com/vicmangontel/munitorum/internal/tui/wizard"
)

// Root is the model the program runs. It shows the setup wizard until the coto
// exists, then the shell, without restarting.
type Root struct {
	wizard        *wizard.Model // nil once the shell is showing
	shell         Model
	width, height int
}

// NewShell returns the root showing the shell of an existing coto.
func NewShell(settings app.Settings) Root {
	return Root{shell: New().WithSettings(settings)}
}

// NewWizard returns the root showing the setup wizard; today prefills the
// opening date.
func NewWizard(svc *app.SetupService, today time.Time) Root {
	return Root{wizard: wizard.New(svc, today), shell: New()}
}

// InWizard reports whether the wizard is showing.
func (r Root) InWizard() bool { return r.wizard != nil }

func (r Root) Init() tea.Cmd {
	if r.wizard != nil {
		return r.wizard.Init()
	}
	return r.shell.Init()
}

func (r Root) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		r.width, r.height = size.Width, size.Height
	}
	if done, ok := msg.(wizard.DoneMsg); ok && r.wizard != nil {
		r.wizard = nil
		r.shell = r.shell.WithSettings(done.Settings)
		next, _ := r.shell.Update(tea.WindowSizeMsg{Width: r.width, Height: r.height})
		r.shell = next.(Model)
		return r, r.shell.Init()
	}
	if r.wizard != nil {
		_, cmd := r.wizard.Update(msg)
		return r, cmd
	}
	next, cmd := r.shell.Update(msg)
	r.shell = next.(Model)
	return r, cmd
}

func (r Root) View() string {
	if r.wizard != nil {
		return r.wizard.View()
	}
	return r.shell.View()
}
