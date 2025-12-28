package router

import (
	"huddle/doc"
	
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"github.com/redis/go-redis/v9"
)




func RoutesSetup (db *gorm.DB, rdb *redis.Client) *gin.Engine{

	r := gin.Default()

	// Entry Route
	r.GET("/", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"name":"Welcome to Huddle E-commerce API"})
	})

	//All Routes
	AdminRouter(r, db, rdb)
	StoreRouter(r, db, rdb)
	ProductRouter(r, db, rdb)

 // //serves app docs and swagger ui
  doc.SwaggerDocs(r)



	return r
   
}