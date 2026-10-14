package cart

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"huddle/pkg/helper"
	"huddle/pkg/model"
	"huddle/pkg/utility"
	"net/http"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

const cartTTL = 7 * 24 * time.Hour

// Each store has its own cart inside a session, so one shopper visiting two
// Huddle stores never mixes their products.
func cartKey(sessionId, storePublicId string) string {
	return fmt.Sprintf("cart:%s:%s", sessionId, storePublicId)
}

func loadItems(ctx context.Context, rdb *redis.Client, key string) ([]model.CartItem, *utility.AppError) {
	items := []model.CartItem{}
	val, err := rdb.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return items, nil
	}
	if err != nil {
		return nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Could not read cart", Err: err}
	}
	if err := json.Unmarshal([]byte(val), &items); err != nil {
		return nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Could not decode cart data", Err: err}
	}
	return items, nil
}

func saveItems(ctx context.Context, rdb *redis.Client, key string, items []model.CartItem) *utility.AppError {
	bytes, err := json.Marshal(items)
	if err != nil {
		return &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Could not encode cart data", Err: err}
	}
	if err := rdb.Set(ctx, key, string(bytes), cartTTL).Err(); err != nil {
		return &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Could not save cart", Err: err}
	}
	return nil
}

// findSellableProduct loads an active product that belongs to the given store.
func findSellableProduct(db *gorm.DB, rdb *redis.Client, storePublicId, publicProductId string) (*model.Products, *utility.AppError) {
	storeId, appErr := helper.GetInternalStoreId(rdb, db, storePublicId)
	if appErr != nil {
		return nil, appErr
	}

	var product model.Products
	err := db.Where("public_id = ? AND store_id = ? AND active = ?", publicProductId, *storeId, true).First(&product).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &utility.AppError{StatusCode: http.StatusNotFound, Message: "Product not found in this store", Err: err}
		}
		return nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Internal Server Error", Err: err}
	}
	return &product, nil
}

func checkStock(product *model.Products, quantity uint) *utility.AppError {
	if product.StockQuantity < 0 || quantity > uint(product.StockQuantity) {
		return &utility.AppError{
			StatusCode: http.StatusConflict,
			Message:    fmt.Sprintf("Only %d of %s left in stock", max(product.StockQuantity, 0), product.Name),
			Err:        errors.New("insufficient stock"),
		}
	}
	return nil
}

func response(sessionId, storePublicId string, items []model.CartItem) *model.CartResponse {
	return &model.CartResponse{SessionId: sessionId, StoreId: storePublicId, Items: items}
}

func AddToCart(req *model.AddCartItemRequest, storePublicId, sessionCookie string, db *gorm.DB, rdb *redis.Client) (string, *model.CartResponse, *utility.AppError) {
	product, appErr := findSellableProduct(db, rdb, storePublicId, req.ProductId)
	if appErr != nil {
		return "", nil, appErr
	}

	sessionId := sessionCookie
	if sessionId == "" {
		sessionId = fmt.Sprintf("cart_%s", strings.ReplaceAll(utility.GenerateUUID().String(), "-", ""))
	}

	ctx := context.Background()
	key := cartKey(sessionId, storePublicId)
	items, appErr := loadItems(ctx, rdb, key)
	if appErr != nil {
		return "", nil, appErr
	}

	// Update existing product quantity or append new item; name and price
	// are refreshed from the database either way.
	index := -1
	quantity := req.Quantity
	for i, item := range items {
		if item.ProductId == req.ProductId {
			index = i
			quantity += item.Quantity
			break
		}
	}

	if appErr := checkStock(product, quantity); appErr != nil {
		return "", nil, appErr
	}

	item := model.CartItem{
		ProductId:   product.PublicId,
		ProductName: product.Name,
		Price:       product.Price,
		Currency:    product.Currency,
		Quantity:    quantity,
	}
	if index >= 0 {
		items[index] = item
	} else {
		items = append(items, item)
	}

	if appErr := saveItems(ctx, rdb, key, items); appErr != nil {
		return "", nil, appErr
	}

	return sessionId, response(sessionId, storePublicId, items), nil
}

func GetCart(storePublicId, sessionCookie string, rdb *redis.Client) (*model.CartResponse, *utility.AppError) {
	if sessionCookie == "" {
		return response("", storePublicId, []model.CartItem{}), nil
	}

	items, appErr := loadItems(context.Background(), rdb, cartKey(sessionCookie, storePublicId))
	if appErr != nil {
		return nil, appErr
	}
	return response(sessionCookie, storePublicId, items), nil
}

func UpdateCartItemQuantity(productId string, quantity uint, storePublicId, sessionCookie string, db *gorm.DB, rdb *redis.Client) (*model.CartResponse, *utility.AppError) {
	if sessionCookie == "" {
		return nil, &utility.AppError{StatusCode: http.StatusNotFound, Message: "Cart session not found"}
	}

	product, appErr := findSellableProduct(db, rdb, storePublicId, productId)
	if appErr != nil {
		return nil, appErr
	}
	if appErr := checkStock(product, quantity); appErr != nil {
		return nil, appErr
	}

	ctx := context.Background()
	key := cartKey(sessionCookie, storePublicId)
	items, appErr := loadItems(ctx, rdb, key)
	if appErr != nil {
		return nil, appErr
	}

	found := false
	for i, item := range items {
		if item.ProductId == productId {
			items[i].Quantity = quantity
			items[i].ProductName = product.Name
			items[i].Price = product.Price
			items[i].Currency = product.Currency
			found = true
			break
		}
	}

	if !found {
		return nil, &utility.AppError{StatusCode: http.StatusNotFound, Message: "Item not found in shopping cart"}
	}

	if appErr := saveItems(ctx, rdb, key, items); appErr != nil {
		return nil, appErr
	}

	return response(sessionCookie, storePublicId, items), nil
}

func RemoveCartItem(productId, storePublicId, sessionCookie string, rdb *redis.Client) (*model.CartResponse, *utility.AppError) {
	if sessionCookie == "" {
		return nil, &utility.AppError{StatusCode: http.StatusNotFound, Message: "Cart session not found"}
	}

	ctx := context.Background()
	key := cartKey(sessionCookie, storePublicId)
	items, appErr := loadItems(ctx, rdb, key)
	if appErr != nil {
		return nil, appErr
	}

	updatedItems := []model.CartItem{}
	found := false
	for _, item := range items {
		if item.ProductId == productId {
			found = true
		} else {
			updatedItems = append(updatedItems, item)
		}
	}

	if !found {
		return nil, &utility.AppError{StatusCode: http.StatusNotFound, Message: "Item not found in shopping cart"}
	}

	if appErr := saveItems(ctx, rdb, key, updatedItems); appErr != nil {
		return nil, appErr
	}

	return response(sessionCookie, storePublicId, updatedItems), nil
}

// ClearCart empties this store's cart only; carts for other stores in the
// same session are kept.
func ClearCart(storePublicId, sessionCookie string, rdb *redis.Client) *utility.AppError {
	if sessionCookie == "" {
		return nil
	}

	if err := rdb.Del(context.Background(), cartKey(sessionCookie, storePublicId)).Err(); err != nil {
		return &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Could not clear cart session", Err: err}
	}

	return nil
}
