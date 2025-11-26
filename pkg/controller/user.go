package controller

import (
	"huddle/pkg/model"
	"huddle/pkg/services"
	"huddle/pkg/utility"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)


type Base struct {
	Db *gorm.DB

}


func (base *Base) Signup (c *gin.Context){
   var req model.Users

   if err := c.ShouldBindJSON(&req); err != nil{
	c.JSON(http.StatusBadRequest, gin.H{"error":"Invalid request"})
	log.Println(err)
	return
   }

   if err := utility.Validate.Struct(&req); err != nil {
	errorMsg := utility.ValidationErrors(err)
	c.JSON(http.StatusBadRequest, errorMsg)
	return 
   }
   
   //create user
    token, err := services.Signup(base.Db, req)
	if err != nil{
		c.JSON(err.Status, gin.H{"err":err.Message})
		log.Println(err.Err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message":"user created succesfully","token":token})
	
}



func (base *Base) Signin (c *gin.Context){
	var req model.Signin

	if err := c.ShouldBindJSON(&req); err != nil{
		c.JSON(http.StatusBadRequest, gin.H{"message":"Inavlid request"})
		return
	}

	if err := utility.Validate.Struct(&req); err != nil{
      errMsg := utility.ValidationErrors(err)
	  c.JSON(http.StatusBadRequest, errMsg)
	  return
	}

   
	token, err := services.Signin(base.Db, req)
	
	if err != nil {
		log.Println()
		c.JSON(err.Status, gin.H{"message":err.Message})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message":"Login succesfull", "token":token})



}