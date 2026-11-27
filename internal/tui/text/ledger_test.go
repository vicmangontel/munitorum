package text

import "testing"

func TestTransferTo_ShowsTheArrowAndTheReceivingAccount(t *testing.T) {
	// Act
	got := TransferTo("Banco (NU)")

	// Assert
	if want := "→ Banco (NU)"; got != want {
		t.Fatalf("TransferTo = %q, want %q", got, want)
	}
}
