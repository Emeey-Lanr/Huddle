package middleware

import (
	"errors"
	"huddle/pkg/response"
	"huddle/pkg/utility"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func AuthMiddleware(c *gin.Context){
	header :=  c.GetHeader("Authorization")
    if header == ""{
       response.Error(c, http.StatusUnauthorized, "Authorization header is missing", errors.New("Missing authorization header"))
	   c.Abort()
	   return
	}

	auth := strings.Fields(header)

	if len(auth) != 2 || auth[0] != "Bearer" {
		response.Error(c, http.StatusUnauthorized, "Invalid Authorization header format", errors.New("Invalid Auth header format"))
		c.Abort()
		return
	}
    
	claims, err := utility.VerifyToken(auth[1])
	if err != nil{
		response.Error(c, http.StatusUnauthorized, "Error Validating token", err)
	  c.Abort()
	  return
	}
    
  c.Set("adminId", claims.AdminId)
  c.Set("email", claims.Email)
  c.Next()

}