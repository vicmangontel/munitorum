package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/vicmangontel/munitorum/internal/tui/text"
	"github.com/vicmangontel/munitorum/internal/tui/theme"
	"github.com/vicmangontel/munitorum/internal/tui/view"
)

const (
	leftWidth     = 28 // left column; the right column takes the rest
	summaryHeight = 7  // border, title, and four rows
)

func (m Model) View() string {
	if m.width == 0 {
		return "" // no size message yet; avoids flashing the too-small text on start
	}
	if m.width < minWidth || m.height < minHeight {
		return text.TooSmall(m.width, m.height, minWidth, minHeight)
	}

	bodyHeight := m.height - 1 // the key-hint bar takes the last line
	mainHeight := bodyHeight * 3 / 5
	rightWidth := m.width - leftWidth

	left := lipgloss.JoinVertical(lipgloss.Left,
		panelBox(text.SummaryTitle, summaryRows(leftWidth-2), leftWidth, summaryHeight, false),
		panelBox(text.PanelViews, m.viewList(), leftWidth, bodyHeight-summaryHeight, m.focus == panelViews),
	)
	current := m.views[m.selected]
	right := lipgloss.JoinVertical(lipgloss.Left,
		panelBox(current.Title(), current.View(), rightWidth, mainHeight, m.focus == panelMain),
		panelBox(text.PanelDetail, theme.Label.Render(text.NoSelection), rightWidth, bodyHeight-mainHeight, m.focus == panelDetail),
	)
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, right)
	screen := lipgloss.JoinVertical(lipgloss.Left, body, m.hintBar())

	if m.overlay != nil {
		screen = placeCentered(screen, m.overlay.render(), m.width, m.height)
	}
	return screen
}

// panelBox draws a bordered panel of exactly w by h cells with a title line.
func panelBox(title, content string, w, h int, focused bool) string {
	style := theme.PanelUnfocused
	if focused {
		style = theme.PanelFocused
	}
	inner := w - style.GetHorizontalFrameSize()
	rows := h - style.GetVerticalFrameSize()
	body := clip(theme.Title.Render(title)+"\n"+content, inner, rows)
	return style.Width(inner).Height(rows).Render(body)
}

// clip cuts s to at most h lines of at most w cells, so a long view can never
// push the layout past the terminal.
func clip(s string, w, h int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > h {
		lines = lines[:h]
	}
	for i, l := range lines {
		lines[i] = ansi.Truncate(l, w, "")
	}
	return strings.Join(lines, "\n")
}

func (m Model) viewList() string {
	rows := make([]string, len(m.views))
	for i, v := range m.views {
		row := fmt.Sprintf("  %d %s", i+1, v.Title())
		if i == m.selected {
			row = theme.SelectedRow.Render(fmt.Sprintf("> %d %s", i+1, v.Title()))
		}
		rows[i] = row
	}
	return strings.Join(rows, "\n")
}

// summaryRows is static until Phase 1 provides accounts and balances.
func summaryRows(w int) string {
	return strings.Join([]string{
		amountRow(text.AccountCash, text.AmountPending, w, lipgloss.NewStyle()),
		amountRow(text.AccountBank, text.AmountPending, w, lipgloss.NewStyle()),
		amountRow(text.AccountTotal, text.AmountPending, w, theme.AmountTotal),
		amountRow(text.Available, text.AmountPending, w, theme.AmountTotal),
	}, "\n")
}

// amountRow puts the label on the left and the amount on the right.
func amountRow(label, amount string, w int, style lipgloss.Style) string {
	gap := max(w-lipgloss.Width(label)-lipgloss.Width(amount), 1)
	return style.Render(label + strings.Repeat(" ", gap) + amount)
}

// hintBar lists the keys of the focused panel, then the global keys. While an
// overlay is open it lists only the keys the overlay accepts.
func (m Model) hintBar() string {
	var hints []view.Hint
	if m.overlay != nil {
		return m.renderHints([]view.Hint{{Key: text.KeyEsc, Desc: text.HintClose}})
	}
	switch m.focus {
	case panelViews:
		hints = []view.Hint{{Key: text.KeyMove, Desc: text.HintMove}, {Key: text.KeyEnter, Desc: text.HintOpen}}
	case panelMain:
		hints = m.views[m.selected].KeyHints()
	}
	hints = append(hints,
		view.Hint{Key: text.KeyTab, Desc: text.HintPanel},
		view.Hint{Key: text.KeyViews, Desc: text.HintView},
		view.Hint{Key: text.KeyHelp, Desc: text.HintHelp},
		view.Hint{Key: text.KeyQuit, Desc: text.HintQuit},
	)
	return m.renderHints(hints)
}

func (m Model) renderHints(hints []view.Hint) string {
	parts := make([]string, len(hints))
	for i, h := range hints {
		parts[i] = h.Key + " " + h.Desc
	}
	return theme.KeyHint.Render(ansi.Truncate(strings.Join(parts, "  "), m.width, ""))
}

// placeCentered draws box over the middle of bg, keeping the shell visible
// around it. bg must be w cells wide on every line.
func placeCentered(bg, box string, w, h int) string {
	bgLines := strings.Split(bg, "\n")
	boxLines := strings.Split(box, "\n")
	boxW := lipgloss.Width(box)
	x := max((w-boxW)/2, 0)
	y := max((h-len(boxLines))/2, 0)
	for i, line := range boxLines {
		row := y + i
		if row >= len(bgLines) {
			break
		}
		bgLines[row] = ansi.Truncate(bgLines[row], x, "") + line + ansi.TruncateLeft(bgLines[row], x+boxW, "")
	}
	return strings.Join(bgLines, "\n")
}
