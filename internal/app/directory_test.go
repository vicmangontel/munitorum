package app_test

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vicmangontel/munitorum/internal/app"
	"github.com/vicmangontel/munitorum/internal/domain"
	"github.com/vicmangontel/munitorum/internal/store"
)

// newDirectory returns a service over a migrated temporary database, the
// database for direct inspection, and the buffer receiving the wide events.
func newDirectory(t *testing.T) (*app.DirectoryService, *sql.DB, *bytes.Buffer) {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := store.Migrate(db, store.Migrations()); err != nil {
		t.Fatal(err)
	}
	events := &bytes.Buffer{}
	log := slog.New(slog.NewJSONHandler(events, nil))
	return app.NewDirectoryService(store.NewVendorRepository(db), fixedClock{today}, log), db, events
}

// create writes a vendor that the test needs as given state; a failure is a
// broken test, not a result.
func create(t *testing.T, svc *app.DirectoryService, in app.VendorInput) app.Vendor {
	t.Helper()
	v, err := svc.CreateVendor(context.Background(), in)
	if err != nil {
		t.Fatalf("arrange: create %q: %v", in.Name, err)
	}
	return v
}

func person(name string) app.VendorInput {
	return app.VendorInput{Kind: domain.VendorPerson, Name: name}
}
func company(name string) app.VendorInput {
	return app.VendorInput{Kind: domain.VendorCompany, Name: name}
}
func service(name string) app.VendorInput {
	return app.VendorInput{Kind: domain.VendorService, Name: name}
}

func personOf(name string, c app.Vendor) app.VendorInput {
	in := person(name)
	in.CompanyID = &c.ID
	return in
}

func listed(t *testing.T, svc *app.DirectoryService, in app.ListVendorsInput) []string {
	t.Helper()
	rows, err := svc.ListVendors(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, len(rows))
	for i, r := range rows {
		names[i] = r.DisplayName()
	}
	return names
}

func equalStrings(a, b []string) bool { return strings.Join(a, "|") == strings.Join(b, "|") }

func TestCreateVendor_SavesAServiceWithOnlyAName(t *testing.T) {
	// Arrange
	svc, db, _ := newDirectory(t)

	// Act
	got, err := svc.CreateVendor(context.Background(), service("Basura"))

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	var phone, email, notes, companyID sql.NullString
	var created string
	err = db.QueryRow(`SELECT phone, email, notes, company_id, created_at FROM vendors WHERE id = ?`, got.ID).
		Scan(&phone, &email, &notes, &companyID, &created)
	if err != nil {
		t.Fatal(err)
	}
	if phone.Valid || email.Valid || notes.Valid || companyID.Valid {
		t.Fatalf("optional columns = %v %v %v %v, want all NULL", phone, email, notes, companyID)
	}
	if created != "2026-11-13 09:30:00" {
		t.Fatalf("created_at = %q, want the clock's time", created)
	}
}

func TestCreateVendor_TrimsTextAndStoresBlankOptionalsAsNull(t *testing.T) {
	// Arrange
	svc, db, _ := newDirectory(t)

	// Act
	got, err := svc.CreateVendor(context.Background(), app.VendorInput{
		Kind: domain.VendorService, Name: "  CFE  ", Phone: "   ", Email: "  ", Notes: " \t ",
	})

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	var name string
	var phone, email, notes sql.NullString
	if err := db.QueryRow(`SELECT name, phone, email, notes FROM vendors WHERE id = ?`, got.ID).Scan(&name, &phone, &email, &notes); err != nil {
		t.Fatal(err)
	}
	if name != "CFE" || phone.Valid || email.Valid || notes.Valid {
		t.Fatalf("stored %q %v %v %v, want the trimmed name and NULLs", name, phone, email, notes)
	}
}

func TestCreateVendor_StoresEveryField(t *testing.T) {
	// Arrange
	svc, _, _ := newDirectory(t)
	acme := create(t, svc, company("Portones Acme"))

	// Act
	got, err := svc.CreateVendor(context.Background(), app.VendorInput{
		Kind: domain.VendorPerson, Name: "Juan Gómez", Phone: "555 1234", Email: "juan@acme.mx",
		CompanyID: &acme.ID, Notes: "Técnico de portones",
	})

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	detail, err := svc.GetVendor(context.Background(), got.ID)
	if err != nil {
		t.Fatal(err)
	}
	v := detail.Vendor
	if v.Kind != domain.VendorPerson || v.Name != "Juan Gómez" || v.Phone != "555 1234" || v.Email != "juan@acme.mx" ||
		v.Notes != "Técnico de portones" || v.CompanyID == nil || *v.CompanyID != acme.ID || v.Deleted {
		t.Fatalf("stored vendor = %+v", v)
	}
}

