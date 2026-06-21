package payments

import "context"

// Manual is the operator-driven provider: a charge "succeeds" because an
// operator recorded an external payment (bank transfer, cash, etc.).
type Manual struct{}

func NewManual() *Manual { return &Manual{} }

func (Manual) Name() string { return "manual" }

func (Manual) Charge(_ context.Context, req ChargeRequest) (ChargeResult, error) {
	return ChargeResult{ProviderRef: req.Ref, Succeeded: true, Raw: map[string]any{"source": "manual"}}, nil
}
