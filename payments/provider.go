package payments

import "context"

// ChargeRequest is a provider-agnostic charge instruction.
type ChargeRequest struct {
	InvoiceID int64
	Currency  string
	Amount    int64
	Ref       string
	Token     string // saved provider card token (recurrent); empty for manual
}

// ChargeResult is the provider's response.
type ChargeResult struct {
	ProviderRef string
	Succeeded   bool
	Raw         map[string]any
}

// Provider is the pluggable payment-provider seam. SP1 ships only `manual`.
type Provider interface {
	Name() string
	Charge(ctx context.Context, req ChargeRequest) (ChargeResult, error)
}