func TestCreateVendor_ReportsEveryInvalidFieldAtOnce(t *testing.T) {
	// Arrange
	svc, _, _ := newDirectory(t)
	acme := create(t, svc, company("Portones Acme"))

	// Act
	_, err := svc.CreateVendor(context.Background(), app.VendorInput{
		Kind: domain.VendorService, Name: " ", Phone: strings.Repeat("5", 31), Email: "sin-arroba",
		CompanyID: &acme.ID, Notes: strings.Repeat("n", 501),
	})

	// Assert
	want := map[string]string{
		"name": domain.CodeVendorNameLength, "phone": domain.CodeVendorPhoneLength,
		"email": domain.CodeVendorEmailInvalid, "company_id": domain.CodeVendorCompanyNotAllowed,
		"notes": domain.CodeVendorNotesLength,
	}
	got := invalidFields(t, err)
	if len(got) != len(want) {
		t.Fatalf("invalid fields = %v, want %v", got, want)
	}
	for f, code := range want {
		if got[f] != code {
			t.Fatalf("invalid fields = %v, want %v", got, want)
		}
	}
}

func TestCreateVendor_RejectsAnUnknownKind(t *testing.T) {
	// Arrange
	svc, _, _ := newDirectory(t)

	// Act
	_, err := svc.CreateVendor(context.Background(), app.VendorInput{Kind: "robot", Name: "R2"})

	// Assert
	if got := invalidFields(t, err); len(got) != 1 || got["kind"] != domain.CodeVendorKindInvalid {
		t.Fatalf("invalid fields = %v, want only the kind", got)
	}
}

func TestCreateVendor_RejectsACompanyThatIsNotALiveCompany(t *testing.T) {
	// Arrange
	svc, _, _ := newDirectory(t)
	ctx := context.Background()
	basura := create(t, svc, service("Basura"))
	maria := create(t, svc, person("María"))
	gone := create(t, svc, company("Cerrada SA"))
	if err := svc.DeleteVendor(ctx, gone.ID); err != nil {
		t.Fatal(err)
	}
	missing := int64(9999)

	cases := []struct {
		name      string
		companyID *int64
	}{
		{"a service", &basura.ID},
		{"another person", &maria.ID},
		{"a deleted company", &gone.ID},
		{"an id that does not exist", &missing},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Act
			in := person("Juan")
			in.CompanyID = c.companyID
			_, err := svc.CreateVendor(ctx, in)

			// Assert
			if got := invalidFields(t, err); len(got) != 1 || got["company_id"] != app.CodeVendorCompanyInvalid {
				t.Fatalf("invalid fields = %v, want only company_id %s", got, app.CodeVendorCompanyInvalid)
			}
			if rows := listed(t, svc, app.ListVendorsInput{Search: "Juan"}); len(rows) != 0 {
				t.Fatalf("rejected vendor was stored: %v", rows)
			}
		})
	}
}

func TestCreateVendor_NamesAreUniqueAmongLiveVendorsOfTheSameKind(t *testing.T) {
	cases := []struct {
		name      string
		existing  app.VendorInput
		deleted   bool
		candidate app.VendorInput
		wantDup   bool
	}{
		{"same name other case and accent, same kind", person("José"), false, person("jose"), true},
		{"identical name", person("José"), false, person("José"), true},
		{"surrounding spaces ignored", person("José"), false, person("  JOSÉ "), true},
		{"a company with the name of a person", person("José"), false, company("José"), false},
		{"a service with the name of a company", company("Basura"), false, service("basura"), false},
		{"a deleted vendor does not block", person("José"), true, person("jose"), false},
		{"different names", person("José"), false, person("Josefa"), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange
			svc, _, _ := newDirectory(t)
			existing := create(t, svc, c.existing)
			if c.deleted {
				if err := svc.DeleteVendor(context.Background(), existing.ID); err != nil {
					t.Fatal(err)
				}
			}

			// Act
			_, err := svc.CreateVendor(context.Background(), c.candidate)

			// Assert
			if !c.wantDup {
				if err != nil {
					t.Fatalf("error = %v, want the vendor saved", err)
				}
				return
			}
			if got := invalidFields(t, err); len(got) != 1 || got["name"] != app.CodeVendorNameDuplicate {
				t.Fatalf("invalid fields = %v, want only name %s", got, app.CodeVendorNameDuplicate)
			}
		})
	}
}

