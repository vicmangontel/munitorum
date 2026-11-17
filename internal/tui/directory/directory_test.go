package directory_test

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/vicmangontel/munitorum/internal/app"
	"github.com/vicmangontel/munitorum/internal/domain"
	"github.com/vicmangontel/munitorum/internal/store"
	"github.com/vicmangontel/munitorum/internal/tui"
)

// settle delivers msgs to model one by one, running every command that comes
// back (a batch in order) and feeding its message in, as the Bubble Tea runtime
// does.
func settle(model tea.Model, msgs ...tea.Msg) tea.Model {
	var deliver func(tea.Msg)
	deliver = func(msg tea.Msg) {
		if msg == nil {
			return
		}
		if batch, ok := msg.(tea.BatchMsg); ok {
			for _, cmd := range batch {
				if cmd != nil {
					deliver(cmd())
				}
			}
			return
		}
		var cmd tea.Cmd
		model, cmd = model.Update(msg)
		if cmd != nil {
			deliver(cmd())
		}
	}
	for _, m := range msgs {
		deliver(m)
	}
	return model
}

// dk turns "enter", "tab", "right", ... into the key message the runtime sends;
// anything else is typed text.
func dk(s string) tea.KeyMsg {
	named := map[string]tea.KeyType{
		"enter": tea.KeyEnter, "tab": tea.KeyTab, "shift+tab": tea.KeyShiftTab, "esc": tea.KeyEsc,
		"up": tea.KeyUp, "down": tea.KeyDown, "left": tea.KeyLeft, "right": tea.KeyRight,
		"backspace": tea.KeyBackspace, "delete": tea.KeyDelete, "ctrl+u": tea.KeyCtrlU,
	}
	if t, ok := named[s]; ok {
		return tea.KeyMsg{Type: t}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

// press delivers the keys, in order.
func press(model tea.Model, keys ...string) tea.Model {
	for _, k := range keys {
		model = settle(model, dk(k))
	}
	return model
}

func isQuit(msg tea.Msg) bool {
	_, ok := msg.(tea.QuitMsg)
	return ok
}

func lastLine(s string) string {
	lines := strings.Split(s, "\n")
	return lines[len(lines)-1]
}

func newDirectoryService(t *testing.T) (*app.DirectoryService, *sql.DB) {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := store.Migrate(db, store.Migrations()); err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return app.NewDirectoryService(store.NewVendorRepository(db), app.SystemClock{}, log), db
}

// openDirectorio returns the shell on the Directorio view, main panel focused,
// with the list already loaded.
func openDirectorio(t *testing.T, svc *app.DirectoryService, w, h int) tea.Model {
	t.Helper()
	m := tui.New().WithDirectory(svc)
	var model tea.Model = m
	model = settle(model, tea.WindowSizeMsg{Width: w, Height: h})
	model = settle(model, m.Init()())
	return press(model, "7")
}

func seed(t *testing.T, svc *app.DirectoryService, in app.VendorInput) app.Vendor {
	t.Helper()
	v, err := svc.CreateVendor(context.Background(), in)
	if err != nil {
		t.Fatalf("arrange: create %q: %v", in.Name, err)
	}
	return v
}

func seedAcme(t *testing.T, svc *app.DirectoryService) (acme, juan app.Vendor) {
	t.Helper()
	acme = seed(t, svc, app.VendorInput{Kind: domain.VendorCompany, Name: "Portones Acme"})
	juan = seed(t, svc, app.VendorInput{Kind: domain.VendorPerson, Name: "Juan Gómez", Phone: "555 0101", Email: "juan@acme.mx", CompanyID: &acme.ID, Notes: "Técnico"})
	return acme, juan
}

func mustContain(t *testing.T, out string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(out, w) {
			t.Errorf("screen does not show %q:\n%s", w, out)
		}
	}
}

func mustNotContain(t *testing.T, out string, nots ...string) {
	t.Helper()
	for _, n := range nots {
		if strings.Contains(out, n) {
			t.Errorf("screen shows %q:\n%s", n, out)
		}
	}
}

func liveNames(t *testing.T, svc *app.DirectoryService) []string {
	t.Helper()
	rows, err := svc.ListVendors(context.Background(), app.ListVendorsInput{})
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, len(rows))
	for i, r := range rows {
		names[i] = r.DisplayName()
	}
	return names
}

