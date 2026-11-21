// Package directory is the Directorio view: the people, companies and services
// the coto works with. It lists, searches and filters them, fills the detail
// panel, and opens the vendor form and the delete confirmation as overlays.
package directory

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/vicmangontel/munitorum/internal/app"
	"github.com/vicmangontel/munitorum/internal/domain"
	"github.com/vicmangontel/munitorum/internal/tui/text"
	"github.com/vicmangontel/munitorum/internal/tui/theme"
	"github.com/vicmangontel/munitorum/internal/tui/view"
)

// Fixed column widths of the list, in cells; Nombre and Empresa share the rest.
const (
	colKind  = 8
	colPhone = 12
	markerW  = 2 // the "> " marker
	gapsW    = 3 // one space between four columns
	minName  = 6
	minCo    = 6
)

// Results of commands the view started.
type (
	// loadedMsg is the list for one load; gen tells a stale load from the current one.
	loadedMsg struct {
		gen  int
		rows []app.VendorRow
		err  error
	}
	detailMsg struct {
		detail app.VendorDetail
		err    error
	}
	editLoadedMsg struct {
		detail app.VendorDetail
		err    error
	}
	restoredMsg struct{ err error }
)

// kindFilters is the order f cycles through; "" is every kind.
var kindFilters = []domain.VendorKind{"", domain.VendorPerson, domain.VendorCompany, domain.VendorService}

// Model is the Directorio view. It is held by pointer and implements view.View,
// view.Detailer and view.KeyCapturer.
type Model struct {
	svc *app.DirectoryService

	rows     []app.VendorRow
	selected int
	loaded   bool
	gen      int
	reselect int64 // vendor to select after the next load; 0 keeps the current one

	kind        domain.VendorKind
	search      textinput.Model
	searching   bool
	showDeleted bool

	detail *app.VendorDetail // of the selected row; nil until read
	notice string            // why the last action failed
	height int               // rows the panel has for the list
	width  int               // cells the panel has for a list line
}

var (
	_ view.View        = (*Model)(nil)
	_ view.Detailer    = (*Model)(nil)
	_ view.KeyCapturer = (*Model)(nil)
)

// New returns the Directorio view over svc.
func New(svc *app.DirectoryService) *Model {
	in := textinput.New()
	in.Prompt = ""
	in.Cursor.SetMode(cursor.CursorStatic) // no blink timers to drive
	return &Model{svc: svc, search: in, height: 10, width: 90}
}

func (m *Model) Title() string { return text.ViewNames[text.ViewDirectory] }

func (m *Model) CapturesKeys() bool { return m.searching }

func (m *Model) Init() tea.Cmd { return m.load() }

// load reads the list for the current filters.
func (m *Model) load() tea.Cmd {
	m.gen++
	gen := m.gen
	in := app.ListVendorsInput{Kind: m.kind, Search: m.search.Value(), IncludeDeleted: m.showDeleted}
	return func() tea.Msg {
		rows, err := m.svc.ListVendors(context.Background(), in)
		return loadedMsg{gen: gen, rows: rows, err: err}
	}
}

// loadDetail reads the selected row for the detail panel.
func (m *Model) loadDetail() tea.Cmd {
	row, ok := m.current()
	if !ok {
		m.detail = nil
		return nil
	}
	return func() tea.Msg {
		d, err := m.svc.GetVendor(context.Background(), row.ID)
		return detailMsg{detail: d, err: err}
	}
}

func (m *Model) current() (app.VendorRow, bool) {
	if m.selected < 0 || m.selected >= len(m.rows) {
		return app.VendorRow{}, false
	}
	return m.rows[m.selected], true
}

func (m *Model) Update(msg tea.Msg) (view.View, tea.Cmd) {
	switch msg := msg.(type) {
	case view.SizeMsg:
		m.height, m.width = msg.Height, msg.Width
	case loadedMsg:
		return m, m.onLoaded(msg)
	case detailMsg:
		if row, ok := m.current(); ok && msg.err == nil && msg.detail.Vendor.ID == row.ID {
			m.detail = &msg.detail
		}
	case editLoadedMsg:
		if msg.err != nil {
			m.notice = describe(msg.err)
			return m, nil
		}
		form := newVendorForm(m.svc, formOpts{edit: &msg.detail, done: closeAfterSave})
		return m, openOverlay(text.VendorEditTitle, form)
	case SavedMsg:
		m.reselect = msg.Vendor.ID
		return m, m.load()
	case ChangedMsg:
		return m, m.load()
	case restoredMsg:
		if msg.err != nil {
			m.notice = describe(msg.err)
			return m, nil
		}
		return m, tea.Batch(m.load(), send(view.ChangedMsg{}))
	case tea.KeyMsg:
		return m.onKey(msg)
	}
	return m, nil
}

