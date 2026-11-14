package tui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestStartupError_ShowsPathAndError(t *testing.T) {
	// Arrange
	screen := NewStartupError(`C:\datos\munitorum.db`, errors.New("file is not a database (26)"))

	// Act
	out := screen.View()

	// Assert
	for _, want := range []string{"No se pudo abrir la base de datos", `C:\datos\munitorum.db`, "file is not a database (26)", "cualquier tecla"} {
		if !strings.Contains(out, want) {
			t.Errorf("error screen missing %q:\n%s", want, out)
		}
	}
}

func TestStartupError_AnyKeyExitsAndOtherMessagesDoNot(t *testing.T) {
	cases := []struct {
		name     string
		msg      tea.Msg
		wantQuit bool
	}{
		{"letter", keyMsg("x"), true},
		{"enter", keyMsg("enter"), true},
		{"escape", keyMsg("esc"), true},
		{"resize", tea.WindowSizeMsg{Width: 80, Height: 24}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange
			screen := NewStartupError("x.db", errors.New("boom"))

			// Act
			_, cmd := screen.Update(c.msg)

			// Assert
			quit := cmd != nil && isQuit(cmd())
			if quit != c.wantQuit {
				t.Fatalf("quit = %v, want %v", quit, c.wantQuit)
			}
		})
	}
}
