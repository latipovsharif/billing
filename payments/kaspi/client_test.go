package kaspi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateQRAndPaymentInfo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/qr/create":
			if r.Header.Get("Authorization") != "Bearer kkey" {
				t.Errorf("missing api key: %q", r.Header.Get("Authorization"))
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"qr_invoice_id": "QI-1", "qr_token": "00020101..."})
		case "/qr/status":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "Wait"})
		default:
			http.Error(w, "no path", 404)
		}
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "kkey", "dev-1", "BIN-1", srv.Client())
	qi, tok, err := c.CreateQR(context.Background(), "inv-7", 5000)
	if err != nil || qi != "QI-1" || tok == "" {
		t.Fatalf("create: %q %q %v", qi, tok, err)
	}
	st, err := c.PaymentInfo(context.Background(), "QI-1")
	if err != nil || st != "pending" {
		t.Fatalf("status = %q %v, want pending", st, err)
	}
}

func TestPaymentInfoMapsPaid(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "Processed"})
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "k", "d", "b", srv.Client())
	st, _ := c.PaymentInfo(context.Background(), "QI-1")
	if st != "paid" {
		t.Fatalf("status = %q, want paid", st)
	}
}
