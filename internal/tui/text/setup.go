package text

import "fmt"

// Setup wizard.
const (
	WizardTitle        = "Configuración inicial"
	WizardCancelPrompt = "¿Cancelar la configuración? No se guardará nada."
	WizardSaving       = "Guardando…"
	WizardSaveFailed   = "No se pudo guardar la configuración:"
)

// WizardStepNames are the titles of the seven steps, in order.
var WizardStepNames = [7]string{"Coto", "Casas", "Cuota", "Cuentas", "Apertura", "Casas al abrir", "Resumen"}

// WizardStepTitle is the heading of a step; step is 1-based.
func WizardStepTitle(step int) string {
	return fmt.Sprintf("%s — paso %d de %d: %s", WizardTitle, step, len(WizardStepNames), WizardStepNames[step-1])
}

// Field labels.
const (
	LabelCommunityName = "Nombre del coto"
	LabelHouseCount    = "Número de casas"
	LabelHousePrefix   = "Prefijo del código"
	LabelFee           = "Cuota mensual"
	LabelCashAccount   = "Cuenta de efectivo"
	LabelBankAccount   = "Cuenta de banco"
	LabelOpeningDate   = "Fecha de apertura (DD/MM/AAAA)"
)

// LabelOpeningBalance is the label of an account's opening balance field.
func LabelOpeningBalance(account string) string { return "Saldo inicial de " + account }

// Defaults shown in the wizard.
const (
	DefaultCashAccount = "Efectivo"
	DefaultBankAccount = "Banco (NU)"
)

// CodePreview shows the first and last house code while the treasurer types.
func CodePreview(first, last string) string {
	if first == last {
		return "Código: " + first
	}
	return fmt.Sprintf("Códigos: %s … %s", first, last)
}

// Step 6, one row per house.
const (
	ColHouse          = "Casa"
	ColCoveredThrough = "Cubierto hasta"
	ColCredit         = "Crédito"
	ColArrears        = "Adeudo inicial"
	HousesRowsHelp    = "Cubierto hasta: MM/AAAA, vacío si la casa no tenía meses cubiertos."
)

// HousesRowPosition tells which rows of the table are on screen.
func HousesRowPosition(from, to, total int) string {
	return fmt.Sprintf("Casas %d–%d de %d", from, to, total)
}

// Step 7, summary.
const (
	SumHouses        = "Casas"
	SumFee           = "Cuota mensual"
	SumBalanceTotal  = "Saldo total"
	SumHousesCovered = "Casas con meses cubiertos"
	SumTotalCredit   = "Crédito total"
	SumTotalArrears  = "Adeudo inicial total"
)

// SumBalance is the summary row of one account's opening balance.
func SumBalance(account string) string { return "Saldo " + account }

// Wizard key-hint descriptions.
const (
	KeyLeft      = "←"
	KeyLeftRight = "←/→"
	HintNext     = "siguiente"
	HintBack     = "atrás"
	HintField    = "campo"
	HintRow      = "casa"
	HintCancel   = "cancelar"
	HintConfirm  = "confirmar y guardar"
	HintChoose   = "elegir"
	HintAccept   = "aceptar"
)

// Yes and No answer a confirmation.
const (
	Yes = "Sí"
	No  = "No"
)

// validationMessages maps an app validation code to its message.
var validationMessages = map[string]string{
	"community_name_length":      "El nombre es obligatorio y admite hasta 60 caracteres.",
	"house_count_range":          "Escriba un número entero de 1 a 200.",
	"house_prefix_length":        "El prefijo admite hasta 10 caracteres.",
	"amount_invalid":             "Escriba un importe válido, por ejemplo 1,234.50.",
	"fee_not_positive":           "La cuota debe ser mayor que cero.",
	"account_name_length":        "El nombre es obligatorio y admite hasta 30 caracteres.",
	"account_name_duplicate":     "Las dos cuentas deben tener nombres distintos.",
	"balance_negative":           "El saldo no puede ser negativo.",
	"date_invalid":               "Escriba la fecha como DD/MM/AAAA.",
	"date_future":                "La fecha no puede ser posterior a hoy.",
	"month_invalid":              "Escriba el mes como MM/AAAA.",
	"houses_mismatch":            "La lista de casas no coincide con el número de casas.",
	"covered_before_opening":     "El mes no puede ser anterior al mes de apertura.",
	"covered_too_far":            "El mes no puede pasar de 36 meses después de la apertura.",
	"credit_out_of_range":        "El crédito debe ser menor que la cuota.",
	"arrears_negative":           "El adeudo no puede ser negativo.",
	"arrears_with_coverage":      "Una casa con adeudo no puede tener meses cubiertos.",
	"arrears_with_credit":        "Una casa con adeudo no puede tener crédito.",
	"vendor_kind_invalid":        "Elija el tipo: Persona, Empresa o Servicio.",
	"vendor_name_length":         "El nombre es obligatorio y admite hasta 80 caracteres.",
	"vendor_phone_length":        "El teléfono admite hasta 30 caracteres.",
	"vendor_email_invalid":       "El correo debe tener un @ con texto antes y después.",
	"vendor_notes_length":        "Las notas admiten hasta 500 caracteres.",
	"vendor_company_not_allowed": "Solo una persona puede tener empresa.",
	"vendor_company_self":        "Un elemento no puede ser su propia empresa.",
	"vendor_company_invalid":     "Elija una empresa activa del directorio.",
	"vendor_name_duplicate":      "Ya existe un elemento activo del mismo tipo con ese nombre.",
	"vendor_kind_in_use":         "La empresa tiene personas activas; cambie o elimine primero a esas personas.",
	"date_before_opening":        "La fecha no puede ser anterior a la fecha de apertura.",
	"amount_not_positive":        "El monto debe ser mayor que cero.",
	"account_invalid":            "Elija una cuenta.",
	"description_length":         "La descripción es obligatoria y admite hasta 120 caracteres.",
	"notes_length":               "Las notas admiten hasta 500 caracteres.",
	"receipt_link_length":        "El enlace del recibo admite hasta 500 caracteres.",
	"category_invalid":           "Elija una categoría activa.",
	"category_reserved":          "Esa categoría la registra otro módulo; elija otra.",
	"vendor_invalid":             "Elija un proveedor activo del directorio.",
	"accounts_same":              "Elija una cuenta de destino distinta de la de origen.",
	"amount_negative":            "El monto contado no puede ser negativo.",
}

// ValidationMessage is the Spanish message for an app validation code. An
// unknown code is shown as is rather than hidden.
func ValidationMessage(code string) string {
	if msg, ok := validationMessages[code]; ok {
		return msg
	}
	return code
}
