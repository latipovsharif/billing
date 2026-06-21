package payments

import (
	"testing"
	"time"

	"billing/base"
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

	svc := NewService(NewManual(), nil)
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

func TestChargeInvoiceNoMethod(t *testing.T) {
	ctx, tx := testdb.TxOrSkip(t)
	cr := catalog.NewRepo()
	pid, _ := cr.CreateProduct(ctx, tx, "cm", "CM", "k")
	planID, _ := cr.CreatePlan(ctx, tx, pid, "pro", "Pro", map[string]any{}, 14)
	_ = cr.AddPrice(ctx, tx, planID, "UZS", "month", 5000)
	cust, _ := customers.NewRepo().Register(ctx, tx, pid, "shopA", "u1", "Shop A", "UZS")
	sub, _ := subscriptions.NewService().Create(ctx, tx, pid, subscriptions.CreateInput{
		CustomerID: cust.ID, PlanCode: "pro", Currency: "UZS", Interval: "month", Trial: true,
	})
	var invID int64
	now := time.Now()
	_ = tx.QueryRow(ctx,
		`INSERT INTO invoice (subscription_id, customer_id, currency, amount, status, period_start, period_end, due_date)
		 VALUES ($1,$2,'UZS',5000,'open',$3,$4,$4) RETURNING id`,
		sub.ID, cust.ID, now, now.AddDate(0, 1, 0)).Scan(&invID)

	svc := NewService(NewManual(), nil)
	if err := svc.ChargeInvoice(ctx, tx, pid, invID); err != ErrNoPaymentMethod {
		t.Fatalf("err = %v, want ErrNoPaymentMethod", err)
	}
}

func TestChargeInvoiceWithTokenSettles(t *testing.T) {
	ctx, tx := testdb.TxOrSkip(t)
	cr := catalog.NewRepo()
	pid, _ := cr.CreateProduct(ctx, tx, "cm", "CM", "k")
	planID, _ := cr.CreatePlan(ctx, tx, pid, "pro", "Pro", map[string]any{}, 14)
	_ = cr.AddPrice(ctx, tx, planID, "UZS", "month", 5000)
	cust, _ := customers.NewRepo().Register(ctx, tx, pid, "shopA", "u1", "Shop A", "UZS")
	sub, _ := subscriptions.NewService().Create(ctx, tx, pid, subscriptions.CreateInput{
		CustomerID: cust.ID, PlanCode: "pro", Currency: "UZS", Interval: "month", Trial: true,
	})
	_, _ = tx.Exec(ctx, `UPDATE subscription SET status='past_due' WHERE id=$1`, sub.ID)
	var invID int64
	now := time.Now()
	_ = tx.QueryRow(ctx,
		`INSERT INTO invoice (subscription_id, customer_id, currency, amount, status, period_start, period_end, due_date)
		 VALUES ($1,$2,'UZS',5000,'open',$3,$4,$4) RETURNING id`,
		sub.ID, cust.ID, now, now.AddDate(0, 1, 0)).Scan(&invID)

	cipher, _ := base.NewCipher(testKey())
	pm := NewPaymentMethodRepo(cipher)
	id, _ := pm.SaveCard(ctx, tx, cust.ID, "manual", "tok", "8600****1", "0530")
	_ = pm.MarkVerifiedDefault(ctx, tx, id, cust.ID)

	svc := NewService(NewManual(), pm)
	if err := svc.ChargeInvoice(ctx, tx, pid, invID); err != nil {
		t.Fatal(err)
	}
	if s, _, _ := subscriptions.NewRepo().Get(ctx, tx, sub.ID); s.Status != subscriptions.Active {
		t.Fatalf("status = %s, want active", s.Status)
	}
}

func TestSettleExternalActivates(t *testing.T) {
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
	now := time.Now()
	_ = tx.QueryRow(ctx,
		`INSERT INTO invoice (subscription_id, customer_id, currency, amount, status, period_start, period_end, due_date)
		 VALUES ($1,$2,'KZT',5000,'open',$3,$4,$4) RETURNING id`,
		sub.ID, cust.ID, now, now.AddDate(0, 1, 0)).Scan(&invID)

	svc := NewService(NewManual(), nil)
	if err := svc.SettleExternal(ctx, tx, pid, invID, "kaspi", "qr-1", map[string]any{"src": "kaspi"}); err != nil {
		t.Fatal(err)
	}
	if s, _, _ := subscriptions.NewRepo().Get(ctx, tx, sub.ID); s.Status != subscriptions.Active {
		t.Fatalf("status = %s, want active", s.Status)
	}
}
