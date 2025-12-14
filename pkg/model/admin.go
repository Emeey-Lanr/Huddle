package model

import (
	"time"

	"github.com/google/uuid"
)

type Admins struct {
	Id uuid.UUID `json:"id" gorm:"uuid"`
	Name string `json:"name" validate:"required"`
	Username string `json:"username" validate:"required"`
	Email string `json:"email" validate:"required, email"`
	Password string `json:"password" validate:"required"`
	CreatedAt  time.Time `json:"created_at" gorm:"column:created_at"`
	UpdatedAt  time.Time `json:"updated_at" gorm:"column:updated_at"`
}
