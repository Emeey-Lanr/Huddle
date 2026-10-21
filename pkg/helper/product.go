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
           return nil, &utility.AppError{StatusCode: http.StatusNotFound, Message: "Store not found", Err: errors.New("Invalid StoreId")}
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
     return &utility.AppError{StatusCode: http.StatusForbidden, Message: "Ownership not verified", Err: errors.New("admin does not own this store")}
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
		  return  nil, &utility.AppError{StatusCode: http.StatusNotFound, Message: "Product not found", Err: err}
	    }else{
		 return  nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Internal Server Err", Err: err}
		}
		}
        
		 if err :=  rdb.Set(connection.Ctx, publicId, product.Id.String(), 30 * time.Minute).Err(); err != nil{
        return  nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Internal Server Err", Err: err}
	 }

	 return &product.Id, nil

	 }
		return nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Internal Server Err", Err: err}
	}

   uuidInternalProductId, err := utility.StringToUUID(internalProductId)
   if err  != nil{
	return  nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Internal Server Err", Err: err}
   }

   return &uuidInternalProductId, nil
}

// VerifyProductOwnership returns the internal product id when the product
// belongs to a store owned by adminId, and 403 otherwise.
func VerifyProductOwnership(db *gorm.DB, rdb *redis.Client, publicProductId, adminId string) (*uuid.UUID, *utility.AppError) {
	adminUUID, err := utility.StringToUUID(adminId)
	if err != nil {
		return nil, &utility.AppError{StatusCode: http.StatusUnauthorized, Message: "Invalid admin id", Err: err}
	}

	internalProductId, helperErr := ValidateProductId(db, rdb, publicProductId)
	if helperErr != nil {
		return nil, helperErr
	}

	var count int64
	err = db.Model(&model.Products{}).
		Joins("JOIN stores ON stores.id = products.store_id").
		Where("products.id = ? AND stores.admin_id = ?", *internalProductId, adminUUID).
		Count(&count).Error
	if err != nil {
		return nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Internal Server Error", Err: err}
	}
	if count == 0 {
		return nil, &utility.AppError{StatusCode: http.StatusForbidden, Message: "Ownership not verified", Err: errors.New("admin does not own this product")}
	}

	return internalProductId, nil
}

const (
	DefaultProductLimit = 20
	MaxProductLimit     = 100
)

var productSortColumns = map[string]string{"created_at": "created_at", "price": "price", "name": "name"}

// NormalizeProductQuery fills in paging defaults, caps the limit and rejects bad filters.
func NormalizeProductQuery(q *model.ProductQuery) *utility.AppError {
	if q.Page < 1 {
		q.Page = 1
	}
	if q.Limit < 1 {
		q.Limit = DefaultProductLimit
	}
	if q.Limit > MaxProductLimit {
		q.Limit = MaxProductLimit
	}
	if q.MinPrice < 0 || q.MaxPrice < 0 {
		return &utility.AppError{StatusCode: http.StatusBadRequest, Message: "Price filters cannot be negative", Err: errors.New("negative price filter")}
	}
	if q.MaxPrice > 0 && q.MinPrice > q.MaxPrice {
		return &utility.AppError{StatusCode: http.StatusBadRequest, Message: "min_price cannot be greater than max_price", Err: errors.New("min_price > max_price")}
	}
	if q.Sort == "" {
		q.Sort = "created_at"
	}
	if _, ok := productSortColumns[q.Sort]; !ok {
		return &utility.AppError{StatusCode: http.StatusBadRequest, Message: "sort must be one of created_at, price, name", Err: errors.New("invalid sort")}
	}
	q.Order = strings.ToLower(q.Order)
	if q.Order == "" {
		q.Order = "desc"
	}
	if q.Order != "asc" && q.Order != "desc" {
		return &utility.AppError{StatusCode: http.StatusBadRequest, Message: "order must be asc or desc", Err: errors.New("invalid order")}
	}
	return nil
}

// GetAllProductQueryChain applies the store and filter conditions (no paging or sorting).
func GetAllProductQueryChain(db *gorm.DB, storeInternalId uuid.UUID, queryData model.ProductQuery)(*gorm.DB){
  query := db.Model(&model.Products{})

  query = query.Where("store_id = ?", storeInternalId)

  if queryData.MinPrice > 0 {
	query = query.Where("price >= ?", queryData.MinPrice)
  }

  if queryData.MaxPrice > 0 {
	query = query.Where("price <= ?", queryData.MaxPrice)
  }

  if queryData.Active != nil {
	query = query.Where("active = ?", *queryData.Active)
  }

  return query
}

// PageProductQuery adds sorting and paging to a normalized query.
func PageProductQuery(query *gorm.DB, q model.ProductQuery) *gorm.DB {
	return query.
		Order(productSortColumns[q.Sort] + " " + q.Order).
		Order("id").
		Offset((q.Page - 1) * q.Limit).
		Limit(q.Limit)
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



// deleteFromCloudinary is a variable so tests can run without Cloudinary.
var deleteFromCloudinary = cloudinary.DeleteImageFromCloudinary

func DeleteImagesInCloudinary (imgPublicId []string)*utility.AppError{
	for _, val := range imgPublicId{
      if err :=  deleteFromCloudinary(val); err != nil{
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
	 // Remove the uploaded files so they don't sit unused in Cloudinary.
	 if  helperErr := DeleteImagesInCloudinary(imgPublicId); helperErr != nil{
        log.Println(err, imgPublicId)  //we need the log so we can manually delete failed images
	}
	return &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Couldn't save images", Err: err}
  }

 return nil
}