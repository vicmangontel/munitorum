package text

import (
	"testing"
	"time"
)

func TestDate_UsesSpanishAbbreviation(t *testing.T) {
	cases := []struct {
		in   time.Time
		want string
	}{
		{time.Date(2026, time.November, 12, 0, 0, 0, 0, time.Local), "12 nov 2026"},
		{time.Date(2026, time.January, 1, 0, 0, 0, 0, time.Local), "1 ene 2026"},
		{time.Date(2027, time.December, 31, 23, 59, 0, 0, time.Local), "31 dic 2027"},
	}
	for _, c := range cases {
		t.Run(c.want, func(t *testing.T) {
			// Act
			got := Date(c.in)

			// Assert
			if got != c.want {
				t.Fatalf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestMonthYear_UsesSpanishAbbreviationForEveryMonth(t *testing.T) {
	want := []string{
		"ene 2026", "feb 2026", "mar 2026", "abr 2026", "may 2026", "jun 2026",
		"jul 2026", "ago 2026", "sep 2026", "oct 2026", "nov 2026", "dic 2026",
	}
	for i, w := range want {
		month := time.Month(i + 1)
		t.Run(w, func(t *testing.T) {
			// Act
			got := MonthYear(2026, month)

			// Assert
			if got != w {
				t.Fatalf("got %q, want %q", got, w)
			}
		})
	}
}

func TestMonthNames_AreLowerCaseSpanishAndAlignWithAbbreviations(t *testing.T) {
	for i, name := range MonthNames {
		t.Run(name, func(t *testing.T) {
			// Assert
			if name[:3] != MonthAbbrevs[i] {
				t.Fatalf("name %q does not start with abbreviation %q", name, MonthAbbrevs[i])
			}
		})
	}
}

func TestDayMonth_PadsTheDayToTwoDigitsSoColumnsLineUp(t *testing.T) {
	cases := []struct {
		name string
		date time.Time
		want string
	}{
		{"single digit day", time.Date(2026, time.November, 5, 0, 0, 0, 0, time.Local), "05 nov"},
		{"two digit day", time.Date(2026, time.March, 25, 0, 0, 0, 0, time.Local), "25 mar"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Act
			got := DayMonth(c.date)

			// Assert
			if got != c.want {
				t.Fatalf("DayMonth = %q, want %q", got, c.want)
			}
		})
	}
}
