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
		return nil, &utility.AppError{StatusCode: http.StatusConflict, Message: "Product already exists", Err: errors.New("Product already exists")}
	 }

	 productId := utility.GenerateUUID()
     productPublicId := fmt.Sprintf("prod_%s", strings.ReplaceAll(utility.GenerateUUID().String(), "-", "")) 

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
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return nil, &utility.AppError{StatusCode: http.StatusConflict, Message: "Product already exists", Err: err}
		}
      return nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Internal Server Error", Err: err}
	}

	 // Cache public -> internal id only once the product exists.
	 if  err := rdb.Set(context.Background(),productPublicId, productId.String(), 30 * time.Minute).Err(); err != nil{
		return nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Internal Server Err", Err: err}
	 }

  return &product, nil
}


func  AddImages (db *gorm.DB, rdb *redis.Client, images []*multipart.FileHeader,  publicProductId, adminId string) (*model.ImagesCreatedResponse, *utility.AppError){
	internalProductId, helperErr := helper.VerifyProductOwnership(db, rdb, publicProductId, adminId)
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

func GetAllProducts (db *gorm.DB, rdb *redis.Client, queryData model.ProductQuery, adminId string) (*model.ProductListResponse, *utility.AppError) {
	if helperErr := helper.NormalizeProductQuery(&queryData); helperErr != nil {
		return nil, helperErr
	}

	adminIdUUID, err := utility.StringToUUID(adminId)
	if err != nil {
		return nil, &utility.AppError{StatusCode: http.StatusUnauthorized, Message: "Invalid admin id", Err: err}
	}

     internalStoreId, helperErr :=   helper.GetInternalStoreId(rdb, db, queryData.StoreId);
	 if helperErr != nil {
		return nil,  helperErr
	 }

	 if helperErr := helper.VerifyOwnerShip(db, *internalStoreId, adminIdUUID); helperErr != nil {
		return nil, helperErr
	 }

	   query :=  helper.GetAllProductQueryChain(db, *internalStoreId, queryData)

	   var total int64
	   if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		 return nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Internal Server Err", Err: err}
	   }

	    products := []model.ProductResponse{}
	   err = helper.PageProductQuery(query, queryData).Find(&products).Error
	   if err != nil{
		 return nil,  &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Internal Server Err", Err: err}
	   }

   return  &model.ProductListResponse{Items: products, Page: queryData.Page, Limit: queryData.Limit, Total: total}, nil
}


func GetAProduct (db *gorm.DB, rdb *redis.Client, publicId, adminId string) (*model.ProductResponse, *[]model.ImagesResponse, *utility.AppError){
	 internalId, helperErr := helper.VerifyProductOwnership(db, rdb, publicId, adminId)
	 if helperErr != nil{
		return nil, nil, helperErr
	 }
	
 var product model.ProductResponse
	 err := db.Model(&model.Products{}).Omit("id, store_id").Where("id = ? AND public_id = ?", internalId, publicId).First(&product).Error 
	 if err != nil{
		 if errors.Is(err, gorm.ErrRecordNotFound){
               	return nil, nil,  &utility.AppError{StatusCode: http.StatusNotFound, Message: "Product not found", Err:err}
		 }
		 return nil, nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Internal Server Error", Err:err}
	 }

	 var productImages []model.ImagesResponse
	 err = db.Model(&model.Images{}).Select("public_id, image_url").Where("product_id = ?", internalId).Find(&productImages).Error
	 if err != nil{
		return nil, nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Internal Server Error", Err:err}

	 }
	
	
   return &product, &productImages, nil

}

func DeleteProduct (db *gorm.DB, rdb *redis.Client, publicProductId, adminId string) *utility.AppError{
	  internalProductId, helperErr := helper.VerifyProductOwnership(db, rdb, publicProductId, adminId)
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

func UpdateProduct(db *gorm.DB, rdb *redis.Client, publicProductId, adminId string, req model.ProductUpdate) (*model.ProductResponse, *utility.AppError) {
	if req.Price != nil && req.Price.IsNegative() {
		return nil, &utility.AppError{StatusCode: http.StatusBadRequest, Message: "Price cannot be negative", Err: errors.New("negative price")}
	}
	if req.Currency != "" && len(req.Currency) != 3 {
		return nil, &utility.AppError{StatusCode: http.StatusBadRequest, Message: "currency must be exactly 3 characters", Err: errors.New("invalid currency")}
	}
	if req.StockQuantity != nil && *req.StockQuantity < 0 {
		return nil, &utility.AppError{StatusCode: http.StatusBadRequest, Message: "Stock quantity cannot be negative", Err: errors.New("negative stock")}
	}

	internalProductId, helperErr := helper.VerifyProductOwnership(db, rdb, publicProductId, adminId)
	if helperErr != nil {
		return nil, helperErr
	}

	updates := make(map[string]interface{})
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.Price != nil {
		updates["price"] = *req.Price
	}
	if req.Currency != "" {
		updates["currency"] = req.Currency
	}
	if req.StockQuantity != nil {
		updates["stock_quantity"] = *req.StockQuantity
	}
	if req.Active != nil {
		updates["active"] = *req.Active
	}
	updates["updated_at"] = time.Now().UTC()

	if len(updates) > 0 {
		if err := db.Model(&model.Products{}).Where("id = ?", internalProductId).Updates(updates).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return nil, &utility.AppError{StatusCode: http.StatusConflict, Message: "Another product in this store already has that name", Err: err}
			}
			return nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Could not update product", Err: err}
		}
	}

	var product model.ProductResponse
	if err := db.Model(&model.Products{}).Omit("id, store_id").Where("id = ?", internalProductId).First(&product).Error; err != nil {
		return nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Could not fetch updated product", Err: err}
	}

	return &product, nil
}