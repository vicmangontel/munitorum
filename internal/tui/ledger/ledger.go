// Package ledger is the Movimientos view: the libro de caja of one month with its
// running balance and totals. It opens the income and expense forms as overlays
// and tells the root when something was saved.
package ledger

import (
	"context"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/vicmangontel/munitorum/internal/app"
	"github.com/vicmangontel/munitorum/internal/domain"
	"github.com/vicmangontel/munitorum/internal/tui/picker"
	"github.com/vicmangontel/munitorum/internal/tui/text"
	"github.com/vicmangontel/munitorum/internal/tui/theme"
	"github.com/vicmangontel/munitorum/internal/tui/view"
)

// Fixed column widths of the list, in cells; the four text columns share the rest.
const (
	colDate    = 6  // 05 nov
	colAmount  = 11 // −$12,345.00
	colBalance = 12
	colReceipt = 1
	markerW    = 2 // the "> " marker
	gapsW      = 7 // one space between eight columns
	minShared  = 1 // each text column keeps at least a cell, so a line never outgrows the panel
)

// Results of commands the view started.
type (
	// loadedMsg is the ledger of one load; gen tells a stale load from the current one.
	loadedMsg struct {
		gen    int
		ledger app.Ledger
		err    error
	}
	// formDataMsg is what a form needs before it can open.
	formDataMsg struct {
		kind       domain.MovementKind
		today      time.Time
		accounts   []app.Account
		categories []app.Category
		vendors    []picker.Item
		err        error
	}
)

// Model is the Movimientos view. It is held by pointer and implements view.View.
type Model struct {
	svc   *app.LedgerService
	cats  *app.CategoryService
	dir   *app.DirectoryService
	clock app.Clock

	month    domain.Month
	data     app.Ledger
	loaded   bool
	gen      int
	selected int

	notice string // why the last action failed
	height int    // rows the panel has
	width  int    // cells the panel has for a line
}

var _ view.View = (*Model)(nil)

// New returns the Movimientos view over the services, showing the month of the clock's today.
func New(svc *app.LedgerService, cats *app.CategoryService, dir *app.DirectoryService, clock app.Clock) *Model {
	return &Model{svc: svc, cats: cats, dir: dir, clock: clock, month: domain.Of(clock.Now()), height: 10, width: 90}
}

func (m *Model) Title() string { return text.ViewNames[text.ViewLedger] }

func (m *Model) Init() tea.Cmd { return m.load() }

// load reads the ledger of the shown month.
func (m *Model) load() tea.Cmd {
	m.gen++
	gen, month := m.gen, m.month
	return func() tea.Msg {
		l, err := m.svc.ListMovements(context.Background(), app.LedgerFilter{Month: month})
		return loadedMsg{gen: gen, ledger: l, err: err}
	}
}

// prepare reads what the form of kind offers and then opens it.
func (m *Model) prepare(kind domain.MovementKind) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		today := m.clock.Now()
		balances, err := m.svc.AccountBalances(ctx, today)
		if err != nil {
			return formDataMsg{kind: kind, err: err}
		}
		categories, err := m.cats.ListCategories(ctx, app.ListCategoriesInput{Direction: kind})
		if err != nil {
			return formDataMsg{kind: kind, err: err}
		}
		data := formDataMsg{kind: kind, today: today, categories: categories}
		for _, b := range balances.Accounts {
			data.accounts = append(data.accounts, b.Account)
		}
		if kind == domain.MovementExpense {
			vendors, err := m.dir.ListVendors(ctx, app.ListVendorsInput{})
			if err != nil {
				return formDataMsg{kind: kind, err: err}
			}
			for _, v := range vendors {
				data.vendors = append(data.vendors, picker.Item{ID: v.ID, Label: v.DisplayName()})
			}
		}
		return data
	}
}

func (m *Model) Update(msg tea.Msg) (view.View, tea.Cmd) {
	switch msg := msg.(type) {
	case view.SizeMsg:
		m.height, m.width = msg.Height, msg.Width
	case loadedMsg:
		m.onLoaded(msg)
	case view.ChangedMsg:
		return m, m.load()
	case formDataMsg:
		if msg.err != nil {
			m.notice = describe(msg.err)
			return m, nil
		}
		title := text.IncomeTitle
		if msg.kind == domain.MovementExpense {
			title = text.ExpenseTitle
		}
		return m, openOverlay(title, newMovementForm(m.svc, msg))
	case tea.KeyMsg:
		return m.onKey(msg)
	}
	return m, nil
}

func (m *Model) onLoaded(msg loadedMsg) {
	if msg.gen != m.gen {
		return // a newer load is on its way
	}
	if msg.err != nil {
		m.notice = text.LedgerLoadFailed + " " + msg.err.Error()
		return
	}
	m.data, m.loaded = msg.ledger, true
	m.selected = max(len(m.data.Rows)-1, 0) // the cursor starts on the last row
}

