package middlware

import (
	"fmt"


	"github.com/gin-gonic/gin"
)


func AuthoriseUser (c *gin.Context){

	fmt.Print("Hello World")
	c.Next()

}