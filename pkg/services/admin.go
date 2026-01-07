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

func AdminSignup(db *gorm.DB, req model.Admins) (*string, *utility.AppError) {
	 _, helperErr, exists := helper.CheckIfAdminExist(req.Email, req.Username, db)
   
	if !exists && helperErr != nil {
		return nil,  helperErr 
	}


	if exists  && helperErr == nil {
       return nil, &utility.AppError{StatusCode: http.StatusUnauthorized, Message: "User already exists", Err:errors.New("User already exists")}
	}
    
   hashedPassword, err := utility.HashPassword(req.Password)
   if err != nil {
         return nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Internal serval Error", Err: err }
   }

   uuid := utility.GenerateUUID()

  err = db.Create(&model.Admins{Id:uuid, Name: req.Name, Username: req.Username, Email: req.Email, Password: hashedPassword}).Error
  if err != nil{
	log.Println(err)
	return nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Couldn't create user", Err: err}
  }

  token, err := utility.CreateToken(uuid, req.Email)
  if err != nil{
	log.Println(err)
	return nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Internal Server Error", Err: err}
  }



 return &token, nil
}


func AdminSignin (db *gorm.DB, req model.AdminSignin) (*string, *utility.AppError){
	admin, helperErr, exists :=  helper.CheckIfAdminExist("", req.Username, db)

      
	 if !exists && helperErr != nil{
		return nil, helperErr
	 }
	 
	if !exists && helperErr == nil {
     return nil, &utility.AppError{StatusCode: http.StatusUnauthorized, Message: "Invalid Crendentials", Err: errors.New("Invalid Crendentials")}
	}

   err := utility.ComparePassword(admin.Password, req.Password)
   if err != nil{
	 return nil, &utility.AppError{StatusCode: http.StatusUnauthorized, Message: "Invalid Crendentials", Err: err}
   }


  token, err := utility.CreateToken(admin.Id, admin.Email)   
  if err != nil{
	return nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Internal Server Error", Err: err}
  }

 return &token, nil
}