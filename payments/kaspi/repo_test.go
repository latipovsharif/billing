package kaspi

import (
	"testing"

	"billing/catalog"
	"billing/customers"
	"billing/subscriptions"
	"billing/testdb"
)

func TestKaspiRepoUpsertAndClaim(t *testing.T) {
	ctx, tx := testdb.TxOrSkip(t)
	cr := catalog.NewRepo()
	pid, _ := cr.CreateProduct(ctx, tx, "cm", "CM", "k")
	planID, _ := cr.CreatePlan(ctx, tx, pid, "pro", "Pro", map[string]any{}, 14)
	_ = cr.AddPrice(ctx, tx, planID, "KZT", "month", 5000)
	cust, _ := customers.NewRepo().Register(ctx, tx, pid, "shopA", "u1", "Shop A", "KZT")
	sub, _ := subscriptions.NewService().Create(ctx, tx, pid, subscriptions.CreateInput{
		CustomerID: cust.ID, PlanCode: "pro", Currency: "KZT", Interval: "month", Trial: true,
	})
	var invID int64
	_ = tx.QueryRow(ctx,
		`INSERT INTO invoice (subscription_id, customer_id, currency, amount, status, period_start, period_end, due_date)
		 VALUES ($1,$2,'KZT',5000,'open',now(),now()+interval '1 month',now()) RETURNING id`,
		sub.ID, cust.ID).Scan(&invID)

	r := NewRepo()
	amount, productID, status, ok, err := r.InvoiceForQR(ctx, tx, invID)
	if err != nil || !ok || amount != 5000 || productID != pid || status != "open" {
		t.Fatalf("InvoiceForQR = %d,%d,%q,%v,%v", amount, productID, status, ok, err)
	}
	if err := r.Save(ctx, tx, invID, "QI-1", "tok"); err != nil {
		t.Fatal(err)
	}
	qi, tok, ok, err := r.PendingByInvoice(ctx, tx, invID)
	if err != nil || !ok || qi != "QI-1" || tok != "tok" {
		t.Fatalf("PendingByInvoice = %q,%q,%v,%v", qi, tok, ok, err)
	}
	claims, err := r.ClaimPending(ctx, tx)
	if err != nil || len(claims) != 1 || claims[0].QRInvoiceID != "QI-1" || claims[0].ProductID != pid {
		t.Fatalf("ClaimPending = %+v, %v", claims, err)
	}
	if err := r.Mark(ctx, tx, claims[0].ID, "paid"); err != nil {
		t.Fatal(err)
	}
	if _, _, ok, _ := r.PendingByInvoice(ctx, tx, invID); ok {
		t.Fatal("expected no pending after Mark paid")
	}
}
