// Package bachs talks to the Bachs payments API (https://docs.bachs.io).
//
// Config comes from the environment:
//   - BACHS_API_KEY: secret key, sk_sandbox_... or sk_live_...
//   - BACHS_WEBHOOK_SECRET: the webhook endpoint's signing secret
//   - BACHS_BASE_URL: defaults to the sandbox, https://sandbox-api.bachs.io
package bachs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const SandboxURL = "https://sandbox-api.bachs.io"

var ErrNotConfigured = errors.New("BACHS_API_KEY is not set")

type Client struct {
	BaseURL string
	APIKey  string
	HTTP    *http.Client
}

// FromEnv builds a client from the environment.
func FromEnv() (*Client, error) {
	key := os.Getenv("BACHS_API_KEY")
	if key == "" {
		return nil, ErrNotConfigured
	}
	base := os.Getenv("BACHS_BASE_URL")
	if base == "" {
		base = SandboxURL
	}
	return &Client{BaseURL: base, APIKey: key, HTTP: &http.Client{Timeout: 20 * time.Second}}, nil
}

type Pricing struct {
	Currency  string `json:"currency"`
	Amount    string `json:"amount"`
	PriceType string `json:"price_type"`
}

type Customer struct {
	Email       string `json:"email"`
	Name        string `json:"name,omitempty"`
	PhoneNumber string `json:"phone_number,omitempty"`
}

// CheckoutRequest is a product-less (pure) checkout for a fixed amount.
type CheckoutRequest struct {
	Pricing          Pricing           `json:"pricing"`
	SuccessURL       string            `json:"success_url"`
	CancelURL        string            `json:"cancel_url,omitempty"`
	Reference        string            `json:"reference"`
	ExpiresInMinutes int               `json:"expires_in_minutes"`
	Customer         *Customer         `json:"customer,omitempty"`
	Metadata         map[string]string `json:"metadata,omitempty"`
	PlatformFee      string            `json:"platform_fee,omitempty"`
}

type Checkout struct {
	CheckoutId  string    `json:"checkout_id"`
	CheckoutURL string    `json:"checkout_url"`
	Status      string    `json:"status"`
	ExpiresAt   time.Time `json:"expires_at"`
	Reference   string    `json:"reference"`
}

// APIError is an error response from Bachs.
type APIError struct {
	StatusCode int
	Detail     string `json:"detail"`
	ErrorCode  string `json:"error_code"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("bachs: %d %s: %s", e.StatusCode, e.ErrorCode, e.Detail)
}

// CreateCheckout creates a hosted checkout. A non-empty accountId sends
// X-Account-Id, making it a direct charge: the sale lands in that seller's
// Bachs balance. idempotencyKey makes retries of the same request safe.
func (c *Client) CreateCheckout(ctx context.Context, accountId, idempotencyKey string, req CheckoutRequest) (*Checkout, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/v1/checkout-sessions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Idempotency-Key", idempotencyKey)
	if accountId != "" {
		httpReq.Header.Set("X-Account-Id", accountId)
	}

	resp, err := c.HTTP.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 300 {
		apiErr := &APIError{StatusCode: resp.StatusCode}
		json.Unmarshal(data, apiErr)
		return nil, apiErr
	}
	var checkout Checkout
	if err := json.Unmarshal(data, &checkout); err != nil {
		return nil, fmt.Errorf("bachs: decode checkout: %w", err)
	}
	if checkout.CheckoutId == "" || checkout.CheckoutURL == "" {
		return nil, errors.New("bachs: checkout response has no checkout_id or checkout_url")
	}
	return &checkout, nil
}
