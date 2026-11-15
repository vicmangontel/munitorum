package app

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/vicmangontel/munitorum/internal/domain"
)

// Field keys of SetupInput, as they appear in ValidationError.Fields.
const (
	FieldCommunityName = "community_name"
	FieldHouseCount    = "house_count"
	FieldHousePrefix   = "house_prefix"
	FieldFeeAmount     = "fee_amount"
	FieldOpeningDate   = "opening_date"
	FieldHouses        = "houses" // the list as a whole
)

// FieldAccountName is the key of account i's name (0 cash, 1 bank).
func FieldAccountName(i int) string { return fmt.Sprintf("account_%d_name", i) }

// FieldOpeningBalance is the key of account i's opening balance.
func FieldOpeningBalance(i int) string { return fmt.Sprintf("account_%d_balance", i) }

// FieldHouse is the key of a field of house row i (0-based); field is one of
// domain.OpeningCoveredThrough, OpeningCredit, OpeningArrears.
func FieldHouse(i int, field string) string { return fmt.Sprintf("houses[%d].%s", i, field) }

// Validation codes. The TUI maps each to a Spanish message.
const (
	CodeCommunityNameLength  = "community_name_length"
	CodeHouseCountRange      = "house_count_range"
	CodeHousePrefixLength    = "house_prefix_length"
	CodeAmountInvalid        = "amount_invalid"
	CodeFeeNotPositive       = "fee_not_positive"
	CodeAccountNameLength    = "account_name_length"
	CodeAccountNameDuplicate = "account_name_duplicate"
	CodeBalanceNegative      = "balance_negative"
	CodeDateInvalid          = "date_invalid"
	CodeDateFuture           = "date_future"
	CodeMonthInvalid         = "month_invalid"
	CodeHousesMismatch       = "houses_mismatch"
)

// Codes lists every code an operation can return, domain codes included.
var Codes = append([]string{
	CodeCommunityNameLength, CodeHouseCountRange, CodeHousePrefixLength,
	CodeAmountInvalid, CodeFeeNotPositive, CodeAccountNameLength,
	CodeAccountNameDuplicate, CodeBalanceNegative, CodeDateInvalid,
	CodeDateFuture, CodeMonthInvalid, CodeHousesMismatch,
}, domain.OpeningIssues...)

// Field length limits (setup rules 1 and 5), in characters.
const (
	maxCommunityName = 60
	maxAccountName   = 30
)

// SetupInput is what the wizard collects. Every value is the text the treasurer
// typed, so that one validation reports unreadable and out-of-range values
// alike and the TUI holds no rules. Houses has one row per house; row i is
// house number i+1.
type SetupInput struct {
	CommunityName string
	HouseCount    string
	HousePrefix   string
	FeeAmount     string
	Accounts      [2]SetupAccount // 0 is the cash account, 1 the bank account
	OpeningDate   string          // DD/MM/AAAA
	Houses        []SetupHouse
}

// SetupAccount is the name and opening balance typed for one account.
type SetupAccount struct{ Name, OpeningBalance string }

// SetupHouse is the opening state typed for one house. CoveredThrough is
// MM/AAAA or empty; empty Credit and Arrears mean zero.
type SetupHouse struct{ CoveredThrough, Credit, Arrears string }

// SetupSummary is the wizard's last screen, computed from a valid input.
type SetupSummary struct {
	HouseCount      int
	FeeAmount       domain.Money
	OpeningBalances [2]domain.Money
	TotalBalance    domain.Money
	HousesCovered   int // houses with a CoveredThrough month
	TotalCredit     domain.Money
	TotalArrears    domain.Money
}

// parsedSetup is a SetupInput that passed every rule.
type parsedSetup struct {
	name         string
	prefix       string
	count        int
	fee          domain.Money
	accountNames [2]string
	balances     [2]domain.Money
	openingDate  time.Time
	openingMonth domain.Month
	houses       []domain.HouseOpening
}