func TestDirectorio_ListsVendorsWithTheirColumnsAndTheCompanyInTheName(t *testing.T) {
	// Arrange
	svc, _ := newDirectoryService(t)
	seedAcme(t, svc)
	seed(t, svc, app.VendorInput{Kind: domain.VendorService, Name: "Basura"})

	// Act
	model := openDirectorio(t, svc, 120, 40)

	// Assert
	out := model.View()
	mustContain(t, out, "Directorio", "Nombre", "Tipo", "Empresa", "Teléfono",
		"Juan Gómez (Portones Acme)", "Persona", "Basura", "Servicio", "555 0101")
}

func TestDirectorio_ShowsAHelpfulLineWhenTheDirectoryIsEmpty(t *testing.T) {
	// Arrange
	svc, _ := newDirectoryService(t)

	// Act
	model := openDirectorio(t, svc, 120, 40)

	// Assert
	mustContain(t, model.View(), "El directorio está vacío")
}

func TestDirectorio_TheDetailPanelShowsEveryFieldOfAPersonAndItsCompany(t *testing.T) {
	// Arrange
	svc, _ := newDirectoryService(t)
	seedAcme(t, svc)
	model := openDirectorio(t, svc, 120, 40)

	// Act
	model = press(model) // Juan Gómez sorts first and is selected

	// Assert
	mustContain(t, model.View(), "Nombre: Juan Gómez", "Tipo: Persona", "Teléfono: 555 0101",
		"Correo: juan@acme.mx", "Empresa: Portones Acme", "Notas: Técnico")
}

func TestDirectorio_TheDetailPanelListsTheLivePeopleOfACompany(t *testing.T) {
	// Arrange
	svc, _ := newDirectoryService(t)
	seedAcme(t, svc)
	model := openDirectorio(t, svc, 120, 40)

	// Act
	model = press(model, "down") // on Portones Acme

	// Assert
	mustContain(t, model.View(), "Nombre: Portones Acme", "Personas: Juan Gómez")
}

func TestDirectorio_TheDetailPanelShowsDashesForEmptyFields(t *testing.T) {
	// Arrange
	svc, _ := newDirectoryService(t)
	seed(t, svc, app.VendorInput{Kind: domain.VendorService, Name: "Basura"})

	// Act
	model := openDirectorio(t, svc, 120, 40)

	// Assert
	mustContain(t, model.View(), "Teléfono: —", "Correo: —", "Notas: —")
}

func TestDirectorio_NewOpensAFormWhoseErrorsAllShowAtOnce(t *testing.T) {
	// Arrange
	svc, _ := newDirectoryService(t)
	model := openDirectorio(t, svc, 120, 40)
	model = press(model, "n", "tab") // on Nombre, which stays empty
	model = press(model, "tab", strings.Repeat("5", 31), "tab", "sin-arroba")

	// Act
	model = press(model, "enter")

	// Assert
	out := model.View()
	mustContain(t, out, "Nuevo en el directorio",
		"El nombre es obligatorio y admite hasta 80 caracteres.",
		"El teléfono admite hasta 30 caracteres.",
		"El correo debe tener un @")
	if got := liveNames(t, svc); len(got) != 0 {
		t.Fatalf("a rejected form wrote %v", got)
	}
}

func TestDirectorio_AServiceCanBeSavedWithOnlyAName(t *testing.T) {
	// Arrange
	svc, _ := newDirectoryService(t)
	model := openDirectorio(t, svc, 120, 40)

	// Act
	model = press(model, "n", "right", "right", "tab", "Basura", "enter") // kind Servicio

	// Assert
	out := model.View()
	mustNotContain(t, out, "Nuevo en el directorio")
	mustContain(t, out, "Basura", "Servicio")
	if got := liveNames(t, svc); len(got) != 1 || got[0] != "Basura" {
		t.Fatalf("vendors = %v, want only Basura", got)
	}
}