func (m *Model) onKey(msg tea.KeyMsg) (view.View, tea.Cmd) {
	m.notice = ""
	switch msg.String() {
	case "up":
		m.selected = max(m.selected-1, 0)
	case "down":
		m.selected = min(m.selected+1, max(len(m.data.Rows)-1, 0))
	case "[":
		m.month = m.month.Add(-1)
		return m, m.load()
	case "]":
		m.month = m.month.Add(1)
		return m, m.load()
	case "i":
		return m, m.prepare(domain.MovementIncome)
	case "g":
		return m, m.prepare(domain.MovementExpense)
	}
	return m, nil
}

func (m *Model) View() string {
	lines := []string{theme.Label.Render(text.LedgerPeriod(
		text.MonthYear(m.month.Year, m.month.Month), m.data.OpeningBalance.Format(), m.data.ClosingBalance.Format()))}
	w := m.widths()
	lines = append(lines, theme.Label.Render(m.line("  ", w, text.ColDate, text.ColCategory, text.ColParty,
		text.ColDescription, text.ColAccount, rightCell(text.ColAmount, colAmount), rightCell(text.ColBalance, colBalance), text.ColReceipt)))

	if m.loaded && len(m.data.Rows) == 0 {
		lines = append(lines, theme.Label.Render(text.LedgerEmpty))
	}
	room := max(m.height-len(lines)-2, 1) // the totals and the notice keep a line each
	from := min(max(m.selected-room+1, 0), max(len(m.data.Rows)-room, 0))
	for i := from; i < min(from+room, len(m.data.Rows)); i++ {
		lines = append(lines, m.row(i, w))
	}
	t := m.data.Totals
	lines = append(lines, theme.AmountTotal.Render(text.LedgerTotals(
		t.Income.Format(), t.Expense.Format(), t.Adjustments.Format(), t.Net.Format())))
	if m.notice != "" {
		lines = append(lines, theme.Error.Render(m.notice))
	}
	return strings.Join(lines, "\n")
}

// colWidths are the widths of the four text columns for the panel's width.
type colWidths struct{ category, party, description, account int }

func (m *Model) widths() colWidths {
	shared := max(m.width-markerW-gapsW-colDate-colAmount-colBalance-colReceipt, 4*minShared)
	w := colWidths{category: max(shared/5, minShared), party: max(shared/5, minShared), account: max(shared/5, minShared)}
	w.description = max(shared-w.category-w.party-w.account, minShared)
	return w
}

// row is one list line. The selected row is drawn in one style; the others
// colour the amount by direction.
func (m *Model) row(i int, w colWidths) string {
	r := m.data.Rows[i]
	amount, style := amountText(r.Movement)
	party := text.WithDeletedSuffix(r.Vendor, r.VendorDeleted)
	receipt := " "
	switch r.Movement.Receipt {
	case app.ReceiptPending:
		receipt = text.ReceiptPendingMark
	case app.ReceiptDone:
		receipt = text.ReceiptDoneMark
	}

	cells := []string{
		text.DayMonth(r.Movement.Date), r.Category, party, r.Movement.Description, r.Account,
		rightCell(amount, colAmount), rightCell(r.RunningBalance.Format(), colBalance), receipt,
	}
	if i == m.selected {
		return theme.SelectedRow.Render("> " + m.line("", w, cells...))
	}
	// The amount is the sixth cell: pad it plain, then colour only its text.
	pad := strings.Repeat(" ", colAmount-lipgloss.Width(amount))
	cells[5] = pad + style.Render(amount)
	return m.line("  ", w, cells...)
}

// line joins the eight cells of a line, cutting and padding the text columns to
// their widths. The date, amount, balance and receipt cells are used as given.
func (m *Model) line(marker string, w colWidths, cells ...string) string {
	return marker + strings.Join([]string{
		cell(cells[0], colDate), cell(cells[1], w.category), cell(cells[2], w.party),
		cell(cells[3], w.description), cell(cells[4], w.account),
		cells[5], cells[6], cells[7],
	}, " ")
}

// amountText is the signed amount of a movement and the style it is drawn in:
// income +$150.00 Positive, expense −$350.00 Negative.
func amountText(m app.Movement) (string, lipgloss.Style) {
	switch m.Kind {
	case domain.MovementIncome:
		return "+" + m.Amount.Format(), theme.AmountIncome
	case domain.MovementExpense:
		return (-m.Amount).Format(), theme.AmountExpense
	}
	return m.Amount.Format(), lipgloss.NewStyle()
}

// cell cuts s to w cells with an ellipsis and pads it to exactly w.
func cell(s string, w int) string {
	s = ansi.Truncate(s, w, "…")
	return s + strings.Repeat(" ", max(w-lipgloss.Width(s), 0))
}

// rightCell pads s on the left to w cells, so amounts line up on their last digit.
func rightCell(s string, w int) string {
	return strings.Repeat(" ", max(w-lipgloss.Width(s), 0)) + s
}

func (m *Model) KeyHints() []view.Hint {
	return []view.Hint{
		{Key: text.KeyMove, Desc: text.HintMove},
		{Key: text.KeyIncome, Desc: text.HintIncome},
		{Key: text.KeyExpense, Desc: text.HintExpense},
		{Key: text.KeyMonths, Desc: text.HintMonth},
	}
}
