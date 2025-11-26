package helper

import (
	"errors"
	"huddle/pkg/model"
	"huddle/pkg/utility"
	"net/http"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type Base struct {
	Db *gorm.DB
	Email string
	Name string
	Password string
}

func (b *Base) CheckIfUserExist() (model.Users, *utility.AppError ){
	var userFound model.Users
	err := b.Db.Where("email = ? OR name = ?", b.Email, b.Name).First(&userFound).Error

 
  if errors.Is(err, gorm.ErrRecordNotFound){
	return userFound,  nil
  }

   if err == nil{
	return userFound, &utility.AppError{Status: http.StatusForbidden , Message:"Email or Name already exist", Err: err}
  } 


  return userFound, nil

}


func(b *Base) HashPassword()(string, *utility.AppError){
	hashedPassword, err:= bcrypt.GenerateFromPassword([]byte(b.Password), bcrypt.DefaultCost);

	if err != nil{
		    
	 return "", &utility.AppError{Status: 500, Message:"Error hasing password", Err: err}
	}

	return string(hashedPassword), nil
}


func (b *Base) VerifyPassword (hashedPassword string) *utility.AppError{
    err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(b.Password))

	if err != nil {
        return &utility.AppError{Status: http.StatusBadRequest, Message: "Invalid email or password", Err: err}
	}

	return nil
}