func TestDirectorio_TheCompanyFieldIsShownForAPersonOnly(t *testing.T) {
	cases := []struct {
		name      string
		kindKeys  []string // from the form's first field
		wantShown bool
	}{
		{"Persona", nil, true},
		{"Empresa", []string{"right"}, false},
		{"Servicio", []string{"right", "right"}, false},
		{"back to Persona", []string{"right", "right", "right"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange
			svc, _ := newDirectoryService(t)
			model := openDirectorio(t, svc, 120, 40)
			model = press(model, "n")
			if len(c.kindKeys) > 1 {
				model = press(model, c.kindKeys[:len(c.kindKeys)-1]...)
			}

			// Act
			if len(c.kindKeys) > 0 {
				model = press(model, c.kindKeys[len(c.kindKeys)-1])
			}

			// Assert
			shown := strings.Contains(model.View(), "(ninguna)")
			if shown != c.wantShown {
				t.Fatalf("company field shown = %v, want %v:\n%s", shown, c.wantShown, model.View())
			}
		})
	}
}

func TestDirectorio_APersonIsLinkedToACompanyChosenInThePicker(t *testing.T) {
	// Arrange
	svc, _ := newDirectoryService(t)
	seed(t, svc, app.VendorInput{Kind: domain.VendorCompany, Name: "Portones Acme"})
	seed(t, svc, app.VendorInput{Kind: domain.VendorCompany, Name: "Jardines SA"})
	model := openDirectorio(t, svc, 120, 40)
	model = press(model, "n", "tab", "Juan Gómez", "tab", "tab", "tab") // on the company field

	// Act
	model = press(model, "enter")
	picking := model.View()
	model = press(model, "acme", "enter")
	chosen := model.View()
	model = press(model, "tab", "enter") // notes, then save

	// Assert
	mustContain(t, picking, "Portones Acme", "Jardines SA")
	mustContain(t, chosen, "Empresa   Portones Acme")
	mustContain(t, model.View(), "Juan Gómez (Portones Acme)")
	if got := liveNames(t, svc); len(got) != 3 || got[1] != "Juan Gómez (Portones Acme)" {
		t.Fatalf("vendors = %v, want Juan Gómez linked to Portones Acme", got)
	}
}

func TestDirectorio_ThePickerNeverOffersADeletedCompany(t *testing.T) {
	// Arrange
	svc, _ := newDirectoryService(t)
	seed(t, svc, app.VendorInput{Kind: domain.VendorCompany, Name: "Portones Acme"})
	gone := seed(t, svc, app.VendorInput{Kind: domain.VendorCompany, Name: "Cerrada SA"})
	if err := svc.DeleteVendor(context.Background(), gone.ID); err != nil {
		t.Fatal(err)
	}
	model := openDirectorio(t, svc, 120, 40)
	model = press(model, "n", "tab", "tab", "tab", "tab") // company field

	// Act
	model = press(model, "enter")

	// Assert
	out := model.View()
	mustContain(t, out, "Portones Acme")
	mustNotContain(t, out, "Cerrada SA")
}

func TestDirectorio_PlusInThePickerCreatesACompanyAndReturnsItSelectedInTheForm(t *testing.T) {
	// Arrange
	svc, _ := newDirectoryService(t)
	model := openDirectorio(t, svc, 120, 40)
	model = press(model, "n", "tab", "Juan Gómez", "tab", "tab", "tab", "enter") // picker open, no companies

	// Act
	model = press(model, "+")
	creating := model.View()
	model = press(model, "tab", "Nueva SA", "enter") // the new company's name, then save it
	back := model.View()
	model = press(model, "tab", "enter") // notes, then save the person

	// Assert
	mustContain(t, creating, "Empresa", "‹ Empresa ›")
	mustNotContain(t, creating, "(ninguna)") // a company has no company field
	mustContain(t, back, "Nueva SA")
	mustContain(t, model.View(), "Juan Gómez (Nueva SA)")
	rows, err := svc.ListVendors(context.Background(), app.ListVendorsInput{Kind: domain.VendorCompany})
	if err != nil || len(rows) != 1 || rows[0].Name != "Nueva SA" {
		t.Fatalf("companies = %v (%v), want the one created inline", rows, err)
	}
}

func TestDirectorio_TheKindOfAnInlineCompanyCannotBeChanged(t *testing.T) {
	// Arrange
	svc, _ := newDirectoryService(t)
	model := openDirectorio(t, svc, 120, 40)
	model = press(model, "n", "tab", "tab", "tab", "tab", "enter", "+")

	// Act
	model = press(model, "right", "right", "left")

	// Assert
	mustContain(t, model.View(), "‹ Empresa ›")
	mustNotContain(t, model.View(), "Persona", "Servicio")
}

