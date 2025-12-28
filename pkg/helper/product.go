package helper

import (
	"errors"
	"huddle/db/connection"
	"huddle/pkg/model"
	"huddle/pkg/utility"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
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
 log.Println(uuidStoreId, val, "JHGFDFGHJK")
 if err != nil{
	return nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Internal Server, getting internal store id", Err: err}
 }
 log.Println(uuidStoreId)
   return  &uuidStoreId, nil
   
}


func VerifyOwnerShip (db *gorm.DB, storeId, adminId uuid.UUID) *utility.AppError{
	log.Println(storeId, adminId)
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
	log.Println(storeId)
	var product model.Products
	if err := db.Where("store_id = ? AND name = ?",  storeId, name).First(&product).Error; err != nil{
		if errors.Is(err, gorm.ErrRecordNotFound){
			return  nil, false, nil
		}
		return nil, false, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Internal Server Error", Err: err}
	}
	return &product, true, nil
	
}