func TestListVendors_ShowsAPersonWithTheirCompanyAsNameAndCompany(t *testing.T) {
	// Arrange
	svc, _, _ := newDirectory(t)
	acme := create(t, svc, company("Portones Acme"))
	create(t, svc, personOf("Juan Gómez", acme))

	// Act
	names := listed(t, svc, app.ListVendorsInput{Kind: domain.VendorPerson})

	// Assert
	if want := []string{"Juan Gómez (Portones Acme)"}; !equalStrings(names, want) {
		t.Fatalf("display names = %v, want %v", names, want)
	}
}

func TestListVendors_SearchMatchesNamePhoneEmailAndCompanyIgnoringCaseAndAccents(t *testing.T) {
	// Arrange
	svc, _, _ := newDirectory(t)
	acme := create(t, svc, company("Portones Acme"))
	create(t, svc, app.VendorInput{Kind: domain.VendorPerson, Name: "Juan Gómez", Phone: "555 0101", Email: "jg@correo.mx", CompanyID: &acme.ID})
	create(t, svc, person("José Pérez"))
	create(t, svc, service("Basura"))

	cases := []struct {
		name, search string
		want         []string
	}{
		{"by company name finds the company and its person", "acme", []string{"Juan Gómez (Portones Acme)", "Portones Acme"}},
		{"company name in upper case", "ACME", []string{"Juan Gómez (Portones Acme)", "Portones Acme"}},
		{"name without accent", "gomez", []string{"Juan Gómez (Portones Acme)"}},
		{"name with accent typed for an unaccented name", "ÚAN", []string{"Juan Gómez (Portones Acme)"}},
		{"accent in the typed text only", "jose", []string{"José Pérez"}},
		{"phone", "0101", []string{"Juan Gómez (Portones Acme)"}},
		{"email", "CORREO", []string{"Juan Gómez (Portones Acme)"}},
		{"blank search lists everything", "   ", []string{"Basura", "José Pérez", "Juan Gómez (Portones Acme)", "Portones Acme"}},
		{"nothing matches", "zzz", []string{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Act
			got := listed(t, svc, app.ListVendorsInput{Search: c.search})

			// Assert
			if !equalStrings(got, c.want) {
				t.Fatalf("search %q = %v, want %v", c.search, got, c.want)
			}
		})
	}
}

func TestListVendors_FiltersByKind(t *testing.T) {
	// Arrange
	svc, _, _ := newDirectory(t)
	create(t, svc, person("Ana"))
	create(t, svc, company("Beta SA"))
	create(t, svc, service("Basura"))

	cases := []struct {
		name string
		kind domain.VendorKind
		want []string
	}{
		{"every kind", "", []string{"Ana", "Basura", "Beta SA"}},
		{"persons", domain.VendorPerson, []string{"Ana"}},
		{"companies", domain.VendorCompany, []string{"Beta SA"}},
		{"services", domain.VendorService, []string{"Basura"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Act
			got := listed(t, svc, app.ListVendorsInput{Kind: c.kind})

			// Assert
			if !equalStrings(got, c.want) {
				t.Fatalf("list = %v, want %v", got, c.want)
			}
		})
	}
}

func TestListVendors_OrdersByNameIgnoringAccentsAndCase(t *testing.T) {
	// Arrange
	svc, _, _ := newDirectory(t)
	for _, name := range []string{"zoe", "Álvaro", "beto", "Ana"} {
		create(t, svc, person(name))
	}

	// Act
	got := listed(t, svc, app.ListVendorsInput{})

	// Assert
	if want := []string{"Álvaro", "Ana", "beto", "zoe"}; !equalStrings(got, want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
}

func TestListVendors_ExcludesDeletedVendorsUnlessAsked(t *testing.T) {
	// Arrange
	svc, _, _ := newDirectory(t)
	ctx := context.Background()
	create(t, svc, service("Basura"))
	cfe := create(t, svc, service("CFE"))
	if err := svc.DeleteVendor(ctx, cfe.ID); err != nil {
		t.Fatal(err)
	}

	// Act
	live := listed(t, svc, app.ListVendorsInput{})
	rows, err := svc.ListVendors(ctx, app.ListVendorsInput{IncludeDeleted: true})

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"Basura"}; !equalStrings(live, want) {
		t.Fatalf("live list = %v, want %v", live, want)
	}
	if len(rows) != 2 || rows[0].Deleted || !rows[1].Deleted || rows[1].Name != "CFE" {
		t.Fatalf("list with deleted = %+v, want Basura live then CFE deleted", rows)
	}
}