func TestDirectorio_EscInThePickerReturnsToTheFormAndASecondEscClosesIt(t *testing.T) {
	// Arrange
	svc, _ := newDirectoryService(t)
	seed(t, svc, app.VendorInput{Kind: domain.VendorCompany, Name: "Portones Acme"})
	model := openDirectorio(t, svc, 120, 40)
	model = press(model, "n", "tab", "Juan Gómez", "tab", "tab", "tab", "enter")

	// Act
	model = press(model, "esc")
	afterFirst := model.View()
	model = press(model, "esc")

	// Assert
	mustContain(t, afterFirst, "Nuevo en el directorio", "Juan Gómez", "(ninguna)")
	mustNotContain(t, model.View(), "Nuevo en el directorio")
	if got := liveNames(t, svc); len(got) != 1 {
		t.Fatalf("vendors = %v, want nothing written", got)
	}
}

func TestDirectorio_EscInTheInlineCompanyFormReturnsToThePickerList(t *testing.T) {
	// Arrange
	svc, _ := newDirectoryService(t)
	seed(t, svc, app.VendorInput{Kind: domain.VendorCompany, Name: "Portones Acme"})
	model := openDirectorio(t, svc, 120, 40)
	model = press(model, "n", "tab", "tab", "tab", "tab", "enter", "+")

	// Act
	model = press(model, "esc")

	// Assert
	mustContain(t, model.View(), "Buscar:", "Portones Acme")
	mustNotContain(t, model.View(), "‹ Empresa ›")
}

func TestDirectorio_ACompanyCanBeRemovedFromAPersonWithDelete(t *testing.T) {
	// Arrange
	svc, _ := newDirectoryService(t)
	seedAcme(t, svc)
	model := openDirectorio(t, svc, 120, 40)
	model = press(model, "e", "tab", "tab", "tab", "tab") // edit Juan: company field

	// Act
	model = press(model, "delete")
	cleared := model.View()
	model = press(model, "tab", "enter")

	// Assert
	mustContain(t, cleared, "(ninguna)")
	if got := liveNames(t, svc); len(got) != 2 || got[0] != "Juan Gómez" {
		t.Fatalf("vendors = %v, want Juan without a company", got)
	}
}

func TestDirectorio_EditShowsTheStoredValuesAndSavesTheChange(t *testing.T) {
	// Arrange
	svc, _ := newDirectoryService(t)
	seedAcme(t, svc)
	model := openDirectorio(t, svc, 120, 40)

	// Act
	model = press(model, "e")
	editing := model.View()
	model = press(model, "tab", "ctrl+u", "Juan Pérez", "enter")

	// Assert
	mustContain(t, editing, "Editar del directorio", "Juan Gómez", "555 0101", "juan@acme.mx", "Portones Acme", "Técnico")
	got := liveNames(t, svc)
	if len(got) != 2 || got[0] != "Juan Pérez (Portones Acme)" {
		t.Fatalf("vendors = %v, want the edited name", got)
	}
}

func TestDirectorio_EditIsIgnoredOnADeletedRow(t *testing.T) {
	// Arrange
	svc, _ := newDirectoryService(t)
	gone := seed(t, svc, app.VendorInput{Kind: domain.VendorService, Name: "Basura"})
	if err := svc.DeleteVendor(context.Background(), gone.ID); err != nil {
		t.Fatal(err)
	}
	model := openDirectorio(t, svc, 120, 40)
	model = press(model, ".")

	// Act
	model = press(model, "e", "d")

	// Assert
	mustNotContain(t, model.View(), "Editar del directorio", "Eliminar del directorio")
}

func TestDirectorio_DecliningTheDeleteConfirmationWritesNothing(t *testing.T) {
	cases := []struct {
		name string
		keys []string
	}{
		{"No is the default answer", []string{"d", "enter"}},
		{"esc", []string{"d", "esc"}},
		{"choosing No explicitly", []string{"d", "right", "left", "enter"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange
			svc, db := newDirectoryService(t)
			seed(t, svc, app.VendorInput{Kind: domain.VendorService, Name: "Basura"})
			model := openDirectorio(t, svc, 120, 40)

			// Act
			model = press(model, c.keys...)

			// Assert
			mustNotContain(t, model.View(), "Eliminar del directorio")
			mustContain(t, model.View(), "Basura")
			var deleted int
			if err := db.QueryRow(`SELECT COUNT(deleted_at) FROM vendors`).Scan(&deleted); err != nil || deleted != 0 {
				t.Fatalf("deleted rows = %d (%v), want none", deleted, err)
			}
		})
	}
}

