package middleware

import (
	"errors"
	"huddle/pkg/response"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

func StoreContextMiddleware (c *gin.Context){
 public_store_id :=	c.GetHeader("X-Store-Id")
 log.Println(public_store_id)
 if public_store_id == "" {
   response.Error(c, http.StatusUnauthorized, "X-Store-Id header is missing", errors.New("Missing x-store-id"))
   c.Abort()
   return
 }

 c.Set("X-Store-Id", public_store_id)
 c.Next()
}