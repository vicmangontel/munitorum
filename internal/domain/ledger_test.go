package domain

import (
	"testing"
	"time"
)

func day(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.Local) }

const (
	cash int64 = 1
	bank int64 = 2
)

func TestEntryEffect_FollowsTheKindOfMovement(t *testing.T) {
	cases := []struct {
		name    string
		entry   Entry
		account int64
		want    Money
	}{
		{"income raises its account", Entry{Kind: MovementIncome, AccountID: cash, Amount: 15000}, cash, 15000},
		{"income does not touch the other account", Entry{Kind: MovementIncome, AccountID: cash, Amount: 15000}, bank, 0},
		{"expense lowers its account", Entry{Kind: MovementExpense, AccountID: cash, Amount: 17850}, cash, -17850},
		{"transfer lowers the source", Entry{Kind: MovementTransfer, AccountID: cash, ToAccountID: bank, Amount: 60000}, cash, -60000},
		{"transfer raises the destination", Entry{Kind: MovementTransfer, AccountID: cash, ToAccountID: bank, Amount: 60000}, bank, 60000},
		{"transfer leaves the total unchanged", Entry{Kind: MovementTransfer, AccountID: cash, ToAccountID: bank, Amount: 60000}, 0, 0},
		{"adjustment adds its signed amount", Entry{Kind: MovementAdjustment, AccountID: cash, Amount: -2350}, cash, -2350},
		{"income counts in the total", Entry{Kind: MovementIncome, AccountID: bank, Amount: 500}, 0, 500},
		{"expense counts in the total", Entry{Kind: MovementExpense, AccountID: bank, Amount: 500}, 0, -500},
		{"adjustment counts in the total", Entry{Kind: MovementAdjustment, AccountID: bank, Amount: 500}, 0, 500},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Act
			got := c.entry.Effect(c.account)

			// Assert
			if got != c.want {
				t.Fatalf("Effect(%d) = %d, want %d", c.account, got, c.want)
			}
		})
	}
}

func TestBalanceAsOf_CountsOnlyEntriesOnOrBeforeTheDate(t *testing.T) {
	entries := []Entry{
		{Date: day(2026, time.November, 5), Kind: MovementExpense, AccountID: cash, Amount: 17850},
		{Date: day(2026, time.November, 10), Kind: MovementIncome, AccountID: cash, Amount: 20000},
		{Date: day(2026, time.November, 12), Kind: MovementTransfer, AccountID: cash, ToAccountID: bank, Amount: 60000},
		{Date: day(2027, time.January, 1), Kind: MovementExpense, AccountID: cash, Amount: 999},
	}
	cases := []struct {
		name    string
		asOf    time.Time
		account int64
		want    Money
	}{
		{"before the first entry", day(2026, time.November, 4), cash, 750000},
		{"on the day of an entry the entry counts", day(2026, time.November, 5), cash, 750000 - 17850},
		{"the time of day is ignored", time.Date(2026, time.November, 5, 23, 59, 0, 0, time.Local), cash, 750000 - 17850},
		{"after the last entry", day(2026, time.December, 1), cash, 750000 - 17850 + 20000 - 60000},
		{"an entry of a later year is not counted although its month number is lower", day(2026, time.December, 31), cash, 750000 - 17850 + 20000 - 60000},
		{"the other account sees the transfer", day(2026, time.December, 1), bank, 60000},
		{"the total ignores the transfer", day(2026, time.December, 1), 0, 750000 - 17850 + 20000},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange: the cash account opens with 7,500.00, the bank with nothing
			opening := Money(750000)
			if c.account == bank {
				opening = 0
			}

			// Act
			got := BalanceAsOf(opening, c.account, entries, c.asOf)

			// Assert
			if got != c.want {
				t.Fatalf("BalanceAsOf = %d, want %d", got, c.want)
			}
		})
	}
}

