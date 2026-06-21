package payme

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// stub returns a Payme JSON-RPC server that dispatches by method.
func stub(t *testing.T, handlers map[string]func() any) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Method string `json:"method"`
			ID     int64  `json:"id"`
		}
		_ = json.Unmarshal(body, &req)
		h, ok := handlers[req.Method]
		if !ok {
			t.Errorf("unexpected method %s", req.Method)
			http.Error(w, "no method", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": req.ID, "result": h()})
	}))
}

func TestCreateAndVerifyCard(t *testing.T) {
	srv := stub(t, map[string]func() any{
		"cards.create":          func() any { return map[string]any{"card": map[string]any{"token": "tok_1", "number": "8600 **** **** 1234"}} },
		"cards.get_verify_code": func() any { return map[string]any{"sent": true} },
		"cards.verify":          func() any { return map[string]any{"card": map[string]any{"token": "tok_1", "verify": true}} },
	})
	defer srv.Close()

	c := NewClient(srv.URL, "mid", "key", srv.Client())
	token, masked, err := c.CreateCard(context.Background(), "8600123412341234", "0530")
	if err != nil || token != "tok_1" {
		t.Fatalf("create: %q %q %v", token, masked, err)
	}
	if !strings.Contains(masked, "1234") {
		t.Fatalf("masked = %q", masked)
	}
	if err := c.SendVerifyCode(context.Background(), token); err != nil {
		t.Fatal(err)
	}
	if err := c.VerifyCard(context.Background(), token, "666666"); err != nil {
		t.Fatal(err)
	}
}

func TestReceiptsPayChargesToken(t *testing.T) {
	srv := stub(t, map[string]func() any{
		"receipts.create": func() any { return map[string]any{"receipt": map[string]any{"_id": "rcp_1"}} },
		"receipts.pay":    func() any { return map[string]any{"receipt": map[string]any{"_id": "rcp_1", "state": float64(4)}} },
	})
	defer srv.Close()

	c := NewClient(srv.URL, "mid", "key", srv.Client())
	ref, err := c.Charge(context.Background(), 99, 500000, "tok_1")
	if err != nil || ref != "rcp_1" {
		t.Fatalf("charge: %q %v", ref, err)
	}
}

func TestPaymeErrorMapped(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"id": 1, "error": map[string]any{"code": -31630, "message": "card not found"}})
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "mid", "key", srv.Client())
	if _, err := c.Charge(context.Background(), 1, 1, "tok"); err == nil {
		t.Fatal("expected payme error")
	}
}