func TestDirectorio_ConfirmingTheDeleteRemovesTheVendorFromTheList(t *testing.T) {
	// Arrange
	svc, _ := newDirectoryService(t)
	seed(t, svc, app.VendorInput{Kind: domain.VendorService, Name: "Basura"})
	model := openDirectorio(t, svc, 120, 40)

	// Act
	model = press(model, "d")
	asking := model.View()
	model = press(model, "left", "enter") // Sí

	// Assert
	mustContain(t, asking, "Eliminar del directorio", "¿Eliminar a Basura del directorio?")
	out := model.View()
	mustNotContain(t, out, "Eliminar del directorio")
	mustContain(t, out, "El directorio está vacío")
}

func TestDirectorio_DeletingACompanyWithPeopleIsRefusedAndNamesThem(t *testing.T) {
	// Arrange
	svc, _ := newDirectoryService(t)
	seedAcme(t, svc)
	model := openDirectorio(t, svc, 120, 40)
	model = press(model, "down", "d", "left") // Portones Acme, asking, Sí

	// Act
	model = press(model, "enter")

	// Assert
	mustContain(t, model.View(), "No se puede eliminar", "Juan Gómez (Portones Acme)")
	if got := liveNames(t, svc); len(got) != 2 {
		t.Fatalf("vendors = %v, want the company kept", got)
	}
}

func TestDirectorio_AnyKeyClosesTheDeleteRefusal(t *testing.T) {
	// Arrange
	svc, _ := newDirectoryService(t)
	seedAcme(t, svc)
	model := openDirectorio(t, svc, 120, 40)
	model = press(model, "down", "d", "left", "enter")
	mustContain(t, model.View(), "No se puede eliminar") // precondition

	// Act
	model = press(model, "x")

	// Assert
	mustNotContain(t, model.View(), "No se puede eliminar")
}

func TestDirectorio_ShowDeletedStrikesThemThroughAndRestoreBringsThemBack(t *testing.T) {
	// Arrange
	svc, _ := newDirectoryService(t)
	seed(t, svc, app.VendorInput{Kind: domain.VendorService, Name: "Basura"})
	cfe := seed(t, svc, app.VendorInput{Kind: domain.VendorService, Name: "CFE"})
	if err := svc.DeleteVendor(context.Background(), cfe.ID); err != nil {
		t.Fatal(err)
	}
	model := openDirectorio(t, svc, 120, 40)
	hidden := model.View()

	// Act
	model = press(model, ".")
	shown := model.View()
	model = press(model, "down", "r")

	// Assert
	mustNotContain(t, hidden, "CFE")
	mustContain(t, shown, "CFE", "con eliminados")
	if got := liveNames(t, svc); len(got) != 2 {
		t.Fatalf("vendors = %v, want CFE restored", got)
	}
}

func TestDirectorio_RestoreDoesNothingOnALiveRow(t *testing.T) {
	// Arrange
	svc, _ := newDirectoryService(t)
	seed(t, svc, app.VendorInput{Kind: domain.VendorService, Name: "Basura"})
	model := openDirectorio(t, svc, 120, 40)

	// Act
	model = press(model, "r")

	// Assert
	mustNotContain(t, model.View(), "No se pudo")
}

func TestDirectorio_RestoringANameThatNowCollidesShowsWhy(t *testing.T) {
	// Arrange
	svc, _ := newDirectoryService(t)
	old := seed(t, svc, app.VendorInput{Kind: domain.VendorPerson, Name: "José"})
	if err := svc.DeleteVendor(context.Background(), old.ID); err != nil {
		t.Fatal(err)
	}
	seed(t, svc, app.VendorInput{Kind: domain.VendorPerson, Name: "jose"})
	model := openDirectorio(t, svc, 120, 40)
	model = press(model, ".", "up") // José (deleted) sorts before jose (live, selected)

	// Act
	model = press(model, "r")

	// Assert
	mustContain(t, model.View(), "Ya existe un elemento activo del mismo tipo con ese nombre.")
}

