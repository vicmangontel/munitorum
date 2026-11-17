package directory

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/vicmangontel/munitorum/internal/app"
	"github.com/vicmangontel/munitorum/internal/tui/confirm"
	"github.com/vicmangontel/munitorum/internal/tui/text"
	"github.com/vicmangontel/munitorum/internal/tui/theme"
	"github.com/vicmangontel/munitorum/internal/tui/view"
)

// wrapWidth keeps the sentences of the delete overlay narrower than a small terminal.
const wrapWidth = 60

var wrapped = lipgloss.NewStyle().Width(wrapWidth)

// deleteDoneMsg is the result of a delete, tagged with the overlay that asked.
type deleteDoneMsg struct {
	target *deleteConfirm
	err    error
}

// deleteConfirm asks before deleting a vendor and deletes it on Sí. A refusal
// (a company with live people) stays on screen until a key is pressed. Esc
// closes the overlay without asking the service anything.
type deleteConfirm struct {
	svc     *app.DirectoryService
	id      int64
	confirm *confirm.Model
	busy    bool
	failure string
}

var _ view.Content = (*deleteConfirm)(nil)

func newDeleteConfirm(svc *app.DirectoryService, row app.VendorRow) *deleteConfirm {
	return &deleteConfirm{svc: svc, id: row.ID, confirm: confirm.New(wrapped.Render(text.ConfirmDeleteVendor(row.DisplayName())))}
}

func (d *deleteConfirm) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case confirm.ResultMsg:
		if d.busy || d.failure != "" {
			return nil
		}
		if !msg.Yes {
			return send(view.CloseOverlayMsg{})
		}
		d.busy = true
		return func() tea.Msg {
			return deleteDoneMsg{target: d, err: d.svc.DeleteVendor(context.Background(), d.id)}
		}
	case deleteDoneMsg:
		if msg.target != d {
			return nil
		}
		d.busy = false
		if msg.err == nil {
			return tea.Batch(send(ChangedMsg{}), send(view.CloseOverlayMsg{}))
		}
		d.failure = describe(msg.err)
	case tea.KeyMsg:
		switch {
		case d.failure != "":
			return send(view.CloseOverlayMsg{})
		case d.busy:
			return nil
		}
		return d.confirm.Update(msg)
	}
	return nil
}

func (d *deleteConfirm) View() string {
	if d.failure != "" {
		return theme.Error.Width(wrapWidth).Render(d.failure)
	}
	return d.confirm.View()
}

func (d *deleteConfirm) KeyHints() []view.Hint {
	if d.failure != "" {
		return nil
	}
	return d.confirm.KeyHints()
}
