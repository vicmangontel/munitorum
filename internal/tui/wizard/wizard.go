// Package wizard is the seven-step setup wizard. It runs full-screen, keeps
// every value in memory, and writes nothing until the summary is confirmed.
package wizard

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vicmangontel/munitorum/internal/app"
	"github.com/vicmangontel/munitorum/internal/domain"
	"github.com/vicmangontel/munitorum/internal/tui/confirm"
	"github.com/vicmangontel/munitorum/internal/tui/form"
	"github.com/vicmangontel/munitorum/internal/tui/text"
	"github.com/vicmangontel/munitorum/internal/tui/theme"
	"github.com/vicmangontel/munitorum/internal/tui/view"
)

// Steps, in order.
const (
	stepCommunity = iota
	stepHouses
	stepFee
	stepAccounts
	stepOpening
	stepHouseRows
	stepSummary
	stepCount
)

// DoneMsg is sent when the coto has been written. The root model replaces the
// wizard with the shell.
type DoneMsg struct{ Settings app.Settings }

// validatedMsg is the result of ValidateSetup for the step it was run on.
type validatedMsg struct {
	step    int
	summary app.SetupSummary
	err     error
}

// savedMsg is the result of CompleteSetup.
type savedMsg struct {
	settings app.Settings
	err      error
}

// Model is the wizard.
type Model struct {
	svc           *app.SetupService
	width, height int
	step          int

	community, houses, fee, accounts, opening *form.Form
	rows                                      *houseRows

	summary app.SetupSummary
	saveErr string // set when writing failed for a reason other than invalid fields
	saving  bool
	cancel  *confirm.Model // non-nil while "cancel the wizard?" is open
}

// New returns the wizard at step 1 with the defaults filled in; today prefills
// the opening date.
func New(svc *app.SetupService, today time.Time) *Model {
	m := &Model{svc: svc}
	m.community = form.New(form.Field{Key: app.FieldCommunityName, Label: text.LabelCommunityName})
	m.houses = form.New(
		form.Field{Key: app.FieldHouseCount, Label: text.LabelHouseCount, Value: strconv.Itoa(domain.DefaultHouseCount)},
		form.Field{Key: app.FieldHousePrefix, Label: text.LabelHousePrefix, Value: domain.DefaultHousePrefix},
	)
	m.fee = form.New(form.Field{Key: app.FieldFeeAmount, Label: text.LabelFee})
	m.accounts = form.New(
		form.Field{Key: app.FieldAccountName(0), Label: text.LabelCashAccount, Value: text.DefaultCashAccount},
		form.Field{Key: app.FieldAccountName(1), Label: text.LabelBankAccount, Value: text.DefaultBankAccount},
	)
	m.opening = form.New(
		form.Field{Key: app.FieldOpeningDate, Label: text.LabelOpeningDate, Value: domain.FormatInputDate(today)},
		form.Field{Key: app.FieldOpeningBalance(0), Value: "0.00"},
		form.Field{Key: app.FieldOpeningBalance(1), Value: "0.00"},
	)
	m.rows = newHouseRows()
	return m
}

func (m *Model) Init() tea.Cmd { return nil }

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case validatedMsg:
		return m, m.onValidated(msg)
	case savedMsg:
		return m, m.onSaved(msg)
	case confirm.ResultMsg:
		m.cancel = nil
		if msg.Yes {
			return m, tea.Quit // rule 11: nothing was written
		}
	case tea.KeyMsg:
		return m, m.onKey(msg)
	}
	return m, nil
}

func (m *Model) onKey(msg tea.KeyMsg) tea.Cmd {
	key := msg.String()
	if key == "ctrl+c" {
		return tea.Quit
	}
	if m.saving {
		return nil
	}
	if m.cancel != nil {
		return m.cancel.Update(msg)
	}
	if key == "esc" {
		m.cancel = confirm.New(text.WizardCancelPrompt)
		return nil
	}
	if m.step == stepSummary {
		switch key {
		case "enter":
			m.saving, m.saveErr = true, ""
			return m.complete()
		case "left", "shift+tab":
			m.step = stepHouseRows
		}
		return nil
	}

	if key == "enter" {
		return m.validate()
	}
	if m.goesBack(key) {
		m.step--
		return nil
	}
	if m.step == stepHouseRows {
		m.rows.update(msg)
		return nil
	}
	return m.current().Update(msg)
}

// goesBack reports whether key leaves the current step backwards: shift+tab on
// the first field, or ← with the cursor before its first character (anywhere
// else those keys move inside the step).
func (m *Model) goesBack(key string) bool {
	if m.step == stepCommunity || (key != "left" && key != "shift+tab") {
		return false
	}
	if m.step == stepHouseRows {
		return m.rows.atStart(key)
	}
	if key == "shift+tab" {
		return m.current().OnFirstField()
	}
	return m.current().AtStart()
}

// current is the form of the current step; the house rows and the summary have none.
func (m *Model) current() *form.Form {
	switch m.step {
	case stepCommunity:
		return m.community
	case stepHouses:
		return m.houses
	case stepFee:
		return m.fee
	case stepAccounts:
		return m.accounts
	default:
		return m.opening
	}
}