func TestDirectorio_RestoringAPersonWhoseCompanyIsDeletedNamesTheCompany(t *testing.T) {
	// Arrange
	svc, _ := newDirectoryService(t)
	acme, juan := seedAcme(t, svc)
	for _, id := range []int64{juan.ID, acme.ID} {
		if err := svc.DeleteVendor(context.Background(), id); err != nil {
			t.Fatal(err)
		}
	}
	model := openDirectorio(t, svc, 120, 40)
	model = press(model, ".") // Juan Gómez is the first row

	// Act
	model = press(model, "r")

	// Assert
	mustContain(t, model.View(), "su empresa «Portones Acme» está eliminada")
}

func TestDirectorio_SearchFiltersTheListAsTheUserTypesAndFindsAPersonByCompany(t *testing.T) {
	cases := []struct {
		name   string
		typed  []string // keys typed after "/"; the last one is the Act
		shows  []string
		hidden []string
	}{
		{"part of the word", []string{"ac"}, []string{"Buscar: ac", "Juan Gómez (Portones Acme)"}, nil},
		{"the whole company name", []string{"ac", "me"}, []string{"Buscar: acme", "Juan Gómez (Portones Acme)", "Portones Acme"}, []string{"Basura"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange
			svc, _ := newDirectoryService(t)
			seedAcme(t, svc)
			seed(t, svc, app.VendorInput{Kind: domain.VendorService, Name: "Basura"})
			model := openDirectorio(t, svc, 120, 40)
			model = press(model, "/")
			model = press(model, c.typed[:len(c.typed)-1]...)

			// Act
			model = press(model, c.typed[len(c.typed)-1])

			// Assert
			mustContain(t, model.View(), c.shows...)
			mustNotContain(t, model.View(), c.hidden...)
		})
	}
}

func TestDirectorio_WhileSearchingEveryKeyIsTextEvenQAndDigits(t *testing.T) {
	// Arrange
	svc, _ := newDirectoryService(t)
	seed(t, svc, app.VendorInput{Kind: domain.VendorService, Name: "Basura", Phone: "5551234"})
	model := openDirectorio(t, svc, 120, 40)

	// Act
	model = settle(model, dk("/"))
	var quit bool
	for _, k := range []string{"q", "5", "?", "1"} {
		next, cmd := model.Update(dk(k))
		var msg tea.Msg
		if cmd != nil {
			msg = cmd()
			quit = quit || isQuit(msg)
		}
		model = settle(next, msg)
	}

	// Assert
	if quit {
		t.Fatal("typing q in the search box quit the application")
	}
	out := model.View()
	mustContain(t, out, "Buscar: q5?1", "Ningún elemento coincide")
	mustNotContain(t, out, "Ayuda")
}

func TestDirectorio_SearchByPhoneUsesDigitsWithoutSwitchingViews(t *testing.T) {
	// Arrange
	svc, _ := newDirectoryService(t)
	seed(t, svc, app.VendorInput{Kind: domain.VendorService, Name: "Basura", Phone: "5551234"})
	seed(t, svc, app.VendorInput{Kind: domain.VendorService, Name: "CFE"})
	model := openDirectorio(t, svc, 120, 40)

	// Act
	model = press(model, "/", "1234")

	// Assert
	out := model.View()
	mustContain(t, out, "Basura")
	mustNotContain(t, out, "CFE")
}

func TestDirectorio_EnterKeepsTheSearch(t *testing.T) {
	// Arrange
	svc, _ := newDirectoryService(t)
	seed(t, svc, app.VendorInput{Kind: domain.VendorService, Name: "Basura"})
	seed(t, svc, app.VendorInput{Kind: domain.VendorService, Name: "CFE"})
	model := openDirectorio(t, svc, 120, 40)
	model = press(model, "/", "cfe")

	// Act
	model = press(model, "enter")

	// Assert
	mustContain(t, model.View(), "Buscar: cfe")
	mustNotContain(t, model.View(), "Basura")
}

func TestDirectorio_EscClearsTheSearch(t *testing.T) {
	// Arrange
	svc, _ := newDirectoryService(t)
	seed(t, svc, app.VendorInput{Kind: domain.VendorService, Name: "Basura"})
	seed(t, svc, app.VendorInput{Kind: domain.VendorService, Name: "CFE"})
	model := openDirectorio(t, svc, 120, 40)
	model = press(model, "/", "cfe")

	// Act
	model = press(model, "esc")

	// Assert
	mustContain(t, model.View(), "Basura", "CFE")
	mustNotContain(t, model.View(), "Buscar:")
}

