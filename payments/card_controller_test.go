package payments

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"billing/base"
	"billing/testdb"

	"github.com/gin-gonic/gin"
)

type fakeBinder struct{}

func (fakeBinder) CreateCard(_ context.Context, _, _ string) (string, string, error) {
	return "tok_1", "8600****1234", nil
}
func (fakeBinder) SendVerifyCode(context.Context, string) error    { return nil }
func (fakeBinder) VerifyCard(context.Context, string, string) error { return nil }
func (fakeBinder) RemoveCard(context.Context, string) error        { return nil }

func TestAddAndVerifyCardHTTP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	pool := testdb.PoolOrSkip(t)
	ctx := context.Background()

	var pid, cid int64
	if err := pool.QueryRow(ctx, `INSERT INTO product (key,name,api_key) VALUES ('cmcard','CM','kcard') RETURNING id`).Scan(&pid); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO customer (product_id, external_ref, owner_user_id, display_name, default_currency)
		 VALUES ($1,'shopcard','u1','Shop','UZS') RETURNING id`, pid).Scan(&cid); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM payment_method WHERE customer_id=$1`, cid)
		_, _ = pool.Exec(ctx, `DELETE FROM customer WHERE id=$1`, cid)
		_, _ = pool.Exec(ctx, `DELETE FROM product WHERE id=$1`, pid)
	})

	cipher, _ := base.NewCipher(testKey())
	pm := NewPaymentMethodRepo(cipher)
	ctrl := NewCardController(pool, fakeBinder{}, pm)

	r := gin.New()
	v1 := r.Group("/v1")
	GetCardRoutes(v1, ctrl)

	body, _ := json.Marshal(map[string]any{"customer_id": cid, "card_number": "8600123412341234", "expire": "0530"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/payment-methods/card", bytes.NewReader(body)))
	if w.Code != http.StatusOK {
		t.Fatalf("add card: %d %s", w.Code, w.Body.String())
	}
	var added struct {
		PaymentMethodID int64 `json:"payment_method_id"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &added)
	if added.PaymentMethodID == 0 {
		t.Fatal("no payment_method_id")
	}

	vb, _ := json.Marshal(map[string]any{"code": "666666"})
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/payment-methods/%d/verify", added.PaymentMethodID), bytes.NewReader(vb)))
	if w.Code != http.StatusOK {
		t.Fatalf("verify: %d %s", w.Code, w.Body.String())
	}

	tok, ok, _ := pm.DefaultVerifiedToken(ctx, pool, cid)
	if !ok || tok != "tok_1" {
		t.Fatalf("default token = %q ok=%v", tok, ok)
	}
}
