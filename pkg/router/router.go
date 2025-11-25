package router

import (
	"huddle/doc"
	
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)




func RoutesSetup (db *gorm.DB) *gin.Engine{

	r := gin.Default()

	// Entry Route
	r.GET("/", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"name":"Welcome to Emeey API"})
	})

	//All Routes
    UserRouter(r, db)

 // //serves app docs and swagger ui
  doc.SwaggerDocs(r)



	return r
   
}