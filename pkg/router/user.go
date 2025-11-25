package router

import (
    "huddle/pkg/controller"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)
 


func UserRouter (r *gin.Engine, db *gorm.DB){
    user := &controller.Base{Db: db}

	userGroup := r.Group("/user")
   
  {
	userGroup.POST("/signup", user.Signup)
  }


  
}