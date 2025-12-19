package utility

import (
	"errors"

	"huddle/pkg/model"
	"time"

	"os"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var secretKey = []byte(os.Getenv("SECRET_KEY"))

func CreateToken (id uuid.UUID, email string) (string, error) {
	claims := jwt.MapClaims{
		"admin_id": id,
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


func VerifyToken (tokenString string) (*model.Claims, error) {
	claims := &model.Claims{}

     token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
	
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("Invalid Signing Method")
		} 
		return secretKey, nil
	 
	 })

	 if err != nil || !token.Valid{
         return nil, err
	 }

	 return claims, nil
}