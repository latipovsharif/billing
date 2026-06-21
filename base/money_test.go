package base

import "testing"

func TestParseAmountRejectsNegative(t *testing.T) {
	if _, err := ParseAmount(-1); err == nil {
		t.Fatal("expected error for negative amount")
	}
}

func TestParseAmountAcceptsZeroAndPositive(t *testing.T) {
	for _, v := range []int64{0, 1, 999_999} {
		if got, err := ParseAmount(v); err != nil || int64(got) != v {
			t.Fatalf("ParseAmount(%d) = %v, %v", v, got, err)
		}
	}
}
