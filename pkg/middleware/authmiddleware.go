package middleware

import (
	"errors"
	"huddle/pkg/response"
	"huddle/pkg/utility"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// AuthMiddleware allows admin tokens only and sets "adminId" and "email".
func AuthMiddleware(c *gin.Context) {
	authenticate(c, utility.RoleAdmin, "adminId")
}

// CustomerAuthMiddleware allows customer tokens only and sets "customerId" and "email".
func CustomerAuthMiddleware(c *gin.Context) {
	authenticate(c, utility.RoleCustomer, "customerId")
}

func authenticate(c *gin.Context, role, idKey string) {
	header := c.GetHeader("Authorization")
	if header == "" {
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
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "Error Validating token", err)
		c.Abort()
		return
	}

	if claims.Role != role {
		response.Error(c, http.StatusForbidden, "You do not have access to this resource", errors.New("wrong token role"))
		c.Abort()
		return
	}

	c.Set(idKey, claims.UserId)
	c.Set("email", claims.Email)
	c.Next()
}
