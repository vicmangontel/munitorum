package ledger

import (
	"context"
	"errors"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/vicmangontel/munitorum/internal/app"
	"github.com/vicmangontel/munitorum/internal/domain"
	"github.com/vicmangontel/munitorum/internal/tui/confirm"
	"github.com/vicmangontel/munitorum/internal/tui/form"
	"github.com/vicmangontel/munitorum/internal/tui/picker"
	"github.com/vicmangontel/munitorum/internal/tui/text"
	"github.com/vicmangontel/munitorum/internal/tui/theme"
	"github.com/vicmangontel/munitorum/internal/tui/view"
)

// fieldManualReceipt is the key of the income form's receipt selector. It has no
// validation error of its own.
const fieldManualReceipt = "manual_receipt"

// wrapWidth keeps the warning narrower than a small terminal.
const wrapWidth = 60

var wrapped = lipgloss.NewStyle().Width(wrapWidth)

// writtenMsg is the result of a write, tagged with the form that started it.
type writtenMsg struct {
	target *movementForm
	err    error
}

// movementForm is the income, expense and transfer form. It implements view.Content and
// view.EscapeHandler: esc leaves the vendor picker, and answers No to the
// negative-balance question, before it closes the form.
type movementForm struct {
	svc        *app.LedgerService
	kind       domain.MovementKind
	accounts   []app.Account
	categories []app.Category // the ones the form offers
	vendors    []picker.Item

	form    *form.Form
	vendor  *picker.Item // the chosen vendor; nil for none
	picker  *picker.Model
	confirm *confirm.Model // non-nil while the negative-balance question is open
	waiting bool           // the write is in flight
	failure string
}

var (
	_ view.Content       = (*movementForm)(nil)
	_ view.EscapeHandler = (*movementForm)(nil)
)

func newMovementForm(svc *app.LedgerService, data formDataMsg) *movementForm {
	f := &movementForm{svc: svc, kind: data.kind, accounts: data.accounts, vendors: data.vendors}
	var accountNames, categoryNames []string
	for _, a := range data.accounts {
		accountNames = append(accountNames, a.Name)
	}
	for _, c := range data.categories {
		if c.SystemKey != "" {
			continue // fee and assessment income is recorded by the Casas and Cooperaciones views
		}
		f.categories = append(f.categories, c)
		categoryNames = append(categoryNames, c.Name)
	}

	var fields []form.Field
	if f.kind == domain.MovementTransfer {
		to := accountNames[len(accountNames)-1] // the last account, so it differs from the first
		fields = []form.Field{
			{Key: app.FieldDate, Label: text.LabelDate, Value: domain.FormatInputDate(data.today)},
			{Key: app.FieldAccount, Label: text.LabelFrom, Choices: accountNames},
			{Key: app.FieldToAccount, Label: text.LabelTo, Choices: accountNames, Value: to},
			{Key: app.FieldAmount, Label: text.LabelAmount},
			{Key: app.FieldDescription, Label: text.LabelDescription},
			{Key: app.FieldNotes, Label: text.LabelNotes},
		}
		f.form = form.New(fields...)
		return f
	}
	fields = []form.Field{
		{Key: app.FieldDate, Label: text.LabelDate, Value: domain.FormatInputDate(data.today)},
		{Key: app.FieldAccount, Label: text.LabelAccount, Choices: accountNames},
		{Key: app.FieldCategory, Label: text.LabelCategory, Choices: categoryNames},
	}
	if f.kind == domain.MovementExpense {
		fields = append(fields, form.Field{Key: app.FieldVendor, Label: text.LabelPayee, Locked: true})
	}
	fields = append(fields,
		form.Field{Key: app.FieldAmount, Label: text.LabelAmount},
		form.Field{Key: app.FieldDescription, Label: text.LabelDescription},
		form.Field{Key: app.FieldNotes, Label: text.LabelNotes},
		form.Field{Key: app.FieldReceiptLink, Label: text.LabelReceiptLink},
	)
	if f.kind == domain.MovementIncome {
		fields = append(fields, form.Field{Key: fieldManualReceipt, Label: text.LabelManualReceipt, Choices: []string{text.No, text.Yes}})
	}
	f.form = form.New(fields...)
	f.showVendor()
	return f
}

// showVendor shows the chosen vendor in the locked field.
func (f *movementForm) showVendor() {
	name := text.VendorNone
	if f.vendor != nil {
		name = f.vendor.Label
	}
	f.form.SetValue(app.FieldVendor, name)
}

func (f *movementForm) HandlesEscape() bool { return f.picker != nil || f.confirm != nil }

func (f *movementForm) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case writtenMsg:
		if msg.target == f {
			return f.onWritten(msg)
		}
	case confirm.ResultMsg:
		if f.confirm != nil {
			f.confirm = nil
			if msg.Yes {
				return f.submit(true)
			}
			return nil // No: back to the form, nothing was written
		}
	case picker.SelectedMsg:
		if f.picker != nil {
			f.vendor, f.picker = &msg.Item, nil
			f.showVendor()
			return nil
		}
	case picker.CancelledMsg:
		f.picker = nil
		return nil
	case tea.KeyMsg:
		return f.onKey(msg)
	}
	if f.picker != nil {
		return f.picker.Update(msg)
	}
	return nil
}

