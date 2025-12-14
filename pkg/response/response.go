package response

import (
 	"github.com/gin-gonic/gin"
)


 

func Success (c *gin.Context, message string, data interface{}){
	c.JSON(200, gin.H{
		"success":true,
		"message": message,
		"data": data,
	})
}

func Created (c *gin.Context, message string, data interface{}){
	c.JSON(201, gin.H{
		"success":true,
		"message": message,
		"data": data,
	})
}

func Error (c *gin.Context, statusCode int, message string, err error){
	c.JSON(statusCode, gin.H{
		"success":false,
		"message": message,
		"error": err.Error(),
	})
}

