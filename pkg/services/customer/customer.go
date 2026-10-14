package customer

import (
	"errors"
	"huddle/pkg/helper"
	"huddle/pkg/model"
	"huddle/pkg/utility"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func Signup(db *gorm.DB, rdb *redis.Client, req model.CustomerSignupReq) (*string, *model.CustomerResponse, *utility.AppError) {
	internalStoreId, helperErr := helper.GetInternalStoreId(rdb, db, req.StoreId)
	if helperErr != nil {
		return nil, nil, helperErr
	}

	var existingCustomer model.Customers
	err := db.Where("store_id = ? AND email = ?", *internalStoreId, req.Email).First(&existingCustomer).Error
	if err == nil {
		return nil, nil, &utility.AppError{StatusCode: http.StatusConflict, Message: "Customer with this email already exists in store", Err: errors.New("customer already exists")}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Internal Server Error", Err: err}
	}

	hashedPassword, err := utility.HashPassword(req.Password)
	if err != nil {
		return nil, nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Error processing password", Err: err}
	}

	customerId := utility.GenerateUUID()
	now := time.Now().UTC()

	customer := model.Customers{
		Id:        customerId,
		StoreId:   *internalStoreId,
		Name:      req.Name,
		Email:     req.Email,
		Password:  hashedPassword,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := db.Create(&customer).Error; err != nil {
		return nil, nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Could not create customer", Err: err}
	}

	token, err := utility.CreateToken(customerId, req.Email, utility.RoleCustomer)
	if err != nil {
		return nil, nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Could not generate auth token", Err: err}
	}

	res := model.CustomerResponse{
		Id:        customer.Id,
		Name:      customer.Name,
		Email:     customer.Email,
		CreatedAt: customer.CreatedAt,
	}

	return &token, &res, nil
}

func Signin(db *gorm.DB, rdb *redis.Client, req model.CustomerSigninReq) (*string, *model.CustomerResponse, *utility.AppError) {
	internalStoreId, helperErr := helper.GetInternalStoreId(rdb, db, req.StoreId)
	if helperErr != nil {
		return nil, nil, helperErr
	}

	var customer model.Customers
	err := db.Where("store_id = ? AND email = ?", *internalStoreId, req.Email).First(&customer).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, &utility.AppError{StatusCode: http.StatusUnauthorized, Message: "Invalid credentials", Err: errors.New("invalid credentials")}
		}
		return nil, nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Internal Server Error", Err: err}
	}

	if err := utility.ComparePassword(customer.Password, req.Password); err != nil {
		return nil, nil, &utility.AppError{StatusCode: http.StatusUnauthorized, Message: "Invalid credentials", Err: errors.New("invalid credentials")}
	}

	token, err := utility.CreateToken(customer.Id, customer.Email, utility.RoleCustomer)
	if err != nil {
		return nil, nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Could not generate auth token", Err: err}
	}

	res := model.CustomerResponse{
		Id:        customer.Id,
		Name:      customer.Name,
		Email:     customer.Email,
		CreatedAt: customer.CreatedAt,
	}

	return &token, &res, nil
}

func GetProfile(db *gorm.DB, customerIdStr string) (*model.CustomerResponse, *utility.AppError) {
	customerId, err := utility.StringToUUID(customerIdStr)
	if err != nil {
		return nil, &utility.AppError{StatusCode: http.StatusBadRequest, Message: "Invalid customer ID", Err: err}
	}

	var customer model.Customers
	if err := db.Where("id = ?", customerId).First(&customer).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &utility.AppError{StatusCode: http.StatusNotFound, Message: "Customer not found", Err: err}
		}
		return nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Internal Server Error", Err: err}
	}

	res := model.CustomerResponse{
		Id:        customer.Id,
		Name:      customer.Name,
		Email:     customer.Email,
		CreatedAt: customer.CreatedAt,
	}

	return &res, nil
}