func TestBalance_CountsEveryEntryWhateverItsDate(t *testing.T) {
	// Arrange
	entries := []Entry{
		{Date: day(2026, time.November, 5), Kind: MovementExpense, AccountID: cash, Amount: 17850},
		{Date: day(2027, time.January, 1), Kind: MovementExpense, AccountID: cash, Amount: 100},
	}

	// Act
	got := Balance(750000, cash, entries)

	// Assert
	if got != 750000-17850-100 {
		t.Fatalf("Balance = %d, want %d", got, 750000-17850-100)
	}
}

func TestRunningBalances_ContinueAcrossAMonthBoundary(t *testing.T) {
	// Arrange: October closes at 7,321.50; November starts from that balance
	october := []Entry{
		{Date: day(2026, time.October, 20), Kind: MovementExpense, AccountID: cash, Amount: 17850},
		{Date: day(2026, time.October, 31), Kind: MovementIncome, AccountID: cash, Amount: 5000},
	}
	november := []Entry{
		{Date: day(2026, time.November, 1), Kind: MovementExpense, AccountID: cash, Amount: 1000},
		{Date: day(2026, time.November, 2), Kind: MovementIncome, AccountID: cash, Amount: 25000},
	}
	start := BalanceAsOf(750000, 0, october, day(2026, time.October, 31))

	// Act
	got := RunningBalances(start, 0, november)

	// Assert
	want := []Money{750000 - 17850 + 5000 - 1000, 750000 - 17850 + 5000 - 1000 + 25000}
	if start != 750000-17850+5000 || len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("start = %d, running = %v; want start %d and %v", start, got, 750000-17850+5000, want)
	}
}

func TestRunningBalances_OfOneAccountFollowsItsTransfers(t *testing.T) {
	// Arrange
	entries := []Entry{
		{Kind: MovementTransfer, AccountID: cash, ToAccountID: bank, Amount: 60000},
		{Kind: MovementExpense, AccountID: bank, Amount: 10000},
	}

	// Act
	got := RunningBalances(0, bank, entries)

	// Assert
	if len(got) != 2 || got[0] != 60000 || got[1] != 50000 {
		t.Fatalf("running balances of the bank = %v, want [60000 50000]", got)
	}
}

func TestSumTotals_SeparatesIncomeExpenseAndAdjustmentsAndIgnoresTransfers(t *testing.T) {
	// Arrange
	entries := []Entry{
		{Kind: MovementIncome, AccountID: cash, Amount: 20000},
		{Kind: MovementIncome, AccountID: bank, Amount: 5000},
		{Kind: MovementExpense, AccountID: cash, Amount: 17850},
		{Kind: MovementTransfer, AccountID: cash, ToAccountID: bank, Amount: 60000},
		{Kind: MovementAdjustment, AccountID: cash, Amount: -2350},
	}

	// Act
	got := SumTotals(entries)

	// Assert
	want := Totals{Income: 25000, Expense: 17850, Adjustments: -2350, Net: 25000 - 17850 - 2350}
	if got != want {
		t.Fatalf("SumTotals = %+v, want %+v", got, want)
	}
}

func TestSumTotals_OfNoEntriesIsZero(t *testing.T) {
	// Act
	got := SumTotals(nil)

	// Assert
	if got != (Totals{}) {
		t.Fatalf("SumTotals(nil) = %+v, want zero", got)
	}
}

func TestCountDifference_IsCountedMinusSystem(t *testing.T) {
	cases := []struct {
		name            string
		counted, system Money
		want            Money
	}{
		{"short", 430000, 432350, -2350},
		{"over", 432350, 430000, 2350},
		{"matching", 432350, 432350, 0},
		{"nothing counted against a positive system", 0, 1000, -1000},
		{"counted against an overdrawn system", 0, -5000, 5000},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := CountDifference(c.counted, c.system); got != c.want {
				t.Fatalf("CountDifference(%d, %d) = %d, want %d", c.counted, c.system, got, c.want)
			}
		})
	}
}
