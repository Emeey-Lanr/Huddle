package model

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Admins struct {
	Id uuid.UUID `json:"id" gorm:"uuid"`
	Name string `json:"name" validate:"required"`
	Username string `json:"username" validate:"required"`
	Email string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=6"`
	CreatedAt  time.Time `json:"created_at" gorm:"column:created_at"`
	UpdatedAt  time.Time `json:"updated_at" gorm:"column:updated_at"`
}



type AdminSignin struct {
	Username string  `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
}

type Claims struct {
	AdminId string `json:"admin_id"`
	Email string `json:"email"`
	jwt.RegisteredClaims
}