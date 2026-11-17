// Package picker is the list a form shows to choose one item by typing: type to
// filter (ignoring case and accents), enter to select, + to create a new item
// without leaving the form, esc to choose nothing. The caller loads the items
// and decides what + creates, so any later form (expenses, reminders, obras) can
// host it.
package picker

import (
	"strings"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/vicmangontel/munitorum/internal/domain"
	"github.com/vicmangontel/munitorum/internal/tui/text"
	"github.com/vicmangontel/munitorum/internal/tui/theme"
	"github.com/vicmangontel/munitorum/internal/tui/view"
)

// maxRows is how many matches show at once; the window follows the selection.
const maxRows = 8

// Item is one choice. Label is what the treasurer sees and types against.
type Item struct {
	ID    int64
	Label string
}

// SelectedMsg is sent when an item was chosen.
type SelectedMsg struct{ Item Item }

// CancelledMsg is sent on esc: no item was chosen.
type CancelledMsg struct{}

// CreatedMsg is what the content built by a Creator sends when it has created
// the item; the picker answers with SelectedMsg for it.
type CreatedMsg struct{ Item Item }

// Creator builds the content that creates an item inline. That content must send
// CreatedMsg when it is done; esc backs out of it to the list.
type Creator func() view.Content

// Model is the picker. It implements view.Content and view.EscapeHandler.
type Model struct {
	items    []Item
	create   Creator // nil when the host cannot create items
	input    textinput.Model
	matches  []Item
	selected int // position in matches
	creating view.Content
}

var (
	_ view.Content       = (*Model)(nil)
	_ view.EscapeHandler = (*Model)(nil)
)

// New returns a picker over items. create may be nil to disable +.
func New(items []Item, create Creator) *Model {
	in := textinput.New()
	in.Prompt = ""
	in.Cursor.SetMode(cursor.CursorStatic) // no blink timers to drive
	in.Focus()
	m := &Model{items: items, create: create, input: in}
	m.refilter()
	return m
}

// HandlesEscape is always true: esc either backs out of an inline create or
// sends CancelledMsg, and the host decides what closing means.
func (m *Model) HandlesEscape() bool { return true }

func (m *Model) Update(msg tea.Msg) tea.Cmd {
	if m.creating != nil {
		return m.updateCreating(msg)
	}
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil
	}
	switch key.String() {
	case "up":
		m.selected = max(m.selected-1, 0)
	case "down":
		m.selected = min(m.selected+1, len(m.matches)-1)
	case "enter":
		if len(m.matches) == 0 {
			return nil
		}
		item := m.matches[m.selected]
		return func() tea.Msg { return SelectedMsg{Item: item} }
	case "esc":
		return func() tea.Msg { return CancelledMsg{} }
	case text.KeyPlus:
		if m.create != nil {
			m.creating = m.create()
			return nil
		}
		fallthrough
	default:
		var cmd tea.Cmd
		before := m.input.Value()
		m.input, cmd = m.input.Update(msg)
		if m.input.Value() != before {
			m.refilter()
		}
		return cmd
	}
	return nil
}

func (m *Model) updateCreating(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case CreatedMsg:
		m.creating = nil
		return func() tea.Msg { return SelectedMsg{Item: msg.Item} }
	case tea.KeyMsg:
		if msg.String() == "esc" {
			m.creating = nil
			return nil
		}
	}
	return m.creating.Update(msg)
}

// refilter keeps the items whose label contains the typed text, in the order
// they were given, and puts the selection on the first.
func (m *Model) refilter() {
	needle := domain.FoldKey(strings.TrimSpace(m.input.Value()))
	m.matches = m.matches[:0:0]
	for _, it := range m.items {
		if strings.Contains(domain.FoldKey(it.Label), needle) {
			m.matches = append(m.matches, it)
		}
	}
	m.selected = 0
}

func (m *Model) View() string {
	if m.creating != nil {
		return m.creating.View()
	}
	lines := []string{theme.Label.Render(text.PickerFilter) + m.input.View()}
	if len(m.matches) == 0 {
		lines = append(lines, theme.Label.Render(text.PickerNoMatch))
	}
	from := min(max(m.selected-maxRows+1, 0), max(len(m.matches)-maxRows, 0))
	for i := from; i < min(from+maxRows, len(m.matches)); i++ {
		if i == m.selected {
			lines = append(lines, theme.SelectedRow.Render("> "+m.matches[i].Label))
		} else {
			lines = append(lines, "  "+m.matches[i].Label)
		}
	}
	return strings.Join(lines, "\n")
}

func (m *Model) KeyHints() []view.Hint {
	if m.creating != nil {
		return append(m.creating.KeyHints(), view.Hint{Key: text.KeyEsc, Desc: text.HintBack})
	}
	hints := []view.Hint{
		{Key: text.KeyMove, Desc: text.HintMove},
		{Key: text.KeyEnter, Desc: text.HintPick},
	}
	if m.create != nil {
		hints = append(hints, view.Hint{Key: text.KeyPlus, Desc: text.HintCreateNew})
	}
	return append(hints, view.Hint{Key: text.KeyEsc, Desc: text.HintNoSelection})
}
