package ledger

// Type classifies a ledger entry.
type Type string

const (
	Charge     Type = "charge"
	Payment    Type = "payment"
	Refund     Type = "refund"
	Adjustment Type = "adjustment"
)

// Entry is an immutable financial record (signed minor units).
type Entry struct {
	CustomerID int64
	InvoiceID  *int64
	PaymentID  *int64
	Type       Type
	Currency   string
	Amount     int64 // signed: charge negative, payment positive
	Ref        string
}
