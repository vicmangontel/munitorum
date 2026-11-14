package domain

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Money is an amount in MXN centavos. It is never a float: rounding never
// happens implicitly.
type Money int64

// ErrInvalidMoney is wrapped by every ParseMoney failure.
var ErrInvalidMoney = errors.New("invalid money amount")

const minus = "−" // U+2212, how negatives are displayed

// ParseMoney reads an amount typed by the treasurer: 1234, 1234.5, 1,234.50,
// $1,234.50, optionally negative with a leading - or −. More than two decimals,
// letters, and empty input are rejected rather than rounded or repaired.
func ParseMoney(s string) (Money, error) {
	bad := func(why string) (Money, error) {
		return 0, fmt.Errorf("%w %q: %s", ErrInvalidMoney, s, why)
	}

	rest := strings.TrimSpace(s)
	negative := false
	switch {
	case strings.HasPrefix(rest, "-"):
		negative, rest = true, rest[1:]
	case strings.HasPrefix(rest, minus):
		negative, rest = true, rest[len(minus):]
	}
	rest = strings.TrimPrefix(rest, "$")
	if rest == "" {
		return bad("empty")
	}

	whole, frac, hasPoint := strings.Cut(rest, ".")
	if hasPoint && (frac == "" || len(frac) > 2) {
		return bad("needs one or two decimals")
	}
	whole, ok := stripGrouping(whole)
	if !ok {
		return bad("not a number")
	}
	if !isDigits(frac) {
		return bad("not a number")
	}

	digits := whole + (frac + "00")[:2]
	n, err := strconv.ParseUint(digits, 10, 64)
	if err != nil {
		return bad("out of range")
	}
	if negative {
		if n > 1<<63 {
			return bad("out of range")
		}
		return Money(-int64(n)), nil // n == 1<<63 wraps to MinInt64, which is correct
	}
	if n > 1<<63-1 {
		return bad("out of range")
	}
	return Money(n), nil
}

// stripGrouping removes thousands separators. Commas are accepted only in
// groups of three after a first group of one to three digits.
func stripGrouping(whole string) (string, bool) {
	if !strings.Contains(whole, ",") {
		return whole, whole != "" && isDigits(whole)
	}
	groups := strings.Split(whole, ",")
	if len(groups[0]) < 1 || len(groups[0]) > 3 {
		return "", false
	}
	for _, g := range groups[1:] {
		if len(g) != 3 {
			return "", false
		}
	}
	joined := strings.Join(groups, "")
	return joined, isDigits(joined)
}

func isDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// Format renders m as $1,234.50, or −$350.00 when negative.
func (m Money) Format() string {
	abs := uint64(m)
	sign := ""
	if m < 0 {
		abs, sign = -uint64(m), minus // two's complement: also right for MinInt64
	}
	whole := strconv.FormatUint(abs/100, 10)
	for i := len(whole) - 3; i > 0; i -= 3 {
		whole = whole[:i] + "," + whole[i:]
	}
	return fmt.Sprintf("%s$%s.%02d", sign, whole, abs%100)
}
