package directory

import (
	"context"
	"errors"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vicmangontel/munitorum/internal/app"
	"github.com/vicmangontel/munitorum/internal/domain"
	"github.com/vicmangontel/munitorum/internal/tui/form"
	"github.com/vicmangontel/munitorum/internal/tui/picker"
	"github.com/vicmangontel/munitorum/internal/tui/text"
	"github.com/vicmangontel/munitorum/internal/tui/theme"
	"github.com/vicmangontel/munitorum/internal/tui/view"
)

// Results of commands the form started, tagged with the form that started them
// so that a form nested in a picker never takes the messages of the one around it.
type (
	writtenMsg struct {
		target *vendorForm
		vendor app.Vendor
		err    error
	}
	companiesMsg struct {
		target *vendorForm
		items  []picker.Item
		err    error
	}
)

// formOpts says what a vendor form edits and what happens when it saved.
type formOpts struct {
	edit      *app.VendorDetail // nil to create
	fixedKind domain.VendorKind // when set, the kind cannot be changed
	done      func(app.Vendor) tea.Cmd
}

// vendorForm is the create and edit form of the directory. It implements
// view.Content and view.EscapeHandler: esc leaves the company picker first.
type vendorForm struct {
	svc     *app.DirectoryService
	opts    formOpts
	form    *form.Form
	company *picker.Item // the chosen company; nil for none
	picker  *picker.Model
	loading bool // the companies of the picker are being read
	waiting bool // the write is in flight
	failure string
}

var (
	_ view.Content       = (*vendorForm)(nil)
	_ view.EscapeHandler = (*vendorForm)(nil)
)

func newVendorForm(svc *app.DirectoryService, opts formOpts) *vendorForm {
	f := &vendorForm{svc: svc, opts: opts}
	choices := []string{text.KindPerson, text.KindCompany, text.KindService}
	var v app.Vendor
	if opts.edit != nil {
		v = opts.edit.Vendor
		if opts.edit.Company != nil {
			f.company = &picker.Item{ID: opts.edit.Company.ID, Label: opts.edit.Company.Name}
		}
	}
	kind := v.Kind
	if opts.fixedKind != "" {
		kind, choices = opts.fixedKind, []string{text.VendorKindName(string(opts.fixedKind))}
	}
	f.form = form.New(
		form.Field{Key: domain.VendorFieldKind, Label: text.LabelVendorKind, Value: text.VendorKindName(string(kind)), Choices: choices},
		form.Field{Key: domain.VendorFieldName, Label: text.LabelVendorName, Value: v.Name},
		form.Field{Key: domain.VendorFieldPhone, Label: text.LabelVendorPhone, Value: v.Phone},
		form.Field{Key: domain.VendorFieldEmail, Label: text.LabelVendorEmail, Value: v.Email},
		form.Field{Key: domain.VendorFieldCompanyID, Label: text.LabelVendorCo, Locked: true},
		form.Field{Key: domain.VendorFieldNotes, Label: text.LabelVendorNotes, Value: v.Notes},
	)
	f.showCompany()
	return f
}

// kind is the kind the form currently has.
func (f *vendorForm) kind() domain.VendorKind {
	return domain.VendorKind(text.VendorKindCode(f.form.Value(domain.VendorFieldKind)))
}

// showCompany shows the company field for a person only, with the chosen company.
func (f *vendorForm) showCompany() {
	f.form.SetHidden(domain.VendorFieldCompanyID, f.kind() != domain.VendorPerson)
	name := text.CompanyNone
	if f.company != nil {
		name = f.company.Label
	}
	f.form.SetValue(domain.VendorFieldCompanyID, name)
}

func (f *vendorForm) HandlesEscape() bool { return f.picker != nil }

func (f *vendorForm) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case writtenMsg:
		if msg.target == f {
			return f.onWritten(msg)
		}
	case companiesMsg:
		if msg.target == f {
			f.onCompanies(msg)
			return nil
		}
	case picker.SelectedMsg:
		if f.picker != nil {
			f.company, f.picker = &msg.Item, nil
			f.showCompany()
			return nil
		}
	case picker.CancelledMsg:
		f.picker = nil
		return nil
	case tea.KeyMsg:
		return f.onKey(msg)
	}
	// Everything else is for the picker, or for the form nested in it.
	if f.picker != nil {
		return f.picker.Update(msg)
	}
	return nil
}

