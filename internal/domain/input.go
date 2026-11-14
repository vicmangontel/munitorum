package domain

import (
	"fmt"
	"strings"
	"time"
)

// The treasurer types dates as DD/MM/AAAA and months as MM/AAAA. Storage uses
// YYYY-MM-DD and YYYY-MM (Month.String).
const (
	inputDateLayout  = "02/01/2006"
	inputMonthLayout = "01/2006"
)

// ParseInputDate reads DD/MM/AAAA as a calendar date at local midnight.
// Anything else, including a day the month does not have, is rejected.
func ParseInputDate(s string) (time.Time, error) {
	t, err := time.ParseInLocation(inputDateLayout, strings.TrimSpace(s), time.Local)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date %q: want DD/MM/AAAA", s)
	}
	return t, nil
}

// FormatInputDate renders a date as DD/MM/AAAA.
func FormatInputDate(t time.Time) string { return t.Format(inputDateLayout) }

// ParseInputMonth reads MM/AAAA.
func ParseInputMonth(s string) (Month, error) {
	t, err := time.Parse(inputMonthLayout, strings.TrimSpace(s))
	if err != nil {
		return Month{}, fmt.Errorf("%w %q: want MM/AAAA", ErrInvalidMonth, s)
	}
	return Of(t), nil
}

// InputString renders a month as MM/AAAA.
func (m Month) InputString() string {
	return fmt.Sprintf("%02d/%04d", int(m.Month), m.Year)
}
