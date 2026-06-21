package payments

import (
	"context"
	"errors"
	"fmt"
	"time"

	"billing/base"
	"billing/ledger"
	"billing/subscriptions"
	"billing/webhooks"
)

// ErrNoPaymentMethod means the customer has no verified default card.
var ErrNoPaymentMethod = errors.New("no verified payment method")

// ErrChargeDeclined means the provider rejected the charge.
var ErrChargeDeclined = errors.New("charge declined")

// Service records payments and drives subscription state on success.
type Service struct {
	provider Provider
	repo     *Repo
	ledger   *ledger.Repo
	subs     *subscriptions.Repo
	outbox   *webhooks.Outbox
	methods  *PaymentMethodRepo // may be nil for manual-only deployments
}

// NewService builds the payments service. methods may be nil when no recurrent
// provider is configured (manual mark-paid still works).
func NewService(p Provider, methods *PaymentMethodRepo) *Service {
	return &Service{
		provider: p, repo: NewRepo(), ledger: ledger.NewRepo(),
		subs: subscriptions.NewRepo(), outbox: webhooks.NewOutbox(), methods: methods,
	}
}

// settleInvoice records the payment + ledger entry, marks the invoice paid, and
// advances the subscription via EvPayment (idempotent: a paid invoice is a
// no-op). Shared by manual MarkPaid and auto ChargeInvoice.
func (s *Service) settleInvoice(ctx context.Context, tx base.PGXDB, productID int64, iv Invoice, providerName, providerRef string, raw map[string]any) error {
	if iv.Status == "paid" {
		return nil
	}
	payID, err := s.repo.InsertPayment(ctx, tx, iv.ID, providerName, providerRef, iv.Currency, iv.Amount, raw)
	if err != nil {
		return err
	}
	if err := s.repo.MarkInvoicePaid(ctx, tx, iv.ID); err != nil {
		return err
	}
	if _, err := s.ledger.Append(ctx, tx, ledger.Entry{
		CustomerID: iv.CustomerID, InvoiceID: &iv.ID, PaymentID: &payID,
		Type: ledger.Payment, Currency: iv.Currency, Amount: iv.Amount, Ref: providerRef,
	}); err != nil {
		return err
	}

	sub, ok, err := s.subs.Get(ctx, tx, iv.SubscriptionID)
	if err != nil || !ok {
		return fmt.Errorf("subscription not found")
	}
	to, ok := subscriptions.Next(sub.Status, subscriptions.EvPayment)
	if ok {
		now := time.Now()
		end := now.AddDate(0, 1, 0)
		if sub.Interval == "year" {
			end = now.AddDate(1, 0, 0)
		}
		if err := s.subs.SetStatus(ctx, tx, sub.ID, sub.Status, to, "payment", "system", &now, &end, nil); err != nil {
			return err
		}
		evt := "subscription.activated"
		if sub.Status == subscriptions.Suspended {
			evt = "subscription.reactivated"
		}
		if err := s.outbox.Enqueue(ctx, tx, productID, evt, webhooks.Payload{
			CustomerID: sub.CustomerID, SubscriptionID: sub.ID, Status: string(to),
		}); err != nil {
			return err
		}
	}
	return s.outbox.Enqueue(ctx, tx, productID, "payment.succeeded", webhooks.Payload{
		CustomerID: sub.CustomerID, SubscriptionID: sub.ID, Status: string(sub.Status),
	})
}

// MarkPaid is the manual/admin path: charge via provider (manual succeeds),
// then settle.
func (s *Service) MarkPaid(ctx context.Context, tx base.PGXDB, productID, invoiceID int64, actor string) error {
	iv, ok, err := s.repo.GetInvoice(ctx, tx, invoiceID)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("invoice not found")
	}
	if iv.Status == "paid" {
		return nil
	}
	res, err := s.provider.Charge(ctx, ChargeRequest{InvoiceID: iv.ID, Currency: iv.Currency, Amount: iv.Amount, Ref: fmt.Sprintf("inv-%d", iv.ID)})
	if err != nil {
		return err
	}
	if !res.Succeeded {
		return ErrChargeDeclined
	}
	return s.settleInvoice(ctx, tx, productID, iv, s.provider.Name(), res.ProviderRef, res.Raw)
}

// ChargeInvoice is the automatic path: resolve the customer's default verified
// token, charge it via the provider, then settle. Returns ErrNoPaymentMethod
// when the customer has no card, ErrChargeDeclined on a hard decline, or the
// provider's error on a transient failure.
func (s *Service) ChargeInvoice(ctx context.Context, tx base.PGXDB, productID, invoiceID int64) error {
	iv, ok, err := s.repo.GetInvoice(ctx, tx, invoiceID)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("invoice not found")
	}
	if iv.Status == "paid" {
		return nil
	}
	if s.methods == nil {
		return ErrNoPaymentMethod
	}
	token, ok, err := s.methods.DefaultVerifiedToken(ctx, tx, iv.CustomerID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNoPaymentMethod
	}
	res, err := s.provider.Charge(ctx, ChargeRequest{
		InvoiceID: iv.ID, Currency: iv.Currency, Amount: iv.Amount,
		Ref: fmt.Sprintf("inv-%d", iv.ID), Token: token,
	})
	if err != nil {
		return err // transient — caller leaves status as-is and retries
	}
	if !res.Succeeded {
		return ErrChargeDeclined
	}
	return s.settleInvoice(ctx, tx, productID, iv, s.provider.Name(), res.ProviderRef, res.Raw)
}
