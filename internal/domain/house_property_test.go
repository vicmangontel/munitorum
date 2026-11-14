package domain

import (
	"strconv"
	"testing"

	"pgregory.net/rapid"
)

func TestHouseCode_IsUniquePerNumberAndOfOneWidth(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		count := rapid.IntRange(MinHouseCount, MaxHouseCount).Draw(t, "count")
		prefix := rapid.StringMatching(`[A-Za-z -]{0,10}`).Draw(t, "prefix")
		wantLen := len(prefix) + max(2, len(strconv.Itoa(count)))

		seen := map[string]int{}
		for n := 1; n <= count; n++ {
			code := HouseCode(prefix, n, count)

			if len(code) != wantLen {
				t.Fatalf("code %q for house %d of %d has length %d, want %d", code, n, count, len(code), wantLen)
			}
			if other, dup := seen[code]; dup {
				t.Fatalf("houses %d and %d share code %q", other, n, code)
			}
			seen[code] = n
		}
	})
}
