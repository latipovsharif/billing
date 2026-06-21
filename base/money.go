package base

import "fmt"

// Amount is an integer monetary value in the currency's minor units
// (e.g. tiyin for UZS). Never use floats for money.
type Amount int64

// ParseAmount validates a raw minor-unit value (must be >= 0 for stored prices).
func ParseAmount(v int64) (Amount, error) {
	if v < 0 {
		return 0, fmt.Errorf("amount must be non-negative, got %d", v)
	}
	return Amount(v), nil
}