func TestListVendors_KeepsTheCompanyNameOfAPersonWhoseCompanyIsDeleted(t *testing.T) {
	// Arrange
	svc, _, _ := newDirectory(t)
	ctx := context.Background()
	acme := create(t, svc, company("Portones Acme"))
	juan := create(t, svc, personOf("Juan", acme))
	for _, id := range []int64{juan.ID, acme.ID} {
		if err := svc.DeleteVendor(ctx, id); err != nil {
			t.Fatal(err)
		}
	}

	// Act
	got := listed(t, svc, app.ListVendorsInput{IncludeDeleted: true, Kind: domain.VendorPerson})

	// Assert
	if want := []string{"Juan (Portones Acme)"}; !equalStrings(got, want) {
		t.Fatalf("list = %v, want %v", got, want)
	}
}

func TestGetVendor_ReturnsALiveCompanyItsLiveMembersOnly(t *testing.T) {
	// Arrange
	svc, _, _ := newDirectory(t)
	ctx := context.Background()
	acme := create(t, svc, company("Portones Acme"))
	create(t, svc, personOf("Zoe", acme))
	gone := create(t, svc, personOf("Beto", acme))
	create(t, svc, personOf("Ana", acme))
	create(t, svc, company("Otra SA"))
	if err := svc.DeleteVendor(ctx, gone.ID); err != nil {
		t.Fatal(err)
	}

	// Act
	detail, err := svc.GetVendor(ctx, acme.ID)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	var members []string
	for _, m := range detail.Members {
		members = append(members, m.Name)
	}
	if !equalStrings(members, []string{"Ana", "Zoe"}) || detail.Company != nil {
		t.Fatalf("members = %v, company = %v; want Ana and Zoe, no company", members, detail.Company)
	}
}

