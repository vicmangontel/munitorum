package directory

import (
	"errors"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vicmangontel/munitorum/internal/app"
	"github.com/vicmangontel/munitorum/internal/tui/text"
	"github.com/vicmangontel/munitorum/internal/tui/view"
)

// SavedMsg is sent after a vendor was created or changed, also when it was
// created inline from a picker, so the list reloads and shows it.
type SavedMsg struct{ Vendor app.Vendor }

// ChangedMsg is sent after a vendor was deleted or restored.
type ChangedMsg struct{}

// send returns a command that delivers msg.
func send(msg tea.Msg) tea.Cmd { return func() tea.Msg { return msg } }

// openOverlay asks the root to show content in an overlay.
func openOverlay(title string, content view.Content) tea.Cmd {
	return send(view.OpenOverlayMsg{Title: title, Content: content})
}

// describe is the Spanish explanation of a failed operation, for the line shown
// to the treasurer. A refusal that names vendors shows the names; they are
// never logged.
func describe(err error) string {
	var conflict app.VendorConflict
	if errors.As(err, &conflict) {
		return text.VendorConflictMessage(conflict.Reason, conflict.Names)
	}
	var invalid app.ValidationError
	if errors.As(err, &invalid) {
		fields := make([]string, 0, len(invalid.Fields))
		for f := range invalid.Fields {
			fields = append(fields, f)
		}
		sort.Strings(fields)
		msgs := make([]string, len(fields))
		for i, f := range fields {
			msgs[i] = text.ValidationMessage(invalid.Fields[f])
		}
		return strings.Join(msgs, " ")
	}
	return text.ActionFailed + " " + err.Error()
}
