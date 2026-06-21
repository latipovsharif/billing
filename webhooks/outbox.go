package webhooks

import (
	"context"
	"encoding/json"

	"billing/base"

	"github.com/google/uuid"
)

// Payload is the data envelope embedded in every event.
type Payload struct {
	CustomerID     int64  `json:"customer_id"`
	SubscriptionID int64  `json:"subscription_id"`
	Status         string `json:"status"`
	PlanCode       string `json:"plan_code,omitempty"`
}

// Outbox enqueues events transactionally with domain writes.
type Outbox struct{}

func NewOutbox() *Outbox { return &Outbox{} }

// Enqueue inserts a pending event row in the caller's tx.
func (o *Outbox) Enqueue(ctx context.Context, db base.PGXDB, productID int64, eventType string, p Payload) error {
	eventID := uuid.NewString()
	body, err := json.Marshal(map[string]any{
		"event_id": eventID,
		"type":     eventType,
		"data":     p,
	})
	if err != nil {
		return err
	}
	_, err = db.Exec(ctx,
		`INSERT INTO webhook_outbox (product_id, event_id, event_type, payload)
		 VALUES ($1,$2,$3,$4)`, productID, eventID, eventType, body)
	return err
}
