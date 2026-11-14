package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vicmangontel/munitorum/internal/tui/view"
)

// recorder is interactive overlay content that remembers the keys it got.
type recorder struct{ keys []string }

func (r *recorder) Update(msg tea.Msg) tea.Cmd {
	if k, ok := msg.(tea.KeyMsg); ok {
		r.keys = append(r.keys, k.String())
	}
	return nil
}
func (r *recorder) View() string { return "contenido interactivo" }
func (r *recorder) KeyHints() []view.Hint {
	return []view.Hint{{Key: "x", Desc: "hacer algo"}}
}

func TestOverlay_InteractiveContentGetsKeysAndEscStillCloses(t *testing.T) {
	// Arrange
	rec := &recorder{}
	m := sized(100, 30).openOverlay("Editar", rec)

	// Act
	next, _ := m.Update(keyMsg("?")) // "?" is text for the content, not help
	m = next.(Model)
	next, _ = m.Update(keyMsg("a"))
	m = next.(Model)
	open := m.overlay != nil
	next, _ = m.Update(keyMsg("esc"))
	m = next.(Model)

	// Assert
	if !open || strings.Join(rec.keys, ",") != "?,a" {
		t.Fatalf("content got keys %v, overlay open = %v", rec.keys, open)
	}
	if m.overlay != nil {
		t.Fatal("esc did not close the overlay")
	}
}

func TestOverlay_ShowsTheContentAndItsKeysInTheHintBar(t *testing.T) {
	// Arrange
	m := sized(100, 30).openOverlay("Editar", &recorder{})

	// Act
	out := m.View()

	// Assert
	for _, want := range []string{"Editar", "contenido interactivo", "x hacer algo", "esc cerrar"} {
		if !strings.Contains(out, want) {
			t.Errorf("view does not show %q:\n%s", want, out)
		}
	}
}
