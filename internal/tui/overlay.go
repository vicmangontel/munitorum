package tui

import (
	"fmt"
	"strings"

	"github.com/vicmangontel/munitorum/internal/tui/text"
	"github.com/vicmangontel/munitorum/internal/tui/theme"
	"github.com/vicmangontel/munitorum/internal/tui/view"
)

// overlay is a centred box drawn over the shell; esc closes it. Forms and
// confirmations from later phases open through the same host.
type overlay struct {
	title string
	lines []string
}

func (o *overlay) render() string {
	body := theme.Title.Render(o.title) + "\n\n" + strings.Join(o.lines, "\n")
	return theme.Overlay.Render(body)
}

// helpOverlay lists the global keys and the keys of the view in the main panel.
func (m Model) helpOverlay() *overlay {
	global := []view.Hint{
		{Key: text.KeyTab, Desc: text.HelpNextPanel},
		{Key: text.KeyShiftTab, Desc: text.HelpPrevPanel},
		{Key: text.KeyViews, Desc: text.HelpGoToView},
		{Key: text.KeyMove, Desc: text.HelpMoveList},
		{Key: text.KeyEnter, Desc: text.HelpOpenView},
		{Key: text.KeyHelp, Desc: text.HelpToggle},
		{Key: text.KeyEsc, Desc: text.HelpClose},
		{Key: text.KeyQuit, Desc: text.HelpQuit},
	}
	current := m.views[m.selected]

	lines := []string{theme.Label.Render(text.HelpGlobalKeys)}
	lines = append(lines, keyLines(global)...)
	lines = append(lines, "", theme.Label.Render(text.HelpViewKeys(current.Title())))
	if hints := current.KeyHints(); len(hints) > 0 {
		lines = append(lines, keyLines(hints)...)
	} else {
		lines = append(lines, text.HelpNoViewKeys)
	}
	return &overlay{title: text.HelpTitle, lines: lines}
}

func keyLines(hints []view.Hint) []string {
	lines := make([]string, len(hints))
	for i, h := range hints {
		lines[i] = fmt.Sprintf("%-10s %s", h.Key, h.Desc)
	}
	return lines
}
