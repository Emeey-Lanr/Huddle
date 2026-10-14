package utility

import (
	"errors"

	"huddle/pkg/model"
	"time"

	"os"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	RoleAdmin    = "admin"
	RoleCustomer = "customer"
)

var ErrMissingSecret = errors.New("SECRET_KEY is not set")

// secretKey is read on every call so the value loaded from .env in main() is used.
func secretKey() ([]byte, error) {
	key := os.Getenv("SECRET_KEY")
	if key == "" {
		return nil, ErrMissingSecret
	}
	return []byte(key), nil
}

func CreateToken(id uuid.UUID, email, role string) (string, error) {
	key, err := secretKey()
	if err != nil {
		return "", err
	}

	claims := jwt.MapClaims{
		"user_id": id,
		"email":   email,
		"role":    role,
		"exp":     time.Now().Add(time.Hour * 1).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	signedToken, err := token.SignedString(key)
	if err != nil {
		return "", err
	}

	return signedToken, nil
}

func VerifyToken(tokenString string) (*model.Claims, error) {
	key, err := secretKey()
	if err != nil {
		return nil, err
	}

	claims := &model.Claims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("Invalid Signing Method")
		}
		return key, nil
	})

	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}
