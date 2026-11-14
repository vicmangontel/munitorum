package domain

import (
	"errors"
	"testing"
	"time"
)

func TestParseMonth_AcceptsYearMonth(t *testing.T) {
	// Act
	got, err := ParseMonth("2026-11")

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if want := (Month{2026, time.November}); got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestParseMonth_RejectsInvalidInput(t *testing.T) {
	cases := []string{"", "2026", "2026-13", "2026-00", "2026-1", "26-01", "2026/01", "2026-01-15", "ene 2026", " 2026-01"}
	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			// Act
			_, err := ParseMonth(in)

			// Assert
			if !errors.Is(err, ErrInvalidMonth) {
				t.Fatalf("ParseMonth(%q) error = %v, want ErrInvalidMonth", in, err)
			}
		})
	}
}

func TestMonthString_PadsToYearMonth(t *testing.T) {
	cases := []struct {
		in   Month
		want string
	}{
		{Month{2026, time.January}, "2026-01"},
		{Month{2026, time.December}, "2026-12"},
		{Month{999, time.March}, "0999-03"},
	}
	for _, c := range cases {
		t.Run(c.want, func(t *testing.T) {
			// Act
			got := c.in.String()

			// Assert
			if got != c.want {
				t.Fatalf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestMonthAdd_MovesAcrossYears(t *testing.T) {
	cases := []struct {
		name string
		from Month
		n    int
		want Month
	}{
		{"zero", Month{2026, time.May}, 0, Month{2026, time.May}},
		{"one month", Month{2026, time.May}, 1, Month{2026, time.June}},
		{"into next year", Month{2026, time.December}, 1, Month{2027, time.January}},
		{"a full year", Month{2026, time.May}, 12, Month{2027, time.May}},
		{"back one month", Month{2026, time.May}, -1, Month{2026, time.April}},
		{"back into previous year", Month{2026, time.January}, -1, Month{2025, time.December}},
		{"back 13 months", Month{2026, time.January}, -13, Month{2024, time.December}},
		{"before year zero", Month{0, time.January}, -1, Month{-1, time.December}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Act
			got := c.from.Add(c.n)

			// Assert
			if got != c.want {
				t.Fatalf("%v.Add(%d) = %v, want %v", c.from, c.n, got, c.want)
			}
		})
	}
}

func TestMonthBefore_ComparesYearThenMonth(t *testing.T) {
	cases := []struct {
		name string
		a, b Month
		want bool
	}{
		{"earlier month", Month{2026, time.January}, Month{2026, time.February}, true},
		{"later month", Month{2026, time.February}, Month{2026, time.January}, false},
		{"same month", Month{2026, time.March}, Month{2026, time.March}, false},
		{"earlier year later month", Month{2025, time.December}, Month{2026, time.January}, true},
		{"later year earlier month", Month{2026, time.January}, Month{2025, time.December}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Act
			got := c.a.Before(c.b)

			// Assert
			if got != c.want {
				t.Fatalf("%v.Before(%v) = %v, want %v", c.a, c.b, got, c.want)
			}
		})
	}
}

func TestOf_DropsTheDay(t *testing.T) {
	// Arrange
	date := time.Date(2026, time.November, 30, 23, 59, 0, 0, time.Local)

	// Act
	got := Of(date)

	// Assert
	if want := (Month{2026, time.November}); got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestMonthLastDay_FollowsTheCalendar(t *testing.T) {
	cases := []struct {
		in   Month
		want int
	}{
		{Month{2026, time.January}, 31},
		{Month{2026, time.April}, 30},
		{Month{2026, time.February}, 28},
		{Month{2028, time.February}, 29},
		{Month{1900, time.February}, 28},
		{Month{2000, time.February}, 29},
		{Month{2026, time.December}, 31},
	}
	for _, c := range cases {
		t.Run(c.in.String(), func(t *testing.T) {
			// Act
			got := c.in.LastDay()

			// Assert
			if got != c.want {
				t.Fatalf("got %d, want %d", got, c.want)
			}
		})
	}
}

func TestMonthClampDay_KeepsDayInsideTheMonth(t *testing.T) {
	cases := []struct {
		name string
		in   Month
		day  int
		want int
	}{
		{"day that exists", Month{2026, time.March}, 15, 15},
		{"last day exactly", Month{2026, time.April}, 30, 30},
		{"31 in a 30-day month", Month{2026, time.April}, 31, 30},
		{"31 in february", Month{2026, time.February}, 31, 28},
		{"29 in leap february", Month{2028, time.February}, 29, 29},
		{"zero becomes first", Month{2026, time.March}, 0, 1},
		{"negative becomes first", Month{2026, time.March}, -5, 1},
		{"first stays", Month{2026, time.March}, 1, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Act
			got := c.in.ClampDay(c.day)

			// Assert
			if got != c.want {
				t.Fatalf("%v.ClampDay(%d) = %d, want %d", c.in, c.day, got, c.want)
			}
		})
	}
}
