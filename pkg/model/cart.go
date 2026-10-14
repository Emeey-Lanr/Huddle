package model

import "github.com/shopspring/decimal"

// AddCartItemRequest is what the shopper sends. Name, price and currency are
// always read from the products table, never from the client.
type AddCartItemRequest struct {
	ProductId string `json:"product_id" validate:"required"`
	Quantity  uint   `json:"quantity" validate:"required,gte=1"`
}

type CartItem struct {
	ProductId   string          `json:"product_id"`
	ProductName string          `json:"product_name"`
	Price       decimal.Decimal `json:"product_price"`
	Currency    string          `json:"currency"`
	Quantity    uint            `json:"quantity"`
}

type CartResponse struct {
	SessionId string     `json:"session_id"`
	StoreId   string     `json:"store_id"`
	Items     []CartItem `json:"items"`
}

type UpdateCartItemRequest struct {
	Quantity uint `json:"quantity" validate:"required,gte=1"`
}
