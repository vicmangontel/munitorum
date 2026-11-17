// Package form is the labelled text-field form used by the wizard and by every
// later phase: tab and shift+tab move between fields and the error of each
// invalid field shows under it, all at once.
package form

import (
	"slices"
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

// Field describes one labelled field. Key is the app.ValidationError key its
// errors are reported under. A field is a text box unless it has Choices, which
// makes it a selector over them (left, right and space change it; Value picks
// the initial one), or is Locked, which shows Value but accepts no typing: the
// host fills it with SetValue, for a value chosen in another component.
type Field struct {
	Key, Label, Value string
	Choices           []string
	Locked            bool
}

type field struct {
	Field
	input  textinput.Model
	err    string // message shown under the field, empty when valid
	hidden bool   // skipped by focus and not drawn
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
		if len(spec.Choices) > 0 && !slices.Contains(spec.Choices, spec.Value) {
			spec.Value = spec.Choices[0]
		}
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

// SetValue replaces the text of the field with the given key.
func (f *Form) SetValue(key, value string) {
	if i := f.index(key); i >= 0 {
		f.fields[i].input.SetValue(value)
	}
}

// SetHidden shows or hides the field with the given key. A hidden field is not
// drawn, focus skips it, and its errors are not shown. Hiding the focused field
// moves focus to the next one.
func (f *Form) SetHidden(key string, hidden bool) {
	i := f.index(key)
	if i < 0 {
		return
	}
	f.fields[i].hidden = hidden
	if hidden && f.focus == i {
		f.setFocus(f.step(1))
	}
}

// Focused returns the key of the focused field.
func (f *Form) Focused() string { return f.fields[f.focus].Key }

// step is the index of the visible field dir places away from the focused one.
func (f *Form) step(dir int) int {
	i := f.focus
	for range f.fields {
		i = (i + dir + len(f.fields)) % len(f.fields)
		if !f.fields[i].hidden {
			return i
		}
	}
	return f.focus
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
		bad = bad && !f.fields[i].hidden
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
			f.setFocus(f.step(1))
			return nil
		case "shift+tab":
			f.setFocus(f.step(-1))
			return nil
		}
	}
	fl := &f.fields[f.focus]
	switch {
	case len(fl.Choices) > 0:
		f.cycle(fl, msg)
		return nil
	case fl.Locked:
		return nil
	}
	var cmd tea.Cmd
	f.fields[f.focus].input, cmd = f.fields[f.focus].input.Update(msg)
	return cmd
}

// cycle moves a selector to the next or previous choice on right, left or space.
func (f *Form) cycle(fl *field, msg tea.Msg) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return
	}
	dir := 0
	switch key.String() {
	case "right", " ":
		dir = 1
	case "left":
		dir = -1
	}
	if dir == 0 {
		return
	}
	i := slices.Index(fl.Choices, fl.input.Value())
	fl.input.SetValue(fl.Choices[(i+dir+len(fl.Choices))%len(fl.Choices)])
}

// View renders one line per field, with its error under it.
func (f *Form) View() string {
	labelWidth := 0
	for _, fl := range f.fields {
		labelWidth = max(labelWidth, lipgloss.Width(fl.Label))
	}
	var b strings.Builder
	for i, fl := range f.fields {
		if fl.hidden {
			continue
		}
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		label := theme.Label.Render(fl.Label + strings.Repeat(" ", labelWidth-lipgloss.Width(fl.Label)))
		b.WriteString(label + "  " + f.render(i))
		if fl.err != "" {
			b.WriteString("\n" + strings.Repeat(" ", labelWidth+2) + theme.Error.Render(fl.err))
		}
	}
	return b.String()
}

// render is the value part of field i: a text box, a selector or a locked value.
func (f *Form) render(i int) string {
	fl := f.fields[i]
	switch {
	case len(fl.Choices) > 0:
		choice := "‹ " + fl.input.Value() + " ›"
		if i == f.focus {
			return theme.SelectedRow.Render(choice)
		}
		return choice
	case fl.Locked:
		if i == f.focus {
			return theme.SelectedRow.Render(fl.input.Value())
		}
		return fl.input.Value()
	}
	return fl.input.View()
}

// KeyHints lists the form's own keys.
func (f *Form) KeyHints() []view.Hint {
	if len(f.fields) < 2 {
		return nil
	}
	return []view.Hint{{Key: text.KeyTab, Desc: text.HintField}}
}
