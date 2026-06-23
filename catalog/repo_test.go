package catalog

import (
	"testing"

	"billing/testdb"
)

func TestCreatePlanWithPriceAndList(t *testing.T) {
	ctx, tx := testdb.TxOrSkip(t)
	r := NewRepo()

	prodID, err := r.CreateProduct(ctx, tx, "cloudmarket", "CloudMarket", "test-key")
	if err != nil {
		t.Fatal(err)
	}
	planID, err := r.CreatePlan(ctx, tx, prodID, "pro", "Pro", map[string]any{"max_users": 10}, 14)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.AddPrice(ctx, tx, planID, "UZS", "month", 5000000); err != nil {
		t.Fatal(err)
	}

	plans, err := r.ListPlansWithPrices(ctx, tx, prodID)
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != 1 || len(plans[0].Prices) != 1 {
		t.Fatalf("got %+v", plans)
	}
	if plans[0].Prices[0].Amount != 5000000 || plans[0].Prices[0].Currency != "UZS" {
		t.Fatalf("bad price: %+v", plans[0].Prices[0])
	}
}

// Provisioning must be re-runnable: re-upserting the same plan code / price
// (currency, interval) updates in place instead of erroring on the unique keys.
func TestUpsertPlanAndPriceIdempotent(t *testing.T) {
	ctx, tx := testdb.TxOrSkip(t)
	r := NewRepo()

	prodID, err := r.CreateProduct(ctx, tx, "cm-idem", "CM", "key-idem")
	if err != nil {
		t.Fatal(err)
	}

	id1, err := r.UpsertPlan(ctx, tx, prodID, "trial", "Trial", map[string]any{"x": 1}, 14)
	if err != nil {
		t.Fatal(err)
	}
	id2, err := r.UpsertPlan(ctx, tx, prodID, "trial", "Trial v2", nil, 7) // same code -> same row, updated
	if err != nil {
		t.Fatal(err)
	}
	if id1 != id2 {
		t.Fatalf("upsert created a new plan: %d != %d", id1, id2)
	}

	pr1, err := r.UpsertPrice(ctx, tx, id1, "UZS", "month", 1000)
	if err != nil {
		t.Fatal(err)
	}
	pr2, err := r.UpsertPrice(ctx, tx, id1, "UZS", "month", 2000) // same (cur,interval) -> same row, new amount
	if err != nil {
		t.Fatal(err)
	}
	if pr1 != pr2 {
		t.Fatalf("upsert created a new price: %d != %d", pr1, pr2)
	}

	plans, err := r.ListPlansWithPrices(ctx, tx, prodID)
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != 1 || plans[0].Name != "Trial v2" || plans[0].TrialDays != 7 {
		t.Fatalf("plan not updated in place: %+v", plans)
	}
	if len(plans[0].Prices) != 1 || plans[0].Prices[0].Amount != 2000 {
		t.Fatalf("price not updated in place: %+v", plans[0].Prices)
	}

	// PlanByID enforces product ownership.
	if _, ok, _ := r.PlanByID(ctx, tx, prodID, id1); !ok {
		t.Fatal("PlanByID should find own plan")
	}
	if _, ok, _ := r.PlanByID(ctx, tx, prodID+999, id1); ok {
		t.Fatal("PlanByID must not cross product boundary")
	}
}