// parseSetup applies setup rules 1 to 9 and reports every invalid field. The
// result is meaningful only when the error is nil.
func parseSetup(in SetupInput, now time.Time) (parsedSetup, error) {
	var p parsedSetup
	var ve ValidationError

	var ok bool
	if p.name, ok = trimmedName(in.CommunityName, maxCommunityName); !ok {
		ve.add(FieldCommunityName, CodeCommunityNameLength)
	}

	count, err := domain.ParseHouseCount(in.HouseCount)
	if err != nil {
		ve.add(FieldHouseCount, CodeHouseCountRange)
	}
	p.count = count

	p.prefix = in.HousePrefix
	if utf8.RuneCountInString(p.prefix) > domain.MaxHousePrefix {
		ve.add(FieldHousePrefix, CodeHousePrefixLength)
	}

	feeOK := true
	if p.fee, err = domain.ParseMoney(in.FeeAmount); err != nil {
		ve.add(FieldFeeAmount, CodeAmountInvalid)
		feeOK = false
	} else if p.fee <= 0 {
		ve.add(FieldFeeAmount, CodeFeeNotPositive)
		feeOK = false
	}

	for i, a := range in.Accounts {
		if p.accountNames[i], ok = trimmedName(a.Name, maxAccountName); !ok {
			ve.add(FieldAccountName(i), CodeAccountNameLength)
		}
		var code string
		if p.balances[i], code = parseBalance(a.OpeningBalance); code != "" {
			ve.add(FieldOpeningBalance(i), code)
		}
	}
	// Full Unicode case folding; the column's NOCASE collation only folds ASCII.
	if _, dup := ve.Fields[FieldAccountName(0)]; !dup && p.accountNames[1] != "" &&
		strings.EqualFold(p.accountNames[0], p.accountNames[1]) {
		ve.add(FieldAccountName(1), CodeAccountNameDuplicate)
	}

	dateOK := true
	if p.openingDate, err = domain.ParseInputDate(in.OpeningDate); err != nil {
		ve.add(FieldOpeningDate, CodeDateInvalid)
		dateOK = false
	} else if today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local); p.openingDate.After(today) {
		ve.add(FieldOpeningDate, CodeDateFuture)
		dateOK = false
	} else {
		p.openingMonth = domain.Of(p.openingDate)
	}

	if _, bad := ve.Fields[FieldHouseCount]; !bad && len(in.Houses) != p.count {
		ve.add(FieldHouses, CodeHousesMismatch)
	}
	p.houses = make([]domain.HouseOpening, len(in.Houses))
	for i, h := range in.Houses {
		p.houses[i] = parseHouse(&ve, i, h, p, dateOK && feeOK)
	}
	return p, ve.err()
}

// parseHouse reads row i and, when the opening month and cuota are known,
// applies the opening rules to it.
func parseHouse(ve *ValidationError, i int, h SetupHouse, p parsedSetup, canCheck bool) domain.HouseOpening {
	var o domain.HouseOpening
	var err error
	parsed := true

	if s := strings.TrimSpace(h.CoveredThrough); s != "" {
		m, monthErr := domain.ParseInputMonth(s)
		if monthErr != nil {
			ve.add(FieldHouse(i, domain.OpeningCoveredThrough), CodeMonthInvalid)
			parsed = false
		}
		o.CoveredThrough = &m
	}
	if o.Credit, err = parseOptionalMoney(h.Credit); err != nil {
		ve.add(FieldHouse(i, domain.OpeningCredit), CodeAmountInvalid)
		parsed = false
	}
	if o.Arrears, err = parseOptionalMoney(h.Arrears); err != nil {
		ve.add(FieldHouse(i, domain.OpeningArrears), CodeAmountInvalid)
		parsed = false
	}

	if canCheck && parsed {
		for _, issue := range o.Check(p.openingMonth, p.fee) {
			ve.add(FieldHouse(i, issue.Field), issue.Code)
		}
	}
	return o
}

// trimmedName trims raw and reports whether it is 1 to limit characters long
// (setup rules 1 and 5).
func trimmedName(raw string, limit int) (string, bool) {
	name := strings.TrimSpace(raw)
	n := utf8.RuneCountInString(name)
	return name, n >= 1 && n <= limit
}

// parseBalance reads an opening balance (setup rule 6); the code is empty when
// the balance is valid.
func parseBalance(raw string) (domain.Money, string) {
	m, err := domain.ParseMoney(raw)
	switch {
	case err != nil:
		return 0, CodeAmountInvalid
	case m < 0:
		return m, CodeBalanceNegative
	}
	return m, ""
}

// parseOptionalMoney reads an amount where empty means zero.
func parseOptionalMoney(s string) (domain.Money, error) {
	if strings.TrimSpace(s) == "" {
		return 0, nil
	}
	return domain.ParseMoney(s)
}

func (p parsedSetup) summary() SetupSummary {
	s := SetupSummary{
		HouseCount:      p.count,
		FeeAmount:       p.fee,
		OpeningBalances: p.balances,
		TotalBalance:    p.balances[0] + p.balances[1],
	}
	for _, h := range p.houses {
		if h.CoveredThrough != nil {
			s.HousesCovered++
		}
		s.TotalCredit += h.Credit
		s.TotalArrears += h.Arrears
	}
	return s
}