func TestGetVendor_ReturnsAPersonsCompanyEvenWhenTheCompanyIsDeleted(t *testing.T) {
	// Arrange
	svc, _, _ := newDirectory(t)
	ctx := context.Background()
	acme := create(t, svc, company("Portones Acme"))
	juan := create(t, svc, personOf("Juan", acme))
	if err := svc.DeleteVendor(ctx, juan.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteVendor(ctx, acme.ID); err != nil {
		t.Fatal(err)
	}

	// Act
	detail, err := svc.GetVendor(ctx, juan.ID)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if detail.Company == nil || detail.Company.Name != "Portones Acme" || !detail.Company.Deleted || !detail.Vendor.Deleted {
		t.Fatalf("detail = %+v, want the deleted company named Portones Acme", detail)
	}
}

func TestGetVendor_ReturnsNotFoundForAnUnknownId(t *testing.T) {
	// Arrange
	svc, _, _ := newDirectory(t)

	// Act
	_, err := svc.GetVendor(context.Background(), 42)

	// Assert
	if !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
}

func TestUpdateVendor_RewritesTheFieldsAndClearsBlankOnes(t *testing.T) {
	// Arrange
	svc, _, _ := newDirectory(t)
	ctx := context.Background()
	v := create(t, svc, app.VendorInput{Kind: domain.VendorService, Name: "Basura", Phone: "555", Email: "a@b", Notes: "x"})

	// Act
	_, err := svc.UpdateVendor(ctx, app.UpdateVendorInput{ID: v.ID, VendorInput: app.VendorInput{Kind: domain.VendorService, Name: "Recolección", Phone: " "}})

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	detail, err := svc.GetVendor(ctx, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := detail.Vendor; got.Name != "Recolección" || got.Phone != "" || got.Email != "" || got.Notes != "" {
		t.Fatalf("stored vendor = %+v, want the new name and blanks cleared", got)
	}
}

func TestUpdateVendor_AVendorMayChangeTheCaseOfItsOwnName(t *testing.T) {
	// Arrange
	svc, _, _ := newDirectory(t)
	v := create(t, svc, person("jose"))

	// Act
	_, err := svc.UpdateVendor(context.Background(), app.UpdateVendorInput{ID: v.ID, VendorInput: person("José")})

	// Assert
	if err != nil {
		t.Fatalf("error = %v, want the rename saved", err)
	}
}

func TestUpdateVendor_RejectsTheNameOfAnotherLiveVendorOfTheSameKind(t *testing.T) {
	// Arrange
	svc, _, _ := newDirectory(t)
	create(t, svc, person("José"))
	other := create(t, svc, person("Pedro"))

	// Act
	_, err := svc.UpdateVendor(context.Background(), app.UpdateVendorInput{ID: other.ID, VendorInput: person("jose")})

	// Assert
	if got := invalidFields(t, err); len(got) != 1 || got["name"] != app.CodeVendorNameDuplicate {
		t.Fatalf("invalid fields = %v, want only the duplicate name", got)
	}
}

func TestUpdateVendor_RejectsMakingAVendorItsOwnCompany(t *testing.T) {
	// Arrange
	svc, _, _ := newDirectory(t)
	v := create(t, svc, person("Ana"))
	in := person("Ana")
	in.CompanyID = &v.ID

	// Act
	_, err := svc.UpdateVendor(context.Background(), app.UpdateVendorInput{ID: v.ID, VendorInput: in})

	// Assert
	if got := invalidFields(t, err); len(got) != 1 || got["company_id"] != domain.CodeVendorCompanySelf {
		t.Fatalf("invalid fields = %v, want only company_id %s", got, domain.CodeVendorCompanySelf)
	}
}

func TestUpdateVendor_ACompanyWithLivePersonsCannotChangeKind(t *testing.T) {
	// Arrange
	svc, _, _ := newDirectory(t)
	acme := create(t, svc, company("Portones Acme"))
	create(t, svc, personOf("Juan", acme))

	// Act
	_, err := svc.UpdateVendor(context.Background(), app.UpdateVendorInput{ID: acme.ID, VendorInput: service("Portones Acme")})

	// Assert
	if got := invalidFields(t, err); len(got) != 1 || got["kind"] != app.CodeVendorKindInUse {
		t.Fatalf("invalid fields = %v, want only kind %s", got, app.CodeVendorKindInUse)
	}
	if kind := storedKind(t, svc, acme.ID); kind != domain.VendorCompany {
		t.Fatalf("stored kind = %s, want it unchanged", kind)
	}
}

func TestUpdateVendor_ACompanyWithoutLivePersonsMayChangeKind(t *testing.T) {
	// Arrange
	svc, _, _ := newDirectory(t)
	ctx := context.Background()
	acme := create(t, svc, company("Portones Acme"))
	juan := create(t, svc, personOf("Juan", acme))
	if err := svc.DeleteVendor(ctx, juan.ID); err != nil {
		t.Fatal(err)
	}

	// Act
	_, err := svc.UpdateVendor(ctx, app.UpdateVendorInput{ID: acme.ID, VendorInput: service("Portones Acme")})

	// Assert
	if err != nil {
		t.Fatalf("error = %v, want the kind changed", err)
	}
	if kind := storedKind(t, svc, acme.ID); kind != domain.VendorService {
		t.Fatalf("stored kind = %s, want service", kind)
	}
}

func TestUpdateVendor_ReturnsNotFoundForAnUnknownOrDeletedVendor(t *testing.T) {
	// Arrange
	svc, _, _ := newDirectory(t)
	ctx := context.Background()
	gone := create(t, svc, service("Basura"))
	if err := svc.DeleteVendor(ctx, gone.ID); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		id   int64
	}{{"unknown", 9999}, {"deleted", gone.ID}}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Act
			_, err := svc.UpdateVendor(ctx, app.UpdateVendorInput{ID: c.id, VendorInput: service("Otro")})

			// Assert
			if !errors.Is(err, app.ErrNotFound) {
				t.Fatalf("error = %v, want ErrNotFound", err)
			}
		})
	}
}

