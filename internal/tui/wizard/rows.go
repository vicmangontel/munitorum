package wizard

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/vicmangontel/munitorum/internal/app"
	"github.com/vicmangontel/munitorum/internal/domain"
	"github.com/vicmangontel/munitorum/internal/tui/text"
	"github.com/vicmangontel/munitorum/internal/tui/theme"
)

// Columns of a house row.
const (
	colCovered = iota
	colCredit
	colArrears
	colCount
)

var colFields = [colCount]string{domain.OpeningCoveredThrough, domain.OpeningCredit, domain.OpeningArrears}

const (
	cellWidth = 12
	colGap    = "  "
)

// houseRows is the step 6 table. One text input edits the focused cell; the
// other cells are plain text, so 200 rows cost 200 small arrays, not 600 inputs.
type houseRows struct {
	prefix   string
	vals     [][colCount]string // per house, in number order
	errs     map[int][colCount]string
	row, col int
	cell     textinput.Model
}

func newHouseRows() *houseRows {
	in := textinput.New()
	in.Prompt = ""
	in.Width = cellWidth
	in.Cursor.SetMode(cursor.CursorStatic)
	in.Focus()
	return &houseRows{cell: in}
}

// resize makes the table n rows, keeping the values of the houses that still
// exist, and puts the cursor on the first cell.
func (h *houseRows) resize(n int, prefix string) {
	h.prefix = prefix
	next := make([][colCount]string, n)
	copy(next, h.vals)
	h.vals = next
	h.errs = nil
	h.row, h.col = 0, 0
	h.load()
}

// setup is the rows as SetupInput wants them, for n houses.
func (h *houseRows) setup(n int) []app.SetupHouse {
	out := make([]app.SetupHouse, n)
	for i := range out {
		if i < len(h.vals) {
			v := h.vals[i]
			out[i] = app.SetupHouse{CoveredThrough: v[colCovered], Credit: v[colCredit], Arrears: v[colArrears]}
		}
	}
	return out
}

// atStart reports whether key, shift+tab or ←, leaves the table backwards: only
// from the first cell of the first row, and ← only with the cursor before its
// first character.
func (h *houseRows) atStart(key string) bool {
	if h.row != 0 || h.col != 0 {
		return false
	}
	return key == "shift+tab" || h.cell.Position() == 0
}

func (h *houseRows) load() {
	h.cell.SetValue(h.vals[h.row][h.col])
	h.cell.CursorEnd()
}

func (h *houseRows) moveTo(row, col int) {
	h.row, h.col = row, col
	h.load()
}

func (h *houseRows) update(msg tea.KeyMsg) {
	switch msg.String() {
	case "up":
		if h.row > 0 {
			h.moveTo(h.row-1, h.col)
		}
	case "down":
		if h.row < len(h.vals)-1 {
			h.moveTo(h.row+1, h.col)
		}
	case "tab":
		switch {
		case h.col < colCount-1:
			h.moveTo(h.row, h.col+1)
		case h.row < len(h.vals)-1:
			h.moveTo(h.row+1, 0)
		}
	case "shift+tab":
		switch {
		case h.col > 0:
			h.moveTo(h.row, h.col-1)
		case h.row > 0:
			h.moveTo(h.row-1, colCount-1)
		}
	default:
		h.cell, _ = h.cell.Update(msg)
		h.vals[h.row][h.col] = h.cell.Value()
	}
}

// setErrors shows the errors of every row and puts the cursor on the first
// invalid cell.
func (h *houseRows) setErrors(ve app.ValidationError) {
	h.errs = map[int][colCount]string{}
	firstRow, firstCol := -1, 0
	for row := range h.vals {
		var msgs [colCount]string
		for col, field := range colFields {
			code, bad := ve.Fields[app.FieldHouse(row, field)]
			if !bad {
				continue
			}
			msgs[col] = text.ValidationMessage(code)
			if firstRow < 0 {
				firstRow, firstCol = row, col
			}
		}
		if msgs != ([colCount]string{}) {
			h.errs[row] = msgs
		}
	}
	if firstRow >= 0 {
		h.moveTo(firstRow, firstCol)
	}
}

// view renders at most visible rows around the focused one.
func (h *houseRows) view(visible int) string {
	total := len(h.vals)
	start := max(0, min(h.row-visible/2, total-visible))
	end := min(start+visible, total)

	codeWidth := len(domain.HouseCode(h.prefix, total, total))
	header := fmt.Sprintf("  %-*s%s%-*s%s%-*s%s%-*s",
		codeWidth, text.ColHouse, colGap,
		cellWidth+1, text.ColCoveredThrough, colGap,
		cellWidth+1, text.ColCredit, colGap,
		cellWidth+1, text.ColArrears)

	lines := []string{theme.Label.Render(header)}
	for i := start; i < end; i++ {
		lines = append(lines, h.rowLine(i, codeWidth))
	}
	lines = append(lines, "",
		theme.Label.Render(text.HousesRowsHelp),
		theme.Label.Render(text.HousesRowPosition(start+1, end, total)))
	return strings.Join(lines, "\n")
}

func (h *houseRows) rowLine(i, codeWidth int) string {
	code := domain.HouseCode(h.prefix, i+1, len(h.vals))
	cells := make([]string, colCount)
	for col := range cells {
		if i == h.row && col == h.col {
			cells[col] = h.cell.View()
		} else {
			cells[col] = fmt.Sprintf("%-*s", cellWidth+1, h.vals[i][col])
		}
	}
	marker := "  "
	if i == h.row {
		marker = "> "
		code = theme.SelectedRow.Render(fmt.Sprintf("%-*s", codeWidth, code))
	} else {
		code = fmt.Sprintf("%-*s", codeWidth, code)
	}
	line := marker + code + colGap + strings.Join(cells, colGap)
	if msgs, bad := h.errs[i]; bad {
		var shown []string
		for _, msg := range msgs {
			if msg != "" {
				shown = append(shown, msg)
			}
		}
		line += "\n" + strings.Repeat(" ", 4) + theme.Error.Render(strings.Join(shown, " "))
	}
	return line
}
