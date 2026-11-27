package domain

import "time"

// MovementKind is what a movement does to the accounts. A category's direction
// (what it classifies) is one of the first two.
type MovementKind string

const (
	MovementIncome     MovementKind = "income"
	MovementExpense    MovementKind = "expense"
	MovementTransfer   MovementKind = "transfer"
	MovementAdjustment MovementKind = "adjustment"
)

// Entry is the part of a live movement that balances depend on.
type Entry struct {
	Date        time.Time
	Kind        MovementKind
	AccountID   int64
	ToAccountID int64 // the receiving account of a transfer; 0 for every other kind
	Amount      Money // positive, except an adjustment, which is signed
}

// Effect is the change the entry makes to the balance of account, or to the
// total across accounts when account is 0 (ledger spec, Effect on balances).
func (e Entry) Effect(account int64) Money {
	onAccount := account == 0 || account == e.AccountID
	switch e.Kind {
	case MovementIncome, MovementAdjustment:
		if onAccount {
			return e.Amount
		}
	case MovementExpense:
		if onAccount {
			return -e.Amount
		}
	case MovementTransfer:
		switch account {
		case 0: // money moves between accounts; the total does not change
		case e.AccountID:
			return -e.Amount
		case e.ToAccountID:
			return e.Amount
		}
	}
	return 0
}

// dayKey orders dates by calendar day, ignoring the time of day.
func dayKey(t time.Time) int {
	y, m, d := t.Date()
	return y*10000 + int(m)*100 + d
}

// Balance is opening plus the effect of every entry on account (0 for the
// total), whatever its date.
func Balance(opening Money, account int64, entries []Entry) Money {
	balance := opening
	for _, e := range entries {
		balance += e.Effect(account)
	}
	return balance
}

// BalanceAsOf is Balance counting only the entries dated on or before asOf.
func BalanceAsOf(opening Money, account int64, entries []Entry, asOf time.Time) Money {
	balance := opening
	for _, e := range entries {
		if dayKey(e.Date) <= dayKey(asOf) {
			balance += e.Effect(account)
		}
	}
	return balance
}

// RunningBalances is the balance of account (0 for the total) after each entry,
// given the balance before the first. The entries must already be in ledger
// order: by date, then by id.
func RunningBalances(start Money, account int64, entries []Entry) []Money {
	out := make([]Money, len(entries))
	balance := start
	for i, e := range entries {
		balance += e.Effect(account)
		out[i] = balance
	}
	return out
}

// Totals are the sums a ledger footer shows. Transfers are in none of them.
type Totals struct {
	Income, Expense, Adjustments Money
	Net                          Money // Income − Expense + Adjustments
}

// SumTotals adds up the entries by kind.
func SumTotals(entries []Entry) Totals {
	var t Totals
	for _, e := range entries {
		switch e.Kind {
		case MovementIncome:
			t.Income += e.Amount
		case MovementExpense:
			t.Expense += e.Amount
		case MovementAdjustment:
			t.Adjustments += e.Amount
		}
	}
	t.Net = t.Income - t.Expense + t.Adjustments
	return t
}

// CountDifference is what a cash count adds to the books: the money counted
// minus the system balance. Zero means the count matches.
func CountDifference(counted, system Money) Money { return counted - system }
