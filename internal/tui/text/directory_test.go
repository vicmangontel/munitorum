package text

import (
	"strings"
	"testing"
)

func TestWithDeletedSuffix_MarksOnlyDeletedVendors(t *testing.T) {
	cases := []struct {
		name    string
		vendor  string
		deleted bool
		want    string
	}{
		{"live vendor is unchanged", "Basura", false, "Basura"},
		{"deleted vendor gets the suffix", "Basura", true, "Basura (eliminado)"},
		{"display name with company", "Juan Gómez (Portones Acme)", true, "Juan Gómez (Portones Acme) (eliminado)"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Act
			got := WithDeletedSuffix(c.vendor, c.deleted)

			// Assert
			if got != c.want {
				t.Fatalf("WithDeletedSuffix(%q, %v) = %q, want %q", c.vendor, c.deleted, got, c.want)
			}
		})
	}
}

func TestVendorKindName_AndVendorKindCode_AreInverseForEveryKind(t *testing.T) {
	cases := []struct{ code, name string }{
		{"person", "Persona"}, {"company", "Empresa"}, {"service", "Servicio"},
	}
	for _, c := range cases {
		t.Run(c.code, func(t *testing.T) {
			// Act
			name, code := VendorKindName(c.code), VendorKindCode(c.name)

			// Assert
			if name != c.name || code != c.code {
				t.Fatalf("name = %q, code = %q; want %q and %q", name, code, c.name, c.code)
			}
		})
	}
}

func TestVendorKindCode_IsEmptyForAnUnknownName(t *testing.T) {
	// Act
	got := VendorKindCode("Robot")

	// Assert
	if got != "" {
		t.Fatalf("VendorKindCode(Robot) = %q, want empty", got)
	}
}

func TestVendorConflictMessage_NamesTheVendorsInvolved(t *testing.T) {
	cases := []struct {
		name   string
		reason string
		names  []string
		want   []string
	}{
		{"company with people", "company_has_members", []string{"Ana (Acme)", "Juan (Acme)"}, []string{"No se puede eliminar", "Ana (Acme), Juan (Acme)"}},
		{"person whose company is deleted", "company_deleted", []string{"Portones Acme"}, []string{"No se puede restaurar", "«Portones Acme»"}},
		{"person whose company is no longer a company", "company_not_company", []string{"Portones Acme"}, []string{"No se puede restaurar", "«Portones Acme» ya no es una empresa"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Act
			got := VendorConflictMessage(c.reason, c.names)

			// Assert
			for _, w := range c.want {
				if !strings.Contains(got, w) {
					t.Fatalf("message %q does not contain %q", got, w)
				}
			}
		})
	}
}

func TestDirectoryFilter_DescribesEveryFilterInForceAndNothingWhenThereIsNone(t *testing.T) {
	cases := []struct {
		name    string
		kind    string
		search  string
		deleted bool
		want    string
	}{
		{"no filter", "", "", false, ""},
		{"kind", "Persona", "", false, "Tipo: Persona"},
		{"search", "", "acme", false, "Buscar: acme"},
		{"deleted", "", "", true, "con eliminados"},
		{"all three", "Empresa", "acme", true, "Tipo: Empresa · Buscar: acme · con eliminados"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Act
			got := DirectoryFilter(c.kind, c.search, c.deleted)

			// Assert
			if got != c.want {
				t.Fatalf("DirectoryFilter = %q, want %q", got, c.want)
			}
		})
	}
}
