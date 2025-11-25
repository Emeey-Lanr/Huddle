package model

import (
	"time"

	"github.com/google/uuid"
)

type Users struct {
	Id        uuid.UUID   `json:"id" gorm:"type:uuid;primarykey"`
	Name      string `json:"name" validate:"required"`
	Email     string `json:"email" validate:"required,email"`
	Password  string `json:"password" validate:"required,gte=6"`
	Wallet    int    `json:"wallet"`
	CreatedAt time.Time `json:"created_at" gorm:"column:created_at"`
}