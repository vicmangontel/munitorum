package text

import (
	"testing"

	"github.com/vicmangontel/munitorum/internal/app"
)

func TestValidationMessage_ExistsForEveryCode(t *testing.T) {
	for _, code := range app.Codes {
		t.Run(code, func(t *testing.T) {
			// Act
			msg := ValidationMessage(code)

			// Assert
			if msg == "" || msg == code {
				t.Fatalf("no Spanish message for code %q", code)
			}
		})
	}
}
