package domain

import (
	"errors"
	"testing"
	"time"
)

func TestParseInputDate_ReadsDayMonthYear(t *testing.T) {
	cases := []struct {
		in      string
		y, d    int
		m       time.Month
		invalid bool
	}{
		{in: "15/05/2026", y: 2026, m: time.May, d: 15},
		{in: " 01/01/2026 ", y: 2026, m: time.January, d: 1},
		{in: "29/02/2028", y: 2028, m: time.February, d: 29},
		{in: "29/02/2026", invalid: true},
		{in: "31/04/2026", invalid: true},
		{in: "2026-05-15", invalid: true},
		{in: "5/5/2026", invalid: true},
		{in: "15/05/26", invalid: true},
		{in: "05/15/2026", invalid: true},
		{in: "", invalid: true},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			// Act
			got, err := ParseInputDate(c.in)

			// Assert
			if c.invalid {
				if err == nil {
					t.Fatalf("ParseInputDate(%q) = %v, want an error", c.in, got)
				}
				return
			}
			if err != nil || got.Year() != c.y || got.Month() != c.m || got.Day() != c.d {
				t.Fatalf("ParseInputDate(%q) = %v, %v; want %d-%d-%d", c.in, got, err, c.y, c.m, c.d)
			}
		})
	}
}

func TestFormatInputDate_RendersDayMonthYear(t *testing.T) {
	// Arrange
	date := time.Date(2026, time.May, 5, 0, 0, 0, 0, time.Local)

	// Act
	got := FormatInputDate(date)

	// Assert
	if got != "05/05/2026" {
		t.Fatalf("FormatInputDate = %q, want 05/05/2026", got)
	}
}

func TestParseInputMonth_ReadsMonthYear(t *testing.T) {
	cases := []struct {
		in      string
		want    Month
		invalid bool
	}{
		{in: "05/2026", want: Month{2026, time.May}},
		{in: " 12/2026 ", want: Month{2026, time.December}},
		{in: "13/2026", invalid: true},
		{in: "00/2026", invalid: true},
		{in: "5/2026", invalid: true},
		{in: "2026-05", invalid: true},
		{in: "05/26", invalid: true},
		{in: "15/05/2026", invalid: true},
		{in: "", invalid: true},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			// Act
			got, err := ParseInputMonth(c.in)

			// Assert
			if c.invalid {
				if !errors.Is(err, ErrInvalidMonth) {
					t.Fatalf("ParseInputMonth(%q) error = %v, want ErrInvalidMonth", c.in, err)
				}
				return
			}
			if err != nil || got != c.want {
				t.Fatalf("ParseInputMonth(%q) = %v, %v; want %v", c.in, got, err, c.want)
			}
		})
	}
}

func TestMonthInputString_PadsMonthAndYear(t *testing.T) {
	// Act
	got := Month{2026, time.May}.InputString()

	// Assert
	if got != "05/2026" {
		t.Fatalf("InputString = %q, want 05/2026", got)
	}
}
