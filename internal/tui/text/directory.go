package text

import (
	"fmt"
	"strings"
)

// ViewDirectory is the position of Directorio in ViewNames.
const ViewDirectory = 6

// Directorio list.
const (
	ColName    = "Nombre"
	ColKind    = "Tipo"
	ColCompany = "Empresa"
	ColPhone   = "Teléfono"

	DirectoryEmpty   = "El directorio está vacío. Presione n para agregar a alguien."
	DirectoryNoMatch = "Ningún elemento coincide con el filtro."
	DirectoryLoading = "Cargando…"
	LoadFailed       = "No se pudo leer el directorio:"
	ActionFailed     = "No se pudo completar la acción:"
	SearchPrompt     = "Buscar: "
)

// Vendor kinds as the treasurer sees them. KindAll is the unfiltered list.
const (
	KindAll     = "Todos"
	KindPerson  = "Persona"
	KindCompany = "Empresa"
	KindService = "Servicio"
)

// VendorKindName is the Spanish name of a kind code (person, company, service).
func VendorKindName(kind string) string {
	switch kind {
	case "person":
		return KindPerson
	case "company":
		return KindCompany
	case "service":
		return KindService
	}
	return kind
}

// VendorKindCode is the kind code of a Spanish kind name; empty when unknown.
func VendorKindCode(name string) string {
	switch name {
	case KindPerson:
		return "person"
	case KindCompany:
		return "company"
	case KindService:
		return "service"
	}
	return ""
}

// DirectoryFilter is the status line above the list; empty when nothing filters.
func DirectoryFilter(kind, search string, deleted bool) string {
	var parts []string
	if kind != "" {
		parts = append(parts, "Tipo: "+kind)
	}
	if search != "" {
		parts = append(parts, "Buscar: "+search)
	}
	if deleted {
		parts = append(parts, "con eliminados")
	}
	return strings.Join(parts, " · ")
}

// DeletedSuffix marks a deleted vendor wherever its name still shows.
const DeletedSuffix = " (eliminado)"

// WithDeletedSuffix appends DeletedSuffix to the name of a deleted vendor.
func WithDeletedSuffix(name string, deleted bool) string {
	if deleted {
		return name + DeletedSuffix
	}
	return name
}

// Directorio forms.
const (
	VendorNewTitle   = "Nuevo en el directorio"
	VendorEditTitle  = "Editar del directorio"
	CompanyNewTitle  = "Nueva empresa"
	DeleteTitle      = "Eliminar del directorio"
	LabelVendorKind  = "Tipo"
	LabelVendorName  = "Nombre"
	LabelVendorPhone = "Teléfono"
	LabelVendorEmail = "Correo"
	LabelVendorCo    = "Empresa"
	LabelVendorNotes = "Notas"
	CompanyNone      = "(ninguna)"
	Saving           = "Guardando…"
)

// Directorio detail panel.
const (
	DetailMembers = "Personas"
	DetailNone    = "—"
)

// ConfirmDeleteVendor asks before a vendor is deleted.
func ConfirmDeleteVendor(name string) string {
	return fmt.Sprintf("¿Eliminar a %s del directorio? Seguirá apareciendo en sus movimientos anteriores.", name)
}

// VendorConflictMessage explains why a delete or restore was refused, naming the
// vendors involved. reason is an app.VendorConflict reason.
func VendorConflictMessage(reason string, names []string) string {
	switch reason {
	case "company_has_members":
		return "No se puede eliminar: la empresa tiene personas activas: " + strings.Join(names, ", ") +
			". Elimínelas primero."
	case "company_deleted":
		return "No se puede restaurar: su empresa «" + strings.Join(names, ", ") +
			"» está eliminada. Restaure primero la empresa."
	case "company_not_company":
		return "No se puede restaurar: «" + strings.Join(names, ", ") +
			"» ya no es una empresa. Cambie su tipo a Empresa primero."
	}
	return reason
}

// Key names and key-hint descriptions of Directorio and the picker.
const (
	KeyNew     = "n"
	KeyDelete  = "d"
	KeySearch  = "/"
	KeyFilter  = "f"
	KeyDeleted = "."
	KeyRestore = "r"
	KeyPlus    = "+"
	KeyRemove  = "supr"

	HintNew         = "nuevo"
	HintDelete      = "eliminar"
	HintSearch      = "buscar"
	HintFilterKind  = "tipo"
	HintShowDeleted = "eliminados"
	HintRestore     = "restaurar"
	HintPick        = "elegir"
	HintCreateNew   = "crear nuevo"
	HintNoSelection = "sin selección"
	HintPickCompany = "elegir empresa"
	HintRemoveCo    = "quitar empresa"
	HintApplySearch = "aplicar"
	HintClearSearch = "borrar"
	PickerNoMatch   = "Sin coincidencias"
	PickerFilter    = "Buscar: "
)
