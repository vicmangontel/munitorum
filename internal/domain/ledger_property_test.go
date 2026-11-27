package domain

import (
	"testing"
	"time"

	"pgregory.net/rapid"
)

// Whatever sequence of movements is recorded across the two accounts, the
// balances of the accounts add up to what came in and went out: transfers move
// money without changing the sum. The expected total is built from the kinds
// directly, not from Entry.Effect.
func TestAccountBalances_SumToOpeningTotalPlusIncomeMinusExpensePlusAdjustments(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		openCash := Money(rapid.Int64Range(0, 10_000_000).Draw(t, "openCash"))
		openBank := Money(rapid.Int64Range(0, 10_000_000).Draw(t, "openBank"))
		n := rapid.IntRange(0, 40).Draw(t, "count")

		entries := make([]Entry, n)
		want := openCash + openBank
		for i := range entries {
			kind := rapid.SampledFrom([]MovementKind{MovementIncome, MovementExpense, MovementTransfer, MovementAdjustment}).Draw(t, "kind")
			from := rapid.SampledFrom([]int64{cash, bank}).Draw(t, "from")
			e := Entry{
				Date:      day(2026, time.November, rapid.IntRange(1, 28).Draw(t, "day")),
				Kind:      kind,
				AccountID: from,
				Amount:    Money(rapid.Int64Range(1, 5_000_000).Draw(t, "amount")),
			}
			switch kind {
			case MovementIncome:
				want += e.Amount
			case MovementExpense:
				want -= e.Amount
			case MovementTransfer:
				e.ToAccountID = cash + bank - from // the other account
			case MovementAdjustment:
				if rapid.Bool().Draw(t, "negative") {
					e.Amount = -e.Amount
				}
				want += e.Amount
			}
			entries[i] = e
		}

		got := Balance(openCash, cash, entries) + Balance(openBank, bank, entries)

		if got != want {
			t.Fatalf("sum of account balances = %d, want opening + income − expense + adjustments = %d (%d entries)", got, want, n)
		}
	})
}

// After a cash count is booked, the books agree with the count: the balance of
// the account on the counted date, with every movement of that day, equals the
// money counted. A count that already matched needs no adjustment.
func TestCashCount_BookedAdjustmentLeavesTheAccountAtTheCountedAmount(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		opening := Money(rapid.Int64Range(0, 10_000_000).Draw(t, "opening"))
		n := rapid.IntRange(0, 30).Draw(t, "count")
		entries := make([]Entry, n)
		for i := range entries {
			from := rapid.SampledFrom([]int64{cash, bank}).Draw(t, "from")
			e := Entry{
				Date:      day(2026, time.November, rapid.IntRange(1, 28).Draw(t, "day")),
				Kind:      rapid.SampledFrom([]MovementKind{MovementIncome, MovementExpense, MovementTransfer, MovementAdjustment}).Draw(t, "kind"),
				AccountID: from,
				Amount:    Money(rapid.Int64Range(1, 5_000_000).Draw(t, "amount")),
			}
			if e.Kind == MovementTransfer {
				e.ToAccountID = cash + bank - from
			}
			entries[i] = e
		}
		countedOn := day(2026, time.November, rapid.IntRange(1, 28).Draw(t, "countedOn"))
		counted := Money(rapid.Int64Range(0, 10_000_000).Draw(t, "counted"))

		system := BalanceAsOf(opening, cash, entries, countedOn)
		if difference := CountDifference(counted, system); difference != 0 {
			entries = append(entries, Entry{Date: countedOn, Kind: MovementAdjustment, AccountID: cash, Amount: difference})
		}

		if got := BalanceAsOf(opening, cash, entries, countedOn); got != counted {
			t.Fatalf("balance after the count = %d, want the counted %d (system was %d)", got, counted, system)
		}
	})
}
