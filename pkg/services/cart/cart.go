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