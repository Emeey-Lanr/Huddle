package services

import (
	"errors"
	"huddle/pkg/helper"
	"huddle/pkg/model"
	"huddle/pkg/utility"
	"net/http"

	"log"

	"gorm.io/gorm"
)


func Signup (db *gorm.DB, req model.Users) (string, *utility.AppError){
	
	helperFunc := &helper.Base{Db: db, Name:req.Name, Email: req.Email, Password: req.Password}
	
  _, err := helperFunc.CheckIfUserExist();
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



func Signin (db *gorm.DB, req model.Signin)(string, *utility.AppError){
   //check if email exist, collect the user data
    helperFunc := &helper.Base{Db: db, Email: req.Email, Name: "", Password: req.Password}
	
	
	
	userFound, appErr := helperFunc.CheckIfUserExist()
	if appErr == nil{
		// the helper function used returns nil if a user is not found
	// so if it's nil, the user doesn't exist in our db and can't signin
	  return "", &utility.AppError{Status: http.StatusForbidden, Message:"Invalid Email or Password", Err: errors.New("User not found")}
	}


	 if appErr := helperFunc.VerifyPassword(userFound.Password); appErr != nil{
		return "", appErr
	 }


   
	 token, appErr := utility.CreateToken(userFound.Id, userFound.Email)
     
	 if appErr != nil {
		return "", appErr
	 }

	 return token, nil 

}