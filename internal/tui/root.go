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
	svc           *app.SetupService     // handed to the shell, which edits settings through it
	dir           *app.DirectoryService // handed to the shell, which shows Directorio through it
	width, height int
}

// NewShell returns the root showing the shell of an existing coto.
func NewShell(svc *app.SetupService, settings app.Settings) Root {
	return Root{shell: New().WithService(svc).WithSettings(settings), svc: svc}
}

// NewWizard returns the root showing the setup wizard; today prefills the
// opening date.
func NewWizard(svc *app.SetupService, today time.Time) Root {
	return Root{wizard: wizard.New(svc, today), shell: New(), svc: svc}
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
		r.shell = r.shell.WithService(r.svc).WithSettings(done.Settings)
		if r.dir != nil {
			r.shell = r.shell.WithDirectory(r.dir)
		}
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

// WithDirectory returns the root whose shell shows Directorio through dir,
// also when the shell appears after the wizard.
func (r Root) WithDirectory(dir *app.DirectoryService) Root {
	r.dir = dir
	r.shell = r.shell.WithDirectory(dir)
	return r
}
