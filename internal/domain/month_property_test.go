package domain

import (
	"testing"
	"time"

	"pgregory.net/rapid"
)

func drawMonth(t *rapid.T) Month {
	return Month{
		Year:  rapid.IntRange(1, 9999).Draw(t, "year"),
		Month: time.Month(rapid.IntRange(1, 12).Draw(t, "month")),
	}
}

func TestMonthAdd_ComposesByAddingOffsets(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		m := drawMonth(t)
		a := rapid.IntRange(-5000, 5000).Draw(t, "a")
		b := rapid.IntRange(-5000, 5000).Draw(t, "b")

		got := m.Add(a).Add(b)

		if want := m.Add(a + b); got != want {
			t.Fatalf("%v.Add(%d).Add(%d) = %v, want %v", m, a, b, got, want)
		}
	})
}

func TestMonthAdd_IsOrderedAndInverted(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		m := drawMonth(t)
		n := rapid.IntRange(1, 5000).Draw(t, "n")

		later := m.Add(n)

		if !m.Before(later) || later.Before(m) {
			t.Fatalf("%v should be before %v.Add(%d) = %v", m, m, n, later)
		}
		if back := later.Add(-n); back != m {
			t.Fatalf("%v.Add(-%d) = %v, want %v", later, n, back, m)
		}
	})
}

func TestParseMonth_RoundTripsString(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		m := drawMonth(t)

		got, err := ParseMonth(m.String())

		if err != nil || got != m {
			t.Fatalf("ParseMonth(%q) = %v, %v; want %v", m.String(), got, err, m)
		}
	})
}

func TestMonthClampDay_StaysInsideTheMonth(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		m := drawMonth(t)
		d := rapid.IntRange(-10, 40).Draw(t, "day")

		got := m.ClampDay(d)

		if got < 1 || got > m.LastDay() {
			t.Fatalf("%v.ClampDay(%d) = %d outside 1..%d", m, d, got, m.LastDay())
		}
		if d >= 1 && d <= m.LastDay() && got != d {
			t.Fatalf("%v.ClampDay(%d) = %d, a valid day must be unchanged", m, d, got)
		}
	})
}
