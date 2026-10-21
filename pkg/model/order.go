package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Order statuses. A new order is pending and holds its stock until it is
// paid, cancelled, or expires.
const (
	OrderPending   = "pending"
	OrderPaid      = "paid"
	OrderFulfilled = "fulfilled"
	OrderCancelled = "cancelled"
	OrderExpired   = "expired"
)

const (
	DeliveryMethodDelivery = "delivery"
	DeliveryMethodPickup   = "pickup"
)

type Orders struct {
	Id             uuid.UUID
	PublicId       string `gorm:"column:public_id"`
	StoreId        uuid.UUID
	CustomerId     *uuid.UUID
	Status         string
	Currency       string
	Total          decimal.Decimal
	CustomerName   string
	CustomerPhone  string
	CustomerEmail  string
	DeliveryMethod string
	Address        string
	City           string
	State          string
	Note           string
	ExpiresAt      time.Time
	PaidAt         *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type OrderItems struct {
	Id              uuid.UUID
	OrderId         uuid.UUID
	ProductId       *uuid.UUID
	ProductPublicId string `gorm:"column:product_public_id"`
	ProductName     string
	UnitPrice       decimal.Decimal
	Quantity        int
	LineTotal       decimal.Decimal
}

// CheckoutRequest holds the shopper's contact and delivery details. Items
// always come from the shopper's cart, and prices from the products table.
type CheckoutRequest struct {
	Name           string `json:"name" validate:"required,max=250"`
	Phone          string `json:"phone" validate:"required"`
	Email          string `json:"email" validate:"omitempty,email,max=250"`
	DeliveryMethod string `json:"delivery_method" validate:"required,oneof=delivery pickup"`
	Address        string `json:"address" validate:"required_if=DeliveryMethod delivery"`
	City           string `json:"city" validate:"required_if=DeliveryMethod delivery,max=100"`
	State          string `json:"state" validate:"required_if=DeliveryMethod delivery,max=100"`
	Note           string `json:"note" validate:"max=1000"`
}

// OrderStatusUpdate is a seller's status change.
type OrderStatusUpdate struct {
	Status string `json:"status" validate:"required,oneof=paid fulfilled cancelled"`
}

type OrderQuery struct {
	Status string `form:"status"`
	Page   int    `form:"page"`
	Limit  int    `form:"limit"`
}

type OrderItemResponse struct {
	ProductId   string          `json:"product_id"`
	ProductName string          `json:"product_name"`
	UnitPrice   decimal.Decimal `json:"unit_price"`
	Quantity    int             `json:"quantity"`
	LineTotal   decimal.Decimal `json:"line_total"`
}

type OrderResponse struct {
	Id             string              `json:"id"`
	Status         string              `json:"status"`
	Currency       string              `json:"currency"`
	Total          decimal.Decimal     `json:"total"`
	Name           string              `json:"name"`
	Phone          string              `json:"phone"`
	Email          string              `json:"email"`
	DeliveryMethod string              `json:"delivery_method"`
	Address        string              `json:"address"`
	City           string              `json:"city"`
	State          string              `json:"state"`
	Note           string              `json:"note"`
	Items          []OrderItemResponse `json:"items"`
	ExpiresAt      time.Time           `json:"expires_at"`
	PaidAt         *time.Time          `json:"paid_at"`
	CreatedAt      time.Time           `json:"created_at"`
	UpdatedAt      time.Time           `json:"updated_at"`
	Payment        *OrderPayment       `json:"payment,omitempty"` // seller views only
}

type OrderListResponse struct {
	Items []OrderResponse `json:"items"`
	Page  int             `json:"page"`
	Limit int             `json:"limit"`
	Total int64           `json:"total"`
}
