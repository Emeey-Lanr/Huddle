package services

import (
	"errors"
	"fmt"
	"huddle/pkg/helper"
	"huddle/pkg/model"
	"huddle/pkg/utility"
	"net/http"
	"strings"
	"time"

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
func toStoreResponse(s model.Stores) model.StoreResponse {
	return model.StoreResponse{
		StoreId:          s.StoreId,
		StoreName:        s.StoreName,
		StoreDescription: s.StoreDescription,
		CreatedAt:        s.CreatedAt,
		UpdatedAt:        s.UpdatedAt,
	}
}

// ownedStore loads a store by its public id and checks the admin owns it.
func ownedStore(db *gorm.DB, publicStoreId string, adminId uuid.UUID) (*model.Stores, *utility.AppError) {
	var store model.Stores
	if err := db.Where("store_id = ?", publicStoreId).First(&store).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &utility.AppError{StatusCode: http.StatusNotFound, Message: "Store not found", Err: err}
		}
		return nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Internal Server Error", Err: err}
	}
	if store.AdminId != adminId {
		return nil, &utility.AppError{StatusCode: http.StatusForbidden, Message: "Ownership not verified", Err: errors.New("admin does not own this store")}
	}
	return &store, nil
}

func ListStores(db *gorm.DB, adminId uuid.UUID) ([]model.StoreResponse, *utility.AppError) {
	var stores []model.Stores
	if err := db.Where("admin_id = ?", adminId).Order("created_at DESC").Order("id").Find(&stores).Error; err != nil {
		return nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Internal Server Error", Err: err}
	}

	resp := make([]model.StoreResponse, 0, len(stores))
	for _, s := range stores {
		resp = append(resp, toStoreResponse(s))
	}
	return resp, nil
}

func GetStore(db *gorm.DB, publicStoreId string, adminId uuid.UUID) (*model.StoreResponse, *utility.AppError) {
	store, appErr := ownedStore(db, publicStoreId, adminId)
	if appErr != nil {
		return nil, appErr
	}
	resp := toStoreResponse(*store)
	return &resp, nil
}

func UpdateStore(db *gorm.DB, publicStoreId string, adminId uuid.UUID, req model.StoreUpdate) (*model.StoreResponse, *utility.AppError) {
	store, appErr := ownedStore(db, publicStoreId, adminId)
	if appErr != nil {
		return nil, appErr
	}

	updates := map[string]interface{}{}
	if name := strings.TrimSpace(req.StoreName); name != "" {
		updates["store_name"] = name
	}
	if req.StoreDescription != "" {
		updates["store_description"] = req.StoreDescription
	}

	if len(updates) > 0 {
		updates["updated_at"] = time.Now().UTC()
		if err := db.Model(&model.Stores{}).Where("id = ?", store.ID).Updates(updates).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return nil, &utility.AppError{StatusCode: http.StatusConflict, Message: "Store name already exists", Err: err}
			}
			return nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Could not update store", Err: err}
		}
	}

	return GetStore(db, publicStoreId, adminId)
}
