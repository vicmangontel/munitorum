package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/vicmangontel/munitorum/internal/domain"
)

// Field keys of the movement inputs, as they appear in ValidationError.Fields.
const (
	FieldDate        = "date"
	FieldAccount     = "account_id" // the account of a movement; the source of a transfer
	FieldToAccount   = "to_account_id"
	FieldCategory    = "category_id"
	FieldVendor      = "vendor_id"
	FieldAmount      = "amount"
	FieldDescription = "description"
	FieldNotes       = "notes"
	FieldReceiptLink = "receipt_link"
)

// Validation codes of the ledger beyond the ones setup shares with it
// (CodeDateInvalid, CodeDateFuture, CodeAmountInvalid).
const (
	CodeDateBeforeOpening = "date_before_opening" // rule 1
	CodeAmountNotPositive = "amount_not_positive" // rule 2
	CodeAccountInvalid    = "account_invalid"     // rule 3: no such account
	CodeDescriptionLength = "description_length"  // rule 3
	CodeNotesLength       = "notes_length"        // rule 3
	CodeReceiptLinkLength = "receipt_link_length" // rule 3
	CodeCategoryInvalid   = "category_invalid"    // rule 4: unknown, inactive, or of the other direction
	CodeCategoryReserved  = "category_reserved"   // rule 4: income recorded by houses or assessments only
	CodeVendorInvalid     = "vendor_invalid"      // not a live vendor
	CodeAccountsSame      = "accounts_same"       // rule 8: a transfer needs two accounts
	CodeAmountNegative    = "amount_negative"     // rule 11: counted money is zero or more
)

// Field length limits (ledger rule 3), in characters.
const (
	maxDescription = 120
	maxNotes       = 500
	maxReceiptLink = 500
)

// ReceiptStatus is the state of a manual receipt (rule 10).
type ReceiptStatus string

const (
	ReceiptNone    ReceiptStatus = "none"
	ReceiptPending ReceiptStatus = "pending"
	ReceiptDone    ReceiptStatus = "done"
)

// Movement is one row of the libro de caja. Amount is positive, except for an
// adjustment, which is signed. Notes and ReceiptLink are empty when not given.
type Movement struct {
	ID          int64
	Date        time.Time
	Kind        domain.MovementKind
	AccountID   int64
	ToAccountID *int64 // transfers only
	Amount      domain.Money
	CategoryID  *int64 // income and expense only
	VendorID    *int64
	Description string
	Notes       string
	ReceiptLink string
	Receipt     ReceiptStatus
}

func (m Movement) entry() domain.Entry {
	e := domain.Entry{Date: m.Date, Kind: m.Kind, AccountID: m.AccountID, Amount: m.Amount}
	if m.ToAccountID != nil {
		e.ToAccountID = *m.ToAccountID
	}
	return e
}

func entriesOf(movements []Movement) []domain.Entry {
	entries := make([]domain.Entry, len(movements))
	for i, m := range movements {
		entries[i] = m.entry()
	}
	return entries
}

// NegativeBalanceWarning is returned, and nothing is written, when a movement
// would leave an account below zero and the treasurer has not confirmed it
// (rule 7). It is not a failure: the UI shows Resulting and asks.
type NegativeBalanceWarning struct {
	Account   Account
	Resulting domain.Money
}

func (w NegativeBalanceWarning) Error() string {
	return fmt.Sprintf("negative balance: account %d would be left at %d cents", w.Account.ID, int64(w.Resulting))
}

// IncomeInput is what the treasurer typed for other income. Date is DD/MM/AAAA
// and Amount a money amount, both as typed, so that one validation reports
// unreadable and out-of-range values alike.
type IncomeInput struct {
	Date                 string
	AccountID            int64
	CategoryID           int64
	Amount               string
	Description          string
	Notes                string
	ReceiptLink          string
	ManualReceiptPending bool
}

// ExpenseInput is what the treasurer typed for an expense. VendorID is optional.
// ConfirmNegative accepts the NegativeBalanceWarning.
type ExpenseInput struct {
	Date            string
	AccountID       int64
	CategoryID      int64
	VendorID        *int64
	Amount          string
	Description     string
	Notes           string
	ReceiptLink     string
	ConfirmNegative bool
}

