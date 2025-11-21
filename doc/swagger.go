package doc

import (
	"github.com/gin-gonic/gin"
		ginSwagger "github.com/swaggo/gin-swagger"
   swaggerFiles "github.com/swaggo/files"
)

func SwaggerDocs (r *gin.Engine){
	r.Static("/docs", "./docs")
	url := ginSwagger.URL("/docs/swagger.yaml")
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, url))

}