// Package text holds every user-visible string. The interface is Spanish only;
// keeping the strings here is what makes a later translation possible.
package text

import "fmt"

const (
	AppName = "Munitorum"
	FlagDB  = "ruta del archivo de base de datos (por omisión: munitorum.db junto al ejecutable)"
)

// Views, in the order of the view list; keys 1 to 8 select them.
var ViewNames = [8]string{
	"Resumen", "Casas", "Movimientos", "Cooperaciones",
	"Obras", "Recordatorios", "Directorio", "Pendientes",
}

// Panels.
const (
	PanelViews      = "Vistas"
	PanelDetail     = "Detalle"
	SummaryTitle    = "Coto" // replaced by the coto name from setup in Phase 1
	NoSelection     = "Sin selección"
	PlaceholderBody = "Esta vista se construye en una fase posterior."
)

// Summary panel rows, static until Phase 1.
const (
	AccountCash   = "Efectivo"
	AccountBank   = "Banco"
	AccountTotal  = "Total"
	Available     = "Disponible"
	AmountPending = "$0.00"
)

// Key names and key-hint descriptions.
const (
	KeyTab      = "tab"
	KeyShiftTab = "shift+tab"
	KeyViews    = "1-8"
	KeyMove     = "↑/↓"
	KeyEnter    = "enter"
	KeyHelp     = "?"
	KeyEsc      = "esc"
	KeyQuit     = "q"

	HintPanel = "panel"
	HintView  = "vista"
	HintMove  = "mover"
	HintOpen  = "abrir"
	HintHelp  = "ayuda"
	HintClose = "cerrar"
	HintQuit  = "salir"
)

// Help overlay.
const (
	HelpTitle      = "Ayuda"
	HelpGlobalKeys = "Teclas generales"
	HelpNextPanel  = "siguiente panel"
	HelpPrevPanel  = "panel anterior"
	HelpGoToView   = "ir a la vista"
	HelpMoveList   = "moverse en la lista de vistas"
	HelpOpenView   = "abrir la vista seleccionada"
	HelpToggle     = "abrir o cerrar esta ayuda"
	HelpClose      = "cerrar"
	HelpQuit       = "salir"
	HelpNoViewKeys = "Esta vista no tiene teclas propias todavía."
)

// HelpViewKeys is the heading of the focused view's key list.
func HelpViewKeys(view string) string { return "Teclas de " + view }

// TooSmall is shown instead of the shell below the minimum terminal size.
func TooSmall(w, h, minW, minH int) string {
	return fmt.Sprintf("Terminal muy pequeña (%d×%d). Se necesitan al menos %d×%d.", w, h, minW, minH)
}

// Startup failures.
const (
	StartupFailed = "No se pudo iniciar:"
	DBOpenFailed  = "No se pudo abrir la base de datos"
	DBPathLabel   = "Archivo:"
	DBErrorLabel  = "Error:"
	AnyKeyToExit  = "Presione cualquier tecla para salir."
)