// TransferInput is what the treasurer typed for a transfer. Description is
// required: the form fills in the generated text when the treasurer leaves it
// blank, since internal/app holds no Spanish. ConfirmNegative accepts the
// NegativeBalanceWarning for the source account.
type TransferInput struct {
	Date            string
	FromAccountID   int64
	ToAccountID     int64
	Amount          string
	Description     string
	Notes           string
	ConfirmNegative bool
}

// CashCountInput is a cash count: the money Counted in AccountID on Date, as
// typed. Describe, supplied by the TUI, words the adjustment from the counted
// and system amounts the service computed, so the stored text always matches
// the difference; internal/app holds no Spanish.
type CashCountInput struct {
	Date      string
	AccountID int64
	Counted   string
	Notes     string
	Describe  func(counted, system domain.Money) string
}

// CashCountResult is the outcome of a cash count. Movement is nil when the
// count matched the system balance and nothing was written.
type CashCountResult struct {
	Difference domain.Money
	Movement   *Movement
}

// LedgerFilter selects the rows of ListMovements. Month is required until the
// all-months filter arrives; a zero AccountID is every account.
type LedgerFilter struct {
	Month     domain.Month
	AccountID int64
}

// LedgerRow is a movement with the names the list shows. Vendor is the display
// name of a vendor (with its company); VendorDeleted tells the UI to mark it.
type LedgerRow struct {
	Movement       Movement
	Category       string
	Vendor         string
	VendorDeleted  bool
	Account        string
	ToAccount      string
	RunningBalance domain.Money // of AccountID, or the total across accounts when it is 0
}

// Ledger is the filtered movements with the balance before the first row and
// after the last, and the sums of the rows.
type Ledger struct {
	Rows           []LedgerRow
	Totals         domain.Totals
	OpeningBalance domain.Money
	ClosingBalance domain.Money
}

// AccountBalance is an account with its balance on a date.
type AccountBalance struct {
	Account Account
	Balance domain.Money
}

// Balances are the account balances on a date, in account order, and their sum.
type Balances struct {
	Accounts []AccountBalance
	Total    domain.Money
}

// LedgerStore is what LedgerService needs from storage inside one unit of work.
type LedgerStore interface {
	CategoryRepository
	// Settings returns ErrNotFound before setup has completed.
	Settings(ctx context.Context) (Settings, error)
	// Vendors returns every vendor, deleted ones included.
	Vendors(ctx context.Context) ([]Vendor, error)
	// Movements returns the live movements ordered by date, then id (rule 9).
	Movements(ctx context.Context) ([]Movement, error)
	// InsertMovement writes a new movement, ignoring m.ID, and returns its id.
	InsertMovement(ctx context.Context, m Movement, now time.Time) (int64, error)
}

// LedgerRepository is a LedgerStore that can also group operations in one
// transaction. Writes that read first (balances) run inside InTx.
type LedgerRepository interface {
	LedgerStore
	// InTx runs fn in one transaction: committed when fn returns nil, rolled back
	// otherwise. fn must use the store it is given, not the repository.
	InTx(ctx context.Context, fn func(LedgerStore) error) error
}

// LedgerService records and lists the movements of the libro de caja.
type LedgerService struct {
	repo  LedgerRepository
	clock Clock
	log   *slog.Logger
}

// NewLedgerService returns a LedgerService.
func NewLedgerService(repo LedgerRepository, clock Clock, log *slog.Logger) *LedgerService {
	return &LedgerService{repo: repo, clock: clock, log: log}
}