func storedKind(t *testing.T, svc *app.DirectoryService, id int64) domain.VendorKind {
	t.Helper()
	d, err := svc.GetVendor(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return d.Vendor.Kind
}

func TestDeleteVendor_ACompanyWithLiveMembersIsRefusedAndTheMembersAreNamed(t *testing.T) {
	// Arrange
	svc, _, _ := newDirectory(t)
	ctx := context.Background()
	acme := create(t, svc, company("Portones Acme"))
	create(t, svc, personOf("Juan Gómez", acme))
	create(t, svc, personOf("Ana Ruiz", acme))

	// Act
	err := svc.DeleteVendor(ctx, acme.ID)

	// Assert
	var conflict app.VendorConflict
	if !errors.As(err, &conflict) || !errors.Is(err, app.ErrConflict) || conflict.Reason != app.ConflictCompanyHasMembers {
		t.Fatalf("error = %v, want a VendorConflict (company_has_members) matching ErrConflict", err)
	}
	if want := []string{"Ana Ruiz (Portones Acme)", "Juan Gómez (Portones Acme)"}; !equalStrings(conflict.Names, want) {
		t.Fatalf("named members = %v, want %v", conflict.Names, want)
	}
	if got := listed(t, svc, app.ListVendorsInput{Kind: domain.VendorCompany}); !equalStrings(got, []string{"Portones Acme"}) {
		t.Fatalf("the company was deleted anyway: %v", got)
	}
}

func TestDeleteVendor_ACompanyIsDeletableOnceItsMembersAreDeleted(t *testing.T) {
	// Arrange
	svc, _, _ := newDirectory(t)
	ctx := context.Background()
	acme := create(t, svc, company("Portones Acme"))
	juan := create(t, svc, personOf("Juan", acme))
	if err := svc.DeleteVendor(ctx, juan.ID); err != nil {
		t.Fatal(err)
	}

	// Act
	err := svc.DeleteVendor(ctx, acme.ID)

	// Assert
	if err != nil {
		t.Fatalf("error = %v, want the company deleted", err)
	}
	if got := listed(t, svc, app.ListVendorsInput{}); len(got) != 0 {
		t.Fatalf("live vendors = %v, want none", got)
	}
}

func TestDeleteVendor_SetsDeletedAtAndNeverRemovesTheRow(t *testing.T) {
	// Arrange
	svc, db, _ := newDirectory(t)
	v := create(t, svc, service("Basura"))

	// Act
	err := svc.DeleteVendor(context.Background(), v.ID)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	var deletedAt sql.NullString
	if err := db.QueryRow(`SELECT deleted_at FROM vendors WHERE id = ?`, v.ID).Scan(&deletedAt); err != nil {
		t.Fatalf("the row is gone: %v", err)
	}
	if deletedAt.String != "2026-11-13 09:30:00" {
		t.Fatalf("deleted_at = %v, want the clock's time", deletedAt)
	}
}

func TestDeleteVendor_ReturnsNotFoundForAnUnknownOrAlreadyDeletedVendor(t *testing.T) {
	// Arrange
	svc, _, _ := newDirectory(t)
	ctx := context.Background()
	gone := create(t, svc, service("Basura"))
	if err := svc.DeleteVendor(ctx, gone.ID); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		id   int64
	}{{"unknown", 9999}, {"already deleted", gone.ID}}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Act
			err := svc.DeleteVendor(ctx, c.id)

			// Assert
			if !errors.Is(err, app.ErrNotFound) {
				t.Fatalf("error = %v, want ErrNotFound", err)
			}
		})
	}
}

func TestRestoreVendor_BringsADeletedVendorBack(t *testing.T) {
	// Arrange
	svc, db, _ := newDirectory(t)
	ctx := context.Background()
	v := create(t, svc, service("Basura"))
	if err := svc.DeleteVendor(ctx, v.ID); err != nil {
		t.Fatal(err)
	}

	// Act
	err := svc.RestoreVendor(ctx, v.ID)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if got := listed(t, svc, app.ListVendorsInput{}); !equalStrings(got, []string{"Basura"}) {
		t.Fatalf("live vendors = %v, want Basura back", got)
	}
	var deletedAt sql.NullString
	if err := db.QueryRow(`SELECT deleted_at FROM vendors WHERE id = ?`, v.ID).Scan(&deletedAt); err != nil || deletedAt.Valid {
		t.Fatalf("deleted_at = %v, err = %v, want NULL", deletedAt, err)
	}
}

func TestRestoreVendor_IsRejectedWhenTheNameNowCollidesWithALiveVendor(t *testing.T) {
	// Arrange
	svc, _, _ := newDirectory(t)
	ctx := context.Background()
	old := create(t, svc, person("José"))
	if err := svc.DeleteVendor(ctx, old.ID); err != nil {
		t.Fatal(err)
	}
	create(t, svc, person("jose"))

	// Act
	err := svc.RestoreVendor(ctx, old.ID)

	// Assert
	if got := invalidFields(t, err); len(got) != 1 || got["name"] != app.CodeVendorNameDuplicate {
		t.Fatalf("invalid fields = %v, want only the duplicate name", got)
	}
	rows, listErr := svc.ListVendors(ctx, app.ListVendorsInput{IncludeDeleted: true})
	if listErr != nil || len(rows) != 2 || !rows[0].Deleted {
		t.Fatalf("rows = %+v, err = %v, want José still deleted", rows, listErr)
	}
}

func TestRestoreVendor_AllowsAnEqualNameOfAnotherKind(t *testing.T) {
	// Arrange
	svc, _, _ := newDirectory(t)
	ctx := context.Background()
	old := create(t, svc, person("José"))
	if err := svc.DeleteVendor(ctx, old.ID); err != nil {
		t.Fatal(err)
	}
	create(t, svc, company("José"))

	// Act
	err := svc.RestoreVendor(ctx, old.ID)

	// Assert
	if err != nil {
		t.Fatalf("error = %v, want the person restored beside the company", err)
	}
}

