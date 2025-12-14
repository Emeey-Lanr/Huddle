package helper

import (
	"errors"
	"huddle/pkg/model"

	"gorm.io/gorm"
)

func CheckIfAdminExist(email, username string, db *gorm.DB ) (bool, model.Admins){
    var admin model.Admins
	err := db.Where("email = ? OR username = ?").First(&admin).Error

	if errors.Is(err, gorm.ErrRecordNotFound){
       return false, model.Admins{}
	}

	return true, admin

}

