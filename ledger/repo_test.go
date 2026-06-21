package ledger

import (
	"testing"

	"billing/catalog"
	"billing/customers"
	"billing/testdb"
)

func TestAppendAndBalance(t *testing.T) {
	ctx, tx := testdb.TxOrSkip(t)
	pid, _ := catalog.NewRepo().CreateProduct(ctx, tx, "cm", "CM", "k")
	cust, _ := customers.NewRepo().Register(ctx, tx, pid, "shopA", "u1", "Shop A", "UZS")

	r := NewRepo()
	if _, err := r.Append(ctx, tx, Entry{CustomerID: cust.ID, Type: Charge, Currency: "UZS", Amount: -5000}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Append(ctx, tx, Entry{CustomerID: cust.ID, Type: Payment, Currency: "UZS", Amount: 5000}); err != nil {
		t.Fatal(err)
	}
	bal, err := r.Balance(ctx, tx, cust.ID, "UZS")
	if err != nil {
		t.Fatal(err)
	}
	if bal != 0 {
		t.Fatalf("balance = %d, want 0", bal)
	}
}
