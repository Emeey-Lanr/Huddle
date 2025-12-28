package services

import (
	"errors"
	"fmt"
	"huddle/pkg/helper"
	"huddle/pkg/model"
	"huddle/pkg/utility"
	"net/http"

	"github.com/google/uuid"
	"gorm.io/gorm"
)



func CreateStore (db *gorm.DB, req model.Stores, adminId uuid.UUID, email string) (*model.Stores, *string,  *utility.AppError){

	exists, _, err := helper.StoreExist(db, adminId, req.StoreName) 
	fmt.Println(exists, err)

	if err != nil && !exists{
		return  nil, nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Internal Server Error", Err: err}
	}

	if err == nil  && exists{
		return nil, nil, &utility.AppError{StatusCode: http.StatusUnauthorized, Message: "Store name already exits",  Err: errors.New("Store name already exists")}
	}


    apiKey, err := utility.GenerateAPIKey()
	if err != nil{
		return nil,nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Internal Server Error", Err: err}
	}
    
	hashedApiKey, err := utility.HashPassword(apiKey) // using the password utility for hashing api key
    if err != nil{
		return nil, nil,  &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Internal Server Error", Err: err}
	}

   storeId :=  fmt.Sprintf("str-%v", utility.GenerateUUID())

    data := model.Stores{ID: utility.GenerateUUID(), AdminId:adminId, StoreName:req.StoreName, StoreDescription: req.StoreDescription, ApiKey: hashedApiKey, StoreId: storeId }
    if err := db.Create(&data).Error; err != nil{
		return nil,nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Internal Sever Error", Err: err}
	}
 
   
  return &data, &apiKey, nil
}