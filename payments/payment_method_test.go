package payments

import (
	"encoding/base64"
	"testing"

	"billing/base"
	"billing/catalog"
	"billing/customers"
	"billing/testdb"
)

func testKey() string { return base64.StdEncoding.EncodeToString(make([]byte, 32)) }

func TestSaveAndResolveDefaultToken(t *testing.T) {
	ctx, tx := testdb.TxOrSkip(t)
	pid, _ := catalog.NewRepo().CreateProduct(ctx, tx, "cm", "CM", "k")
	cust, _ := customers.NewRepo().Register(ctx, tx, pid, "shopA", "u1", "Shop A", "UZS")

	cipher, err := base.NewCipher(testKey())
	if err != nil {
		t.Fatal(err)
	}
	pm := NewPaymentMethodRepo(cipher)

	id, err := pm.SaveCard(ctx, tx, cust.ID, "payme", "tok_123", "8600****1234", "0530")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := pm.DefaultVerifiedToken(ctx, tx, cust.ID); ok {
		t.Fatal("unverified should not resolve")
	}
	if err := pm.MarkVerifiedDefault(ctx, tx, id, cust.ID); err != nil {
		t.Fatal(err)
	}
	tok, ok, err := pm.DefaultVerifiedToken(ctx, tx, cust.ID)
	if err != nil || !ok || tok != "tok_123" {
		t.Fatalf("resolve = %q,%v,%v", tok, ok, err)
	}
}
