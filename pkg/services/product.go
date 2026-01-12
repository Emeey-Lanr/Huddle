package services

import (
	"context"
	"errors"
	"fmt"
	"huddle/internal/cloudinary"
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


func  AddImages (db *gorm.DB, rdb *redis.Client, images []*multipart.FileHeader,  publicProductId string) (*model.ImagesCreatedResponse, *utility.AppError){
	internalProductId, helperErr := helper.ValidateProductId(db,rdb, publicProductId)
	 if helperErr != nil{
		return nil, helperErr
	 }	 
		 
	if  helperErr := helper.ValidateImages(images); helperErr != nil{
		return nil,  helperErr
	}

	uploadedImgPublicId, uploadedImgSecureURL, helperErr :=  helper.UploadImagesToCloudinary(images)
	if helperErr != nil{
		if  len(uploadedImgPublicId) > 0{
            if helperErr := helper.DeleteImagesInCloudinary(uploadedImgPublicId); helperErr != nil{
				 log.Println(images) //we need the log so we can manually delete failed images
				  return nil, helperErr
			}
			
		}
		return nil, helperErr
	}

	fmt.Println(len(uploadedImgPublicId), len(uploadedImgSecureURL))
     if len(uploadedImgPublicId) < 1 {
		return nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Couldn't upload images", Err: errors.New("An error coccured uploading images")}
	 }

	 if helperErr :=  helper.SaveImagesToDb(db, uploadedImgPublicId,uploadedImgSecureURL, *internalProductId); helperErr != nil{
        return nil, helperErr
	 }
	
	 imageResponse := model.ImagesCreatedResponse{ProductId: publicProductId, Images: []model.ImageData{}}
	 
	  for id, val := range uploadedImgPublicId {
			  imageResponse.Images = append(imageResponse.Images, model.ImageData{ImageId: val, ImageURL: uploadedImgSecureURL[id]})
	  }

   

   return &imageResponse, nil
}

func GetAllProducts (db *gorm.DB, rdb *redis.Client, queryData model.ProductQuery) (*[]model.Products, *utility.AppError) {

     internalStoreId, helperErr :=   helper.GetInternalStoreId(rdb, db, queryData.StoreId);
	 if helperErr != nil {
		return nil,  helperErr
	 }

	   query :=  helper.GetAllProductQueryChain(db, *internalStoreId, queryData)
	    var  products  []model.Products
	   err := query.Find(&products).Error
	   if err != nil{
		 return nil,  &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Internal Server Err", Err: err}
	   }
	  
   return  &products, nil
} 

func DeleteProduct (db *gorm.DB, rdb *redis.Client, publicProductId string) *utility.AppError{
	  internalProductId, helperErr := helper.ValidateProductId(db,rdb, publicProductId)
	 if helperErr != nil{
		return  helperErr
	 }	 

	  var images []model.Images
	if err := db.Where("product_id = ?", internalProductId).Find(&images).Error; err != nil {
      return &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Couldn't delete products, unable to retreive image", Err: err}
	}

	if len(images) > 0{
		for _, val := range images{
             if err := cloudinary.DeleteImageFromCloudinary(val.PublicID); err != nil{
				return &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Couldn't delete products: err deleting images", Err: err}
			 }
		}
	}
	 

	if  err := db.Where("id = ?", internalProductId).Delete(&model.Products{}).Error; err != nil{
		return &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Couldn't delete products", Err: err}
	}
	 
   
   



	if err := rdb.Del(context.Background(), publicProductId).Err(); err != nil{
		return &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Couldn't delete products", Err: err}
	}
	return nil

}