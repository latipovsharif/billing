package subscriptions

import (
	"context"
	"fmt"
	"time"

	"billing/base"
	"billing/catalog"
	"billing/webhooks"
)

// CreateInput is the request to start a subscription.
type CreateInput struct {
	CustomerID int64
	PlanCode   string
	Currency   string
	Interval   string // month | year
	Trial      bool
}

// Service orchestrates subscription writes. Methods take a tx so the caller
// controls the transaction boundary (controllers wrap with base.WithTx).
type Service struct {
	catalog *catalog.Repo
	subs    *Repo
	outbox  *webhooks.Outbox
}

func NewService() *Service {
	return &Service{catalog: catalog.NewRepo(), subs: NewRepo(), outbox: webhooks.NewOutbox()}
}

func periodEnd(start time.Time, interval string) time.Time {
	if interval == "year" {
		return start.AddDate(1, 0, 0)
	}
	return start.AddDate(0, 1, 0)
}

// Create starts a trialing (Trial=true) or active subscription. Writes the
// subscription, its history, and a subscription.created outbox event in one tx.
func (s *Service) Create(ctx context.Context, tx base.PGXDB, productID int64, in CreateInput) (Subscription, error) {
	plan, ok, err := s.catalog.PlanByCode(ctx, tx, productID, in.PlanCode)
	if err != nil {
		return Subscription{}, err
	}
	if !ok {
		return Subscription{}, fmt.Errorf("unknown plan %q", in.PlanCode)
	}
	price, ok, err := s.catalog.PriceFor(ctx, tx, plan.ID, in.Currency, in.Interval)
	if err != nil {
		return Subscription{}, err
	}
	if !ok {
		return Subscription{}, fmt.Errorf("no price for %s/%s", in.Currency, in.Interval)
	}

	now := time.Now()
	sub := Subscription{
		CustomerID: in.CustomerID, PlanID: plan.ID, Currency: in.Currency,
		Interval: in.Interval, Amount: price.Amount,
	}
	if in.Trial {
		end := now.AddDate(0, 0, plan.TrialDays)
		sub.Status = Trialing
		sub.TrialEnd = &end
	} else {
		end := periodEnd(now, in.Interval)
		sub.Status = Active
		sub.CurrentPeriodStart = &now
		sub.CurrentPeriodEnd = &end
	}

	id, err := s.subs.Insert(ctx, tx, sub, "create", "system")
	if err != nil {
		return Subscription{}, err
	}
	sub.ID = id

	if err := s.outbox.Enqueue(ctx, tx, productID, "subscription.created", webhooks.Payload{
		CustomerID: in.CustomerID, SubscriptionID: id, Status: string(sub.Status), PlanCode: plan.Code,
	}); err != nil {
		return Subscription{}, err
	}
	return sub, nil
}

// Cancel terminates a subscription immediately.
func (s *Service) Cancel(ctx context.Context, tx base.PGXDB, productID, subID int64, actor string) error {
	sub, ok, err := s.subs.Get(ctx, tx, subID)
	if err != nil || !ok {
		return fmt.Errorf("subscription not found")
	}
	to, ok := Next(sub.Status, EvCancel)
	if !ok {
		return fmt.Errorf("cannot cancel from %s", sub.Status)
	}
	now := time.Now()
	if err := s.subs.SetStatus(ctx, tx, subID, sub.Status, to, "cancel", actor, nil, nil, &now); err != nil {
		return err
	}
	return s.outbox.Enqueue(ctx, tx, productID, "subscription.canceled", webhooks.Payload{
		CustomerID: sub.CustomerID, SubscriptionID: subID, Status: string(to),
	})
}
