package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestQuitKeys(t *testing.T) {
	keys := []struct {
		name string
		msg  tea.KeyMsg
	}{
		{"q", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}},
		{"ctrl+c", tea.KeyMsg{Type: tea.KeyCtrlC}},
	}
	for _, k := range keys {
		t.Run(k.name, func(t *testing.T) {
			_, cmd := New().Update(k.msg)
			if cmd == nil {
				t.Fatal("expected a quit command")
			}
			if _, ok := cmd().(tea.QuitMsg); !ok {
				t.Fatalf("command returned %T, want tea.QuitMsg", cmd())
			}
		})
	}
}

func TestOtherKeysDoNotQuit(t *testing.T) {
	_, cmd := New().Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if cmd != nil {
		t.Fatal("unexpected command for an unbound key")
	}
}
