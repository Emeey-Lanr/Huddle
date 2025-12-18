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
      errMsg := utility.ValidationErrors(err)
     response.Error(c, http.StatusBadRequest, "Invalid request", errMsg)
     return
   }

   token, servicesErr :=  services.AdminSignup(b.Db, req)
   if servicesErr != nil{
     response.Error(c, servicesErr.StatusCode, servicesErr.Message, servicesErr.Err)
     return
    }

  
   response.Created(c, "Registration Succesfull", map[string]string{"token":token})
  
}




func (b *AdminBase) Signin(c * gin.Context){
     var req model.AdminSignin

     if err := c.ShouldBindJSON(&req); err != nil {
       response.Error(c, http.StatusBadRequest, "Invalid request", err)
       return
     }


     err := utility.Validate.Struct(&req)
     if err != nil{
       errMsg := utility.ValidationErrors(err)
      response.Error(c, http.StatusBadRequest,  "Invalid request", errMsg)
      return
     }

  token, servicesErr := services.AdminSignin(b.Db, req)
   if servicesErr != nil{
    response.Error(c, servicesErr.StatusCode, servicesErr.Message, servicesErr.Err)
    return
   }

  response.Success(c, "Login Succesfull", map[string]string{"token":token})

}