func TestRestoreVendor_APersonWhoseCompanyIsDeletedIsRefusedAndTheCompanyIsNamed(t *testing.T) {
	// Arrange
	svc, _, _ := newDirectory(t)
	ctx := context.Background()
	acme := create(t, svc, company("Portones Acme"))
	juan := create(t, svc, personOf("Juan", acme))
	for _, id := range []int64{juan.ID, acme.ID} {
		if err := svc.DeleteVendor(ctx, id); err != nil {
			t.Fatal(err)
		}
	}

	// Act
	err := svc.RestoreVendor(ctx, juan.ID)

	// Assert
	var conflict app.VendorConflict
	if !errors.As(err, &conflict) || !errors.Is(err, app.ErrConflict) || conflict.Reason != app.ConflictCompanyDeleted {
		t.Fatalf("error = %v, want a VendorConflict (company_deleted) matching ErrConflict", err)
	}
	if !equalStrings(conflict.Names, []string{"Portones Acme"}) {
		t.Fatalf("named company = %v, want Portones Acme", conflict.Names)
	}
	if got := listed(t, svc, app.ListVendorsInput{}); len(got) != 0 {
		t.Fatalf("live vendors = %v, want none restored", got)
	}
}

func TestRestoreVendor_APersonKeepsTheCompanyLinkAndIsRestoredOnceTheCompanyIs(t *testing.T) {
	// Arrange
	svc, _, _ := newDirectory(t)
	ctx := context.Background()
	acme := create(t, svc, company("Portones Acme"))
	juan := create(t, svc, personOf("Juan", acme))
	for _, id := range []int64{juan.ID, acme.ID} {
		if err := svc.DeleteVendor(ctx, id); err != nil {
			t.Fatal(err)
		}
	}
	if err := svc.RestoreVendor(ctx, acme.ID); err != nil {
		t.Fatal(err)
	}

	// Act
	err := svc.RestoreVendor(ctx, juan.ID)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if got := listed(t, svc, app.ListVendorsInput{Kind: domain.VendorPerson}); !equalStrings(got, []string{"Juan (Portones Acme)"}) {
		t.Fatalf("persons = %v, want Juan linked to Portones Acme", got)
	}
}

func TestRestoreVendor_ReturnsNotFoundForAnUnknownOrLiveVendor(t *testing.T) {
	// Arrange
	svc, _, _ := newDirectory(t)
	live := create(t, svc, service("Basura"))

	cases := []struct {
		name string
		id   int64
	}{{"unknown", 9999}, {"live", live.ID}}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Act
			err := svc.RestoreVendor(context.Background(), c.id)

			// Assert
			if !errors.Is(err, app.ErrNotFound) {
				t.Fatalf("error = %v, want ErrNotFound", err)
			}
		})
	}
}

func TestDirectoryOperations_EmitOneWideEventWithIdsAndKindAndNoNames(t *testing.T) {
	steps := []struct {
		action        string
		startsDeleted bool // the state the operation needs
		run           func(ctx context.Context, svc *app.DirectoryService, id int64) error
	}{
		{"create_vendor", false, func(ctx context.Context, svc *app.DirectoryService, _ int64) error {
			_, err := svc.CreateVendor(ctx, service("Otro Nombre"))
			return err
		}},
		{"update_vendor", false, func(ctx context.Context, svc *app.DirectoryService, id int64) error {
			_, err := svc.UpdateVendor(ctx, app.UpdateVendorInput{ID: id, VendorInput: service("Nombre Nuevo")})
			return err
		}},
		{"delete_vendor", false, func(ctx context.Context, svc *app.DirectoryService, id int64) error {
			return svc.DeleteVendor(ctx, id)
		}},
		{"restore_vendor", true, func(ctx context.Context, svc *app.DirectoryService, id int64) error {
			return svc.RestoreVendor(ctx, id)
		}},
	}
	for _, s := range steps {
		t.Run(s.action, func(t *testing.T) {
			// Arrange
			svc, _, events := newDirectory(t)
			ctx := context.Background()
			seed := create(t, svc, app.VendorInput{Kind: domain.VendorService, Name: "Sensible Nombre", Phone: "5551234", Email: "x@y.mx", Notes: "Nota privada"})
			if s.startsDeleted {
				if err := svc.DeleteVendor(ctx, seed.ID); err != nil {
					t.Fatal(err)
				}
			}
			events.Reset()

			// Act
			err := s.run(ctx, svc, seed.ID)

			// Assert
			if err != nil {
				t.Fatal(err)
			}
			got := readEvents(t, events)
			if len(got) != 1 {
				t.Fatalf("%d events, want one: %v", len(got), got)
			}
			e := got[0]
			if e["action"] != s.action || e["outcome"] != "ok" || e["level"] != "INFO" || e["kind"] != "service" || e["vendor_id"] == nil {
				t.Fatalf("event = %v, want %s ok at INFO with vendor_id and kind", e, s.action)
			}
			if _, ok := e["duration_ms"]; !ok {
				t.Fatalf("event has no duration_ms: %v", e)
			}
			raw := events.String()
			for _, secret := range []string{"Sensible", "Otro Nombre", "Nombre Nuevo", "5551234", "x@y.mx", "Nota privada"} {
				if strings.Contains(raw, secret) {
					t.Fatalf("event contains %q: %s", secret, raw)
				}
			}
		})
	}
}

