package main

import (
	"testing"
	"time"

	"billing/catalog"
	"billing/customers"
	"billing/jobs"
	"billing/payments"
	"billing/subscriptions"
	"billing/testdb"
)

// Exercises trial -> expire -> past_due -> mark-paid -> active in one tx.
func TestTrialToPaidLifecycle(t *testing.T) {
	ctx, tx := testdb.TxOrSkip(t)
	cr := catalog.NewRepo()
	pid, _ := cr.CreateProduct(ctx, tx, "cm", "CM", "k")
	planID, _ := cr.CreatePlan(ctx, tx, pid, "pro", "Pro", map[string]any{}, 14)
	_ = cr.AddPrice(ctx, tx, planID, "UZS", "month", 5000)
	cust, _ := customers.NewRepo().Register(ctx, tx, pid, "shopA", "u1", "Shop A", "UZS")

	sub, err := subscriptions.NewService().Create(ctx, tx, pid, subscriptions.CreateInput{
		CustomerID: cust.ID, PlanCode: "pro", Currency: "UZS", Interval: "month", Trial: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	// expire trial
	_, _ = tx.Exec(ctx, `UPDATE subscription SET trial_end=$2 WHERE id=$1`, sub.ID, time.Now().Add(-time.Hour))
	if err := jobs.NewRunner(3, nil).TrialExpiry(ctx, tx); err != nil {
		t.Fatal(err)
	}
	if s, _, _ := subscriptions.NewRepo().Get(ctx, tx, sub.ID); s.Status != subscriptions.PastDue {
		t.Fatalf("after trial expiry: %s", s.Status)
	}

	// operator issues + pays an invoice
	var invID int64
	now := time.Now()
	_ = tx.QueryRow(ctx,
		`INSERT INTO invoice (subscription_id, customer_id, currency, amount, status, period_start, period_end, due_date)
		 VALUES ($1,$2,'UZS',5000,'open',$3,$4,$4) RETURNING id`,
		sub.ID, cust.ID, now, now.AddDate(0, 1, 0)).Scan(&invID)
	if err := payments.NewService(payments.NewManual(), nil).MarkPaid(ctx, tx, pid, invID, "operator"); err != nil {
		t.Fatal(err)
	}
	if s, _, _ := subscriptions.NewRepo().Get(ctx, tx, sub.ID); s.Status != subscriptions.Active {
		t.Fatalf("after payment: %s", s.Status)
	}
}
