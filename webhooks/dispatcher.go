package webhooks

import (
	"bytes"
	"context"
	"net/http"
	"time"

	"billing/base"
)

// Dispatcher delivers pending outbox rows to product endpoints.
type Dispatcher struct{ client *http.Client }

func NewDispatcher(c *http.Client) *Dispatcher {
	if c == nil {
		c = &http.Client{Timeout: 10 * time.Second}
	}
	return &Dispatcher{client: c}
}

type due struct {
	id       int64
	url      string
	secret   string
	payload  []byte
	attempts int
}

// DeliverBatch claims up to limit pending rows (FOR UPDATE SKIP LOCKED),
// POSTs each with an HMAC signature, and marks delivered/failed with backoff.
// Returns the number delivered successfully.
func (d *Dispatcher) DeliverBatch(ctx context.Context, db base.PGXDB, limit int) (int, error) {
	rows, err := db.Query(ctx,
		`SELECT o.id, e.url, e.secret, o.payload, o.attempts
		 FROM webhook_outbox o
		 JOIN webhook_endpoint e ON e.product_id=o.product_id AND e.active
		 WHERE o.status='pending' AND o.next_attempt_at <= now()
		 ORDER BY o.id
		 LIMIT $1
		 FOR UPDATE OF o SKIP LOCKED`, limit)
	if err != nil {
		return 0, err
	}
	var batch []due
	for rows.Next() {
		var x due
		if err := rows.Scan(&x.id, &x.url, &x.secret, &x.payload, &x.attempts); err != nil {
			rows.Close()
			return 0, err
		}
		batch = append(batch, x)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	delivered := 0
	for _, x := range batch {
		if d.post(ctx, x) {
			if _, err := db.Exec(ctx,
				`UPDATE webhook_outbox SET status='delivered', delivered_at=now(), attempts=attempts+1 WHERE id=$1`, x.id); err != nil {
				return delivered, err
			}
			delivered++
		} else {
			// exponential backoff in seconds, capped; fail after 12 attempts.
			backoffSecs := 60 * (1 << min(x.attempts, 10))
			if _, err := db.Exec(ctx,
				`UPDATE webhook_outbox
				 SET attempts=attempts+1,
				     next_attempt_at=now() + make_interval(secs => $2),
				     status=CASE WHEN attempts+1 >= 12 THEN 'failed' ELSE 'pending' END
				 WHERE id=$1`, x.id, backoffSecs); err != nil {
				return delivered, err
			}
		}
	}
	return delivered, nil
}

func (d *Dispatcher) post(ctx context.Context, x due) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, x.url, bytes.NewReader(x.payload))
	if err != nil {
		return false
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Billing-Signature", Sign(x.secret, x.payload))
	req.Header.Set("X-Billing-Timestamp", time.Now().UTC().Format(time.RFC3339))
	resp, err := d.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}