func TestDirectoryOperations_EmitAValidationEventAtWarnWhenInputIsRejected(t *testing.T) {
	// Arrange
	svc, _, events := newDirectory(t)
	ctx := context.Background()
	acme := create(t, svc, company("Portones Acme"))
	create(t, svc, personOf("Juan Gómez", acme))

	steps := []struct {
		name    string
		run     func() error
		invalid float64 // invalid_field_count; 0 when the event carries none
	}{
		{"a rejected create", func() error { _, err := svc.CreateVendor(ctx, service("")); return err }, 1},
		{"a company with members refused deletion", func() error { return svc.DeleteVendor(ctx, acme.ID) }, 0},
	}
	for _, s := range steps {
		t.Run(s.name, func(t *testing.T) {
			events.Reset()

			// Act
			err := s.run()

			// Assert
			if err == nil {
				t.Fatal("want an error")
			}
			got := readEvents(t, events)
			if len(got) != 1 || got[0]["outcome"] != "validation" || got[0]["level"] != "WARN" {
				t.Fatalf("events = %v, want one validation event at WARN", got)
			}
			if n, _ := got[0]["invalid_field_count"].(float64); n != s.invalid {
				t.Fatalf("invalid_field_count = %v, want %v", got[0]["invalid_field_count"], s.invalid)
			}
			if raw := events.String(); strings.Contains(raw, "Juan") || strings.Contains(raw, "Acme") {
				t.Fatalf("event names a vendor: %s", raw)
			}
		})
	}
}

func TestDirectoryOperations_EmitAnErrorEventAtErrorWhenTheVendorIsNotFound(t *testing.T) {
	// Arrange
	svc, _, events := newDirectory(t)

	// Act
	err := svc.DeleteVendor(context.Background(), 9999)

	// Assert
	got := readEvents(t, events)
	if !errors.Is(err, app.ErrNotFound) || len(got) != 1 || got[0]["outcome"] != "error" || got[0]["level"] != "ERROR" || got[0]["vendor_id"] != float64(9999) {
		t.Fatalf("error = %v, events = %v; want one error event for vendor 9999", err, got)
	}
}

func TestRestoreVendor_APersonWhoseCompanyIsNoLongerACompanyIsRefusedAndThatVendorIsNamed(t *testing.T) {
	// Arrange
	svc, _, _ := newDirectory(t)
	ctx := context.Background()
	acme := create(t, svc, company("Portones Acme"))
	juan := create(t, svc, personOf("Juan", acme))
	if err := svc.DeleteVendor(ctx, juan.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpdateVendor(ctx, app.UpdateVendorInput{ID: acme.ID, VendorInput: service("Portones Acme")}); err != nil {
		t.Fatalf("arrange: change the kind of the company: %v", err)
	}

	// Act
	err := svc.RestoreVendor(ctx, juan.ID)

	// Assert
	var conflict app.VendorConflict
	if !errors.As(err, &conflict) || !errors.Is(err, app.ErrConflict) || conflict.Reason != app.ConflictCompanyNotCompany {
		t.Fatalf("error = %v, want a VendorConflict (company_not_company) matching ErrConflict", err)
	}
	if !equalStrings(conflict.Names, []string{"Portones Acme"}) {
		t.Fatalf("named vendor = %v, want Portones Acme", conflict.Names)
	}
	if got := listed(t, svc, app.ListVendorsInput{Kind: domain.VendorPerson}); len(got) != 0 {
		t.Fatalf("live persons = %v, want none restored", got)
	}
}
