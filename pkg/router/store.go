package router


import (
	"github.com/gin-gonic/gin"
		"gorm.io/gorm"
)

func StoreRouter(r *gin.Engine, db *gorm.DB){

	storeRoutes := r.Group("/store")

	{
		storeRoutes.POST("/create")
	}

}