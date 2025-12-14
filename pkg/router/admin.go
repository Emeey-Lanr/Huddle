package router

import (
	"huddle/pkg/controller"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	
	"github.com/redis/go-redis/v9"
)


func AdminRouter (r *gin.Engine, db *gorm.DB, rdb *redis.Client){
	
    
	adminBase := &controller.AdminBase{Db: db, Rdb: rdb}

   adminGroup := r.Group("/admin")

   {
	adminGroup.POST("/create", adminBase.Signup)
   }

}