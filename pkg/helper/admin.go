package helper

import (
	"errors"
	"huddle/pkg/model"
	"huddle/pkg/utility"
	"net/http"

	"gorm.io/gorm"
)

func CheckIfAdminExist(email, username string, db *gorm.DB ) (model.Admins, *utility.AppError){
    var admin model.Admins
	err := db.Where("email = ? OR username = ?", email, username).First(&admin).Error

	if err != nil {
	   if errors.Is(err, gorm.ErrRecordNotFound){
		 return model.Admins{}, nil
	   }else{
		return model.Admins{}, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Internal Server Error", Err: err}
	   }
}

 return admin, nil

}

            