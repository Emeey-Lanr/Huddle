package helper

import (
	"errors"
	"huddle/pkg/model"
     "github.com/google/uuid"
	"gorm.io/gorm"
)




func StoreExist (db *gorm.DB, adminId uuid.UUID, storeName string)(bool,  model.Stores, error ){
    var store model.Stores
  err := db.Where("admin_id = ? AND store_name = ?",  adminId, storeName).First(&store).Error

   
  if err != nil{
	if errors.Is(err,  gorm.ErrRecordNotFound){
     return false, store,  nil
  }
      return false, store, err //checks for db or syntax error

  }
  
  
   return true, store, nil
}