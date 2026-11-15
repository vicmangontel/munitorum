// Package settings is the Ajustes overlay: it edits the coto name, the account
// names and the opening balances. It runs inside the overlay host and sends
// SavedMsg after every successful save so the shell can reload its summary.
package settings

import (
	"context"
	"errors"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vicmangontel/munitorum/internal/app"
	"github.com/vicmangontel/munitorum/internal/tui/confirm"
	"github.com/vicmangontel/munitorum/internal/tui/form"
	"github.com/vicmangontel/munitorum/internal/tui/text"
	"github.com/vicmangontel/munitorum/internal/tui/theme"
	"github.com/vicmangontel/munitorum/internal/tui/view"
)

// SavedMsg is sent after a change was written, with the settings as stored.
type SavedMsg struct{ Settings app.Settings }

// validatedMsg is the result of validating a new opening balance.
type validatedMsg struct{ err error }

// failedMsg is the result of a write that failed.
type failedMsg struct{ err error }

// awaiting is the operation whose result the overlay is waiting for. Results
// that arrive while it is not waiting for them are stale and are ignored.
type awaiting int

const (
	awaitingNothing awaiting = iota
	awaitingValidation
	awaitingWrite
)

// kind is what a row of the list edits.
type kind int

const (
	kindCommunity kind = iota
	kindAccountName
	kindOpeningBalance
)

// row is one editable value; account is the index in Settings.Accounts.
type row struct {
	kind    kind
	account int
}

// Model is the overlay content. It implements view.Content.
type Model struct {
	svc      *app.SetupService
	settings app.Settings
	rows     []row
	selected int

	form    *form.Form // non-nil while a value is being edited
	editing row
	confirm *confirm.Model // non-nil while the new opening balance awaits confirmation
	failure string         // a save error that is not a validation error
	waiting awaiting       // keys are ignored while an operation is in flight
}

var _ view.Content = (*Model)(nil)

// New returns the overlay for the given settings.
func New(svc *app.SetupService, s app.Settings) *Model {
	m := &Model{svc: svc}
	m.load(s)
	return m
}

func (m *Model) load(s app.Settings) {
	m.settings = s
	m.rows = []row{{kind: kindCommunity}}
	for i := range s.Accounts {
		m.rows = append(m.rows, row{kind: kindAccountName, account: i})
	}
	for i := range s.Accounts {
		m.rows = append(m.rows, row{kind: kindOpeningBalance, account: i})
	}
	m.selected = min(m.selected, len(m.rows)-1)
}

func (m *Model) label(r row) string {
	switch r.kind {
	case kindCommunity:
		return text.LabelCommunityName
	case kindAccountName:
		if r.account == 0 {
			return text.LabelCashAccount
		}
		return text.LabelBankAccount
	default:
		return text.LabelOpeningBalance(m.settings.Accounts[r.account].Name)
	}
}

func (m *Model) value(r row) string {
	switch r.kind {
	case kindCommunity:
		return m.settings.CommunityName
	case kindAccountName:
		return m.settings.Accounts[r.account].Name
	default:
		return m.settings.Accounts[r.account].OpeningBalance.Format()
	}
}

// key is the validation field key the operations report r's errors under.
func key(r row) string {
	switch r.kind {
	case kindCommunity:
		return app.FieldCommunityName
	case kindAccountName:
		return app.FieldAccountName(r.account)
	default:
		return app.FieldOpeningBalance(r.account)
	}
}

func (m *Model) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.onKey(msg)
	case validatedMsg:
		return m.onValidated(msg)
	case confirm.ResultMsg:
		return m.onAnswer(msg)
	case SavedMsg:
		m.onSaved(msg)
	case failedMsg:
		m.onFailed(msg)
	}
	return nil
}

func (m *Model) onKey(msg tea.KeyMsg) tea.Cmd {
	switch {
	case m.waiting != awaitingNothing:
		return nil
	case m.confirm != nil:
		return m.confirm.Update(msg)
	case m.form != nil:
		if msg.String() == "enter" {
			return m.submit()
		}
		return m.form.Update(msg)
	}
	switch msg.String() {
	case "up":
		m.selected = max(m.selected-1, 0)
	case "down":
		m.selected = min(m.selected+1, len(m.rows)-1)
	case "e":
		m.startEdit()
	}
	return nil
}

func (m *Model) startEdit() {
	m.editing = m.rows[m.selected]
	m.failure = ""
	m.form = form.New(form.Field{Key: key(m.editing), Label: m.label(m.editing), Value: m.value(m.editing)})
}