func (f *vendorForm) onKey(msg tea.KeyMsg) tea.Cmd {
	if f.waiting || f.loading {
		return nil
	}
	if f.picker != nil {
		return f.picker.Update(msg)
	}
	onCompany := f.form.Focused() == domain.VendorFieldCompanyID
	switch key := msg.String(); {
	case key == "enter" && onCompany:
		return f.openPicker()
	case key == "enter":
		return f.submit()
	case (key == "backspace" || key == "delete") && onCompany:
		f.company = nil
		f.showCompany()
		return nil
	}
	cmd := f.form.Update(msg)
	f.showCompany() // the kind may have changed
	return cmd
}

// openPicker reads the live companies and then shows the picker over them.
func (f *vendorForm) openPicker() tea.Cmd {
	f.loading, f.failure = true, ""
	return func() tea.Msg {
		rows, err := f.svc.ListVendors(context.Background(), app.ListVendorsInput{Kind: domain.VendorCompany})
		items := make([]picker.Item, len(rows))
		for i, r := range rows {
			items[i] = picker.Item{ID: r.ID, Label: r.DisplayName()}
		}
		return companiesMsg{target: f, items: items, err: err}
	}
}

func (f *vendorForm) onCompanies(msg companiesMsg) {
	f.loading = false
	if msg.err != nil {
		f.failure = describe(msg.err)
		return
	}
	f.picker = picker.New(msg.items, f.createCompany)
}

// createCompany is what + does in the picker: a form for a new company whose
// result comes back to the picker as the selected item.
func (f *vendorForm) createCompany() view.Content {
	return newVendorForm(f.svc, formOpts{
		fixedKind: domain.VendorCompany,
		done: func(v app.Vendor) tea.Cmd {
			created := picker.CreatedMsg{Item: picker.Item{ID: v.ID, Label: v.Name}}
			return tea.Batch(send(SavedMsg{Vendor: v}), send(created))
		},
	})
}

// submit writes the vendor. The company is sent for a person only.
func (f *vendorForm) submit() tea.Cmd {
	in := app.VendorInput{
		Kind:  f.kind(),
		Name:  f.form.Value(domain.VendorFieldName),
		Phone: f.form.Value(domain.VendorFieldPhone),
		Email: f.form.Value(domain.VendorFieldEmail),
		Notes: f.form.Value(domain.VendorFieldNotes),
	}
	if in.Kind == domain.VendorPerson && f.company != nil {
		id := f.company.ID
		in.CompanyID = &id
	}
	f.waiting, f.failure = true, ""
	return func() tea.Msg {
		ctx := context.Background()
		var v app.Vendor
		var err error
		if f.opts.edit != nil {
			v, err = f.svc.UpdateVendor(ctx, app.UpdateVendorInput{ID: f.opts.edit.Vendor.ID, VendorInput: in})
		} else {
			v, err = f.svc.CreateVendor(ctx, in)
		}
		return writtenMsg{target: f, vendor: v, err: err}
	}
}

func (f *vendorForm) onWritten(msg writtenMsg) tea.Cmd {
	f.waiting = false
	if msg.err == nil {
		return f.opts.done(msg.vendor)
	}
	var invalid app.ValidationError
	if errors.As(msg.err, &invalid) && f.form.SetErrors(invalid) {
		return nil
	}
	f.failure = describe(msg.err)
	return nil
}

func (f *vendorForm) View() string {
	if f.picker != nil {
		return f.picker.View()
	}
	out := f.form.View()
	switch {
	case f.failure != "":
		out += "\n" + theme.Error.Render(f.failure)
	case f.waiting || f.loading:
		out += "\n" + theme.Label.Render(text.Saving)
	}
	return out
}

func (f *vendorForm) KeyHints() []view.Hint {
	if f.picker != nil {
		return f.picker.KeyHints()
	}
	if f.form.Focused() == domain.VendorFieldCompanyID {
		return []view.Hint{
			{Key: text.KeyEnter, Desc: text.HintPickCompany},
			{Key: text.KeyRemove, Desc: text.HintRemoveCo},
			{Key: text.KeyTab, Desc: text.HintField},
		}
	}
	return []view.Hint{{Key: text.KeyEnter, Desc: text.HintSave}, {Key: text.KeyTab, Desc: text.HintField}}
}
