// Package payme is the Payme Subscribe (recurrent) adapter: card tokenization
// (cards.*) and recurrent charges (receipts.*). PAN is transited, never stored
// or logged.
package payme

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Client calls the Payme Subscribe JSON-RPC API.
type Client struct {
	url        string
	merchantID string
	key        string
	http       *http.Client
}

func NewClient(url, merchantID, key string, hc *http.Client) *Client {
	if hc == nil {
		hc = &http.Client{Timeout: 20 * time.Second}
	}
	return &Client{url: url, merchantID: merchantID, key: key, http: hc}
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *rpcError) Error() string { return fmt.Sprintf("payme error %d: %s", e.Code, e.Message) }

// call issues a JSON-RPC method and decodes result into out.
func (c *Client) call(ctx context.Context, method string, params any, out any) error {
	body, err := json.Marshal(map[string]any{"id": time.Now().UnixNano(), "method": method, "params": params})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Auth", c.merchantID+":"+c.key)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var envelope struct {
		Result json.RawMessage `json:"result"`
		Error  *rpcError       `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return err
	}
	if envelope.Error != nil {
		return envelope.Error
	}
	if out != nil && len(envelope.Result) > 0 {
		return json.Unmarshal(envelope.Result, out)
	}
	return nil
}

// CreateCard tokenizes a card (save=true). Returns (token, masked number).
// The PAN argument is never logged.
func (c *Client) CreateCard(ctx context.Context, number, expire string) (token, masked string, err error) {
	var res struct {
		Card struct {
			Token  string `json:"token"`
			Number string `json:"number"`
		} `json:"card"`
	}
	err = c.call(ctx, "cards.create", map[string]any{
		"card": map[string]any{"number": number, "expire": expire},
		"save": true,
	}, &res)
	if err != nil {
		return "", "", err
	}
	return res.Card.Token, res.Card.Number, nil
}

// SendVerifyCode asks Payme to SMS a verification code to the cardholder.
func (c *Client) SendVerifyCode(ctx context.Context, token string) error {
	return c.call(ctx, "cards.get_verify_code", map[string]any{"token": token}, nil)
}

// VerifyCard confirms the card with the SMS code, making the token chargeable.
func (c *Client) VerifyCard(ctx context.Context, token, code string) error {
	return c.call(ctx, "cards.verify", map[string]any{"token": token, "code": code}, nil)
}

// RemoveCard deletes a saved card token.
func (c *Client) RemoveCard(ctx context.Context, token string) error {
	return c.call(ctx, "cards.remove", map[string]any{"token": token}, nil)
}

// Charge creates a receipt for the invoice and pays it with the saved token.
// Returns the Payme receipt id as the provider reference.
func (c *Client) Charge(ctx context.Context, invoiceID, amount int64, token string) (string, error) {
	var created struct {
		Receipt struct {
			ID string `json:"_id"`
		} `json:"receipt"`
	}
	if err := c.call(ctx, "receipts.create", map[string]any{
		"amount":  amount,
		"account": map[string]any{"invoice_id": fmt.Sprintf("%d", invoiceID)},
	}, &created); err != nil {
		return "", err
	}
	var paid struct {
		Receipt struct {
			ID    string `json:"_id"`
			State int    `json:"state"`
		} `json:"receipt"`
	}
	if err := c.call(ctx, "receipts.pay", map[string]any{
		"id": created.Receipt.ID, "token": token,
	}, &paid); err != nil {
		return "", err
	}
	if paid.Receipt.State != 4 { // 4 = paid
		return "", fmt.Errorf("payme receipt state %d (not paid)", paid.Receipt.State)
	}
	return paid.Receipt.ID, nil
}
