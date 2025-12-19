package controller

import (
	"errors"
	"huddle/pkg/response"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)


type StoreBase struct {
	Db *gorm.DB
  Rdb *redis.Client
}

func (b *StoreBase) Create (c *gin.Context){
  adminId, adminExists := c.Get("adminId")
  Email, emailExists := c.Get("email")

  if !adminExists || !emailExists{
   response.Error(c, http.StatusUnauthorized, "Not allowed", errors.New("Not allowed"))
  }
 
  log.Println(adminId, Email)

}
