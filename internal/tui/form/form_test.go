package form

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vicmangontel/munitorum/internal/app"
)

func runes(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }

func newForm() *Form {
	return New(
		Field{Key: "a", Label: "Alfa"},
		Field{Key: "b", Label: "Beta", Value: "init"},
		Field{Key: "c", Label: "Gamma"},
	)
}

func TestForm_ShowsTheErrorOfEveryInvalidFieldAtOnce(t *testing.T) {
	// Arrange
	f := newForm()
	ve := app.ValidationError{Fields: map[string]string{"a": app.CodeCommunityNameLength, "c": app.CodeFeeNotPositive, "other": app.CodeDateFuture}}

	// Act
	bad := f.SetErrors(ve)

	// Assert
	view := f.View()
	if !bad {
		t.Fatal("SetErrors reported no invalid field")
	}
	for _, want := range []string{"hasta 60 caracteres", "mayor que cero"} {
		if !strings.Contains(view, want) {
			t.Errorf("view does not show %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "posterior a hoy") {
		t.Errorf("view shows the error of a field that is not in the form:\n%s", view)
	}
}

func TestForm_SetErrorsMovesFocusToTheFirstInvalidFieldAndClearsOldErrors(t *testing.T) {
	// Arrange
	f := newForm()
	f.SetErrors(app.ValidationError{Fields: map[string]string{"b": app.CodeAmountInvalid}})

	// Act
	f.SetErrors(app.ValidationError{Fields: map[string]string{"c": app.CodeFeeNotPositive}})

	// Assert
	if f.focus != 2 {
		t.Fatalf("focus = %d, want the third field", f.focus)
	}
	if strings.Contains(f.View(), "importe válido") {
		t.Fatalf("the old error is still shown:\n%s", f.View())
	}
	if f.SetErrors(app.ValidationError{}) {
		t.Fatal("SetErrors reported an invalid field for no errors")
	}
}

func TestForm_TabAndShiftTabCycleThroughTheFields(t *testing.T) {
	cases := []struct {
		name string
		keys []tea.KeyMsg
		want int
	}{
		{"tab moves forward", []tea.KeyMsg{{Type: tea.KeyTab}}, 1},
		{"tab wraps from the last field", []tea.KeyMsg{{Type: tea.KeyTab}, {Type: tea.KeyTab}, {Type: tea.KeyTab}}, 0},
		{"shift+tab wraps from the first field", []tea.KeyMsg{{Type: tea.KeyShiftTab}}, 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange
			f := newForm()

			// Act
			for _, k := range c.keys {
				f.Update(k)
			}

			// Assert
			if f.focus != c.want {
				t.Fatalf("focus = %d, want %d", f.focus, c.want)
			}
		})
	}
}

func TestForm_TypingEditsOnlyTheFocusedField(t *testing.T) {
	// Arrange
	f := newForm()
	f.Update(tea.KeyMsg{Type: tea.KeyTab})

	// Act
	f.Update(runes("X"))

	// Assert
	if f.Value("b") != "initX" || f.Value("a") != "" || f.Value("c") != "" {
		t.Fatalf("values = %q %q %q", f.Value("a"), f.Value("b"), f.Value("c"))
	}
}

func TestForm_AtStartIsTrueOnlyOnTheFirstFieldBeforeItsFirstCharacter(t *testing.T) {
	// Arrange
	f := New(Field{Key: "a", Label: "Alfa", Value: "ab"}, Field{Key: "b", Label: "Beta"})

	// Act / Assert
	if f.AtStart() {
		t.Fatal("AtStart with the cursor at the end of the text")
	}
	f.Update(tea.KeyMsg{Type: tea.KeyLeft})
	f.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if !f.AtStart() {
		t.Fatal("not AtStart with the cursor before the first character")
	}
	f.Update(tea.KeyMsg{Type: tea.KeyTab})
	if f.AtStart() {
		t.Fatal("AtStart on the second field")
	}
}
