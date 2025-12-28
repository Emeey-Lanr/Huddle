package controller

import (
	"errors"
	"huddle/pkg/response"
		"huddle/pkg/model"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"huddle/pkg/utility"
	"huddle/pkg/services"
)


type StoreBase struct {
	Db *gorm.DB
  Rdb *redis.Client
}

func (b *StoreBase) Create (c *gin.Context){
  adminId, adminExists := c.Get("adminId")
  Email, emailExists := c.Get("email")

  uuidAdminId, err :=  utility.StringToUUID(adminId.(string))
  if err != nil {
	 response.Error(c, http.StatusInternalServerError, "Internal Server Err", err)
  }

  if !adminExists || !emailExists{
   response.Error(c, http.StatusUnauthorized, "Access Not allowed", errors.New("Not allowed"))
   return
  }

  var req  model.Stores
 if err := c.ShouldBindJSON(&req); err != nil{
	response.Error(c, http.StatusBadRequest, "Bad Request", err)
	return
 }

 if err := utility.Validate.Struct(&req); err != nil{
   err := utility.ValidationErrors(err)
   if err != nil{
	response.Error(c, http.StatusBadRequest, "Invalid Json Format", err)
	return
   }
 }
 
 
 data, apiKey, serviceErr := services.CreateStore(b.Db, req, uuidAdminId, Email.(string))
  
 if serviceErr != nil {
   response.Error(c, serviceErr.StatusCode, serviceErr.Message, serviceErr.Err)
   return
 }
  
 
 response.Created(c, "Store Created Succesfully", gin.H{"store_name":data.StoreName, "store_id":data.StoreId,  "api_key":apiKey, "created_at":data.CreatedAt})

}
