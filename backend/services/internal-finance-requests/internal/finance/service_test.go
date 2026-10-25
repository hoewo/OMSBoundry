package finance

import (
	"math"
	"testing"
)

func TestTotalLinesRejectsOverflow(t *testing.T) {
	if value, err := totalLines([]Line{{AmountMinor: math.MaxInt64 - 1}, {AmountMinor: 1}}); err != nil || value != math.MaxInt64 {
		t.Fatalf("valid boundary total=%d err=%v", value, err)
	}
	if _, err := totalLines([]Line{{AmountMinor: math.MaxInt64}, {AmountMinor: 1}}); err == nil {
		t.Fatal("overflowing exact-money total must be rejected")
	}
}