func (f *movementForm) onKey(msg tea.KeyMsg) tea.Cmd {
	switch {
	case f.waiting:
		return nil
	case f.confirm != nil:
		return f.confirm.Update(msg)
	case f.picker != nil:
		return f.picker.Update(msg)
	}
	onVendor := f.form.Focused() == app.FieldVendor
	switch key := msg.String(); {
	case key == "enter" && onVendor:
		f.picker = picker.New(f.vendors, nil)
		return nil
	case key == "enter":
		return f.submit(false)
	case (key == "backspace" || key == "delete") && onVendor:
		f.vendor = nil
		f.showVendor()
		return nil
	}
	return f.form.Update(msg)
}

// accountID and categoryID are the ids of the names the selectors show (accountID
// reads the selector with the given key); 0 when
// there is none, which the service reports as invalid.
func (f *movementForm) accountID(key string) int64 {
	for _, a := range f.accounts {
		if a.Name == f.form.Value(key) {
			return a.ID
		}
	}
	return 0
}

func (f *movementForm) categoryID() int64 {
	for _, c := range f.categories {
		if c.Name == f.form.Value(app.FieldCategory) {
			return c.ID
		}
	}
	return 0
}

// submit writes what the form holds. confirmed accepts a negative balance.
func (f *movementForm) submit(confirmed bool) tea.Cmd {
	date, account, category := f.form.Value(app.FieldDate), f.accountID(app.FieldAccount), f.categoryID()
	amount, description := f.form.Value(app.FieldAmount), f.form.Value(app.FieldDescription)
	notes, link := f.form.Value(app.FieldNotes), f.form.Value(app.FieldReceiptLink)

	var record func(context.Context) error
	switch f.kind {
	case domain.MovementTransfer:
		if strings.TrimSpace(description) == "" {
			description = text.TransferDescription(f.form.Value(app.FieldAccount), f.form.Value(app.FieldToAccount))
		}
		in := app.TransferInput{
			Date: date, FromAccountID: account, ToAccountID: f.accountID(app.FieldToAccount), Amount: amount,
			Description: description, Notes: notes, ConfirmNegative: confirmed,
		}
		record = func(ctx context.Context) error { _, err := f.svc.RecordTransfer(ctx, in); return err }
	case domain.MovementIncome:
		in := app.IncomeInput{
			Date: date, AccountID: account, CategoryID: category, Amount: amount, Description: description,
			Notes: notes, ReceiptLink: link, ManualReceiptPending: f.form.Value(fieldManualReceipt) == text.Yes,
		}
		record = func(ctx context.Context) error { _, err := f.svc.RecordIncome(ctx, in); return err }
	default:
		in := app.ExpenseInput{
			Date: date, AccountID: account, CategoryID: category, Amount: amount, Description: description,
			Notes: notes, ReceiptLink: link, ConfirmNegative: confirmed,
		}
		if f.vendor != nil {
			id := f.vendor.ID
			in.VendorID = &id
		}
		record = func(ctx context.Context) error { _, err := f.svc.RecordExpense(ctx, in); return err }
	}

	f.waiting, f.failure = true, ""
	return func() tea.Msg { return writtenMsg{target: f, err: record(context.Background())} }
}

func (f *movementForm) onWritten(msg writtenMsg) tea.Cmd {
	f.waiting = false
	var warning app.NegativeBalanceWarning
	var invalid app.ValidationError
	switch {
	case msg.err == nil:
		return tea.Batch(send(view.ChangedMsg{}), send(view.CloseOverlayMsg{}))
	case errors.As(msg.err, &warning):
		question := text.NegativeBalanceQuestion
		if f.kind == domain.MovementTransfer {
			question = text.NegativeBalanceTransferQuestion
		}
		f.confirm = confirm.New(wrapped.Render(question(warning.Account.Name, warning.Resulting.Format())))
	case errors.As(msg.err, &invalid) && f.form.SetErrors(invalid):
	default:
		f.failure = describe(msg.err)
	}
	return nil
}

func (f *movementForm) View() string {
	if f.confirm != nil {
		return f.confirm.View()
	}
	if f.picker != nil {
		return f.picker.View()
	}
	out := f.form.View()
	switch {
	case f.failure != "":
		out += "\n" + theme.Error.Render(f.failure)
	case f.waiting:
		out += "\n" + theme.Label.Render(text.Saving)
	}
	return out
}

func (f *movementForm) KeyHints() []view.Hint {
	switch {
	case f.confirm != nil:
		return f.confirm.KeyHints()
	case f.picker != nil:
		return f.picker.KeyHints()
	case f.form.Focused() == app.FieldVendor:
		return []view.Hint{
			{Key: text.KeyEnter, Desc: text.HintPickPayee},
			{Key: text.KeyRemove, Desc: text.HintRemovePayee},
			{Key: text.KeyTab, Desc: text.HintField},
		}
	}
	return []view.Hint{{Key: text.KeyEnter, Desc: text.HintSave}, {Key: text.KeyTab, Desc: text.HintField}}
}
