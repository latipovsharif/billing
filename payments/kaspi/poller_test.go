package kaspi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"billing/catalog"
	"billing/customers"
	"billing/payments"
	"billing/subscriptions"
	"billing/testdb"
)

func TestPollerSettlesPaid(t *testing.T) {
	ctx, tx := testdb.TxOrSkip(t)
	cr := catalog.NewRepo()
	pid, _ := cr.CreateProduct(ctx, tx, "cm", "CM", "k")
	planID, _ := cr.CreatePlan(ctx, tx, pid, "pro", "Pro", map[string]any{}, 14)
	_ = cr.AddPrice(ctx, tx, planID, "KZT", "month", 5000)
	cust, _ := customers.NewRepo().Register(ctx, tx, pid, "shopA", "u1", "Shop A", "KZT")
	sub, _ := subscriptions.NewService().Create(ctx, tx, pid, subscriptions.CreateInput{
		CustomerID: cust.ID, PlanCode: "pro", Currency: "KZT", Interval: "month", Trial: true,
	})
	_, _ = tx.Exec(ctx, `UPDATE subscription SET status='past_due' WHERE id=$1`, sub.ID)
	var invID int64
	_ = tx.QueryRow(ctx,
		`INSERT INTO invoice (subscription_id, customer_id, currency, amount, status, period_start, period_end, due_date)
		 VALUES ($1,$2,'KZT',5000,'open',now(),now()+interval '1 month',now()) RETURNING id`,
		sub.ID, cust.ID).Scan(&invID)
	_ = NewRepo().Save(ctx, tx, invID, "QI-1", "tok")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":"Processed"}`))
	}))
	defer srv.Close()
	client := NewClient(srv.URL, "k", "d", "b", srv.Client())

	p := NewPoller(client, NewRepo(), payments.NewService(payments.NewManual(), nil))
	if err := p.Poll(ctx, tx); err != nil {
		t.Fatal(err)
	}
	if s, _, _ := subscriptions.NewRepo().Get(ctx, tx, sub.ID); s.Status != subscriptions.Active {
		t.Fatalf("status = %s, want active", s.Status)
	}
	if _, _, ok, _ := NewRepo().PendingByInvoice(ctx, tx, invID); ok {
		t.Fatal("expected kaspi_payment no longer pending")
	}
}
