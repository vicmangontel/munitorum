package domain

import (
	"errors"
	"testing"
	"time"
)

func TestHouseCode_PadsToTheWidthOfTheCount(t *testing.T) {
	cases := []struct {
		name          string
		prefix        string
		number, count int
		want          string
	}{
		{"CASA first of 18", "CASA ", 1, 18, "CASA 01"},
		{"CASA last of 18", "CASA ", 18, 18, "CASA 18"},
		{"dash prefix first of 18", "C-", 1, 18, "C-01"},
		{"dash prefix last of 18", "C-", 18, 18, "C-18"},
		{"count 120 pads to three digits", "CASA ", 7, 120, "CASA 007"},
		{"count 120 last house", "CASA ", 120, 120, "CASA 120"},
		{"count 100 is three digits", "C-", 5, 100, "C-005"},
		{"count 99 stays two digits", "C-", 5, 99, "C-05"},
		{"count 1 still pads to two digits", "C-", 1, 1, "C-01"},
		{"empty prefix gives the bare number", "", 3, 18, "03"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Act
			got := HouseCode(c.prefix, c.number, c.count)

			// Assert
			if got != c.want {
				t.Fatalf("HouseCode(%q, %d, %d) = %q, want %q", c.prefix, c.number, c.count, got, c.want)
			}
		})
	}
}

func TestParseHouseCount_AcceptsOneToTwoHundredOnly(t *testing.T) {
	cases := []struct {
		in      string
		want    int
		invalid bool
	}{
		{in: "18", want: 18},
		{in: "1", want: 1},
		{in: "200", want: 200},
		{in: " 18 ", want: 18},
		{in: "0", invalid: true},
		{in: "201", invalid: true},
		{in: "-3", invalid: true},
		{in: "", invalid: true},
		{in: "1.5", invalid: true},
		{in: "dieciocho", invalid: true},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			// Act
			got, err := ParseHouseCount(c.in)

			// Assert
			if c.invalid {
				if !errors.Is(err, ErrInvalidHouseCount) {
					t.Fatalf("ParseHouseCount(%q) error = %v, want ErrInvalidHouseCount", c.in, err)
				}
				return
			}
			if err != nil || got != c.want {
				t.Fatalf("ParseHouseCount(%q) = %d, %v; want %d", c.in, got, err, c.want)
			}
		})
	}
}

func TestHouseOpeningCheck_AppliesOpeningRules(t *testing.T) {
	opening := Month{2026, time.May}
	fee := Money(15000)
	month := func(y int, m time.Month) *Month { return &Month{y, m} }

	cases := []struct {
		name string
		in   HouseOpening
		want []OpeningIssue
	}{
		{"nothing covered, nothing owed", HouseOpening{}, nil},
		{"covered through the opening month", HouseOpening{CoveredThrough: month(2026, time.May)}, nil},
		{"covered 36 months past the opening month", HouseOpening{CoveredThrough: month(2029, time.May)}, nil},
		{"covered before the opening month", HouseOpening{CoveredThrough: month(2026, time.April)},
			[]OpeningIssue{{OpeningCoveredThrough, CodeCoveredBeforeOpening}}},
		{"covered 37 months past the opening month", HouseOpening{CoveredThrough: month(2029, time.June)},
			[]OpeningIssue{{OpeningCoveredThrough, CodeCoveredTooFar}}},
		{"credit just below the cuota", HouseOpening{Credit: 14999}, nil},
		{"credit equal to the cuota", HouseOpening{Credit: 15000},
			[]OpeningIssue{{OpeningCredit, CodeCreditOutOfRange}}},
		{"credit above the cuota", HouseOpening{Credit: 20000},
			[]OpeningIssue{{OpeningCredit, CodeCreditOutOfRange}}},
		{"negative credit", HouseOpening{Credit: -1},
			[]OpeningIssue{{OpeningCredit, CodeCreditOutOfRange}}},
		{"arrears alone", HouseOpening{Arrears: 15000}, nil},
		{"negative arrears", HouseOpening{Arrears: -1},
			[]OpeningIssue{{OpeningArrears, CodeArrearsNegative}}},
		{"arrears with coverage", HouseOpening{CoveredThrough: month(2026, time.June), Arrears: 100},
			[]OpeningIssue{{OpeningArrears, CodeArrearsWithCoverage}}},
		{"arrears with credit", HouseOpening{Credit: 100, Arrears: 100},
			[]OpeningIssue{{OpeningArrears, CodeArrearsWithCredit}}},
		{"every field invalid is reported at once",
			HouseOpening{CoveredThrough: month(2026, time.January), Credit: 15000, Arrears: 100},
			[]OpeningIssue{
				{OpeningCoveredThrough, CodeCoveredBeforeOpening},
				{OpeningCredit, CodeCreditOutOfRange},
				{OpeningArrears, CodeArrearsWithCoverage},
			}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Act
			got := c.in.Check(opening, fee)

			// Assert
			if len(got) != len(c.want) {
				t.Fatalf("issues = %v, want %v", got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Fatalf("issues = %v, want %v", got, c.want)
				}
			}
		})
	}
}
