package model

import (
	"time"

	"github.com/google/uuid"
)

type Stores struct {
	ID uuid.UUID `json:"id" gorm:"id"`
	AdminId uuid.UUID `json:"admin_id" gorm:"column:admin_id"` 
	StoreName string `json:"store_name" gorm:"column:store_name"`
	StoreDescription string `json:"store_description" gorm:"column:store_description"`
	ApiKey string `json:"api_key" gorm:"column:api_key"`
	StoreId string `json:"store_id" gorm:"column:store_id"`
	CreatedAt time.Time `json:"created_at" gorm:"column:created_at"`
	UpdatedAt time.Time `json:"updated_at" gorm:"column:updated_at"` 	
}
