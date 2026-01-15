package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type Products struct {
	Id uuid.UUID `json:"id" gorm:"uuid"`
	StoreId uuid.UUID `json:"store_id" gorm:"column:store_id"`
	PublicId string `json:"public_id" gorm:"column:public_id"`
	Name   string `json:"name" validate:"required"`
	Description string `json:"description" validate:"required,min=10"`
	Price  decimal.Decimal  `json:"price" validate:"required"`
	Currency string `json:"currency" validate:"required,gte=3"`
	StockQuantity int `json:"stock_quantity" gorm:"column:stock_quantity"`
	Active bool `json:"active"`
	CreatedAt time.Time `json:"created_at" gorm:"column:created_at"`
	UpdateAt time.Time `json:"updated_at" gorm:"column:updated_at"`
}
type ProductResponse struct {
	PublicId string `json:"id" gorm:"column:public_id"`
	Name   string `json:"name" validate:"required"`
	Description string `json:"description" validate:"required,min=10"`
	Price  decimal.Decimal  `json:"price" validate:"required"`
	Currency string `json:"currency" validate:"required,gte=3"`
	StockQuantity int `json:"stock_quantity" gorm:"column:stock_quantity"`
	Active bool `json:"active"`
	CreatedAt time.Time `json:"created_at" gorm:"column:created_at"`
	UpdateAt time.Time `json:"updated_at" gorm:"column:updated_at"`
}


// product query
type ProductQuery struct {
	StoreId string `form:"store_id" binding:"required"`
	Page int `form:"page"`
	Limit int `form:"limit"`
	MinPrice int `form:"min_price"`
	MaxPrice int `form:"max-price"`
	Sort string `form:"sort"`
	Order string `form:"order"`
}


type Images struct {
	Id uuid.UUID 
	ProductId uuid.UUID `json:"product_id" gorm:"uuid" gorm:"column:product_id"`
	PublicID  string `json:"public_id" gorm:"column:public_id"`
	ImageURL   string `json:"image_url" gorm:"column:image_url"`
	
}

type ImageData struct {
  ImageId string `json:"image_id"`
  ImageURL string `json:"image_url"`
}

type ImagesCreatedResponse struct {
	ProductId string `json:"product_id"`
   Images []ImageData `json:"images"`
}


// image created repsonse
type ImagesResponse struct {
	ProductId string `json:"id" gorm:"column:public_id"`
	ImageUrl string `json:"img_url" gorm:"column:image_url"`
}