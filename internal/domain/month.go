package domain

import (
	"errors"
	"fmt"
	"time"
)

// Month is a calendar month, the unit fees are charged in.
type Month struct {
	Year  int
	Month time.Month
}

// ErrInvalidMonth is wrapped by every ParseMonth failure.
var ErrInvalidMonth = errors.New("invalid month")

// ParseMonth reads YYYY-MM.
func ParseMonth(s string) (Month, error) {
	t, err := time.Parse("2006-01", s)
	if err != nil {
		return Month{}, fmt.Errorf("%w %q: want YYYY-MM", ErrInvalidMonth, s)
	}
	return Of(t), nil
}

// Of is the month containing date.
func Of(date time.Time) Month {
	return Month{Year: date.Year(), Month: date.Month()}
}

// String is YYYY-MM, the form stored in the database.
func (m Month) String() string {
	return fmt.Sprintf("%04d-%02d", m.Year, int(m.Month))
}

// index counts months from year 0 so that month arithmetic is integer arithmetic.
func (m Month) index() int { return m.Year*12 + int(m.Month) - 1 }

// Add moves m by n months; n may be negative.
func (m Month) Add(n int) Month {
	i := m.index() + n
	year, month := i/12, i%12
	if month < 0 { // floor division for months before year 0
		year, month = year-1, month+12
	}
	return Month{Year: year, Month: time.Month(month + 1)}
}

// Before reports whether m is earlier than o.
func (m Month) Before(o Month) bool { return m.index() < o.index() }

// LastDay is the number of days in the month.
func (m Month) LastDay() int {
	return time.Date(m.Year, m.Month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// ClampDay limits a day-of-month setting to a day the month actually has, so a
// reminder on the 31st falls on the 28th in February.
func (m Month) ClampDay(d int) int {
	return min(max(d, 1), m.LastDay())
}
