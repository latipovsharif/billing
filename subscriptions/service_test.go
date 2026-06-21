package subscriptions

import (
	"testing"

	"billing/catalog"
	"billing/customers"
	"billing/testdb"
)

func TestCreateTrialWritesHistoryAndOutbox(t *testing.T) {
	ctx, tx := testdb.TxOrSkip(t)
	cr := catalog.NewRepo()
	pid, _ := cr.CreateProduct(ctx, tx, "cm", "CM", "k")
	planID, _ := cr.CreatePlan(ctx, tx, pid, "pro", "Pro", map[string]any{}, 14)
	_ = cr.AddPrice(ctx, tx, planID, "UZS", "month", 5000000)
	cust, _ := customers.NewRepo().Register(ctx, tx, pid, "shopA", "u1", "Shop A", "UZS")

	svc := NewService()
	sub, err := svc.Create(ctx, tx, pid, CreateInput{
		CustomerID: cust.ID, PlanCode: "pro", Currency: "UZS", Interval: "month", Trial: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if sub.Status != Trialing {
		t.Fatalf("status = %s, want trialing", sub.Status)
	}

	var historyN, outboxN int
	_ = tx.QueryRow(ctx, `SELECT count(*) FROM subscription_status_history WHERE subscription_id=$1`, sub.ID).Scan(&historyN)
	_ = tx.QueryRow(ctx, `SELECT count(*) FROM webhook_outbox WHERE product_id=$1`, pid).Scan(&outboxN)
	if historyN != 1 {
		t.Fatalf("history rows = %d, want 1", historyN)
	}
	if outboxN != 1 {
		t.Fatalf("outbox rows = %d, want 1 (subscription.created)", outboxN)
	}
}
