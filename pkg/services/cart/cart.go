package cart

import (
	"context"
	"encoding/json"
	"fmt"
	"huddle/pkg/model"
	"huddle/pkg/utility"
	"net/http"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func AddToCart(req *model.CartItem, sessionCookie string, db *gorm.DB, rdb *redis.Client) (string, *model.CartResponse, *utility.AppError) {
	sessionId := sessionCookie
	if sessionId == "" {
		sessionId = fmt.Sprintf("cart_%s", strings.ReplaceAll(utility.GenerateUUID().String(), "-", ""))
	}

	redisKey := fmt.Sprintf("cart:%s", sessionId)
	ctx := context.Background()

	var items []model.CartItem
	val, err := rdb.Get(ctx, redisKey).Result()
	if err == nil && val != "" {
		_ = json.Unmarshal([]byte(val), &items)
	}

	// Update existing product quantity or append new item
	found := false
	for i, item := range items {
		if item.ProductId == req.ProductId {
			items[i].Quantity += req.Quantity
			found = true
			break
		}
	}
	if !found {
		items = append(items, *req)
	}

	bytes, err := json.Marshal(items)
	if err != nil {
		return "", nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Could not encode cart data", Err: err}
	}

	if err := rdb.Set(ctx, redisKey, string(bytes), 7*24*time.Hour).Err(); err != nil {
		return "", nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Could not save cart item to cache", Err: err}
	}

	response := &model.CartResponse{
		SessionId: sessionId,
		Items:     items,
	}

	return sessionId, response, nil
}

func GetCart(sessionCookie string, rdb *redis.Client) (*model.CartResponse, *utility.AppError) {
	if sessionCookie == "" {
		return &model.CartResponse{SessionId: "", Items: []model.CartItem{}}, nil
	}

	redisKey := fmt.Sprintf("cart:%s", sessionCookie)
	ctx := context.Background()

	var items []model.CartItem
	val, err := rdb.Get(ctx, redisKey).Result()
	if err == nil && val != "" {
		_ = json.Unmarshal([]byte(val), &items)
	}

	if items == nil {
		items = []model.CartItem{}
	}

	return &model.CartResponse{
		SessionId: sessionCookie,
		Items:     items,
	}, nil
}

func UpdateCartItemQuantity(productId string, quantity uint, sessionCookie string, rdb *redis.Client) (*model.CartResponse, *utility.AppError) {
	if sessionCookie == "" {
		return nil, &utility.AppError{StatusCode: http.StatusNotFound, Message: "Cart session not found"}
	}

	redisKey := fmt.Sprintf("cart:%s", sessionCookie)
	ctx := context.Background()

	val, err := rdb.Get(ctx, redisKey).Result()
	if err != nil || val == "" {
		return nil, &utility.AppError{StatusCode: http.StatusNotFound, Message: "Cart session not found or expired"}
	}

	var items []model.CartItem
	if err := json.Unmarshal([]byte(val), &items); err != nil {
		return nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Could not decode cart data", Err: err}
	}

	found := false
	for i, item := range items {
		if item.ProductId == productId {
			items[i].Quantity = quantity
			found = true
			break
		}
	}

	if !found {
		return nil, &utility.AppError{StatusCode: http.StatusNotFound, Message: "Item not found in shopping cart"}
	}

	bytes, err := json.Marshal(items)
	if err != nil {
		return nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Could not encode cart data", Err: err}
	}

	if err := rdb.Set(ctx, redisKey, string(bytes), 7*24*time.Hour).Err(); err != nil {
		return nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Could not update cart in cache", Err: err}
	}

	return &model.CartResponse{
		SessionId: sessionCookie,
		Items:     items,
	}, nil
}

func RemoveCartItem(productId string, sessionCookie string, rdb *redis.Client) (*model.CartResponse, *utility.AppError) {
	if sessionCookie == "" {
		return nil, &utility.AppError{StatusCode: http.StatusNotFound, Message: "Cart session not found"}
	}

	redisKey := fmt.Sprintf("cart:%s", sessionCookie)
	ctx := context.Background()

	val, err := rdb.Get(ctx, redisKey).Result()
	if err != nil || val == "" {
		return nil, &utility.AppError{StatusCode: http.StatusNotFound, Message: "Cart session not found or expired"}
	}

	var items []model.CartItem
	if err := json.Unmarshal([]byte(val), &items); err != nil {
		return nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Could not decode cart data", Err: err}
	}

	var updatedItems []model.CartItem
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

	if updatedItems == nil {
		updatedItems = []model.CartItem{}
	}

	bytes, err := json.Marshal(updatedItems)
	if err != nil {
		return nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Could not encode cart data", Err: err}
	}

	if err := rdb.Set(ctx, redisKey, string(bytes), 7*24*time.Hour).Err(); err != nil {
		return nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Could not update cart in cache", Err: err}
	}

	return &model.CartResponse{
		SessionId: sessionCookie,
		Items:     updatedItems,
	}, nil
}

func ClearCart(sessionCookie string, rdb *redis.Client) *utility.AppError {
	if sessionCookie == "" {
		return nil
	}

	redisKey := fmt.Sprintf("cart:%s", sessionCookie)
	ctx := context.Background()

	if err := rdb.Del(ctx, redisKey).Err(); err != nil {
		return &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Could not clear cart session", Err: err}
	}

	return nil
}