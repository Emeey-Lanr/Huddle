package services

import (
	"errors"
	"huddle/pkg/helper"
	"huddle/pkg/model"
	"huddle/pkg/utility"
	"log"
	"net/http"

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
 
	 product := model.Products{Id: productId, 
		StoreId:*internalStoreId,
		  Name:req.Name, 
		  Description:req.Description,
		   Price:  req.Price,
		    Currency:req.Currency, 
			StockQuantity: req.StockQuantity, 
			Active: true}

		
	if err :=  db.Create(&product).Error;  err != nil{
      return nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Internal Server Error", Err: err}
	}

	log.Println(product)
  return &product, nil
}


