package text

import "fmt"

// ViewLedger is the position of Movimientos in ViewNames.
const ViewLedger = 2

// Movimientos list.
const (
	ColDate        = "Fecha"
	ColCategory    = "Categoría"
	ColParty       = "De/Para"
	ColDescription = "Descripción"
	ColAccount     = "Cuenta"
	ColAmount      = "Monto"
	ColBalance     = "Saldo"
	ColReceipt     = "R"

	ReceiptPendingMark = "○"
	ReceiptDoneMark    = "●"

	LedgerEmpty      = "No hay movimientos este mes. Presione i para un ingreso o g para un gasto."
	LedgerLoadFailed = "No se pudo leer el libro de caja:"
)

// LedgerPeriod is the status line above the list: the month and the balance
// before and after it. Amounts are already formatted.
func LedgerPeriod(month, opening, closing string) string {
	return fmt.Sprintf("%s · Saldo inicial %s · Saldo final %s", month, opening, closing)
}

// LedgerTotals is the footer of the list. Amounts are already formatted.
func LedgerTotals(income, expense, adjustments, net string) string {
	return fmt.Sprintf("Ingresos %s · Gastos %s · Ajustes %s · Neto %s", income, expense, adjustments, net)
}

// Income and expense forms.
const (
	IncomeTitle  = "Nuevo ingreso"
	ExpenseTitle = "Nuevo gasto"

	LabelDate          = "Fecha (DD/MM/AAAA)"
	LabelAccount       = "Cuenta"
	LabelCategory      = "Categoría"
	LabelPayee         = "Proveedor"
	LabelAmount        = "Monto"
	LabelDescription   = "Descripción"
	LabelNotes         = "Notas"
	LabelReceiptLink   = "Enlace del recibo"
	LabelManualReceipt = "Recibo manual pendiente"

	VendorNone = "(ninguno)"
)

// NegativeBalanceQuestion asks whether to record an expense that leaves an
// account below zero. Account and resulting are the account name and its
// formatted resulting balance.
func NegativeBalanceQuestion(account, resulting string) string {
	return fmt.Sprintf("Este gasto deja la cuenta %s en %s. ¿Continuar?", account, resulting)
}

// Key names and key-hint descriptions of Movimientos.
const (
	KeyIncome  = "i"
	KeyExpense = "g"
	KeyMonths  = "[ ]"

	HintIncome      = "ingreso"
	HintExpense     = "gasto"
	HintMonth       = "mes"
	HintPickPayee   = "elegir proveedor"
	HintRemovePayee = "quitar proveedor"
)
