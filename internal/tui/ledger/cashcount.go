package ledger

import (
	"context"
	"errors"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vicmangontel/munitorum/internal/app"
	"github.com/vicmangontel/munitorum/internal/domain"
	"github.com/vicmangontel/munitorum/internal/tui/form"
	"github.com/vicmangontel/munitorum/internal/tui/text"
	"github.com/vicmangontel/munitorum/internal/tui/theme"
	"github.com/vicmangontel/munitorum/internal/tui/view"
)

// Results of commands the Arqueo overlay started, tagged with the overlay that
// started them.
type (
	// balanceMsg is the system balance of one account on one date; key is the
	// account and date it was read for, so a late answer for older input is ignored.
	balanceMsg struct {
		target  *countForm
		key     string
		balance domain.Money
		err     error
	}
	countWrittenMsg struct {
		target *countForm
		result app.CashCountResult
		err    error
	}
)

// countForm is the Arqueo overlay: account, date and counted amount, with the
// system balance of that account on that date and the difference shown as the
// amount is typed. It implements view.Content.
type countForm struct {
	svc      *app.LedgerService
	accounts []app.Account

	form      *form.Form
	system    domain.Money // of the account and date in loadedKey
	known     bool         // system has been read for loadedKey
	loadedKey string
	waiting   bool // the write is in flight
	failure   string
}

var _ view.Content = (*countForm)(nil)

// newCountForm opens on the first account (the cash box) and today, whose
// system balance the caller has already read.
func newCountForm(svc *app.LedgerService, data formDataMsg) *countForm {
	f := &countForm{svc: svc, accounts: data.accounts}
	var names []string
	for _, a := range data.accounts {
		names = append(names, a.Name)
	}
	f.form = form.New(
		form.Field{Key: app.FieldDate, Label: text.LabelDate, Value: domain.FormatInputDate(data.today)},
		form.Field{Key: app.FieldAccount, Label: text.LabelAccount, Choices: names},
		form.Field{Key: app.FieldAmount, Label: text.LabelCounted},
	)
	f.loadedKey = f.key()
	if len(data.balances) > 0 {
		f.system, f.known = data.balances[0].Balance, true
	}
	return f
}

// key identifies the account and date the system balance depends on.
func (f *countForm) key() string {
	return f.form.Value(app.FieldAccount) + "|" + f.form.Value(app.FieldDate)
}

func (f *countForm) accountID() int64 {
	for _, a := range f.accounts {
		if a.Name == f.form.Value(app.FieldAccount) {
			return a.ID
		}
	}
	return 0
}

// reload reads the system balance when the account or the date changed.
func (f *countForm) reload() tea.Cmd {
	key := f.key()
	if key == f.loadedKey {
		return nil
	}
	f.loadedKey, f.known = key, false
	date, err := domain.ParseInputDate(f.form.Value(app.FieldDate))
	if err != nil {
		return nil // nothing to read until the date is complete
	}
	account := f.accountID()
	return func() tea.Msg {
		balances, err := f.svc.AccountBalances(context.Background(), date)
		msg := balanceMsg{target: f, key: key, err: err}
		for _, b := range balances.Accounts {
			if b.Account.ID == account {
				msg.balance = b.Balance
			}
		}
		return msg
	}
}

func (f *countForm) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case balanceMsg:
		if msg.target == f && msg.key == f.key() {
			if msg.err != nil {
				f.failure = describe(msg.err)
				return nil
			}
			f.system, f.known = msg.balance, true
		}
	case countWrittenMsg:
		if msg.target == f {
			return f.onWritten(msg)
		}
	case tea.KeyMsg:
		return f.onKey(msg)
	}
	return nil
}

func (f *countForm) onKey(msg tea.KeyMsg) tea.Cmd {
	switch {
	case f.waiting:
		return nil
	case msg.String() == "enter":
		return f.submit()
	}
	cmd := f.form.Update(msg)
	return tea.Batch(cmd, f.reload())
}

func (f *countForm) submit() tea.Cmd {
	in := app.CashCountInput{
		Date: f.form.Value(app.FieldDate), AccountID: f.accountID(), Counted: f.form.Value(app.FieldAmount),
		Describe: func(counted, system domain.Money) string {
			return text.CashCountDescription(counted.Format(), system.Format())
		},
	}
	f.waiting, f.failure = true, ""
	return func() tea.Msg {
		result, err := f.svc.RecordCashCount(context.Background(), in)
		return countWrittenMsg{target: f, result: result, err: err}
	}
}

// onWritten closes the overlay after an adjustment was written. A count that
// matched wrote nothing and the overlay keeps showing Cuadra.
func (f *countForm) onWritten(msg countWrittenMsg) tea.Cmd {
	f.waiting = false
	var invalid app.ValidationError
	switch {
	case msg.err == nil && msg.result.Movement != nil:
		return tea.Batch(send(view.ChangedMsg{}), send(view.CloseOverlayMsg{}))
	case msg.err == nil:
	case errors.As(msg.err, &invalid) && f.form.SetErrors(invalid):
	default:
		f.failure = describe(msg.err)
	}
	return nil
}

func (f *countForm) View() string {
	out := f.form.View()
	if f.known {
		out += "\n" + theme.Label.Render(text.CountSystem(f.system.Format()))
		if counted, err := domain.ParseMoney(f.form.Value(app.FieldAmount)); err == nil && counted >= 0 {
			out += "\n" + differenceLine(domain.CountDifference(counted, f.system))
		}
	}
	switch {
	case f.failure != "":
		out += "\n" + theme.Error.Render(f.failure)
	case f.waiting:
		out += "\n" + theme.Label.Render(text.Saving)
	}
	return out
}

// differenceLine shows Cuadra when the count matches, else the signed difference.
func differenceLine(difference domain.Money) string {
	if difference == 0 {
		return theme.AmountIncome.Render(text.CountBalanced)
	}
	amount, style := signedAmount(difference)
	return style.Render(text.CountDifference(amount))
}

func (f *countForm) KeyHints() []view.Hint {
	return []view.Hint{{Key: text.KeyEnter, Desc: text.HintSave}, {Key: text.KeyTab, Desc: text.HintField}}
}
