package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type Products struct {
	Id uuid.UUID `json:"id" gorm:"uuid"`
	StoreId uuid.UUID `json:"store_id" gorm:"column:store_id"`
	Name   string `json:"name" validate:"required"`
	Description string `json:"description" validate:"required,min=10"`
	Price  decimal.Decimal  `json:"price" validate:"required"`
	Currency string `json:"currency" validate:"required,gte=3"`
	StockQuantity int `json:"stock_quantity" gorm:"column:stock_quantity"`
	Active bool `json:"active"`
	CreatedAt time.Time `json:"created_at" gorm:"column:created_at"`
	UpdateAt time.Time `json:"updated_at" gorm:"column:updated_at"`
}