// emit writes the wide event of one operation; see logEvent.
func (s *LedgerService) emit(ctx context.Context, action string, started time.Time, err error, m Movement) {
	fields := []any{"account_id", m.AccountID}
	if m.ToAccountID != nil {
		fields = append(fields, "to_account_id", *m.ToAccountID)
	}
	if m.CategoryID != nil {
		fields = append(fields, "category_id", *m.CategoryID)
	}
	if m.VendorID != nil {
		fields = append(fields, "vendor_id", *m.VendorID)
	}
	if m.Amount != 0 {
		fields = append(fields, "amount_cents", int64(m.Amount))
	}
	if m.ID != 0 {
		fields = append(fields, "movement_id", m.ID)
	}
	logEvent(ctx, s.log, action, started, err, fields)
}

// RecordIncome applies ledger rules 1 to 4 and 10 and writes the income. The
// category must be an active income category without a system key.
func (s *LedgerService) RecordIncome(ctx context.Context, in IncomeInput) (created Movement, err error) {
	started := time.Now()
	var m Movement
	defer func() { s.emit(ctx, "record_income", started, err, m) }()

	err = s.repo.InTx(ctx, func(tx LedgerStore) error {
		var err error
		m, _, err = s.check(ctx, tx, domain.MovementIncome, movementFields{
			date: in.Date, amount: in.Amount, description: in.Description, notes: in.Notes,
			receiptLink: in.ReceiptLink, accountID: in.AccountID, categoryID: in.CategoryID,
		})
		if err != nil {
			return err
		}
		m.Receipt = ReceiptNone
		if in.ManualReceiptPending {
			m.Receipt = ReceiptPending
		}
		if err := s.insert(ctx, tx, &m); err != nil {
			return err
		}
		created = m
		return nil
	})
	return created, err
}

// RecordExpense applies ledger rules 1 to 4 and 7 and writes the expense. When
// it would leave the account below zero it writes nothing and returns a
// NegativeBalanceWarning, unless ConfirmNegative is set.
func (s *LedgerService) RecordExpense(ctx context.Context, in ExpenseInput) (created Movement, err error) {
	started := time.Now()
	var m Movement
	defer func() { s.emit(ctx, "record_expense", started, err, m) }()

	err = s.repo.InTx(ctx, func(tx LedgerStore) error {
		var settings Settings
		var err error
		m, settings, err = s.check(ctx, tx, domain.MovementExpense, movementFields{
			date: in.Date, amount: in.Amount, description: in.Description, notes: in.Notes,
			receiptLink: in.ReceiptLink, accountID: in.AccountID, categoryID: in.CategoryID, vendorID: in.VendorID,
		})
		if err != nil {
			return err
		}
		m.Receipt = ReceiptNone
		if !in.ConfirmNegative {
			if err := checkNegative(ctx, tx, settings, m); err != nil {
				return err
			}
		}
		if err := s.insert(ctx, tx, &m); err != nil {
			return err
		}
		created = m
		return nil
	})
	return created, err
}

// RecordTransfer applies ledger rules 1 to 3, 7 and 8 and writes one transfer.
// Only the source account is checked for a negative balance: when the transfer
// would leave it below zero, nothing is written and a NegativeBalanceWarning is
// returned, unless ConfirmNegative is set.
func (s *LedgerService) RecordTransfer(ctx context.Context, in TransferInput) (created Movement, err error) {
	started := time.Now()
	var m Movement
	defer func() { s.emit(ctx, "record_transfer", started, err, m) }()

	err = s.repo.InTx(ctx, func(tx LedgerStore) error {
		var settings Settings
		var err error
		m, settings, err = s.check(ctx, tx, domain.MovementTransfer, movementFields{
			date: in.Date, amount: in.Amount, description: in.Description, notes: in.Notes,
			accountID: in.FromAccountID, toAccountID: in.ToAccountID,
		})
		if err != nil {
			return err
		}
		m.Receipt = ReceiptNone
		if !in.ConfirmNegative {
			if err := checkNegative(ctx, tx, settings, m); err != nil {
				return err
			}
		}
		if err := s.insert(ctx, tx, &m); err != nil {
			return err
		}
		created = m
		return nil
	})
	return created, err
}

