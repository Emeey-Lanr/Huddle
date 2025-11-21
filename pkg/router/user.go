package router

import (
	middlware "huddle/pkg/middleware"

	"huddle/pkg/controller"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)
 


func UserRouter (r *gin.Engine, db *gorm.DB){
    user := controller.Base{Db: db}

	userGroup := r.Group("/user", middlware.AuthoriseUser)
   
  {
	userGroup.POST("/signup", user.Signup)
  }


  
}