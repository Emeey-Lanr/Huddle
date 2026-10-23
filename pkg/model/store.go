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

// StoreDomains is a custom domain a store's storefront runs on.
type StoreDomains struct {
	Id        uuid.UUID
	StoreId   uuid.UUID // stores.id, not the public store id
	Domain    string    // bare lowercase hostname, e.g. "chiomasbags.com"
	CreatedAt time.Time
}

// StoreDomainRequest adds a domain; a full link such as "https://chiomasbags.com/" is accepted.
type StoreDomainRequest struct {
	Domain string `json:"domain" validate:"required,max=300"`
}

// StoreDomainResponse is a store domain as sellers see it.
type StoreDomainResponse struct {
	Domain    string    `json:"domain"`
	Origin    string    `json:"origin"`
	CreatedAt time.Time `json:"created_at"`
}

// StoreUpdate request; empty fields are left unchanged.
type StoreUpdate struct {
	StoreName        string `json:"store_name" validate:"omitempty,max=150"`
	StoreDescription string `json:"store_description" validate:"omitempty,min=10"`
}
