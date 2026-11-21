package text

import (
	"fmt"
	"time"
)

// MonthNames are the Spanish month names, lower case, January first.
var MonthNames = [12]string{
	"enero", "febrero", "marzo", "abril", "mayo", "junio",
	"julio", "agosto", "septiembre", "octubre", "noviembre", "diciembre",
}

// MonthAbbrevs are the three-letter forms used in dates, January first.
var MonthAbbrevs = [12]string{
	"ene", "feb", "mar", "abr", "may", "jun",
	"jul", "ago", "sep", "oct", "nov", "dic",
}

// Date formats a business date as 12 nov 2026.
func Date(t time.Time) string {
	return fmt.Sprintf("%d %s %d", t.Day(), MonthAbbrevs[t.Month()-1], t.Year())
}

// DayMonth formats a business date as 05 nov, for a list that already shows
// the month's year.
func DayMonth(t time.Time) string {
	return fmt.Sprintf("%02d %s", t.Day(), MonthAbbrevs[t.Month()-1])
}

// MonthYear formats a month as nov 2026.
func MonthYear(year int, month time.Month) string {
	return fmt.Sprintf("%s %d", MonthAbbrevs[month-1], year)
}
