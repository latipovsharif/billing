package payments

import (
	"testing"
	"time"

	"billing/catalog"
	"billing/customers"
	"billing/subscriptions"
	"billing/testdb"
)

func TestMarkPaidActivatesAndRecordsLedger(t *testing.T) {
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
	var invID int64
	now := time.Now()
	_ = tx.QueryRow(ctx,
		`INSERT INTO invoice (subscription_id, customer_id, currency, amount, status, period_start, period_end, due_date)
		 VALUES ($1,$2,'UZS',5000,'open',$3,$4,$4) RETURNING id`,
		sub.ID, cust.ID, now, now.AddDate(0, 1, 0)).Scan(&invID)

	svc := NewService(NewManual())
	if err := svc.MarkPaid(ctx, tx, pid, invID, "operator"); err != nil {
		t.Fatal(err)
	}

	got, _, _ := subscriptions.NewRepo().Get(ctx, tx, sub.ID)
	if got.Status != subscriptions.Active {
		t.Fatalf("status = %s, want active", got.Status)
	}
	var payN, ledN int
	_ = tx.QueryRow(ctx, `SELECT count(*) FROM payment WHERE invoice_id=$1`, invID).Scan(&payN)
	_ = tx.QueryRow(ctx, `SELECT count(*) FROM ledger_entry WHERE customer_id=$1 AND type='payment'`, cust.ID).Scan(&ledN)
	if payN != 1 || ledN != 1 {
		t.Fatalf("payN=%d ledN=%d, want 1/1", payN, ledN)
	}
}
