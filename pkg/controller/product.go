package controller

import (
	"errors"
	"huddle/pkg/model"
	"huddle/pkg/response"
	"huddle/pkg/services"
	"huddle/pkg/utility"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	// "log"
)

type ProductBase struct {
	Db *gorm.DB
	Rdb *redis.Client
}



func (b *ProductBase) Add (c *gin.Context){
	adminId, adminExists :=  c.Get("adminId")
    storePublicId, storeIdExists := c.Get("X-Store-Id")
 	 if !adminExists || !storeIdExists{
		response.Error(c, http.StatusInternalServerError, "Internal Server Error", errors.New("Internal Server Error"))
		return
	 }

	 var req model.Products
	 if err := c.ShouldBindJSON(&req); err != nil{
		response.Error(c, http.StatusBadRequest, "Bad Request", err)
		return
	 } 

	 if err := utility.Validate.Struct(&req); err != nil{
		 err := utility.ValidationErrors(err)
		 if err != nil{
			response.Error(c, http.StatusBadRequest, "Bad Request", err)
			return 
		 }
	 }

  product, serviceErr := services.AddProduct(req, b.Db, b.Rdb, adminId.(string), storePublicId.(string))
 if serviceErr != nil{
	response.Error(c, serviceErr.StatusCode, serviceErr.Message, serviceErr.Err)
	return
 }

 response.Created(c, "Product Added Succesfully", gin.H{"id":product.PublicId})
} 


func (b *ProductBase) AddImages (c *gin.Context) {
   productId :=	c.Param("id")

   form, err :=  c.MultipartForm()
   if err != nil{
     response.Error(c, http.StatusBadGateway, "Bad Request", err)
	 return
   }
     files := form.File["image"]
 serviceErr := services.AddImages(b.Db, b.Rdb, files, productId)
	if serviceErr != nil{
		response.Error(c,serviceErr.StatusCode, serviceErr.Message, serviceErr.Err)
		return 
	}
   
	response.Created(c, "Upload succesfully Image/s succesfully create", nil)
}