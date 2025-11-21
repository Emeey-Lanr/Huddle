package controller

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)


type Base struct {
	Db *gorm.DB
}


func (base *Base) Signup (c *gin.Context){
   
	
}