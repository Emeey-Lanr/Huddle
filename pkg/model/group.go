package model

import (
	"time"

	"github.com/google/uuid"
)

// ProductGroups sort a store's products for shoppers ("Shoes", "Bags"). A
// product is in one group or none.
type ProductGroups struct {
	Id        uuid.UUID `gorm:"column:id"`
	PublicId  string    `gorm:"column:public_id"`
	StoreId   uuid.UUID `gorm:"column:store_id"`
	Name      string    `gorm:"column:name"`
	Slug      string    `gorm:"column:slug"`
	Position  int       `gorm:"column:position"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

// ProductGroupRequest creates a group, or changes one (empty or nil fields are
// left as they are).
type ProductGroupRequest struct {
	Name     string `json:"name"`
	Position *int   `json:"position"`
}

// ProductGroupProductsRequest puts products into a group, moving them out of
// any group they were in.
type ProductGroupProductsRequest struct {
	ProductIds []string `json:"product_ids" validate:"required,min=1,max=100"`
}

// ProductGroupResponse is a group as the seller sees it.
type ProductGroupResponse struct {
	Id         string    `json:"id"`
	Name       string    `json:"name"`
	Slug       string    `json:"slug"`
	Position   int       `json:"position"`
	ProductIds []string  `json:"product_ids"`
	CreatedAt  time.Time `json:"created_at"`
}

// StorefrontGroup is a group as shoppers see it: only groups with at least
// one product on sale are listed.
type StorefrontGroup struct {
	Name     string `json:"name"`
	Slug     string `json:"slug"`
	Products int64  `json:"products"`
}
