package jobs

import (
	"context"
	"testing"

	"billing/base"
	"billing/catalog"
	"billing/customers"
	"billing/subscriptions"
	"billing/testdb"
)

// fakeCharger settles by marking the invoice paid + moving the sub to active,
// mimicking a successful payments.Service.ChargeInvoice (without Payme).
type fakeCharger struct{ called int }

func (f *fakeCharger) ChargeInvoice(ctx context.Context, db base.PGXDB, productID, invoiceID int64) error {
	f.called++
	if _, err := db.Exec(ctx, `UPDATE invoice SET status='paid' WHERE id=$1`, invoiceID); err != nil {
		return err
	}
	var subID int64
	if err := db.QueryRow(ctx, `SELECT subscription_id FROM invoice WHERE id=$1`, invoiceID).Scan(&subID); err != nil {
		return err
	}
	_, err := db.Exec(ctx, `UPDATE subscription SET status='active' WHERE id=$1`, subID)
	return err
}

func seedPastDueSub(t *testing.T, ctx context.Context, tx base.PGXDB) (pid, custID, subID int64) {
	t.Helper()
	cr := catalog.NewRepo()
	pid, _ = cr.CreateProduct(ctx, tx, "cm", "CM", "k")
	planID, _ := cr.CreatePlan(ctx, tx, pid, "pro", "Pro", map[string]any{}, 14)
	_ = cr.AddPrice(ctx, tx, planID, "UZS", "month", 5000)
	cust, _ := customers.NewRepo().Register(ctx, tx, pid, "shopA", "u1", "Shop A", "UZS")
	sub, _ := subscriptions.NewService().Create(ctx, tx, pid, subscriptions.CreateInput{
		CustomerID: cust.ID, PlanCode: "pro", Currency: "UZS", Interval: "month", Trial: true,
	})
	_, _ = tx.Exec(ctx, `UPDATE subscription SET status='past_due' WHERE id=$1`, sub.ID)
	return pid, cust.ID, sub.ID
}

func TestDunningChargesPastDueWithCard(t *testing.T) {
	ctx, tx := testdb.TxOrSkip(t)
	_, custID, subID := seedPastDueSub(t, ctx, tx)
	_, _ = tx.Exec(ctx,
		`INSERT INTO payment_method (customer_id, provider, token_enc, card_masked, expire, verified, is_default)
		 VALUES ($1,'payme',$2,'8600****1','0530',TRUE,TRUE)`, custID, []byte("x"))

	fc := &fakeCharger{}
	if err := NewRunner(3, fc).DunningCharge(ctx, tx); err != nil {
		t.Fatal(err)
	}
	if fc.called != 1 {
		t.Fatalf("charger called %d times, want 1", fc.called)
	}
	if s, _, _ := subscriptions.NewRepo().Get(ctx, tx, subID); s.Status != subscriptions.Active {
		t.Fatalf("status = %s, want active", s.Status)
	}
}

func TestDunningSkipsWithoutCard(t *testing.T) {
	ctx, tx := testdb.TxOrSkip(t)
	_, _, _ = seedPastDueSub(t, ctx, tx)
	fc := &fakeCharger{}
	if err := NewRunner(3, fc).DunningCharge(ctx, tx); err != nil {
		t.Fatal(err)
	}
	if fc.called != 0 {
		t.Fatalf("charger called %d, want 0 (no card)", fc.called)
	}
}
