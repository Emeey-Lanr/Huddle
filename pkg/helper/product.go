package helper

import (
	"errors"
	"fmt"
	"huddle/db/connection"
	"huddle/pkg/model"
	"huddle/pkg/utility"
		"huddle/internal/cloudinary"
	"log"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	"github.com/google/uuid"

	"github.com/redis/go-redis/v9"
	_ "golang.org/x/image/webp"
	"gorm.io/gorm"
)

func GetInternalStoreId(rdb *redis.Client, db *gorm.DB,  storePublicId string ) (*uuid.UUID, *utility.AppError){
	
   val, err := rdb.Get(connection.Ctx, storePublicId).Result()
   if err == redis.Nil{
	   // caching when redis is empty
	      var store model.Stores
	   if  err :=  db.Where("store_id = ?", storePublicId).First(&store).Error; err != nil{
		 if errors.Is(err, gorm.ErrRecordNotFound){
           return nil, &utility.AppError{StatusCode: http.StatusUnauthorized, Message: "Store Creation not allowed", Err: errors.New("Invalid StoreId")}
		 }
		 return nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Internal Server", Err: err}
	   }

	   

	 if err := rdb.Set(connection.Ctx, storePublicId, store.ID.String(), 30 * time.Minute).Err(); err != nil{
		return nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Internal Server", Err: err}
	 }  
	return &store.ID, nil 

   }else if err != nil{
	return nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Internal Server", Err: err}
   }


 uuidStoreId, err := utility.StringToUUID(val)
 if err != nil{
	return nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Internal Server, getting internal store id", Err: err}
 }

   return  &uuidStoreId, nil
   
}


func VerifyOwnerShip (db *gorm.DB, storeId, adminId uuid.UUID) *utility.AppError{
    var store model.Stores
 if err := db.Where("admin_id = ? AND id = ?", adminId, storeId).First(&store).Error; err != nil{
   if errors.Is(err, gorm.ErrRecordNotFound){
     return &utility.AppError{StatusCode: http.StatusUnauthorized, Message: "Status Not Verfied", Err: errors.New("Status Not Verified")}
   }
   return &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Internal Server Error", Err: err}
 }

 return nil
}


func DoesProductExist(db *gorm.DB,name string, storeId uuid.UUID)(*model.Products,bool, *utility.AppError){
	//storeId internal store id
	var product model.Products
	if err := db.Where("store_id = ? AND name = ?",  storeId, name).First(&product).Error; err != nil{
		if errors.Is(err, gorm.ErrRecordNotFound){
			return  nil, false, nil
		}
		return nil, false, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Internal Server Error", Err: err}
	}
	return &product, true, nil
	
}


func ValidateProductId(db *gorm.DB, rdb *redis.Client, publicId string)(*uuid.UUID, *utility.AppError){
	var product model.Products
	
	 internalProductId, err :=  rdb.Get(connection.Ctx, publicId).Result()

	 if err != nil{
		if err == redis.Nil{
			if 	err := db.Where("public_id = ?", publicId).First(&product).Error; err != nil{
	      if errors.Is(err, gorm.ErrRecordNotFound){
		  return  nil, &utility.AppError{StatusCode: http.StatusUnauthorized, Message: "Invalid Id", Err: err}
	    }else{
		 return  nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Internal Server Err", Err: err}
		}
		}
        
		 if err :=  rdb.Set(connection.Ctx, publicId, product.Id.String(), 30 * time.Minute).Err(); err != nil{
        return  nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Internal Server Err", Err: err}
	 }

	 return &product.Id, nil

	 }
	}
 

   uuidInternalProductId, err := utility.StringToUUID(internalProductId)
   if err  != nil{
	return  nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Internal Server Err", Err: err}
   }

   return &uuidInternalProductId, nil
}



func ValidateImages(images  []*multipart.FileHeader)*utility.AppError{
	 if len(images) < 1 || len(images) > 5{
		return &utility.AppError{StatusCode: http.StatusBadRequest, Message: "Invalid Image file", Err:errors.New("Image upload should a min of 1 and max of 5") }
	}
	
	for _, val := range images{
		if val.Size  > 5242880{
			return &utility.AppError{StatusCode: http.StatusBadRequest, Message: "Image file size exceeded", Err: errors.New("Each Image max size should be 5mb")}
		}
		
        if val.Size == 0 {
			return &utility.AppError{StatusCode: http.StatusBadRequest, Message: "Image file is below minimum", Err: errors.New("Each Image max size should not be 0 but greater and lesser or equals to 5mb")}
		}
	    
		file, err := val.Open()
		if err != nil{
			return &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Internal Server Error", Err: err}
		}
		_, _, err = image.Decode(file)
		defer file.Close() 

		if err != nil {
			return &utility.AppError{StatusCode: http.StatusBadRequest, Message: "Invalid Image Format", Err: err}
		}

	}
	
   
	return nil
}



func UploadImagesToCloudinary (images []*multipart.FileHeader)([]string,[]string, *utility.AppError){
	uploadedImgPublicId := []string{}
	uploadedImgSecureURL := []string{}


  for _, val := range images {
    result, err :=  cloudinary.UploadImageToCloudinary(val, fmt.Sprintf("img_%s", strings.ReplaceAll(utility.GenerateUUID().String(), "-", "")))
	if err != nil{
		return uploadedImgPublicId,uploadedImgSecureURL, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Couldn't Upload Images", Err: err}
	}
	if result.PublicID == "" || result.SecureURL == ""{
		return nil, nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Couldn't Upload Images", Err: errors.New("Cloudinary returns no public id or url")}
	}
    uploadedImgPublicId, uploadedImgSecureURL = append(uploadedImgPublicId, result.PublicID), append(uploadedImgSecureURL, result.SecureURL)
  }
  return  uploadedImgPublicId, uploadedImgSecureURL, nil
}



func DeleteImagesInCloudinary (imgPublicId []string)*utility.AppError{
	for _, val := range imgPublicId{
      if err :=  cloudinary.DeleteImageFromCloudinary(val); err != nil{
		return &utility.AppError{StatusCode: http.StatusInternalServerError, Message:"Internal Server Error", Err: err}
	  }
	}
	return nil
	
}

func SaveImagesToDb (db *gorm.DB, imgPublicId, imgURL []string , productId uuid.UUID)*utility.AppError{
 err := db.Transaction(func(tx *gorm.DB) error {
   for id, url := range imgURL{
	imgData := model.Images{Id: utility.GenerateUUID(), ProductId: productId, PublicID: imgPublicId[id], ImageURL: url }
	 if  err := tx.Create(&imgData).Error; err != nil{
	  return err
  }

   }

   return nil
  })
  
  if err != nil{
	 if  helperErr := DeleteImagesInCloudinary(imgPublicId); helperErr != nil{
        log.Println(err, imgPublicId)  //we need the log so we can manually delete failed images
	    return &utility.AppError{StatusCode: http.StatusInternalServerError, Message:"Internal Server Error", Err: err}
	}
  }

 return nil
}