// RecordCashCount applies ledger rule 11. The system balance is the account's
// balance on Date including every movement of that day. A count that matches
// writes nothing; otherwise one adjustment for the difference is written. It
// returns an error, without writing, when in.Describe is nil.
func (s *LedgerService) RecordCashCount(ctx context.Context, in CashCountInput) (result CashCountResult, err error) {
	started := time.Now()
	known := false // the difference was computed
	defer func() {
		fields := []any{"account_id", in.AccountID}
		if known {
			fields = append(fields, "difference_cents", int64(result.Difference))
		}
		if result.Movement != nil {
			fields = append(fields, "movement_id", result.Movement.ID)
		}
		logEvent(ctx, s.log, "record_cash_count", started, err, fields)
	}()
	if in.Describe == nil {
		return CashCountResult{}, errors.New("record cash count: no Describe function")
	}

	err = s.repo.InTx(ctx, func(tx LedgerStore) error {
		settings, err := tx.Settings(ctx)
		if err != nil {
			return fmt.Errorf("check cash count: %w", err)
		}
		var ve ValidationError
		date, code := s.checkDate(settings, in.Date)
		if code != "" {
			ve.add(FieldDate, code)
		}
		i := slices.IndexFunc(settings.Accounts, func(a Account) bool { return a.ID == in.AccountID })
		if i < 0 {
			ve.add(FieldAccount, CodeAccountInvalid)
		}
		counted, perr := domain.ParseMoney(in.Counted)
		switch {
		case perr != nil:
			ve.add(FieldAmount, CodeAmountInvalid)
		case counted < 0:
			ve.add(FieldAmount, CodeAmountNegative)
		}
		notes := strings.TrimSpace(in.Notes)
		if utf8.RuneCountInString(notes) > maxNotes {
			ve.add(FieldNotes, CodeNotesLength)
		}
		if err := ve.err(); err != nil {
			return err
		}

		movements, err := tx.Movements(ctx)
		if err != nil {
			return fmt.Errorf("check cash count: %w", err)
		}
		account := settings.Accounts[i]
		system := domain.BalanceAsOf(account.OpeningBalance, account.ID, entriesOf(movements), date)
		result.Difference, known = domain.CountDifference(counted, system), true
		if result.Difference == 0 {
			return nil
		}
		m := Movement{
			Date: date, Kind: domain.MovementAdjustment, AccountID: account.ID, Amount: result.Difference,
			Description: in.Describe(counted, system), Notes: notes, Receipt: ReceiptNone,
		}
		if err := s.insert(ctx, tx, &m); err != nil {
			return err
		}
		result.Movement = &m
		return nil
	})
	if err != nil {
		result.Movement = nil // the transaction rolled back, so nothing was written
	}
	return result, err
}

func (s *LedgerService) insert(ctx context.Context, tx LedgerStore, m *Movement) error {
	id, err := tx.InsertMovement(ctx, *m, s.clock.Now())
	if err != nil {
		return fmt.Errorf("record %s: %w", m.Kind, err)
	}
	m.ID = id
	return nil
}

// movementFields is what the record operations share; the text is as typed.
type movementFields struct {
	date, amount, description, notes, receiptLink string
	accountID, toAccountID, categoryID            int64 // toAccountID: transfers only; categoryID: income and expense only
	vendorID                                      *int64
}

