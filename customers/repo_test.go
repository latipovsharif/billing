package customers

import (
	"testing"

	"billing/catalog"
	"billing/testdb"
)

func TestRegisterIsIdempotent(t *testing.T) {
	ctx, tx := testdb.TxOrSkip(t)
	pid, err := catalog.NewRepo().CreateProduct(ctx, tx, "cm", "CM", "k")
	if err != nil {
		t.Fatal(err)
	}
	r := NewRepo()

	c1, err := r.Register(ctx, tx, pid, "shopA", "user-1", "Shop A", "UZS")
	if err != nil {
		t.Fatal(err)
	}
	c2, err := r.Register(ctx, tx, pid, "shopA", "user-1", "Shop A", "UZS")
	if err != nil {
		t.Fatal(err)
	}
	if c1.ID != c2.ID {
		t.Fatalf("expected same customer id, got %d and %d", c1.ID, c2.ID)
	}
}