// input assembles what the treasurer has typed so far. Rows are sized to the
// house count; an unreadable count leaves them empty for ValidateSetup to report.
func (m *Model) input() app.SetupInput {
	in := app.SetupInput{
		CommunityName: m.community.Value(app.FieldCommunityName),
		HouseCount:    m.houses.Value(app.FieldHouseCount),
		HousePrefix:   m.houses.Value(app.FieldHousePrefix),
		FeeAmount:     m.fee.Value(app.FieldFeeAmount),
		OpeningDate:   m.opening.Value(app.FieldOpeningDate),
	}
	for i := range in.Accounts {
		in.Accounts[i] = app.SetupAccount{
			Name:           m.accounts.Value(app.FieldAccountName(i)),
			OpeningBalance: m.opening.Value(app.FieldOpeningBalance(i)),
		}
	}
	if n, err := domain.ParseHouseCount(in.HouseCount); err == nil {
		in.Houses = m.rows.setup(n)
	}
	return in
}

// validate runs ValidateSetup on everything typed so far.
func (m *Model) validate() tea.Cmd {
	step, in := m.step, m.input()
	return func() tea.Msg {
		summary, err := m.svc.ValidateSetup(in)
		return validatedMsg{step: step, summary: summary, err: err}
	}
}

func (m *Model) complete() tea.Cmd {
	in := m.input()
	return func() tea.Msg {
		settings, err := m.svc.CompleteSetup(context.Background(), in)
		return savedMsg{settings: settings, err: err}
	}
}

// onValidated advances when the step's own fields are valid. Errors in fields
// of later steps are expected until the treasurer reaches them.
func (m *Model) onValidated(msg validatedMsg) tea.Cmd {
	var ve app.ValidationError
	if msg.err != nil && !errors.As(msg.err, &ve) {
		m.saveErr = msg.err.Error()
		return nil
	}
	if msg.step != m.step {
		return nil
	}
	if msg.step == stepHouseRows {
		if len(ve.Fields) > 0 {
			m.showErrors(ve)
			return nil
		}
		m.summary = msg.summary
		m.enter(stepSummary)
		return nil
	}
	if m.current().SetErrors(ve) {
		return nil
	}
	m.enter(msg.step + 1)
	return nil
}

// onSaved opens the shell on success. Invalid fields send the wizard to the
// step holding the first one; any other error is shown on the summary.
func (m *Model) onSaved(msg savedMsg) tea.Cmd {
	m.saving = false
	if msg.err == nil {
		return func() tea.Msg { return DoneMsg{Settings: msg.settings} }
	}
	var ve app.ValidationError
	if errors.As(msg.err, &ve) {
		m.showErrors(ve)
		return nil
	}
	m.saveErr = msg.err.Error()
	return nil
}

// showErrors moves to the step holding the first invalid field and shows the
// errors of that step.
func (m *Model) showErrors(ve app.ValidationError) {
	first := stepCount
	for key := range ve.Fields {
		first = min(first, stepOf(key))
	}
	m.step = first
	if first == stepHouseRows {
		m.rows.setErrors(ve)
		return
	}
	m.current().SetErrors(ve)
}

// enter moves forward to step, preparing what that step shows.
func (m *Model) enter(step int) {
	m.step = step
	switch step {
	case stepOpening:
		for i := range 2 {
			name := strings.TrimSpace(m.accounts.Value(app.FieldAccountName(i)))
			m.opening.SetLabel(app.FieldOpeningBalance(i), text.LabelOpeningBalance(name))
		}
	case stepHouseRows:
		if n, err := domain.ParseHouseCount(m.houses.Value(app.FieldHouseCount)); err == nil {
			m.rows.resize(n, m.houses.Value(app.FieldHousePrefix))
		}
	}
}

// stepOf is the step that holds a ValidationError field.
func stepOf(key string) int {
	switch {
	case key == app.FieldCommunityName:
		return stepCommunity
	case key == app.FieldHouseCount || key == app.FieldHousePrefix:
		return stepHouses
	case key == app.FieldFeeAmount:
		return stepFee
	case key == app.FieldAccountName(0) || key == app.FieldAccountName(1):
		return stepAccounts
	case key == app.FieldOpeningDate || key == app.FieldOpeningBalance(0) || key == app.FieldOpeningBalance(1):
		return stepOpening
	default:
		return stepHouseRows
	}
}

func (m *Model) View() string {
	var body string
	var hints []view.Hint
	switch {
	case m.cancel != nil:
		body, hints = m.cancel.View(), m.cancel.KeyHints()
	case m.saving:
		body = text.WizardSaving
	default:
		body, hints = m.stepView()
	}

	title := theme.Title.Render(text.WizardStepTitle(m.step + 1))
	style := theme.Overlay
	if m.width > 0 {
		style = style.MaxWidth(m.width)
	}
	box := style.Render(title + "\n\n" + body)
	return box + "\n" + m.hintLine(hints)
}

func (m *Model) hintLine(hints []view.Hint) string {
	parts := make([]string, len(hints))
	for i, h := range hints {
		parts[i] = h.Key + " " + h.Desc
	}
	return theme.KeyHint.Render(strings.Join(parts, "  "))
}