// check applies rules 1 to 4 to f and returns the movement to write. Every
// invalid field is reported at once. The movement is filled in as far as the
// input was valid, so that a rejected input can still be logged by its ids.
func (s *LedgerService) check(ctx context.Context, tx LedgerStore, kind domain.MovementKind, f movementFields) (Movement, Settings, error) {
	settings, err := tx.Settings(ctx)
	if err != nil {
		return Movement{}, Settings{}, fmt.Errorf("check movement: %w", err)
	}
	m := Movement{
		Kind: kind, AccountID: f.accountID, VendorID: f.vendorID,
		Description: strings.TrimSpace(f.description), Notes: strings.TrimSpace(f.notes),
		ReceiptLink: strings.TrimSpace(f.receiptLink),
	}
	var ve ValidationError

	date, code := s.checkDate(settings, f.date)
	if code != "" {
		ve.add(FieldDate, code)
	} else {
		m.Date = date
	}

	switch amount, err := domain.ParseMoney(f.amount); {
	case err != nil:
		ve.add(FieldAmount, CodeAmountInvalid)
	case amount <= 0:
		ve.add(FieldAmount, CodeAmountNotPositive)
	default:
		m.Amount = amount
	}

	hasAccount := func(id int64) bool {
		return slices.ContainsFunc(settings.Accounts, func(a Account) bool { return a.ID == id })
	}
	if !hasAccount(f.accountID) {
		ve.add(FieldAccount, CodeAccountInvalid)
	}
	if kind == domain.MovementTransfer {
		m.ToAccountID = &f.toAccountID
		switch {
		case !hasAccount(f.toAccountID):
			ve.add(FieldToAccount, CodeAccountInvalid)
		case f.toAccountID == f.accountID:
			ve.add(FieldToAccount, CodeAccountsSame)
		}
	} else {
		m.CategoryID = &f.categoryID
		categories, err := tx.Categories(ctx)
		if err != nil {
			return m, settings, fmt.Errorf("check movement: %w", err)
		}
		if code := categoryProblem(categories, kind, f.categoryID); code != "" {
			ve.add(FieldCategory, code)
		}
	}
	if f.vendorID != nil {
		vendors, err := tx.Vendors(ctx)
		if err != nil {
			return m, settings, fmt.Errorf("check movement: %w", err)
		}
		if !slices.ContainsFunc(vendors, func(v Vendor) bool { return v.ID == *f.vendorID && !v.Deleted }) {
			ve.add(FieldVendor, CodeVendorInvalid)
		}
	}

	if n := utf8.RuneCountInString(m.Description); n < 1 || n > maxDescription {
		ve.add(FieldDescription, CodeDescriptionLength)
	}
	if utf8.RuneCountInString(m.Notes) > maxNotes {
		ve.add(FieldNotes, CodeNotesLength)
	}
	if utf8.RuneCountInString(m.ReceiptLink) > maxReceiptLink {
		ve.add(FieldReceiptLink, CodeReceiptLinkLength)
	}
	return m, settings, ve.err()
}

// checkDate applies rule 1 to the date as typed: it returns the date, or the
// code of why it cannot be used.
func (s *LedgerService) checkDate(settings Settings, typed string) (time.Time, string) {
	now := s.clock.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	date, err := domain.ParseInputDate(typed)
	switch {
	case err != nil:
		return time.Time{}, CodeDateInvalid
	case date.Before(settings.OpeningDate):
		return time.Time{}, CodeDateBeforeOpening
	case date.After(today):
		return time.Time{}, CodeDateFuture
	}
	return date, ""
}

// categoryProblem is the code for why category id cannot be used by a movement
// of kind (rule 4); empty when it can.
func categoryProblem(all []Category, kind domain.MovementKind, id int64) string {
	i := slices.IndexFunc(all, func(c Category) bool { return c.ID == id })
	switch {
	case i < 0, !all[i].Active, all[i].Direction != kind:
		return CodeCategoryInvalid
	case all[i].SystemKey != "":
		return CodeCategoryReserved
	}
	return ""
}

// checkNegative returns a NegativeBalanceWarning when writing m would leave its
// account's balance, over every live movement of every date, below zero (rule 7).
func checkNegative(ctx context.Context, tx LedgerStore, settings Settings, m Movement) error {
	movements, err := tx.Movements(ctx)
	if err != nil {
		return fmt.Errorf("check balance: %w", err)
	}
	i := slices.IndexFunc(settings.Accounts, func(a Account) bool { return a.ID == m.AccountID })
	account := settings.Accounts[i] // check has already proved the account exists
	resulting := domain.Balance(account.OpeningBalance, account.ID, append(entriesOf(movements), m.entry()))
	if resulting < 0 {
		return NegativeBalanceWarning{Account: account, Resulting: resulting}
	}
	return nil
}

