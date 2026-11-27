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
	KeyIncome   = "i"
	KeyExpense  = "g"
	KeyTransfer = "t"
	KeyCount    = "a"
	KeyMonths   = "[ ]"

	HintIncome      = "ingreso"
	HintExpense     = "gasto"
	HintTransfer    = "transferencia"
	HintCount       = "arqueo"
	HintMonth       = "mes"
	HintPickPayee   = "elegir proveedor"
	HintRemovePayee = "quitar proveedor"
)

// Transfer form and the list labels of transfers and adjustments.
const (
	TransferTitle = "Nueva transferencia"

	LabelFrom = "Desde"
	LabelTo   = "Hacia"

	LabelTransfer   = "Transferencia" // Categoría cell of a transfer
	LabelAdjustment = "Ajuste"        // Categoría cell of a cash-count adjustment
)

// TransferDescription is the description the transfer form fills in when the
// treasurer leaves it blank.
func TransferDescription(from, to string) string {
	return fmt.Sprintf("Transferencia %s → %s", from, to)
}

// NegativeBalanceTransferQuestion asks whether to record a transfer that leaves
// the source account below zero. Account and resulting are the account name and
// its formatted resulting balance.
func NegativeBalanceTransferQuestion(account, resulting string) string {
	return fmt.Sprintf("Esta transferencia deja la cuenta %s en %s. ¿Continuar?", account, resulting)
}

// Arqueo overlay.
const (
	CountTitle = "Arqueo"

	LabelCounted  = "Monto contado"
	CountBalanced = "Cuadra"
)

// CashCountDescription is the description of the adjustment a cash count
// writes. Counted and system are already formatted.
func CashCountDescription(counted, system string) string {
	return fmt.Sprintf("Arqueo: contado %s, sistema %s", counted, system)
}

// CountSystem is the line that shows the system balance of the counted account.
func CountSystem(balance string) string { return "Saldo del sistema: " + balance }

// CountDifference is the line that shows counted minus system.
func CountDifference(difference string) string { return "Diferencia: " + difference }

// TransferTo is the De/Para cell of a transfer row: the receiving account.
func TransferTo(account string) string { return "→ " + account }
