package domain

import (
	"testing"

	"pgregory.net/rapid"
)

// Whatever the treasurer sees on screen must parse back to the same amount,
// otherwise editing a saved amount would silently change it.
func TestParseMoney_RoundTripsFormat(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		m := Money(rapid.Int64().Draw(t, "centavos"))

		got, err := ParseMoney(m.Format())

		if err != nil {
			t.Fatalf("ParseMoney(%q) error: %v", m.Format(), err)
		}
		if got != m {
			t.Fatalf("ParseMoney(Format(%d)) = %d", int64(m), int64(got))
		}
	})
}

// Small amounts hit the sign and grouping edges far more often than the full
// int64 range does.
func TestParseMoney_RoundTripsFormatForSmallAmounts(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		m := Money(rapid.Int64Range(-10_000_000, 10_000_000).Draw(t, "centavos"))

		got, err := ParseMoney(m.Format())

		if err != nil {
			t.Fatalf("ParseMoney(%q) error: %v", m.Format(), err)
		}
		if got != m {
			t.Fatalf("ParseMoney(Format(%d)) = %d", int64(m), int64(got))
		}
	})
}
