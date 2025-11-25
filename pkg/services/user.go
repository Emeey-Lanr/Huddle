package services

import (

	"huddle/pkg/helper"
	"huddle/pkg/model"
	"huddle/pkg/utility"

	"gorm.io/gorm"
	"log"
)


func Signup (db *gorm.DB, req model.Users) (string, *utility.AppError){
	
	helperFunc := &helper.Base{Db: db, Name:req.Name, Email: req.Email, Password: req.Password}
	
  err := helperFunc.CheckIfUserExist();
 if err !=  nil{
	log.Println(err)
	return "", err
 }

 hashedPassword, err := helperFunc.HashPassword()
 if err != nil{
	log.Println(err)
	return "", err
 }

 uuid := utility.GenerateUUID()


 result := db.Create(&model.Users{Id: uuid, Name: req.Name, Email: req.Email, Password: hashedPassword})
 if result.Error != nil {
   return "", &utility.AppError{Status:500, Message:"Error creating user", Err: result.Error}
 }

 generatedToken, err := utility.CreateToken(uuid, req.Email)
 if err != nil{
	log.Println(err)
	return "", err
 }
 
 return generatedToken, nil
}