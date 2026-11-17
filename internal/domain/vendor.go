package domain

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// FoldKey is the form of s used to compare and search names without regard to
// case or accents: lower case, decomposed, combining marks dropped. José, JOSÉ
// and jose fold equal. It is not trimmed; callers trim what the treasurer typed.
func FoldKey(s string) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(strings.ToLower(s)) {
		if !unicode.Is(unicode.Mn, r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// VendorKind is what a directory entry is.
type VendorKind string

const (
	VendorPerson  VendorKind = "person"
	VendorCompany VendorKind = "company"
	VendorService VendorKind = "service"
)

// Valid reports whether k is one of the three kinds.
func (k VendorKind) Valid() bool {
	return k == VendorPerson || k == VendorCompany || k == VendorService
}

// Vendor field limits (directory rule 1), in characters.
const (
	MaxVendorName  = 80
	MaxVendorPhone = 30
	MaxVendorNotes = 500
)

// Fields of VendorDraft, as named by VendorIssue and by validation field keys.
const (
	VendorFieldKind      = "kind"
	VendorFieldName      = "name"
	VendorFieldPhone     = "phone"
	VendorFieldEmail     = "email"
	VendorFieldCompanyID = "company_id"
	VendorFieldNotes     = "notes"
)

// Issue codes reported by VendorDraft.Check.
const (
	CodeVendorKindInvalid       = "vendor_kind_invalid"
	CodeVendorNameLength        = "vendor_name_length"
	CodeVendorPhoneLength       = "vendor_phone_length"
	CodeVendorEmailInvalid      = "vendor_email_invalid"
	CodeVendorNotesLength       = "vendor_notes_length"
	CodeVendorCompanyNotAllowed = "vendor_company_not_allowed"
	CodeVendorCompanySelf       = "vendor_company_self"
)

// VendorIssues lists every code Check can report.
var VendorIssues = []string{
	CodeVendorKindInvalid, CodeVendorNameLength, CodeVendorPhoneLength,
	CodeVendorEmailInvalid, CodeVendorNotesLength,
	CodeVendorCompanyNotAllowed, CodeVendorCompanySelf,
}

// VendorDraft is the typed content of a directory entry. ID is 0 for a new one.
type VendorDraft struct {
	ID                        int64
	Kind                      VendorKind
	Name, Phone, Email, Notes string
	CompanyID                 *int64
}

// VendorIssue is one broken rule on one field of a vendor.
type VendorIssue struct{ Field, Code string }

// Trimmed returns the draft with surrounding spaces removed from every text field.
func (v VendorDraft) Trimmed() VendorDraft {
	v.Name = strings.TrimSpace(v.Name)
	v.Phone = strings.TrimSpace(v.Phone)
	v.Email = strings.TrimSpace(v.Email)
	v.Notes = strings.TrimSpace(v.Notes)
	return v
}

// Check applies directory rules 1, 3 and 4 to a trimmed draft and returns at
// most one issue per field. That the company exists, is live and is a company
// needs the other vendors, so the application checks it.
func (v VendorDraft) Check() []VendorIssue {
	var issues []VendorIssue
	if !v.Kind.Valid() {
		issues = append(issues, VendorIssue{VendorFieldKind, CodeVendorKindInvalid})
	}
	if n := utf8.RuneCountInString(v.Name); n < 1 || n > MaxVendorName {
		issues = append(issues, VendorIssue{VendorFieldName, CodeVendorNameLength})
	}
	if utf8.RuneCountInString(v.Phone) > MaxVendorPhone {
		issues = append(issues, VendorIssue{VendorFieldPhone, CodeVendorPhoneLength})
	}
	if v.Email != "" && !validEmail(v.Email) {
		issues = append(issues, VendorIssue{VendorFieldEmail, CodeVendorEmailInvalid})
	}
	if utf8.RuneCountInString(v.Notes) > MaxVendorNotes {
		issues = append(issues, VendorIssue{VendorFieldNotes, CodeVendorNotesLength})
	}
	switch {
	case v.CompanyID == nil:
	case v.Kind != VendorPerson:
		issues = append(issues, VendorIssue{VendorFieldCompanyID, CodeVendorCompanyNotAllowed})
	case v.ID != 0 && *v.CompanyID == v.ID:
		issues = append(issues, VendorIssue{VendorFieldCompanyID, CodeVendorCompanySelf})
	}
	return issues
}

// validEmail is one '@' with text on both sides; nothing stricter is required.
func validEmail(s string) bool {
	local, domain, ok := strings.Cut(s, "@")
	return ok && local != "" && domain != "" && !strings.Contains(domain, "@")
}

// VendorDisplayName is how a vendor shows everywhere: a person with a company
// is "Name (Company)".
func VendorDisplayName(name, company string) string {
	if company == "" {
		return name
	}
	return name + " (" + company + ")"
}
