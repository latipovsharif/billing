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
