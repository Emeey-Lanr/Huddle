package services

import (
	"errors"
	"fmt"
	"huddle/pkg/helper"
	"huddle/pkg/model"
	"huddle/pkg/utility"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func CreateStore(db *gorm.DB, req model.Stores, adminId uuid.UUID, email string) (*model.Stores, *string, *utility.AppError) {
	exists, _, err := helper.StoreExist(db, adminId, req.StoreName)

	if err != nil && !exists {
		return nil, nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Internal Server Error", Err: err}
	}

	if err == nil && exists {
		return nil, nil, &utility.AppError{StatusCode: http.StatusConflict, Message: "Store name already exists", Err: errors.New("store name already exists")}
	}

	apiKey, err := utility.GenerateAPIKey()
	if err != nil {
		return nil, nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Internal Server Error", Err: err}
	}

	hashedApiKey := utility.HashAPIKey(apiKey)

	storeId := fmt.Sprintf("str-%s", strings.ReplaceAll(utility.GenerateUUID().String(), "-", ""))

	data := model.Stores{
		ID:               utility.GenerateUUID(),
		AdminId:          adminId,
		StoreName:        req.StoreName,
		StoreDescription: req.StoreDescription,
		ApiKey:           hashedApiKey,
		StoreId:          storeId,
	}

	if err := db.Create(&data).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return nil, nil, &utility.AppError{StatusCode: http.StatusConflict, Message: "Store name already exists", Err: err}
		}
		return nil, nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Internal Server Error", Err: err}
	}

	return &data, &apiKey, nil
}