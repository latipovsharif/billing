package payme

import (
	"context"

	"billing/payments"
)

// Provider adapts the Payme client to payments.Provider (recurrent charge).
type Provider struct{ client *Client }

func NewProvider(client *Client) *Provider { return &Provider{client: client} }

func (Provider) Name() string { return "payme" }

// Charge pays the invoice with the saved token via receipts.create+pay.
func (p Provider) Charge(ctx context.Context, req payments.ChargeRequest) (payments.ChargeResult, error) {
	if req.Token == "" {
		return payments.ChargeResult{Succeeded: false}, nil
	}
	ref, err := p.client.Charge(ctx, req.InvoiceID, req.Amount, req.Token)
	if err != nil {
		return payments.ChargeResult{}, err
	}
	return payments.ChargeResult{ProviderRef: ref, Succeeded: true, Raw: map[string]any{"receipt_id": ref}}, nil
}
