package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type ResponseEnvelope struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

func Success(c *gin.Context, message string, data interface{}) {
	c.JSON(http.StatusOK, ResponseEnvelope{
		Success: true,
		Message: message,
		Data:    data,
	})
}

func Created(c *gin.Context, message string, data interface{}) {
	c.JSON(http.StatusCreated, ResponseEnvelope{
		Success: true,
		Message: message,
		Data:    data,
	})
}

func Error(c *gin.Context, statusCode int, message string, err error) {
	errStr := ""
	if err != nil {
		errStr = err.Error()
	}
	c.JSON(statusCode, ResponseEnvelope{
		Success: false,
		Message: message,
		Error:   errStr,
	})
}
