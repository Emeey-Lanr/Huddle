package controller

import (
	"huddle/pkg/model"
	"huddle/pkg/response"
	"huddle/pkg/services"
	"huddle/pkg/utility"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type AdminBase struct {
	Db *gorm.DB
  Rdb *redis.Client
}


func (b *AdminBase) Signup(c *gin.Context){
	 var req model.Admins

   if err := c.ShouldBindJSON(&req); err != nil{
     response.Error(c, http.StatusBadRequest, "Bad Request", err)
    return
   }

   err :=  utility.Validate.Struct(&req)
   if err != nil{
     _, errMsg := utility.ValidationErrors(err)
     response.Error(c, http.StatusBadRequest, errMsg, err)
   }

   token, servicesErr :=  services.AdminSignup(b.Db, req)
   if servicesErr != nil{
     response.Error(c, servicesErr.StatusCode, servicesErr.Message, servicesErr.Err)
   }


   response.Created(c, "Registration Succesfull", map[string]string{"token":token})
  
}