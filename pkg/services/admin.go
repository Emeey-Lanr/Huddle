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
		return nil, helperErr
	}

	if exists && helperErr == nil {
		return nil, &utility.AppError{StatusCode: http.StatusConflict, Message: "User already exists", Err: errors.New("admin user already exists")}
	}

	hashedPassword, err := utility.HashPassword(req.Password)
	if err != nil {
		return nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Internal Server Error", Err: err}
	}

	id := utility.GenerateUUID()

	err = db.Create(&model.Admins{
		Id:       id,
		Name:     req.Name,
		Username: req.Username,
		Email:    req.Email,
		Password: hashedPassword,
	}).Error

	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return nil, &utility.AppError{StatusCode: http.StatusConflict, Message: "User already exists", Err: err}
	}
	if err != nil {
		log.Printf("AdminSignup database error: %v", err)
		return nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Could not create admin user", Err: err}
	}

	token, err := utility.CreateToken(id, req.Email, utility.RoleAdmin)
	if err != nil {
		log.Printf("AdminSignup token generation error: %v", err)
		return nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Internal Server Error", Err: err}
	}

	return &token, nil
}

func AdminSignin(db *gorm.DB, req model.AdminSignin) (*string, *utility.AppError) {
	admin, helperErr, exists := helper.CheckIfAdminExist("", req.Username, db)

	if !exists && helperErr != nil {
		return nil, helperErr
	}

	if !exists && helperErr == nil {
		return nil, &utility.AppError{StatusCode: http.StatusUnauthorized, Message: "Invalid credentials", Err: errors.New("invalid credentials")}
	}

	err := utility.ComparePassword(admin.Password, req.Password)
	if err != nil {
		return nil, &utility.AppError{StatusCode: http.StatusUnauthorized, Message: "Invalid credentials", Err: err}
	}

	token, err := utility.CreateToken(admin.Id, admin.Email, utility.RoleAdmin)
	if err != nil {
		return nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Internal Server Error", Err: err}
	}

	return &token, nil
}