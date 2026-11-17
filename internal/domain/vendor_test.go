package domain

import (
	"strings"
	"testing"
)

func TestFoldKey_IgnoresCaseAndAccents(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"accent and case", "José", "jose"},
		{"already folded", "jose", "jose"},
		{"upper case with accent", "JOSÉ", "jose"},
		{"tilde on n", "Peña", "pena"},
		{"diaeresis", "Güero", "guero"},
		{"precomposed and decomposed forms fold equal", "é", "e"},
		{"spaces and digits are kept", "Casa 12 B", "casa 12 b"},
		{"empty", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Act
			got := FoldKey(c.in)

			// Assert
			if got != c.want {
				t.Fatalf("FoldKey(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestVendorDraftCheck_AppliesTheFieldRules(t *testing.T) {
	two, one := int64(2), int64(1)
	cases := []struct {
		name  string
		draft VendorDraft
		want  map[string]string // field -> code; nil when valid
	}{
		{"service with only a name", VendorDraft{Kind: VendorService, Name: "Basura"}, nil},
		{"unknown kind", VendorDraft{Kind: "robot", Name: "X"}, map[string]string{"kind": CodeVendorKindInvalid}},
		{"empty kind", VendorDraft{Name: "X"}, map[string]string{"kind": CodeVendorKindInvalid}},
		{"empty name", VendorDraft{Kind: VendorService}, map[string]string{"name": CodeVendorNameLength}},
		{"one character name", VendorDraft{Kind: VendorService, Name: "X"}, nil},
		{"80 character name", VendorDraft{Kind: VendorService, Name: strings.Repeat("ñ", 80)}, nil},
		{"81 character name", VendorDraft{Kind: VendorService, Name: strings.Repeat("ñ", 81)}, map[string]string{"name": CodeVendorNameLength}},
		{"30 character phone", VendorDraft{Kind: VendorService, Name: "X", Phone: strings.Repeat("5", 30)}, nil},
		{"31 character phone", VendorDraft{Kind: VendorService, Name: "X", Phone: strings.Repeat("5", 31)}, map[string]string{"phone": CodeVendorPhoneLength}},
		{"email with one @", VendorDraft{Kind: VendorService, Name: "X", Email: "a@b"}, nil},
		{"no email", VendorDraft{Kind: VendorService, Name: "X"}, nil},
		{"email without @", VendorDraft{Kind: VendorService, Name: "X", Email: "ab"}, map[string]string{"email": CodeVendorEmailInvalid}},
		{"email with nothing before @", VendorDraft{Kind: VendorService, Name: "X", Email: "@b"}, map[string]string{"email": CodeVendorEmailInvalid}},
		{"email with nothing after @", VendorDraft{Kind: VendorService, Name: "X", Email: "a@"}, map[string]string{"email": CodeVendorEmailInvalid}},
		{"email with two @", VendorDraft{Kind: VendorService, Name: "X", Email: "a@b@c"}, map[string]string{"email": CodeVendorEmailInvalid}},
		{"500 character notes", VendorDraft{Kind: VendorService, Name: "X", Notes: strings.Repeat("n", 500)}, nil},
		{"501 character notes", VendorDraft{Kind: VendorService, Name: "X", Notes: strings.Repeat("n", 501)}, map[string]string{"notes": CodeVendorNotesLength}},
		{"person with a company", VendorDraft{Kind: VendorPerson, Name: "X", CompanyID: &two}, nil},
		{"service with a company", VendorDraft{Kind: VendorService, Name: "X", CompanyID: &two}, map[string]string{"company_id": CodeVendorCompanyNotAllowed}},
		{"company with a company", VendorDraft{Kind: VendorCompany, Name: "X", CompanyID: &two}, map[string]string{"company_id": CodeVendorCompanyNotAllowed}},
		{"person as its own company", VendorDraft{ID: 1, Kind: VendorPerson, Name: "X", CompanyID: &one}, map[string]string{"company_id": CodeVendorCompanySelf}},
		{"new person with company id 1", VendorDraft{Kind: VendorPerson, Name: "X", CompanyID: &one}, nil},
		{"every field invalid at once", VendorDraft{Kind: "robot", Name: "", Phone: strings.Repeat("5", 31), Email: "x", Notes: strings.Repeat("n", 501)},
			map[string]string{"kind": CodeVendorKindInvalid, "name": CodeVendorNameLength, "phone": CodeVendorPhoneLength, "email": CodeVendorEmailInvalid, "notes": CodeVendorNotesLength}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Act
			issues := c.draft.Check()

			// Assert
			got := map[string]string{}
			for _, is := range issues {
				got[is.Field] = is.Code
			}
			if len(got) != len(c.want) || len(issues) != len(c.want) {
				t.Fatalf("issues = %v, want %v", issues, c.want)
			}
			for f, code := range c.want {
				if got[f] != code {
					t.Fatalf("issues = %v, want %v", issues, c.want)
				}
			}
		})
	}
}

func TestVendorDraftTrimmed_RemovesSurroundingSpacesFromEveryText(t *testing.T) {
	// Arrange
	d := VendorDraft{Name: " a ", Phone: " b ", Email: " c@d ", Notes: " e "}

	// Act
	got := d.Trimmed()

	// Assert
	if got.Name != "a" || got.Phone != "b" || got.Email != "c@d" || got.Notes != "e" {
		t.Fatalf("Trimmed() = %+v", got)
	}
}

func TestVendorDisplayName_AppendsTheCompanyOnlyWhenThereIsOne(t *testing.T) {
	cases := []struct{ name, vendor, company, want string }{
		{"no company", "Basura", "", "Basura"},
		{"with company", "Juan Gómez", "Portones Acme", "Juan Gómez (Portones Acme)"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Act
			got := VendorDisplayName(c.vendor, c.company)

			// Assert
			if got != c.want {
				t.Fatalf("VendorDisplayName = %q, want %q", got, c.want)
			}
		})
	}
}
