package services

import (
	"errors"
	"huddle/pkg/helper"
	"huddle/pkg/model"
	"huddle/pkg/utility"
	"net/http"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Storefront services back the public routes. The store is already resolved
// from X-Store-Key, and only active products are visible.

func GetStorefrontStore(db *gorm.DB, storeId uuid.UUID) (*model.StorefrontStore, *utility.AppError) {
	var store model.Stores
	if err := db.Where("id = ?", storeId).First(&store).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &utility.AppError{StatusCode: http.StatusNotFound, Message: "Store not found", Err: err}
		}
		return nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Internal Server Error", Err: err}
	}
	return &model.StorefrontStore{StoreId: store.StoreId, StoreName: store.StoreName, StoreDescription: store.StoreDescription}, nil
}

func ListStorefrontProducts(db *gorm.DB, storeId uuid.UUID, sq model.StorefrontProductQuery) (*model.ProductListResponse, *utility.AppError) {
	active := true
	q := model.ProductQuery{
		Page: sq.Page, Limit: sq.Limit, MinPrice: sq.MinPrice, MaxPrice: sq.MaxPrice,
		Sort: sq.Sort, Order: sq.Order, Active: &active,
	}
	if appErr := helper.NormalizeProductQuery(&q); appErr != nil {
		return nil, appErr
	}

	query := helper.GetAllProductQueryChain(db, storeId, q)
	var total int64
	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Internal Server Error", Err: err}
	}

	products := []model.ProductResponse{}
	if err := helper.PageProductQuery(query, q).Find(&products).Error; err != nil {
		return nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Internal Server Error", Err: err}
	}
	return &model.ProductListResponse{Items: products, Page: q.Page, Limit: q.Limit, Total: total}, nil
}

func GetStorefrontProduct(db *gorm.DB, storeId uuid.UUID, publicProductId string) (*model.ProductResponse, []model.ImagesResponse, *utility.AppError) {
	var product model.Products
	err := db.Where("public_id = ? AND store_id = ? AND active = ?", publicProductId, storeId, true).First(&product).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, &utility.AppError{StatusCode: http.StatusNotFound, Message: "Product not found", Err: err}
		}
		return nil, nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Internal Server Error", Err: err}
	}

	images := []model.ImagesResponse{}
	if err := db.Model(&model.Images{}).Select("public_id, image_url").Where("product_id = ?", product.Id).Find(&images).Error; err != nil {
		return nil, nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Internal Server Error", Err: err}
	}

	return &model.ProductResponse{
		PublicId:      product.PublicId,
		Name:          product.Name,
		Description:   product.Description,
		Price:         product.Price,
		Currency:      product.Currency,
		StockQuantity: product.StockQuantity,
		Active:        product.Active,
		CreatedAt:     product.CreatedAt,
		UpdateAt:      product.UpdateAt,
	}, images, nil
}