func TestDirectorio_AfterEnterKeysAreShellCommandsAgain(t *testing.T) {
	// Arrange
	svc, _ := newDirectoryService(t)
	seed(t, svc, app.VendorInput{Kind: domain.VendorService, Name: "CFE"})
	model := openDirectorio(t, svc, 120, 40)
	model = press(model, "/", "cfe", "enter")

	// Act
	model = press(model, "2") // view 2, not text

	// Assert
	mustContain(t, model.View(), "Esta vista se construye")
	mustNotContain(t, model.View(), "Buscar: cfe2")
}

func TestDirectorio_FCyclesTheKindFilterThroughEveryKindAndBack(t *testing.T) {
	steps := []struct {
		status string
		shows  []string
		hides  []string
	}{
		{"Tipo: Persona", []string{"Juan Gómez"}, []string{"Basura"}},
		{"Tipo: Empresa", []string{"Portones Acme"}, []string{"Juan Gómez (", "Basura"}},
		{"Tipo: Servicio", []string{"Basura"}, []string{"Juan", "Portones Acme"}},
		{"", []string{"Juan Gómez", "Basura", "Portones Acme"}, nil},
	}
	for n, s := range steps {
		t.Run(s.status, func(t *testing.T) {
			// Arrange
			svc, _ := newDirectoryService(t)
			seedAcme(t, svc)
			seed(t, svc, app.VendorInput{Kind: domain.VendorService, Name: "Basura"})
			model := openDirectorio(t, svc, 120, 40)
			for range n {
				model = press(model, "f")
			}

			// Act
			model = press(model, "f")

			// Assert
			out := model.View()
			if s.status != "" {
				mustContain(t, out, s.status)
			}
			mustContain(t, out, s.shows...)
			mustNotContain(t, out, s.hides...)
		})
	}
}

func TestDirectorio_TheListScrollsToKeepTheSelectedRowOnScreen(t *testing.T) {
	// Arrange
	svc, _ := newDirectoryService(t)
	for _, name := range []string{"Aa", "Bb", "Cc", "Dd", "Ee", "Ff", "Gg", "Hh", "Ii", "Jj", "Kk", "Ll", "Mm", "Nn", "Ñu", "Oo"} {
		seed(t, svc, app.VendorInput{Kind: domain.VendorService, Name: name})
	}
	model := openDirectorio(t, svc, 80, 24) // about eight list rows
	before := model.View()

	// Act
	for range 15 {
		model = press(model, "down")
	}

	// Assert
	mustContain(t, before, "Aa")
	mustNotContain(t, before, "Oo")
	mustContain(t, model.View(), "> Oo", "Nombre: Oo")
	mustNotContain(t, model.View(), "Aa  ")
}

func TestDirectorio_SelectionStaysOnTheSameVendorWhenTheListReloads(t *testing.T) {
	// Arrange
	svc, _ := newDirectoryService(t)
	seed(t, svc, app.VendorInput{Kind: domain.VendorService, Name: "Basura"})
	seed(t, svc, app.VendorInput{Kind: domain.VendorService, Name: "CFE"})
	model := openDirectorio(t, svc, 120, 40)
	model = press(model, "down") // CFE

	// Act
	model = press(model, ".") // reloads with deleted rows included

	// Assert
	mustContain(t, model.View(), "> CFE", "Nombre: CFE")
}

func TestDirectorio_ANewVendorIsSelectedAfterItIsSaved(t *testing.T) {
	// Arrange
	svc, _ := newDirectoryService(t)
	seed(t, svc, app.VendorInput{Kind: domain.VendorService, Name: "Basura"})
	model := openDirectorio(t, svc, 120, 40)

	// Act
	model = press(model, "n", "right", "right", "tab", "Zeta", "enter")

	// Assert
	mustContain(t, model.View(), "> Zeta", "Nombre: Zeta")
}

func TestDirectorio_KeyHintsListTheViewsKeysAndTheSearchKeysWhileSearching(t *testing.T) {
	// Arrange
	svc, _ := newDirectoryService(t)
	model := openDirectorio(t, svc, 160, 40)

	// Act
	normal := lastLine(model.View())
	searching := lastLine(press(model, "/").View())

	// Assert
	mustContain(t, normal, "n nuevo", "e editar", "d eliminar", "/ buscar", "f tipo", ". eliminados", "r restaurar")
	mustContain(t, searching, "enter aplicar", "esc borrar")
}

