package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Payment statuses.
const (
	PaymentOpen        = "open"
	PaymentPaid        = "paid"
	PaymentExpired     = "expired"
	PaymentNeedsRefund = "needs_refund"
	PaymentReview      = "review"
)

type Payments struct {
	Id          uuid.UUID
	PublicId    string `gorm:"column:public_id"`
	OrderId     uuid.UUID
	StoreId     uuid.UUID
	Provider    string
	CheckoutId  string `gorm:"column:checkout_id"`
	CheckoutURL string `gorm:"column:checkout_url"`
	AccountId   *string
	Status      string
	Amount      decimal.Decimal
	Currency    string
	ExpiresAt   time.Time
	PaidAt      *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type WebhookEvents struct {
	EventId    string `gorm:"primaryKey"`
	Provider   string
	Type       string
	ReceivedAt time.Time
}

// PayRequest says where Bachs sends the shopper after paying or giving up.
type PayRequest struct {
	SuccessURL string `json:"success_url" validate:"required,http_url"`
	CancelURL  string `json:"cancel_url" validate:"omitempty,http_url"`
}

type PaymentResponse struct {
	PaymentId   string          `json:"payment_id"`
	CheckoutURL string          `json:"checkout_url"`
	Amount      decimal.Decimal `json:"amount"`
	Currency    string          `json:"currency"`
	ExpiresAt   time.Time       `json:"expires_at"`
}