// ListMovements returns the live movements of f.Month (of one account when
// f.AccountID is set) in ledger order, each with its running balance, the sums
// of the rows, and the balance before the first day and after the last row. A
// transfer is in the list of both its accounts. It returns ErrNotFound for an
// unknown account.
//
// debt: reads every live movement and filters in Go; fine for a coto's few
// thousand movements a year, move to SQL when a month takes noticeable time.
func (s *LedgerService) ListMovements(ctx context.Context, f LedgerFilter) (Ledger, error) {
	settings, err := s.repo.Settings(ctx)
	if err != nil {
		return Ledger{}, fmt.Errorf("list movements: %w", err)
	}
	categories, err := s.repo.Categories(ctx)
	if err != nil {
		return Ledger{}, fmt.Errorf("list movements: %w", err)
	}
	vendors, err := s.repo.Vendors(ctx)
	if err != nil {
		return Ledger{}, fmt.Errorf("list movements: %w", err)
	}
	movements, err := s.repo.Movements(ctx)
	if err != nil {
		return Ledger{}, fmt.Errorf("list movements: %w", err)
	}

	opening := domain.Money(0)
	for _, a := range settings.Accounts {
		if f.AccountID == 0 || a.ID == f.AccountID {
			opening += a.OpeningBalance
		}
	}
	if f.AccountID != 0 && !slices.ContainsFunc(settings.Accounts, func(a Account) bool { return a.ID == f.AccountID }) {
		return Ledger{}, fmt.Errorf("list movements: account %d: %w", f.AccountID, ErrNotFound)
	}

	dayBefore := time.Date(f.Month.Year, f.Month.Month, 0, 0, 0, 0, 0, time.Local)
	start := domain.BalanceAsOf(opening, f.AccountID, entriesOf(movements), dayBefore)

	var rows []Movement
	for _, m := range movements {
		touches := f.AccountID == 0 || m.AccountID == f.AccountID || (m.ToAccountID != nil && *m.ToAccountID == f.AccountID)
		if domain.Of(m.Date) == f.Month && touches {
			rows = append(rows, m)
		}
	}
	entries := entriesOf(rows)
	running := domain.RunningBalances(start, f.AccountID, entries)

	accountNames := map[int64]string{}
	for _, a := range settings.Accounts {
		accountNames[a.ID] = a.Name
	}
	categoryNames := map[int64]string{}
	for _, c := range categories {
		categoryNames[c.ID] = c.Name
	}
	byID := vendorsByID(vendors)

	ledger := Ledger{Totals: domain.SumTotals(entries), OpeningBalance: start, ClosingBalance: start}
	for i, m := range rows {
		row := LedgerRow{Movement: m, Account: accountNames[m.AccountID], RunningBalance: running[i]}
		if m.CategoryID != nil {
			row.Category = categoryNames[*m.CategoryID]
		}
		if m.ToAccountID != nil {
			row.ToAccount = accountNames[*m.ToAccountID]
		}
		if m.VendorID != nil {
			v := byID[*m.VendorID]
			company := ""
			if v.CompanyID != nil {
				company = byID[*v.CompanyID].Name
			}
			row.Vendor, row.VendorDeleted = domain.VendorDisplayName(v.Name, company), v.Deleted
		}
		ledger.Rows = append(ledger.Rows, row)
		ledger.ClosingBalance = running[i]
	}
	return ledger, nil
}

// AccountBalances returns every account's balance on asOf (opening balance plus
// the live movements dated on or before it) and their total.
func (s *LedgerService) AccountBalances(ctx context.Context, asOf time.Time) (Balances, error) {
	settings, err := s.repo.Settings(ctx)
	if err != nil {
		return Balances{}, fmt.Errorf("account balances: %w", err)
	}
	movements, err := s.repo.Movements(ctx)
	if err != nil {
		return Balances{}, fmt.Errorf("account balances: %w", err)
	}
	entries := entriesOf(movements)

	var out Balances
	for _, a := range settings.Accounts {
		balance := domain.BalanceAsOf(a.OpeningBalance, a.ID, entries, asOf)
		out.Accounts = append(out.Accounts, AccountBalance{Account: a, Balance: balance})
		out.Total += balance
	}
	return out, nil
}
