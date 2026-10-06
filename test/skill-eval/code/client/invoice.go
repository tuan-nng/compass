// Package client is the Go client other services use to call billing-api.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// Client calls the billing API.
type Client struct {
	BaseURL string
	HTTP    *http.Client
}

// Invoice is the v2 invoice resource.
type Invoice struct {
	ID          string `json:"id,omitempty"`
	CustomerID  string `json:"customer_id"`
	AmountCents int64  `json:"amount_cents"`
}

// CreateInvoice calls POST /v2/invoices once.
func (c *Client) CreateInvoice(ctx context.Context, in Invoice) (Invoice, error) {
	body, err := json.Marshal(in)
	if err != nil {
		return Invoice{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/v2/invoices", bytes.NewReader(body))
	if err != nil {
		return Invoice{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return Invoice{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Invoice{}, fmt.Errorf("create invoice: %s", resp.Status)
	}
	var out Invoice
	err = json.NewDecoder(resp.Body).Decode(&out)
	return out, err
}
