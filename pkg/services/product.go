package services

import (
	"context"
	"errors"
	"fmt"
	"huddle/pkg/helper"
	"huddle/pkg/model"
	"huddle/pkg/utility"
	"log"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)




func AddProduct (req model.Products, db *gorm.DB , rdb *redis.Client, adminId, storePublicId string ) (*model.Products, *utility.AppError){
     adminIdUUID, err := utility.StringToUUID(adminId)
	 if err != nil {
		return nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Internal Server Error, converting in service", Err: err}
	 }

	 internalStoreId, helperErr := helper.GetInternalStoreId(rdb, db, storePublicId)
	 log.Println(internalStoreId)
	 if helperErr != nil{
       return nil, helperErr
	 }

	
    
	 if helperErr := helper.VerifyOwnerShip(db, *internalStoreId, adminIdUUID); helperErr != nil{
		return nil,helperErr
	 }


	  _, exists, helperErr := helper.DoesProductExist(db, req.Name, *internalStoreId)
     if  !exists && helperErr != nil{
		return nil, helperErr
	 }

	 if exists && helperErr == nil{
		return nil, &utility.AppError{StatusCode: http.StatusUnauthorized, Message: "Product already exists", Err: errors.New("Product already exists")}
	 }

	 productId := utility.GenerateUUID()
     productPublicId := fmt.Sprintf("prod_%s", strings.ReplaceAll(utility.GenerateUUID().String(), "-", "")) 
	 
	 // Redis
	 if  err := rdb.Set(context.Background(),productPublicId, productId.String(), 30 * time.Minute).Err(); err != nil{
		return nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Internal Server Err", Err: err}
	 }

	 product := model.Products{Id: productId, 
		StoreId:*internalStoreId,
		PublicId: productPublicId,
		  Name:req.Name, 
		  Description:req.Description,
		   Price:  req.Price,
		    Currency:req.Currency, 
			StockQuantity: req.StockQuantity, 
			Active: true}

		
	if err :=  db.Create(&product).Error;  err != nil{
      return nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Internal Server Error", Err: err}
	}

  return &product, nil
}


func  AddImages (db *gorm.DB, rdb *redis.Client, images []*multipart.FileHeader,  publicProductId string) (*utility.AppError){
	 internalProductId, helperErr := helper.ValidateProductId(db,rdb, publicProductId)
	 if helperErr != nil{
		return helperErr
	 }	 
		 
    
	if  helperErr := helper.ValidateImages(images); helperErr != nil{
		return helperErr
	}

	uploadedImages, helperErr :=  helper.UploadImagesToCloudinary(images)
	if helperErr != nil{
		if  len(*uploadedImages) > 0{
            if helperErr := helper.DeleteImagesInCloudinary(uploadedImages); helperErr != nil{
				 log.Println(images) //we need the log so we can manually delete failed images
				  return helperErr
			}
			
		}
		return helperErr
	}

	 if helperErr :=  helper.SaveImagesToDb(db, uploadedImages, *internalProductId); helperErr != nil{
        return helperErr
	 }
	 

   return nil
}

