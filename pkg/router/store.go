package router

import (
	"huddle/pkg/controller"
	"huddle/pkg/middleware"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func StoreRouter(r *gin.Engine, db *gorm.DB, rdb *redis.Client){
     storeBase := &controller.StoreBase{Db: db, Rdb: rdb}

	storeRoutes := r.Group("/store")
	storeRoutes.Use(middleware.AuthMiddleware)
	
	{
		storeRoutes.POST("/create", storeBase.Create)
	}

}