package webhooks

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"billing/catalog"
	"billing/testdb"
)

func TestDispatcherDeliversAndSigns(t *testing.T) {
	ctx, tx := testdb.TxOrSkip(t)
	var received atomic.Int32
	var gotSig string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSig = r.Header.Get("X-Billing-Signature")
		_, _ = io.ReadAll(r.Body)
		received.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	pid, _ := catalog.NewRepo().CreateProduct(ctx, tx, "cm", "CM", "k")
	_, _ = tx.Exec(ctx, `INSERT INTO webhook_endpoint (product_id, url, secret) VALUES ($1,$2,'sek')`, pid, srv.URL)
	_ = NewOutbox().Enqueue(ctx, tx, pid, "subscription.created", Payload{CustomerID: 1, SubscriptionID: 1, Status: "trialing"})

	d := NewDispatcher(srv.Client())
	n, err := d.DeliverBatch(ctx, tx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 || received.Load() != 1 {
		t.Fatalf("delivered n=%d received=%d, want 1/1", n, received.Load())
	}
	if gotSig == "" {
		t.Fatal("missing signature header")
	}
}