// closeAfterSave is what a form opened from the list does once it saved: tell
// the list, then close the overlay.
func closeAfterSave(v app.Vendor) tea.Cmd {
	return tea.Batch(send(SavedMsg{Vendor: v}), send(view.ChangedMsg{}), send(view.CloseOverlayMsg{}))
}

func (m *Model) onLoaded(msg loadedMsg) tea.Cmd {
	if msg.gen != m.gen {
		return nil // a newer load is on its way
	}
	if msg.err != nil {
		m.notice = text.LoadFailed + " " + msg.err.Error()
		return nil
	}
	keep := m.reselect
	if row, ok := m.current(); ok && keep == 0 {
		keep = row.ID
	}
	m.reselect = 0
	m.rows, m.loaded = msg.rows, true
	m.selected = min(m.selected, max(len(m.rows)-1, 0))
	for i, r := range m.rows {
		if r.ID == keep {
			m.selected = i
		}
	}
	m.detail = nil
	return m.loadDetail()
}

func (m *Model) onKey(msg tea.KeyMsg) (view.View, tea.Cmd) {
	m.notice = ""
	key := msg.String()
	if m.searching {
		return m.onSearchKey(msg)
	}
	switch key {
	case "up", "down":
		return m, m.move(key)
	case "n":
		form := newVendorForm(m.svc, formOpts{done: closeAfterSave})
		return m, openOverlay(text.VendorNewTitle, form)
	case "e":
		if row, ok := m.current(); ok && !row.Deleted {
			return m, func() tea.Msg {
				d, err := m.svc.GetVendor(context.Background(), row.ID)
				return editLoadedMsg{detail: d, err: err}
			}
		}
	case "d":
		if row, ok := m.current(); ok && !row.Deleted {
			return m, openOverlay(text.DeleteTitle, newDeleteConfirm(m.svc, row))
		}
	case "r":
		if row, ok := m.current(); ok && row.Deleted {
			return m, func() tea.Msg {
				return restoredMsg{err: m.svc.RestoreVendor(context.Background(), row.ID)}
			}
		}
	case "/":
		m.searching = true
		m.search.Focus()
	case "f":
		m.kind = nextKind(m.kind)
		return m, m.load()
	case ".":
		m.showDeleted = !m.showDeleted
		return m, m.load()
	}
	return m, nil
}

func (m *Model) move(key string) tea.Cmd {
	if key == "up" {
		m.selected = max(m.selected-1, 0)
	} else {
		m.selected = min(m.selected+1, max(len(m.rows)-1, 0))
	}
	m.detail = nil
	return m.loadDetail()
}

// onSearchKey edits the search text; the list follows every change. Enter keeps
// the filter and leaves the box, esc clears the filter too.
func (m *Model) onSearchKey(msg tea.KeyMsg) (view.View, tea.Cmd) {
	switch msg.String() {
	case "up", "down":
		return m, m.move(msg.String())
	case "enter":
		m.searching = false
		m.search.Blur()
		return m, nil
	case "esc":
		m.searching = false
		m.search.Blur()
		m.search.SetValue("")
		return m, m.load()
	}
	before := m.search.Value()
	var cmd tea.Cmd
	m.search, cmd = m.search.Update(msg)
	if m.search.Value() == before {
		return m, cmd
	}
	return m, tea.Batch(cmd, m.load())
}

func nextKind(k domain.VendorKind) domain.VendorKind {
	for i, f := range kindFilters {
		if f == k {
			return kindFilters[(i+1)%len(kindFilters)]
		}
	}
	return ""
}