func TestDirectorio_OtherViewsKeepTheNoSelectionDetailAndThePlaceholder(t *testing.T) {
	// Arrange
	svc, _ := newDirectoryService(t)
	seed(t, svc, app.VendorInput{Kind: domain.VendorService, Name: "Basura"})
	model := openDirectorio(t, svc, 120, 40)

	// Act
	model = press(model, "2") // Casas

	// Assert
	mustContain(t, model.View(), "Sin selección", "Esta vista se construye")
	mustNotContain(t, model.View(), "Nombre: Basura")
}

func TestRoot_WithDirectoryShowsTheDirectorioView(t *testing.T) {
	// Arrange
	svc, _ := newDirectoryService(t)
	seed(t, svc, app.VendorInput{Kind: domain.VendorService, Name: "Basura"})
	root := tui.NewShell(nil, app.Settings{CommunityName: "Coto Los Pinos"}).WithDirectory(svc)

	// Act
	var model tea.Model = root
	model = settle(model, tea.WindowSizeMsg{Width: 120, Height: 40})
	model = settle(model, root.Init()())
	model = press(model, "7")

	// Assert
	mustContain(t, model.View(), "Basura")
}

func TestDirectorio_ChangingTheKindOfACompanyWithPeopleShowsTheErrorUnderTheKind(t *testing.T) {
	// Arrange
	svc, _ := newDirectoryService(t)
	seedAcme(t, svc)
	model := openDirectorio(t, svc, 120, 40)
	model = press(model, "down", "e") // edit Portones Acme

	// Act
	model = press(model, "right", "enter") // Empresa -> Servicio

	// Assert
	mustContain(t, model.View(), "Editar del directorio", "La empresa tiene personas activas")
}

func TestDirectorio_ASaveThatFailsForAnotherReasonShowsTheFailureInTheForm(t *testing.T) {
	// Arrange
	svc, _ := newDirectoryService(t)
	basura := seed(t, svc, app.VendorInput{Kind: domain.VendorService, Name: "Basura"})
	model := openDirectorio(t, svc, 120, 40)
	model = press(model, "e")
	if err := svc.DeleteVendor(context.Background(), basura.ID); err != nil { // deleted while the form was open
		t.Fatal(err)
	}

	// Act
	model = press(model, "enter")

	// Assert
	mustContain(t, model.View(), "Editar del directorio", "No se pudo completar la acción:")
}

func TestDirectorio_AllFourColumnHeadersAreVisibleAtTheMinimumTerminalSize(t *testing.T) {
	// Arrange
	svc, _ := newDirectoryService(t)
	seedAcme(t, svc)

	// Act
	model := openDirectorio(t, svc, 80, 24)

	// Assert
	header := ""
	for _, line := range strings.Split(model.View(), "\n") {
		if strings.Contains(line, "Nombre  ") {
			header = line // the list header, not the detail panel
		}
	}
	mustContain(t, header, "Nombre", "Tipo", "Empresa", "Teléfono")
}

func TestDirectorio_TheListHeaderIsAlignedWithTheDataColumns(t *testing.T) {
	// Arrange
	svc, _ := newDirectoryService(t)
	seedAcme(t, svc)
	model := openDirectorio(t, svc, 80, 24)

	// Act
	out := model.View()

	// Assert
	var header, row string
	for _, line := range strings.Split(out, "\n") {
		switch {
		case strings.Contains(line, "Nombre  "):
			header = line
		case strings.Contains(line, "Persona") && strings.Contains(line, "555"):
			row = line
		}
	}
	if header == "" || row == "" {
		t.Fatalf("header %q or data row %q not found:\n%s", header, row, out)
	}
	cell := func(line string, idx int) int { return lipgloss.Width(line[:idx]) } // display cells, not bytes
	kind := strings.Index(row, "Persona")
	company := kind + len("Persona") + strings.Index(row[kind+len("Persona"):], "Portones")
	want := map[string]int{
		"Tipo":     cell(row, kind),
		"Empresa":  cell(row, company),
		"Teléfono": cell(row, strings.Index(row, "555")),
	}
	for name, wantAt := range want {
		if got := cell(header, strings.Index(header, name)); got != wantAt {
			t.Errorf("header %q starts at cell %d, its data starts at cell %d:\n%s\n%s", name, got, wantAt, header, row)
		}
	}
}
