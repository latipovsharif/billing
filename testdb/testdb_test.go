package testdb

import "testing"

func TestTxOrSkipConnects(t *testing.T) {
	ctx, tx := TxOrSkip(t)
	var one int
	if err := tx.QueryRow(ctx, "SELECT 1").Scan(&one); err != nil {
		t.Fatal(err)
	}
	if one != 1 {
		t.Fatalf("got %d", one)
	}
}
