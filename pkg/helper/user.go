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

func (b *Base) CheckIfUserExist() *utility.AppError {
	var userFound model.Users
	err := b.Db.Where("email = ? OR name = ?", b.Email, b.Name).First(&userFound).Error

  if err == nil{
	return &utility.AppError{Status: http.StatusForbidden , Message:"Email or Name already exist", Err: err}
  } 

  if errors.Is(err, gorm.ErrRecordNotFound){
	return nil
  }

  return nil

}


func(b *Base) HashPassword()(string, *utility.AppError){
	hashedPassword, err:= bcrypt.GenerateFromPassword([]byte(b.Password), bcrypt.DefaultCost);

	if err != nil{
		    
	 return "", &utility.AppError{Status: 500, Message:"Error hasing password", Err: err}
	}

	return string(hashedPassword), nil
}