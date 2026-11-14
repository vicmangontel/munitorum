// Package form is the labelled text-field form used by the wizard and by every
// later phase: tab and shift+tab move between fields and the error of each
// invalid field shows under it, all at once.
package form

import (
	"strings"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/vicmangontel/munitorum/internal/app"
	"github.com/vicmangontel/munitorum/internal/tui/text"
	"github.com/vicmangontel/munitorum/internal/tui/theme"
	"github.com/vicmangontel/munitorum/internal/tui/view"
)

const inputWidth = 32

// Field describes one labelled text field. Key is the app.ValidationError key
// its errors are reported under.
type Field struct {
	Key, Label, Value string
}

type field struct {
	Field
	input textinput.Model
	err   string // message shown under the field, empty when valid
}

// Form is a list of text fields with one focused. It implements view.Content.
type Form struct {
	fields []field
	focus  int
}

var _ view.Content = (*Form)(nil)

// New returns a form with the first field focused.
func New(fields ...Field) *Form {
	f := &Form{fields: make([]field, len(fields))}
	for i, spec := range fields {
		in := textinput.New()
		in.Prompt = ""
		in.Width = inputWidth
		in.Cursor.SetMode(cursor.CursorStatic) // no blink timers to drive
		in.SetValue(spec.Value)
		in.CursorEnd()
		f.fields[i] = field{Field: spec, input: in}
	}
	f.setFocus(0)
	return f
}

func (f *Form) setFocus(i int) {
	f.fields[f.focus].input.Blur()
	f.focus = i
	f.fields[i].input.Focus()
}

// Value returns the text of the field with the given key.
func (f *Form) Value(key string) string {
	if i := f.index(key); i >= 0 {
		return f.fields[i].input.Value()
	}
	return ""
}

// SetLabel changes the label of the field with the given key.
func (f *Form) SetLabel(key, label string) {
	if i := f.index(key); i >= 0 {
		f.fields[i].Label = label
	}
}

func (f *Form) index(key string) int {
	for i := range f.fields {
		if f.fields[i].Key == key {
			return i
		}
	}
	return -1
}

// SetErrors shows the error of every field of this form that ve reports, clears
// the others, and moves the cursor to the first invalid field. It reports
// whether any field of this form is invalid; errors for fields of other forms
// are ignored.
func (f *Form) SetErrors(ve app.ValidationError) bool {
	first := -1
	for i := range f.fields {
		code, bad := ve.Fields[f.fields[i].Key]
		f.fields[i].err = ""
		if bad {
			f.fields[i].err = text.ValidationMessage(code)
			if first < 0 {
				first = i
			}
		}
	}
	if first >= 0 {
		f.setFocus(first)
	}
	return first >= 0
}

// ClearErrors removes every shown error.
func (f *Form) ClearErrors() {
	for i := range f.fields {
		f.fields[i].err = ""
	}
}

// AtStart reports whether the first field is focused with the cursor before
// its first character: the one place where moving back leaves the form.
func (f *Form) AtStart() bool {
	return f.focus == 0 && f.fields[0].input.Position() == 0
}

// OnFirstField reports whether the first field is focused.
func (f *Form) OnFirstField() bool { return f.focus == 0 }

// Update moves focus on tab and shift+tab and passes everything else to the
// focused field.
func (f *Form) Update(msg tea.Msg) tea.Cmd {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "tab":
			f.setFocus((f.focus + 1) % len(f.fields))
			return nil
		case "shift+tab":
			f.setFocus((f.focus + len(f.fields) - 1) % len(f.fields))
			return nil
		}
	}
	var cmd tea.Cmd
	f.fields[f.focus].input, cmd = f.fields[f.focus].input.Update(msg)
	return cmd
}

// View renders one line per field, with its error under it.
func (f *Form) View() string {
	labelWidth := 0
	for _, fl := range f.fields {
		labelWidth = max(labelWidth, lipgloss.Width(fl.Label))
	}
	var b strings.Builder
	for i, fl := range f.fields {
		if i > 0 {
			b.WriteString("\n")
		}
		label := theme.Label.Render(fl.Label + strings.Repeat(" ", labelWidth-lipgloss.Width(fl.Label)))
		b.WriteString(label + "  " + fl.input.View())
		if fl.err != "" {
			b.WriteString("\n" + strings.Repeat(" ", labelWidth+2) + theme.Error.Render(fl.err))
		}
	}
	return b.String()
}

// KeyHints lists the form's own keys.
func (f *Form) KeyHints() []view.Hint {
	if len(f.fields) < 2 {
		return nil
	}
	return []view.Hint{{Key: text.KeyTab, Desc: text.HintField}}
}
