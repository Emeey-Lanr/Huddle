package setup

import (
	"huddle/doc"
	routes "huddle/pkg/router"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)




func RoutesSetup (db *gorm.DB) {

	r := gin.Default()

	

	r.GET("/", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"name":"Welcome to Emeey API"})
	})
   routes.UserRouter(r, db)

  doc.SwaggerDocs(r)//serves app docs and swagger ui



	r.Run(":8080")
   
}