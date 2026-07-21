package model

type CartItem struct {
	ProductId   string  `json:"product_id" validate:"required"`
	ProductName string  `json:"product_name" validate:"required"`
	Price       float64 `json:"product_price" validate:"required"`
	Currency    string  `json:"currency" validate:"required"`
	Quantity    uint    `json:"quantity" validate:"required,gte=1"`
}

type CartResponse struct {
	SessionId string     `json:"session_id"`
	Items     []CartItem `json:"items"`
}
