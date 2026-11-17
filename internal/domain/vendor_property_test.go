package domain

import (
	"strings"
	"testing"

	"pgregory.net/rapid"
)

// Folding must not tell apart two spellings that differ only by case or by an
// accent: whatever name is typed, adding an acute accent to its vowels and
// changing its case folds to the same key.
func TestFoldKey_IgnoresAccentsAndCaseOnAnyName(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		name := rapid.StringMatching(`[a-zA-Z ]{0,30}`).Draw(t, "name")
		accented := strings.NewReplacer("a", "á", "e", "é", "i", "í", "o", "ó", "u", "ú").Replace(name)

		if FoldKey(strings.ToUpper(accented)) != FoldKey(name) {
			t.Fatalf("FoldKey(%q) != FoldKey(%q)", strings.ToUpper(accented), name)
		}
	})
}
