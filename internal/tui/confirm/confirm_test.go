package confirm

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func answerOf(t *testing.T, m *Model, keys ...tea.KeyType) ResultMsg {
	t.Helper()
	var cmd tea.Cmd
	for _, k := range keys {
		cmd = m.Update(tea.KeyMsg{Type: k})
	}
	if cmd == nil {
		t.Fatal("no answer produced")
	}
	return cmd().(ResultMsg)
}

func TestConfirm_EnterAnswersNoByDefault(t *testing.T) {
	// Arrange
	m := New("¿Seguro?")

	// Act
	got := answerOf(t, m, tea.KeyEnter)

	// Assert
	if got.Yes {
		t.Fatal("enter answered Sí without moving the selection")
	}
}

func TestConfirm_MovingTheSelectionThenEnterAnswersYes(t *testing.T) {
	cases := []struct {
		name string
		keys []tea.KeyType
		want bool
	}{
		{"right", []tea.KeyType{tea.KeyRight, tea.KeyEnter}, true},
		{"left", []tea.KeyType{tea.KeyLeft, tea.KeyEnter}, true},
		{"tab", []tea.KeyType{tea.KeyTab, tea.KeyEnter}, true},
		{"moved twice returns to No", []tea.KeyType{tea.KeyRight, tea.KeyLeft, tea.KeyEnter}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange
			m := New("¿Seguro?")

			// Act
			got := answerOf(t, m, c.keys...)

			// Assert
			if got.Yes != c.want {
				t.Fatalf("Yes = %v, want %v", got.Yes, c.want)
			}
		})
	}
}

func TestConfirm_EscAnswersNoEvenWhenYesIsSelected(t *testing.T) {
	// Arrange
	m := New("¿Seguro?")

	// Act
	got := answerOf(t, m, tea.KeyRight, tea.KeyEsc)

	// Assert
	if got.Yes {
		t.Fatal("esc answered Sí")
	}
}
