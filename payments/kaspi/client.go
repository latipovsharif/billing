// Package kaspi is the Kaspi QR (poll-based) payment adapter: create a QR for an
// invoice, poll its status, settle on payment. No card/PAN passes through.
package kaspi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Client calls the Kaspi QR merchant API. NOTE: the wire paths/field names below
// follow the public SDK shape and MUST be confirmed against Kaspi merchant docs
// once credentials exist; the structure (create + poll) is correct.
type Client struct {
	baseURL   string
	apiKey    string
	deviceTok string
	orgBIN    string
	http      *http.Client
}

func NewClient(baseURL, apiKey, deviceToken, orgBIN string, hc *http.Client) *Client {
	if hc == nil {
		hc = &http.Client{Timeout: 20 * time.Second}
	}
	return &Client{baseURL: baseURL, apiKey: apiKey, deviceTok: deviceToken, orgBIN: orgBIN, http: hc}
}

func (c *Client) do(ctx context.Context, method, path string, body any, out any) error {
	var rdr *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(b)
	} else {
		rdr = bytes.NewReader(nil)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, rdr)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("kaspi %s: status %d", path, resp.StatusCode)
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

// CreateQR creates a QR for an order (orderID = inv-<invoiceID>). Returns the
// Kaspi QR invoice id (for polling) and the QR token (rendered by the frontend).
func (c *Client) CreateQR(ctx context.Context, orderID string, amount int64) (qrInvoiceID, qrToken string, err error) {
	var res struct {
		QRInvoiceID string `json:"qr_invoice_id"`
		QRToken     string `json:"qr_token"`
	}
	err = c.do(ctx, http.MethodPost, "/qr/create", map[string]any{
		"order_id": orderID, "amount": amount,
		"device_token": c.deviceTok, "org_bin": c.orgBIN,
	}, &res)
	if err != nil {
		return "", "", err
	}
	return res.QRInvoiceID, res.QRToken, nil
}

// PaymentInfo polls a QR invoice and maps the Kaspi status to one of
// pending|paid|canceled|expired.
func (c *Client) PaymentInfo(ctx context.Context, qrInvoiceID string) (string, error) {
	var res struct {
		Status string `json:"status"`
	}
	if err := c.do(ctx, http.MethodGet, "/qr/status?id="+qrInvoiceID, nil, &res); err != nil {
		return "", err
	}
	return mapStatus(res.Status), nil
}

// mapStatus normalizes Kaspi status strings.
func mapStatus(s string) string {
	switch s {
	case "Processed", "Paid":
		return "paid"
	case "Error", "Canceled", "Declined":
		return "canceled"
	case "Expired", "Timeout":
		return "expired"
	default: // Wait, Created, QrTokenCreated, ...
		return "pending"
	}
}

// Cancel cancels a QR invoice.
func (c *Client) Cancel(ctx context.Context, qrInvoiceID string) error {
	return c.do(ctx, http.MethodPost, "/qr/cancel", map[string]any{"qr_invoice_id": qrInvoiceID}, nil)
}

// Refund refunds a paid QR invoice.
func (c *Client) Refund(ctx context.Context, qrInvoiceID string, amount int64) error {
	return c.do(ctx, http.MethodPost, "/qr/refund", map[string]any{"qr_invoice_id": qrInvoiceID, "amount": amount}, nil)
}
