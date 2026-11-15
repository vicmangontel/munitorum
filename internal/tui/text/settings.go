package text

import "fmt"

// Ajustes overlay.
const (
	SettingsTitle      = "Ajustes"
	SettingsSaveFailed = "No se pudo guardar el cambio:"

	KeyF2        = "F2"
	KeyE         = "e"
	HintSettings = "ajustes"
	HintEdit     = "editar"
	HintSave     = "guardar"
	HelpSettings = "abrir Ajustes (nombre del coto, cuentas, saldos iniciales)"
)

// ConfirmOpeningBalance asks before an opening balance changes, because every
// balance of the coto is derived from it.
func ConfirmOpeningBalance(account, from, to string) string {
	return fmt.Sprintf("Cambiar el saldo inicial de %s de %s a %s modifica todos los saldos. ¿Continuar?", account, from, to)
}
