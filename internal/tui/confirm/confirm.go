// Package confirm is the Sí / No confirmation, with No selected by default.
package confirm

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/vicmangontel/munitorum/internal/tui/text"
	"github.com/vicmangontel/munitorum/internal/tui/theme"
	"github.com/vicmangontel/munitorum/internal/tui/view"
)

// ResultMsg is sent when the treasurer answers. esc answers No.
type ResultMsg struct{ Yes bool }

// Model is a question with two answers. It implements view.Content.
type Model struct {
	prompt string
	yes    bool // the selected answer; false (No) until the treasurer moves
}

var _ view.Content = (*Model)(nil)

// New returns a confirmation of prompt with No selected.
func New(prompt string) *Model { return &Model{prompt: prompt} }

func (m *Model) Update(msg tea.Msg) tea.Cmd {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil
	}
	switch key.String() {
	case "left", "right", "tab", "shift+tab":
		m.yes = !m.yes
	case "enter":
		return answer(m.yes)
	case "esc":
		return answer(false)
	}
	return nil
}

func answer(yes bool) tea.Cmd {
	return func() tea.Msg { return ResultMsg{Yes: yes} }
}

func (m *Model) View() string {
	yes, no := "  "+text.Yes+"  ", "  "+text.No+"  "
	if m.yes {
		yes = theme.SelectedRow.Render("[ " + text.Yes + " ]")
	} else {
		no = theme.SelectedRow.Render("[ " + text.No + " ]")
	}
	return m.prompt + "\n\n" + yes + "   " + no
}

func (m *Model) KeyHints() []view.Hint {
	return []view.Hint{
		{Key: text.KeyLeftRight, Desc: text.HintChoose},
		{Key: text.KeyEnter, Desc: text.HintAccept},
		{Key: text.KeyEsc, Desc: text.No},
	}
}
