package utility

import (
	"time"

	"os"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var secretKey = []byte(os.Getenv("SECRET_KEY"))

func CreateToken (id uuid.UUID, email string) (string, error) {
	claims := jwt.MapClaims{
		"user_id": id,
		"email":email,
		"exp":time.Now().Add(time.Hour * 1).Unix(),
	}

  token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

   signedToken, err := token.SignedString(secretKey)

   if err != nil{
	return "", err
   }

   return signedToken, nil
}