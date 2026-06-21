package kaspi

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"billing/catalog"
	"billing/customers"
	"billing/subscriptions"
	"billing/testdb"

	"github.com/gin-gonic/gin"
)

type fakeQRMaker struct{ n int }

func (f *fakeQRMaker) CreateQR(_ context.Context, _ string, _ int64) (string, string, error) {
	f.n++
	return fmt.Sprintf("QI-%d", f.n), "tok", nil
}

func TestCreateQREndpointIdempotent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	pool := testdb.PoolOrSkip(t)
	ctx := context.Background()

	cr := catalog.NewRepo()
	var pid int64
	_ = pool.QueryRow(ctx, `INSERT INTO product (key,name,api_key) VALUES ('cmk','CM','kk') RETURNING id`).Scan(&pid)
	planID, _ := cr.CreatePlan(ctx, pool, pid, "pro", "Pro", map[string]any{}, 14)
	_ = cr.AddPrice(ctx, pool, planID, "KZT", "month", 5000)
	cust, _ := customers.NewRepo().Register(ctx, pool, pid, "shopk", "u1", "Shop", "KZT")
	sub, _ := subscriptions.NewService().Create(ctx, pool, pid, subscriptions.CreateInput{
		CustomerID: cust.ID, PlanCode: "pro", Currency: "KZT", Interval: "month", Trial: true,
	})
	var invID int64
	_ = pool.QueryRow(ctx,
		`INSERT INTO invoice (subscription_id, customer_id, currency, amount, status, period_start, period_end, due_date)
		 VALUES ($1,$2,'KZT',5000,'open',now(),now()+interval '1 month',now()) RETURNING id`,
		sub.ID, cust.ID).Scan(&invID)
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM kaspi_payment WHERE invoice_id=$1`, invID)
		_, _ = pool.Exec(ctx, `DELETE FROM invoice WHERE id=$1`, invID)
		_, _ = pool.Exec(ctx, `DELETE FROM subscription WHERE id=$1`, sub.ID)
		_, _ = pool.Exec(ctx, `DELETE FROM customer WHERE id=$1`, cust.ID)
		_, _ = pool.Exec(ctx, `DELETE FROM plan WHERE id=$1`, planID)
		_, _ = pool.Exec(ctx, `DELETE FROM product WHERE id=$1`, pid)
	})

	maker := &fakeQRMaker{}
	r := gin.New()
	v1 := r.Group("/v1")
	GetRoutes(v1, NewController(pool, maker, NewRepo()))

	call := func() int {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/invoices/%d/kaspi-qr", invID), nil))
		return w.Code
	}
	if code := call(); code != http.StatusOK {
		t.Fatalf("first create: %d", code)
	}
	if code := call(); code != http.StatusOK {
		t.Fatalf("second create: %d", code)
	}
	if maker.n != 1 {
		t.Fatalf("CreateQR called %d times, want 1 (idempotent reuse)", maker.n)
	}
}
