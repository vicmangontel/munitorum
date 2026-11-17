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

func selectorForm() *Form {
	return New(
		Field{Key: "kind", Label: "Tipo", Choices: []string{"Persona", "Empresa", "Servicio"}},
		Field{Key: "name", Label: "Nombre"},
		Field{Key: "company", Label: "Empresa", Locked: true, Value: "(ninguna)"},
		Field{Key: "notes", Label: "Notas"},
	)
}

func TestForm_ASelectorStartsOnItsValueOrItsFirstChoiceAndCyclesWithLeftRightAndSpace(t *testing.T) {
	cases := []struct {
		name string
		keys []tea.KeyMsg
		want string
	}{
		{"starts on the first choice", nil, "Persona"},
		{"right moves to the next", []tea.KeyMsg{{Type: tea.KeyRight}}, "Empresa"},
		{"space moves to the next", []tea.KeyMsg{runes(" ")}, "Empresa"},
		{"right wraps to the first", []tea.KeyMsg{{Type: tea.KeyRight}, {Type: tea.KeyRight}, {Type: tea.KeyRight}}, "Persona"},
		{"left wraps to the last", []tea.KeyMsg{{Type: tea.KeyLeft}}, "Servicio"},
		{"letters change nothing", []tea.KeyMsg{runes("x")}, "Persona"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange
			f := selectorForm()

			// Act
			for _, k := range c.keys {
				f.Update(k)
			}

			// Assert
			if got := f.Value("kind"); got != c.want {
				t.Fatalf("kind = %q, want %q", got, c.want)
			}
		})
	}
}

func TestForm_ASelectorWithAnInitialValueStartsOnIt(t *testing.T) {
	// Act
	f := New(Field{Key: "kind", Label: "Tipo", Value: "Empresa", Choices: []string{"Persona", "Empresa"}})

	// Assert
	if got := f.Value("kind"); got != "Empresa" {
		t.Fatalf("kind = %q, want Empresa", got)
	}
}

func TestForm_ALockedFieldShowsItsValueAndIgnoresTyping(t *testing.T) {
	// Arrange
	f := selectorForm()
	f.Update(keyTab())
	f.Update(keyTab())

	// Act
	f.Update(runes("xyz"))
	f.SetValue("company", "Portones Acme")

	// Assert
	if f.Focused() != "company" || f.Value("company") != "Portones Acme" {
		t.Fatalf("focused = %q, value = %q; want company set by the host only", f.Focused(), f.Value("company"))
	}
	if !strings.Contains(f.View(), "Portones Acme") {
		t.Fatalf("view does not show the locked value:\n%s", f.View())
	}
}

func keyTab() tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyTab} }

func TestForm_AHiddenFieldIsNotDrawn(t *testing.T) {
	// Arrange
	f := selectorForm()
	f.SetHidden("company", true)

	// Act
	out := f.View()

	// Assert
	if strings.Contains(out, "Empresa  ") || strings.Contains(out, "(ninguna)") {
		t.Fatalf("view draws the hidden field:\n%s", out)
	}
}

func TestForm_TabSkipsAHiddenField(t *testing.T) {
	// Arrange
	f := selectorForm()
	f.SetHidden("company", true)
	f.Update(keyTab()) // kind -> name

	// Act
	f.Update(keyTab()) // name -> notes, skipping company

	// Assert
	if f.Focused() != "notes" {
		t.Fatalf("focus = %q, want notes", f.Focused())
	}
}

func TestForm_ShiftTabSkipsAHiddenFieldBackwards(t *testing.T) {
	// Arrange
	f := selectorForm()
	f.SetHidden("company", true)
	f.Update(keyTab())
	f.Update(keyTab()) // on notes

	// Act
	f.Update(tea.KeyMsg{Type: tea.KeyShiftTab})

	// Assert
	if f.Focused() != "name" {
		t.Fatalf("focus = %q, want name", f.Focused())
	}
}

func TestForm_ShiftTabFromTheFirstFieldWrapsToTheLastVisibleField(t *testing.T) {
	// Arrange
	f := selectorForm()
	f.SetHidden("company", true)

	// Act
	f.Update(tea.KeyMsg{Type: tea.KeyShiftTab})

	// Assert
	if f.Focused() != "notes" {
		t.Fatalf("focus = %q, want notes", f.Focused())
	}
}

func TestForm_AHiddenFieldHasNoError(t *testing.T) {
	// Arrange
	f := selectorForm()
	f.SetHidden("company", true)

	// Act
	bad := f.SetErrors(app.ValidationError{Fields: map[string]string{"company": app.CodeFeeNotPositive}})

	// Assert
	if bad {
		t.Fatal("SetErrors reported an error for a hidden field")
	}
}

func TestForm_ShowingAHiddenFieldAgainDrawsItAndRestoresItInTheTabOrder(t *testing.T) {
	// Arrange
	f := selectorForm()
	f.SetHidden("company", true)

	// Act
	f.SetHidden("company", false)
	f.Update(keyTab())
	f.Update(keyTab())

	// Assert
	if f.Focused() != "company" || !strings.Contains(f.View(), "(ninguna)") {
		t.Fatalf("focus = %q, view:\n%s; want the company field back", f.Focused(), f.View())
	}
}

func TestForm_HidingTheFocusedFieldMovesFocusToTheNextVisibleOne(t *testing.T) {
	// Arrange
	f := selectorForm()
	f.Update(keyTab())
	f.Update(keyTab()) // company

	// Act
	f.SetHidden("company", true)

	// Assert
	if f.Focused() != "notes" {
		t.Fatalf("focus = %q, want notes", f.Focused())
	}
}

func TestForm_ASelectorIsDrawnBetweenArrowsWithItsCurrentChoice(t *testing.T) {
	// Arrange
	f := selectorForm()

	// Act
	f.Update(tea.KeyMsg{Type: tea.KeyRight})

	// Assert
	if !strings.Contains(f.View(), "‹ Empresa ›") {
		t.Fatalf("view does not draw the selector:\n%s", f.View())
	}
}