// submit saves a name straight away; a new opening balance is validated first
// and then confirmed, because every balance of the coto changes with it.
func (m *Model) submit() tea.Cmd {
	typed := m.form.Value(key(m.editing))
	r, account := m.editing, m.settings.Accounts[m.editing.account]
	svc := m.svc
	m.failure = ""

	switch r.kind {
	case kindCommunity:
		m.waiting = awaitingWrite
		return func() tea.Msg {
			s, err := svc.UpdateCommunityName(context.Background(), typed)
			if err != nil {
				return failedMsg{err}
			}
			return SavedMsg{Settings: s}
		}
	case kindAccountName:
		m.waiting = awaitingWrite
		return func() tea.Msg {
			_, err := svc.RenameAccount(context.Background(), app.RenameAccountInput{AccountID: account.ID, Name: typed})
			return reloaded(svc, err)
		}
	default:
		m.waiting = awaitingValidation
		return func() tea.Msg {
			return validatedMsg{err: svc.ValidateOpeningBalance(context.Background(), app.OpeningBalanceInput{AccountID: account.ID, Amount: typed})}
		}
	}
}

// onValidated asks for confirmation when the new opening balance is valid.
func (m *Model) onValidated(msg validatedMsg) tea.Cmd {
	if m.waiting != awaitingValidation {
		return nil
	}
	m.waiting = awaitingNothing
	if msg.err != nil {
		m.showError(msg.err)
		return nil
	}
	account := m.settings.Accounts[m.editing.account]
	to := m.form.Value(key(m.editing))
	m.confirm = confirm.New(text.ConfirmOpeningBalance(account.Name, account.OpeningBalance.Format(), strings.TrimSpace(to)))
	return nil
}

// onAnswer writes the new opening balance on Sí; No returns to the list and
// writes nothing.
func (m *Model) onAnswer(msg confirm.ResultMsg) tea.Cmd {
	if m.confirm == nil {
		return nil
	}
	m.confirm = nil
	if !msg.Yes {
		m.form = nil
		return nil
	}
	typed := m.form.Value(key(m.editing))
	account, svc := m.settings.Accounts[m.editing.account], m.svc
	m.waiting = awaitingWrite
	return func() tea.Msg {
		_, err := svc.UpdateOpeningBalance(context.Background(), app.OpeningBalanceInput{AccountID: account.ID, Amount: typed})
		return reloaded(svc, err)
	}
}

// reloaded is the result of a write: the stored settings, or the failure.
func reloaded(svc *app.SetupService, err error) tea.Msg {
	if err != nil {
		return failedMsg{err}
	}
	s, err := svc.GetSettings(context.Background())
	if err != nil {
		return failedMsg{err}
	}
	return SavedMsg{Settings: s}
}

// onSaved shows the stored settings. They are current whenever they arrive, so
// they are applied even when the overlay no longer waits for them.
func (m *Model) onSaved(msg SavedMsg) {
	m.waiting = awaitingNothing
	m.form, m.confirm = nil, nil
	m.load(msg.Settings)
}

func (m *Model) onFailed(msg failedMsg) {
	if m.waiting != awaitingWrite {
		return
	}
	m.waiting = awaitingNothing
	m.showError(msg.err)
}

// showError puts invalid fields under the form; any other error is shown as text.
func (m *Model) showError(err error) {
	var ve app.ValidationError
	if errors.As(err, &ve) && m.form.SetErrors(ve) {
		return
	}
	m.failure = err.Error()
}

func (m *Model) View() string {
	switch {
	case m.confirm != nil:
		return m.confirm.View()
	case m.form != nil:
		out := m.form.View()
		if m.failure != "" {
			out += "\n" + theme.Error.Render(text.SettingsSaveFailed+" "+m.failure)
		}
		return out
	}
	labelWidth := 0
	for _, r := range m.rows {
		labelWidth = max(labelWidth, len([]rune(m.label(r))))
	}
	lines := make([]string, len(m.rows))
	for i, r := range m.rows {
		label := m.label(r)
		line := label + strings.Repeat(" ", labelWidth-len([]rune(label))) + "  " + m.value(r)
		if i == m.selected {
			lines[i] = theme.SelectedRow.Render("> " + line)
		} else {
			lines[i] = "  " + line
		}
	}
	return strings.Join(lines, "\n")
}

func (m *Model) KeyHints() []view.Hint {
	switch {
	case m.confirm != nil:
		return []view.Hint{{Key: text.KeyLeftRight, Desc: text.HintChoose}, {Key: text.KeyEnter, Desc: text.HintAccept}}
	case m.form != nil:
		return []view.Hint{{Key: text.KeyEnter, Desc: text.HintSave}}
	}
	return []view.Hint{{Key: text.KeyMove, Desc: text.HintMove}, {Key: text.KeyE, Desc: text.HintEdit}}
}
