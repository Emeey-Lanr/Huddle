package bachs

import (
	"context"
	"errors"
	"net/http"
)

// RefundRequest refunds a charge: in full when Amount is empty.
type RefundRequest struct {
	ChargeId       string `json:"charge_id"`
	Amount         string `json:"amount,omitempty"`
	Reference      string `json:"reference"`
	Reason         string `json:"reason,omitempty"`
	IdempotencyKey string `json:"idempotency_key"`
}

type Refund struct {
	RefundId  string `json:"refund_id"`
	ChargeId  string `json:"charge_id"`
	Reference string `json:"reference"`
	Status    string `json:"status"`
}

// RefundSucceeded says whether a refund status from Bachs means the money
// left: the API says "success", the refund.paid event "paid".
func RefundSucceeded(status string) bool {
	return status == "success" || status == "paid"
}

// CreateRefund refunds a charge. A non-empty accountId sends X-Account-Id,
// as a direct charge is refunded from the seller's balance. reference is
// also the idempotency key, so retrying the same refund is safe.
func (c *Client) CreateRefund(ctx context.Context, accountId string, req RefundRequest) (*Refund, error) {
	req.IdempotencyKey = req.Reference
	var refund Refund
	if err := c.do(ctx, http.MethodPost, "/v1/refunds", accountId, req.Reference, req, &refund); err != nil {
		return nil, err
	}
	if refund.RefundId == "" {
		return nil, errors.New("bachs: refund response has no refund_id")
	}
	return &refund, nil
}

type Balance struct {
	Currency         string `json:"currency"`
	AvailableBalance string `json:"available_balance"`
}

// Balances reads an account's balances; accountId empty means the platform.
func (c *Client) Balances(ctx context.Context, accountId string) ([]Balance, error) {
	var resp struct {
		Balances []Balance `json:"balances"`
	}
	if err := c.do(ctx, http.MethodGet, "/v1/balances", accountId, "", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Balances, nil
}
