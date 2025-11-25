package doc

import (
	"github.com/gin-gonic/gin"
		ginSwagger "github.com/swaggo/gin-swagger"
   swaggerFiles "github.com/swaggo/files"
)

func SwaggerDocs (r *gin.Engine){
	r.Static("/doc", "./doc")
	url := ginSwagger.URL("/doc/swagger.yaml")
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, url))

}