package services

import (
	"errors"
	"huddle/pkg/helper"
	"huddle/pkg/model"
	"huddle/pkg/utility"
	"log"
	"net/http"

	"gorm.io/gorm"
)

func AdminSignup(db *gorm.DB, req model.Admins) (string, *utility.AppError) {
	adminExist, _ := helper.CheckIfAdminExist(req.Email, req.Username, db)
	if !adminExist {
		return "", &utility.AppError{StatusCode:http.StatusForbidden,  Message:"User alredy exist", Err: errors.New("User already exists") }
	}
    
   hashedPassword, err := utility.HashPassword(req.Password)
   if err != nil {
         return "", &utility.AppError{StatusCode: http.StatusBadGateway, Message: "Error occured creating user", Err: err }
   }

   uuid := utility.GenerateUUID()

  err = db.Create(&model.Admins{Id:uuid, Name: req.Name, Username: req.Username, Email: req.Email, Password: hashedPassword}).Error
  if err != nil{
	log.Println(err)
	return "", &utility.AppError{StatusCode: http.StatusBadRequest, Message: "Unable to create an account", Err: err}
  }

  token, err := utility.CreateToken(uuid, req.Email)
  if err != nil{
	log.Println(err)
	return "", &utility.AppError{StatusCode: http.StatusConflict, Message: "Unable to create an account", Err: err}
  }



 return token, nil
}