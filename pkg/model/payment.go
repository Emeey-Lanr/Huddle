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
	PaymentRefunded    = "refunded"
)

// Refund statuses.
const (
	RefundProcessing = "processing"
	RefundPaid       = "paid"
	RefundFailed     = "failed"
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
	ChargeId    *string
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

// BachsCapabilities is the last known status of a seller account's capability.
type BachsCapabilities struct {
	AccountId string `gorm:"primaryKey"`
	Name      string `gorm:"primaryKey"`
	Status    string
	ChangedAt time.Time
}

// OnboardingRequest says where Bachs sends the seller after onboarding
// (return_url) and where an expired or used link leads (refresh_url, which
// should ask Huddle for a new link).
type OnboardingRequest struct {
	ReturnURL  string `json:"return_url" validate:"required,http_url"`
	RefreshURL string `json:"refresh_url" validate:"required,http_url"`
}

type OnboardingLink struct {
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expires_at"`
}

// PaymentSetup tells a seller whether their store can take online payments.
type PaymentSetup struct {
	// Connected is true once the store has a Bachs account.
	Connected bool `json:"connected"`
	// Ready is true when shoppers can pay the store online.
	Ready        bool               `json:"ready"`
	Capabilities []CapabilityStatus `json:"capabilities"`
	// Outstanding lists details Bachs still needs from the seller.
	Outstanding []string `json:"outstanding"`
}

type CapabilityStatus struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type Refunds struct {
	Id            uuid.UUID
	PublicId      string `gorm:"column:public_id"`
	PaymentId     uuid.UUID
	StoreId       uuid.UUID
	BachsRefundId *string
	Amount        decimal.Decimal
	Currency      string
	Reason        string
	CancelOrder   bool
	Status        string
	FailureReason string
	CompletedAt   *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// RefundRequest is a seller's refund of a payment: in full, or Amount of it.
// Bachs takes one refund per payment, so the rest of a partial refund can't
// be refunded later.
type RefundRequest struct {
	Amount *decimal.Decimal `json:"amount"`
	Reason string           `json:"reason" validate:"max=500"`
	// KeepOrder refunds without cancelling the order, e.g. for one missing
	// item when the rest is still delivered.
	KeepOrder bool `json:"keep_order"`
}

type RefundResponse struct {
	RefundId      string          `json:"refund_id"`
	PaymentId     string          `json:"payment_id"`
	Status        string          `json:"status"`
	Amount        decimal.Decimal `json:"amount"`
	Currency      string          `json:"currency"`
	Reason        string          `json:"reason"`
	CancelOrder   bool            `json:"cancel_order"`
	FailureReason string          `json:"failure_reason,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
	CompletedAt   *time.Time      `json:"completed_at"`
}

// OrderPayment is the payment behind an order, shown to its seller.
type OrderPayment struct {
	PaymentId    string          `json:"payment_id"`
	Status       string          `json:"status"`
	Amount       decimal.Decimal `json:"amount"`
	Currency     string          `json:"currency"`
	PaidAt       *time.Time      `json:"paid_at"`
	RefundStatus string          `json:"refund_status,omitempty"`
}

// PaymentIssue is a payment the seller must act on: money that arrived for
// an order that could not take it (needs_refund), or that did not match the
// order (review).
type PaymentIssue struct {
	OrderPayment
	OrderId   string    `json:"order_id"`
	CreatedAt time.Time `json:"created_at"`
}
