package jobs

import (
	"testing"
	"time"

	"billing/catalog"
	"billing/customers"
	"billing/subscriptions"
	"billing/testdb"
)

func TestTrialExpiryMovesToPastDue(t *testing.T) {
	ctx, tx := testdb.TxOrSkip(t)
	cr := catalog.NewRepo()
	pid, _ := cr.CreateProduct(ctx, tx, "cm", "CM", "k")
	planID, _ := cr.CreatePlan(ctx, tx, pid, "pro", "Pro", map[string]any{}, 14)
	_ = cr.AddPrice(ctx, tx, planID, "UZS", "month", 5000)
	cust, _ := customers.NewRepo().Register(ctx, tx, pid, "shopA", "u1", "Shop A", "UZS")

	sub, _ := subscriptions.NewService().Create(ctx, tx, pid, subscriptions.CreateInput{
		CustomerID: cust.ID, PlanCode: "pro", Currency: "UZS", Interval: "month", Trial: true,
	})
	_, _ = tx.Exec(ctx, `UPDATE subscription SET trial_end=$2 WHERE id=$1`, sub.ID, time.Now().Add(-time.Hour))

	r := NewRunner(3)
	if err := r.TrialExpiry(ctx, tx); err != nil {
		t.Fatal(err)
	}
	got, _, _ := subscriptions.NewRepo().Get(ctx, tx, sub.ID)
	if got.Status != subscriptions.PastDue {
		t.Fatalf("status = %s, want past_due", got.Status)
	}
}