func (m *Model) View() string {
	var lines []string
	switch status := m.status(); {
	case m.searching:
		lines = append(lines, theme.Label.Render(text.SearchPrompt)+m.search.View())
	case status != "":
		lines = append(lines, theme.Label.Render(status))
	}
	lines = append(lines, theme.Label.Render(m.columns("  ", text.ColName, text.ColKind, text.ColCompany, text.ColPhone)))

	switch {
	case !m.loaded:
		lines = append(lines, theme.Label.Render(text.DirectoryLoading))
	case len(m.rows) == 0 && m.status() == "":
		lines = append(lines, theme.Label.Render(text.DirectoryEmpty))
	case len(m.rows) == 0:
		lines = append(lines, theme.Label.Render(text.DirectoryNoMatch))
	}
	room := max(m.height-len(lines)-1, 1) // one line is kept for the notice
	from := min(max(m.selected-room+1, 0), max(len(m.rows)-room, 0))
	for i := from; i < min(from+room, len(m.rows)); i++ {
		lines = append(lines, m.row(i))
	}
	if m.notice != "" {
		lines = append(lines, theme.Error.Render(m.notice))
	}
	return strings.Join(lines, "\n")
}

// status describes the filters in force; empty when none is.
func (m *Model) status() string {
	kind := ""
	if m.kind != "" {
		kind = text.VendorKindName(string(m.kind))
	}
	return text.DirectoryFilter(kind, strings.TrimSpace(m.search.Value()), m.showDeleted)
}

func (m *Model) row(i int) string {
	r := m.rows[i]
	line := m.columns("  ", r.DisplayName(), text.VendorKindName(string(r.Kind)), r.CompanyName, r.Phone)
	switch {
	case i == m.selected && r.Deleted:
		return theme.SelectedRow.Strikethrough(true).Render("> " + line[2:])
	case i == m.selected:
		return theme.SelectedRow.Render("> " + line[2:])
	case r.Deleted:
		return theme.Deleted.Render(line)
	}
	return line
}

// columns lays out one list line: marker, then the four columns.
func (m *Model) columns(marker, name, kind, company, phone string) string {
	shared := max(m.width-markerW-gapsW-colKind-colPhone, minName+minCo)
	nameW := max(shared*3/5, minName)
	companyW := max(shared-nameW, minCo)
	return fmt.Sprintf("%s%s %s %s %s", marker, cell(name, nameW), cell(kind, colKind), cell(company, companyW), cell(phone, colPhone))
}

// cell cuts s to w cells with an ellipsis and pads it to exactly w.
func cell(s string, w int) string {
	s = ansi.Truncate(s, w, "…")
	return s + strings.Repeat(" ", w-lipgloss.Width(s))
}

// Detail lists every field of the selected vendor, the people of a company, or
// the company of a person.
func (m *Model) Detail() string {
	row, ok := m.current()
	if !ok || m.detail == nil || m.detail.Vendor.ID != row.ID {
		return ""
	}
	v := m.detail.Vendor
	field := func(label, value string) string {
		if value == "" {
			value = text.DetailNone
		}
		return theme.Label.Render(label+": ") + value
	}
	lines := []string{
		field(text.LabelVendorName, text.WithDeletedSuffix(v.Name, v.Deleted)),
		field(text.LabelVendorKind, text.VendorKindName(string(v.Kind))),
		field(text.LabelVendorPhone, v.Phone),
		field(text.LabelVendorEmail, v.Email),
	}
	if v.Kind == domain.VendorPerson {
		company := ""
		if c := m.detail.Company; c != nil {
			company = text.WithDeletedSuffix(c.Name, c.Deleted)
		}
		lines = append(lines, field(text.LabelVendorCo, company))
	}
	if v.Kind == domain.VendorCompany {
		names := make([]string, len(m.detail.Members))
		for i, p := range m.detail.Members {
			names[i] = p.Name
		}
		lines = append(lines, field(text.DetailMembers, strings.Join(names, ", ")))
	}
	return strings.Join(append(lines, field(text.LabelVendorNotes, v.Notes)), "\n")
}

func (m *Model) KeyHints() []view.Hint {
	if m.searching {
		return []view.Hint{
			{Key: text.KeyEnter, Desc: text.HintApplySearch},
			{Key: text.KeyEsc, Desc: text.HintClearSearch},
		}
	}
	return []view.Hint{
		{Key: text.KeyMove, Desc: text.HintMove},
		{Key: text.KeyNew, Desc: text.HintNew},
		{Key: text.KeyE, Desc: text.HintEdit},
		{Key: text.KeyDelete, Desc: text.HintDelete},
		{Key: text.KeySearch, Desc: text.HintSearch},
		{Key: text.KeyFilter, Desc: text.HintFilterKind},
		{Key: text.KeyDeleted, Desc: text.HintShowDeleted},
		{Key: text.KeyRestore, Desc: text.HintRestore},
	}
}
