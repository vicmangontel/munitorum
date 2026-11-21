package ledger

import (
	"errors"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vicmangontel/munitorum/internal/app"
	"github.com/vicmangontel/munitorum/internal/tui/text"
	"github.com/vicmangontel/munitorum/internal/tui/view"
)

// send returns a command that delivers msg.
func send(msg tea.Msg) tea.Cmd { return func() tea.Msg { return msg } }

// openOverlay asks the root to show content in an overlay.
func openOverlay(title string, content view.Content) tea.Cmd {
	return send(view.OpenOverlayMsg{Title: title, Content: content})
}

// describe is the Spanish explanation of a failed operation, for the line shown
// to the treasurer.
func describe(err error) string {
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
