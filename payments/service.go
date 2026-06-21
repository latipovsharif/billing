package payments

import (
	"context"
	"fmt"
	"time"

	"billing/base"
	"billing/ledger"
	"billing/subscriptions"
	"billing/webhooks"
)

// Service records payments and drives subscription state on success.
type Service struct {
	provider Provider
	repo     *Repo
	ledger   *ledger.Repo
	subs     *subscriptions.Repo
	outbox   *webhooks.Outbox
}

func NewService(p Provider) *Service {
	return &Service{provider: p, repo: NewRepo(), ledger: ledger.NewRepo(), subs: subscriptions.NewRepo(), outbox: webhooks.NewOutbox()}
}

// MarkPaid charges via the provider (manual = always succeeds), appends a
// payment + ledger entry, flips the invoice to paid, and advances the
// subscription (EvPayment). All in the caller's tx.
func (s *Service) MarkPaid(ctx context.Context, tx base.PGXDB, productID, invoiceID int64, actor string) error {
	iv, ok, err := s.repo.GetInvoice(ctx, tx, invoiceID)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("invoice not found")
	}
	if iv.Status == "paid" {
		return nil // idempotent
	}

	res, err := s.provider.Charge(ctx, ChargeRequest{InvoiceID: iv.ID, Currency: iv.Currency, Amount: iv.Amount, Ref: fmt.Sprintf("inv-%d", iv.ID)})
	if err != nil {
		return err
	}
	if !res.Succeeded {
		return fmt.Errorf("charge declined")
	}

	payID, err := s.repo.InsertPayment(ctx, tx, iv.ID, s.provider.Name(), res.ProviderRef, iv.Currency, iv.Amount, res.Raw)
	if err != nil {
		return err
	}
	if err := s.repo.MarkInvoicePaid(ctx, tx, iv.ID); err != nil {
		return err
	}
	if _, err := s.ledger.Append(ctx, tx, ledger.Entry{
		CustomerID: iv.CustomerID, InvoiceID: &iv.ID, PaymentID: &payID,
		Type: ledger.Payment, Currency: iv.Currency, Amount: iv.Amount, Ref: res.ProviderRef,
	}); err != nil {
		return err
	}

	sub, ok, err := s.subs.Get(ctx, tx, iv.SubscriptionID)
	if err != nil || !ok {
		return fmt.Errorf("subscription not found")
	}
	to, ok := subscriptions.Next(sub.Status, subscriptions.EvPayment)
	if !ok {
		// payment recorded but no transition (e.g. already active) — fine
		return s.emitPayment(ctx, tx, productID, sub, payID)
	}
	now := time.Now()
	end := now.AddDate(0, 1, 0)
	if sub.Interval == "year" {
		end = now.AddDate(1, 0, 0)
	}
	if err := s.subs.SetStatus(ctx, tx, sub.ID, sub.Status, to, "payment", actor, &now, &end, nil); err != nil {
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
	return s.emitPayment(ctx, tx, productID, sub, payID)
}

func (s *Service) emitPayment(ctx context.Context, tx base.PGXDB, productID int64, sub subscriptions.Subscription, payID int64) error {
	return s.outbox.Enqueue(ctx, tx, productID, "payment.succeeded", webhooks.Payload{
		CustomerID: sub.CustomerID, SubscriptionID: sub.ID, Status: string(sub.Status),
	})
}
