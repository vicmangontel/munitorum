package wizard

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/vicmangontel/munitorum/internal/app"
	"github.com/vicmangontel/munitorum/internal/domain"
	"github.com/vicmangontel/munitorum/internal/tui/text"
	"github.com/vicmangontel/munitorum/internal/tui/theme"
	"github.com/vicmangontel/munitorum/internal/tui/view"
)

// stepView renders the current step and lists the keys it accepts.
func (m *Model) stepView() (string, []view.Hint) {
	next := view.Hint{Key: text.KeyEnter, Desc: text.HintNext}
	back := view.Hint{Key: text.KeyShiftTab + "/" + text.KeyLeft, Desc: text.HintBack}
	cancel := view.Hint{Key: text.KeyEsc, Desc: text.HintCancel}

	switch m.step {
	case stepSummary:
		return m.summaryView(), []view.Hint{
			{Key: text.KeyEnter, Desc: text.HintConfirm},
			{Key: text.KeyLeft, Desc: text.HintBack},
			cancel,
		}
	case stepHouseRows:
		return m.rows.view(m.visibleRows()), []view.Hint{
			{Key: text.KeyMove, Desc: text.HintRow},
			{Key: text.KeyTab, Desc: text.HintField},
			next, back, cancel,
		}
	}

	f := m.current()
	body := f.View()
	if m.step == stepHouses {
		if preview := m.codePreview(); preview != "" {
			body += "\n\n" + theme.Label.Render(preview)
		}
	}
	hints := append([]view.Hint{next}, f.KeyHints()...)
	if m.step != stepCommunity {
		hints = append(hints, back)
	}
	return body, append(hints, cancel)
}

// codePreview shows the first and last house code for the count and prefix as
// typed; empty while the count is not valid.
func (m *Model) codePreview() string {
	n, err := domain.ParseHouseCount(m.houses.Value(app.FieldHouseCount))
	if err != nil {
		return ""
	}
	prefix := m.houses.Value(app.FieldHousePrefix)
	return text.CodePreview(domain.HouseCode(prefix, 1, n), domain.HouseCode(prefix, n, n))
}

// Lines the box and the other parts of the house-row step take around the table.
const stepChrome = 16

func (m *Model) visibleRows() int {
	if m.height == 0 {
		return 10
	}
	return max(m.height-stepChrome, 3)
}

func (m *Model) summaryView() string {
	s := m.summary
	cash := strings.TrimSpace(m.accounts.Value(app.FieldAccountName(0)))
	bank := strings.TrimSpace(m.accounts.Value(app.FieldAccountName(1)))
	rows := [][2]string{
		{text.SumHouses, strconv.Itoa(s.HouseCount)},
		{text.SumFee, s.FeeAmount.Format()},
		{text.SumBalance(cash), s.OpeningBalances[0].Format()},
		{text.SumBalance(bank), s.OpeningBalances[1].Format()},
		{text.SumBalanceTotal, s.TotalBalance.Format()},
		{text.SumHousesCovered, strconv.Itoa(s.HousesCovered)},
		{text.SumTotalCredit, s.TotalCredit.Format()},
		{text.SumTotalArrears, s.TotalArrears.Format()},
	}
	labelWidth, amountWidth := 0, 0
	for _, r := range rows {
		labelWidth = max(labelWidth, lipgloss.Width(r[0]))
		amountWidth = max(amountWidth, lipgloss.Width(r[1]))
	}
	lines := make([]string, len(rows))
	for i, r := range rows {
		lines[i] = fmt.Sprintf("%s  %*s", theme.Label.Render(fmt.Sprintf("%-*s", labelWidth, r[0])), amountWidth, r[1])
	}
	out := strings.Join(lines, "\n")
	if m.saveErr != "" {
		out += "\n\n" + theme.Error.Render(text.WizardSaveFailed+" "+m.saveErr)
	}
	return out
}
