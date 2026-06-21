package payments

import (
	"context"
	"net/http"
	"strconv"

	"billing/base"
	"billing/middleware"
	"billing/response"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

// pool can both begin a tx and run queries (a *pgxpool.Pool).
type pool interface {
	base.Beginner
	base.PGXDB
}

type Controller struct {
	pool pool
	svc  *Service
}

func NewController(p pool) *Controller {
	return &Controller{pool: p, svc: NewService(NewManual(), nil)}
}

// MarkPaid POST /v1/admin/invoices/:id/mark-paid
func (ct *Controller) MarkPaid(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	pid := middleware.ProductID(c)
	err := base.WithTx(c.Request.Context(), ct.pool, func(tx pgx.Tx) error {
		return ct.svc.MarkPaid(c.Request.Context(), tx, pid, id, "operator")
	})
	if err != nil {
		response.Err(c, http.StatusBadRequest, "mark_paid_failed", err.Error())
		return
	}
	response.OK(c, http.StatusOK, gin.H{"status": "paid"})
}

// CardBinder is the provider-side card API the card controller needs.
// Satisfied by *payme.Client.
type CardBinder interface {
	CreateCard(ctx context.Context, number, expire string) (token, masked string, err error)
	SendVerifyCode(ctx context.Context, token string) error
	VerifyCard(ctx context.Context, token, code string) error
	RemoveCard(ctx context.Context, token string) error
}

// CardController exposes card binding over HTTP. PAN is never logged or stored.
type CardController struct {
	pool    pool
	binder  CardBinder
	methods *PaymentMethodRepo
}

func NewCardController(p pool, binder CardBinder, methods *PaymentMethodRepo) *CardController {
	return &CardController{pool: p, binder: binder, methods: methods}
}

type addCardReq struct {
	CustomerID int64  `json:"customer_id" binding:"required"`
	CardNumber string `json:"card_number" binding:"required"`
	Expire     string `json:"expire" binding:"required"`
}

// AddCard POST /v1/payment-methods/card — tokenize + request SMS code.
func (ct *CardController) AddCard(c *gin.Context) {
	var req addCardReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Err(c, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	ctx := c.Request.Context()
	token, masked, err := ct.binder.CreateCard(ctx, req.CardNumber, req.Expire)
	if err != nil {
		response.Err(c, http.StatusBadGateway, "card_create_failed", err.Error())
		return
	}
	if err := ct.binder.SendVerifyCode(ctx, token); err != nil {
		response.Err(c, http.StatusBadGateway, "verify_code_failed", err.Error())
		return
	}
	var id int64
	err = base.WithTx(ctx, ct.pool, func(tx pgx.Tx) error {
		var e error
		id, e = ct.methods.SaveCard(ctx, tx, req.CustomerID, "payme", token, masked, req.Expire)
		return e
	})
	if err != nil {
		response.Err(c, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	response.OK(c, http.StatusOK, gin.H{"payment_method_id": id, "card_masked": masked, "needs_verification": true})
}

type verifyCardReq struct {
	Code string `json:"code" binding:"required"`
}

// VerifyCard POST /v1/payment-methods/:id/verify
func (ct *CardController) VerifyCard(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req verifyCardReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Err(c, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	ctx := c.Request.Context()
	token, customerID, ok, err := ct.methods.TokenByID(ctx, ct.pool, id)
	if err != nil || !ok {
		response.Err(c, http.StatusNotFound, "not_found", "card not found")
		return
	}
	if err := ct.binder.VerifyCard(ctx, token, req.Code); err != nil {
		response.Err(c, http.StatusBadGateway, "verify_failed", err.Error())
		return
	}
	if err := base.WithTx(ctx, ct.pool, func(tx pgx.Tx) error {
		return ct.methods.MarkVerifiedDefault(ctx, tx, id, customerID)
	}); err != nil {
		response.Err(c, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	response.OK(c, http.StatusOK, gin.H{"verified": true})
}

// ListCards GET /v1/payment-methods?customer_id=
func (ct *CardController) ListCards(c *gin.Context) {
	cid, _ := strconv.ParseInt(c.Query("customer_id"), 10, 64)
	cards, err := ct.methods.List(c.Request.Context(), ct.pool, cid)
	if err != nil {
		response.Err(c, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	response.OK(c, http.StatusOK, gin.H{"cards": cards})
}

// RemoveCard DELETE /v1/payment-methods/:id
func (ct *CardController) RemoveCard(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	ctx := c.Request.Context()
	token, _, ok, err := ct.methods.TokenByID(ctx, ct.pool, id)
	if err != nil {
		response.Err(c, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if ok {
		_ = ct.binder.RemoveCard(ctx, token)
	}
	if err := ct.methods.Delete(ctx, ct.pool, id); err != nil {
		response.Err(c, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	response.OK(c, http.StatusOK, gin.H{"removed": true})
}
