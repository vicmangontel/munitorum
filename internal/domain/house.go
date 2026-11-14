package domain

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// House count and prefix defaults and limits (setup rules 2 and 3).
const (
	MinHouseCount      = 1
	MaxHouseCount      = 200
	DefaultHouseCount  = 18
	DefaultHousePrefix = "CASA "
	MaxHousePrefix     = 10 // characters
)

// MaxPrepaidMonths is how far past the opening month a house may be covered.
const MaxPrepaidMonths = 36

// ErrInvalidHouseCount is wrapped by every ParseHouseCount failure.
var ErrInvalidHouseCount = errors.New("invalid house count")

// ParseHouseCount reads the number of houses typed by the treasurer.
func ParseHouseCount(s string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n < MinHouseCount || n > MaxHouseCount {
		return 0, fmt.Errorf("%w %q: want a whole number from %d to %d", ErrInvalidHouseCount, s, MinHouseCount, MaxHouseCount)
	}
	return n, nil
}

// HouseCode is the prefix followed by the house number zero-padded to
// max(2, digits(count)): CASA 01 … CASA 18, or C-001 … C-120.
func HouseCode(prefix string, number, count int) string {
	width := max(2, len(strconv.Itoa(count)))
	return fmt.Sprintf("%s%0*d", prefix, width, number)
}

// HouseOpening is what a house had already paid or owed when the treasurer took over.
type HouseOpening struct {
	CoveredThrough *Month // nil when no month is covered
	Credit         Money
	Arrears        Money
}

// Fields of HouseOpening, as named by OpeningIssue.
const (
	OpeningCoveredThrough = "covered_through"
	OpeningCredit         = "credit"
	OpeningArrears        = "arrears"
)

// Issue codes reported by HouseOpening.Check.
const (
	CodeCoveredBeforeOpening = "covered_before_opening"
	CodeCoveredTooFar        = "covered_too_far"
	CodeCreditOutOfRange     = "credit_out_of_range"
	CodeArrearsNegative      = "arrears_negative"
	CodeArrearsWithCoverage  = "arrears_with_coverage"
	CodeArrearsWithCredit    = "arrears_with_credit"
)

// OpeningIssues lists every code Check can report.
var OpeningIssues = []string{
	CodeCoveredBeforeOpening, CodeCoveredTooFar, CodeCreditOutOfRange,
	CodeArrearsNegative, CodeArrearsWithCoverage, CodeArrearsWithCredit,
}

// OpeningIssue is one broken rule on one field of a house's opening.
type OpeningIssue struct{ Field, Code string }

// Check applies setup rules 8 and 9 and returns at most one issue per field.
// opening is the opening month and fee the initial monthly cuota.
func (h HouseOpening) Check(opening Month, fee Money) []OpeningIssue {
	var issues []OpeningIssue

	if h.CoveredThrough != nil {
		switch {
		case h.CoveredThrough.Before(opening):
			issues = append(issues, OpeningIssue{OpeningCoveredThrough, CodeCoveredBeforeOpening})
		case opening.Add(MaxPrepaidMonths).Before(*h.CoveredThrough):
			issues = append(issues, OpeningIssue{OpeningCoveredThrough, CodeCoveredTooFar})
		}
	}
	if h.Credit < 0 || h.Credit >= fee {
		issues = append(issues, OpeningIssue{OpeningCredit, CodeCreditOutOfRange})
	}
	switch {
	case h.Arrears < 0:
		issues = append(issues, OpeningIssue{OpeningArrears, CodeArrearsNegative})
	case h.Arrears > 0 && h.CoveredThrough != nil:
		issues = append(issues, OpeningIssue{OpeningArrears, CodeArrearsWithCoverage})
	case h.Arrears > 0 && h.Credit > 0:
		issues = append(issues, OpeningIssue{OpeningArrears, CodeArrearsWithCredit})
	}
	return issues
}
