package domain

import (
	"errors"
	"testing"
)

func TestParseMoney_AcceptsTreasurerInput(t *testing.T) {
	cases := []struct {
		in   string
		want Money
	}{
		{"1234", 123400},
		{"1234.5", 123450},
		{"1,234.50", 123450},
		{"$1,234.50", 123450},
		{"0", 0},
		{"0.05", 5},
		{"  $12.00  ", 1200},
		{"1,234,567.89", 123456789},
		{"-350", -35000},
		{"−350.00", -35000},
		{"-$350.00", -35000},
		{"−$1,000.5", -100050},
		{"92233720368547758.07", Money(1<<63 - 1)},
		{"-92233720368547758.08", Money(-1 << 63)},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			// Act
			got, err := ParseMoney(c.in)

			// Assert
			if err != nil {
				t.Fatalf("ParseMoney(%q) error: %v", c.in, err)
			}
			if got != c.want {
				t.Fatalf("ParseMoney(%q) = %d, want %d", c.in, got, c.want)
			}
		})
	}
}

func TestParseMoney_RejectsInvalidInput(t *testing.T) {
	cases := map[string]string{
		"empty":             "",
		"blank":             "   ",
		"only symbol":       "$",
		"only sign":         "-",
		"three decimals":    "12.345",
		"letters":           "12a",
		"words":             "abc",
		"trailing point":    "12.",
		"no whole part":     ".5",
		"two points":        "1.2.3",
		"bad grouping":      "1,23.00",
		"grouping too wide": "1234,567",
		"leading comma":     ",123",
		"double comma":      "1,,234",
		"sign after symbol": "$-5",
		"double sign":       "--5",
		"plus sign":         "+5",
		"inner space":       "1 234",
		"above max":         "92233720368547758.08",
		"below min":         "-92233720368547758.09",
		"far above max":     "999999999999999999999",
		"comma in decimals": "1.234,5",
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			// Act
			_, err := ParseMoney(in)

			// Assert
			if !errors.Is(err, ErrInvalidMoney) {
				t.Fatalf("ParseMoney(%q) error = %v, want ErrInvalidMoney", in, err)
			}
		})
	}
}

func TestMoneyFormat_RendersPesos(t *testing.T) {
	cases := []struct {
		in   Money
		want string
	}{
		{0, "$0.00"},
		{5, "$0.05"},
		{100, "$1.00"},
		{99999, "$999.99"},
		{123450, "$1,234.50"},
		{100000000, "$1,000,000.00"},
		{-35000, "−$350.00"},
		{-1, "−$0.01"},
		{Money(1<<63 - 1), "$92,233,720,368,547,758.07"},
		{Money(-1 << 63), "−$92,233,720,368,547,758.08"},
	}
	for _, c := range cases {
		t.Run(c.want, func(t *testing.T) {
			// Act
			got := c.in.Format()

			// Assert
			if got != c.want {
				t.Fatalf("Money(%d).Format() = %q, want %q", int64(c.in), got, c.want)
			}
		})
	}
}
