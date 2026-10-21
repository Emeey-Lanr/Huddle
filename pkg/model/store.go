package model

import (
	"time"

	"github.com/google/uuid"
)

type Stores struct {
	ID uuid.UUID `json:"id" gorm:"uuid"`
	AdminId uuid.UUID `json:"admin_id" gorm:"column:admin_id"` 
	StoreName string `json:"store_name" validate:"required" gorm:"column:store_name"`
	StoreDescription string `json:"store_description" validate:"required,min=10" gorm:"column:store_description"`
	ApiKey string `json:"api_key" gorm:"column:api_key"`
	StoreId string `json:"store_id" gorm:"column:store_id"`
	// BachsAccountId is the seller's Bachs sub-account; never set from requests.
	BachsAccountId *string `json:"-" gorm:"column:bachs_account_id"`
	CreatedAt time.Time `json:"created_at" gorm:"column:created_at"`
	UpdatedAt time.Time `json:"updated_at" gorm:"column:updated_at"` 	
}

// StoreResponse is what admins see; it never includes the API key or internal ids.
type StoreResponse struct {
	StoreId          string    `json:"store_id" gorm:"column:store_id"`
	StoreName        string    `json:"store_name" gorm:"column:store_name"`
	StoreDescription string    `json:"store_description" gorm:"column:store_description"`
	CreatedAt        time.Time `json:"created_at" gorm:"column:created_at"`
	UpdatedAt        time.Time `json:"updated_at" gorm:"column:updated_at"`
}

// StorefrontStore is the public store info shoppers see.
type StorefrontStore struct {
	StoreId          string `json:"store_id"`
	StoreName        string `json:"store_name"`
	StoreDescription string `json:"store_description"`
}

// StoreUpdate request; empty fields are left unchanged.
type StoreUpdate struct {
	StoreName        string `json:"store_name" validate:"omitempty,max=150"`
	StoreDescription string `json:"store_description" validate:"omitempty,min=10"`
}
