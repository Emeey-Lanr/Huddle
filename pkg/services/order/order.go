package order

import (
	"context"
	"errors"
	"fmt"
	"huddle/pkg/model"
	"huddle/pkg/services"
	"huddle/pkg/services/cart"
	"huddle/pkg/utility"
	"log"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	defaultHold     = 30 * time.Minute
	checkoutLockTTL = 30 * time.Second
	defaultLimit    = 20
	maxLimit        = 100
)

// HoldDuration is how long a pending order keeps its stock before it expires.
// ORDER_HOLD_MINUTES overrides the 30 minute default.
func HoldDuration() time.Duration {
	if m, err := strconv.Atoi(os.Getenv("ORDER_HOLD_MINUTES")); err == nil && m > 0 {
		return time.Duration(m) * time.Minute
	}
	return defaultHold
}

var phonePattern = regexp.MustCompile(`^\+?[0-9]{7,15}$`)

// normalizePhone drops spaces, dashes, dots and brackets, so "0803 123 4567"
// and "+234-803-123-4567" are both accepted.
func normalizePhone(phone string) (string, bool) {
	p := strings.NewReplacer(" ", "", "-", "", ".", "", "(", "", ")", "").Replace(strings.TrimSpace(phone))
	return p, phonePattern.MatchString(p)
}

func internalErr(err error) *utility.AppError {
	return &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Internal Server Error", Err: err}
}

// Checkout turns the shopper's cart for this store into a pending order. Stock
// is taken from each product inside one transaction, so two shoppers can never
// buy the same last item. customerId is nil for guest checkout.
func Checkout(db *gorm.DB, rdb *redis.Client, storeId uuid.UUID, storePublicId, session string, customerId *uuid.UUID, customerEmail string, req model.CheckoutRequest) (*model.OrderResponse, *utility.AppError) {
	phone, ok := normalizePhone(req.Phone)
	if !ok {
		return nil, &utility.AppError{StatusCode: http.StatusBadRequest, Message: "phone must be a valid phone number", Err: errors.New("invalid phone")}
	}
	if req.Email == "" {
		req.Email = customerEmail
	}
	if req.DeliveryMethod == model.DeliveryMethodPickup {
		req.Address, req.City, req.State = "", "", ""
	}

	emptyCart := &utility.AppError{StatusCode: http.StatusBadRequest, Message: "Your cart is empty", Err: errors.New("empty cart")}
	if session == "" {
		return nil, emptyCart
	}

	ctx := context.Background()
	lockKey := fmt.Sprintf("checkout-lock:%s:%s", session, storePublicId)
	locked, err := rdb.SetNX(ctx, lockKey, "1", checkoutLockTTL).Result()
	if err != nil {
		return nil, internalErr(err)
	}
	if !locked {
		return nil, &utility.AppError{StatusCode: http.StatusConflict, Message: "Checkout is already in progress for this cart", Err: errors.New("checkout locked")}
	}
	defer rdb.Del(ctx, lockKey)

	cartResp, appErr := cart.GetCart(storePublicId, session, rdb)
	if appErr != nil {
		return nil, appErr
	}
	if len(cartResp.Items) == 0 {
		return nil, emptyCart
	}

	if customerId != nil {
		var count int64
		if err := db.Model(&model.Customers{}).Where("id = ? AND store_id = ?", *customerId, storeId).Count(&count).Error; err != nil {
			return nil, internalErr(err)
		}
		if count == 0 {
			return nil, &utility.AppError{StatusCode: http.StatusForbidden, Message: "This account belongs to another store", Err: errors.New("customer not in store")}
		}
	}

	// A fixed order of product updates keeps concurrent checkouts from deadlocking.
	cartItems := cartResp.Items
	sort.Slice(cartItems, func(i, j int) bool { return cartItems[i].ProductId < cartItems[j].ProductId })

	now := time.Now().UTC()
	order := model.Orders{
		Id:             utility.GenerateUUID(),
		PublicId:       "ord_" + strings.ReplaceAll(utility.GenerateUUID().String(), "-", ""),
		StoreId:        storeId,
		CustomerId:     customerId,
		Status:         model.OrderPending,
		CustomerName:   strings.TrimSpace(req.Name),
		CustomerPhone:  phone,
		CustomerEmail:  strings.TrimSpace(req.Email),
		DeliveryMethod: req.DeliveryMethod,
		Address:        strings.TrimSpace(req.Address),
		City:           strings.TrimSpace(req.City),
		State:          strings.TrimSpace(req.State),
		Note:           strings.TrimSpace(req.Note),
		ExpiresAt:      now.Add(HoldDuration()),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	items := make([]model.OrderItems, 0, len(cartItems))

	txErr := db.Transaction(func(tx *gorm.DB) error {
		total := decimal.Zero
		for _, ci := range cartItems {
			item, appErr := reserve(tx, storeId, ci.ProductId, int(ci.Quantity))
			if appErr != nil {
				return appErr
			}
			if order.Currency == "" {
				order.Currency = item.currency
			} else if order.Currency != item.currency {
				return &utility.AppError{StatusCode: http.StatusBadRequest, Message: "All items in an order must use the same currency", Err: errors.New("mixed currencies")}
			}

			productId := item.id
			lineTotal := item.price.Mul(decimal.NewFromInt(int64(ci.Quantity)))
			total = total.Add(lineTotal)
			items = append(items, model.OrderItems{
				Id:              utility.GenerateUUID(),
				OrderId:         order.Id,
				ProductId:       &productId,
				ProductPublicId: ci.ProductId,
				ProductName:     item.name,
				UnitPrice:       item.price,
				Quantity:        int(ci.Quantity),
				LineTotal:       lineTotal,
			})
		}
		order.Total = total

		if err := tx.Create(&order).Error; err != nil {
			return err
		}
		return tx.Create(&items).Error
	})
	if txErr != nil {
		var appErr *utility.AppError
		if errors.As(txErr, &appErr) {
			return nil, appErr
		}
		return nil, internalErr(txErr)
	}

	// The order is saved; a cart that fails to clear only costs the shopper a
	// manual clear, so it does not fail the checkout.
	if appErr := cart.ClearCart(storePublicId, session, rdb); appErr != nil {
		log.Printf("checkout %s: could not clear cart: %v", order.PublicId, appErr.Err)
	}

	return toResponse(order, items), nil
}

type reserved struct {
	id       uuid.UUID
	name     string
	price    decimal.Decimal
	currency string
}

// reserve takes quantity from a product's stock, only if the product is
// active, belongs to the store and has enough left. Prices come from the
// database at this moment, not from the cart.
func reserve(tx *gorm.DB, storeId uuid.UUID, publicProductId string, quantity int) (*reserved, *utility.AppError) {
	var row struct {
		Id       uuid.UUID
		Name     string
		Price    decimal.Decimal
		Currency string
	}
	err := tx.Raw(`UPDATE products SET stock_quantity = stock_quantity - ?, updated_at = NOW()
		WHERE public_id = ? AND store_id = ? AND active AND stock_quantity >= ?
		RETURNING id, name, price, currency`, quantity, publicProductId, storeId, quantity).Scan(&row).Error
	if err != nil {
		return nil, internalErr(err)
	}
	if row.Id != uuid.Nil {
		return &reserved{id: row.Id, name: row.Name, price: row.Price, currency: row.Currency}, nil
	}

	var product model.Products
	err = tx.Where("public_id = ? AND store_id = ?", publicProductId, storeId).First(&product).Error
	if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && !product.Active) {
		return nil, &utility.AppError{StatusCode: http.StatusConflict, Message: "An item in your cart is no longer available", Err: fmt.Errorf("product %s unavailable", publicProductId)}
	}
	if err != nil {
		return nil, internalErr(err)
	}
	message := fmt.Sprintf("Only %d of %s left in stock", product.StockQuantity, product.Name)
	if product.StockQuantity <= 0 {
		message = product.Name + " is sold out"
	}
	return nil, &utility.AppError{
		StatusCode: http.StatusConflict,
		Message:    message,
		Err:        errors.New("insufficient stock"),
	}
}

// restoreStock puts the stock of the given orders back on sale.
func restoreStock(tx *gorm.DB, orderIds []uuid.UUID) error {
	if len(orderIds) == 0 {
		return nil
	}
	return tx.Exec(`UPDATE products p SET stock_quantity = p.stock_quantity + s.qty, updated_at = NOW()
		FROM (SELECT product_id, SUM(quantity) AS qty FROM order_items
		      WHERE order_id IN ? AND product_id IS NOT NULL GROUP BY product_id) s
		WHERE p.id = s.product_id`, orderIds).Error
}

func toResponse(o model.Orders, items []model.OrderItems) *model.OrderResponse {
	resp := &model.OrderResponse{
		Id: o.PublicId, Status: o.Status, Currency: o.Currency, Total: o.Total,
		Name: o.CustomerName, Phone: o.CustomerPhone, Email: o.CustomerEmail,
		DeliveryMethod: o.DeliveryMethod, Address: o.Address, City: o.City, State: o.State, Note: o.Note,
		Items:     make([]model.OrderItemResponse, 0, len(items)),
		ExpiresAt: o.ExpiresAt, PaidAt: o.PaidAt, CreatedAt: o.CreatedAt, UpdatedAt: o.UpdatedAt,
	}
	for _, it := range items {
		resp.Items = append(resp.Items, model.OrderItemResponse{
			ProductId: it.ProductPublicId, ProductName: it.ProductName,
			UnitPrice: it.UnitPrice, Quantity: it.Quantity, LineTotal: it.LineTotal,
		})
	}
	return resp
}

func findOrder(db *gorm.DB, storeId uuid.UUID, publicOrderId string) (*model.OrderResponse, *utility.AppError) {
	var o model.Orders
	if err := db.Where("public_id = ? AND store_id = ?", publicOrderId, storeId).First(&o).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &utility.AppError{StatusCode: http.StatusNotFound, Message: "Order not found", Err: err}
		}
		return nil, internalErr(err)
	}
	var items []model.OrderItems
	if err := db.Where("order_id = ?", o.Id).Order("product_name").Find(&items).Error; err != nil {
		return nil, internalErr(err)
	}
	return toResponse(o, items), nil
}

// listOrders pages through the orders matched by query, newest first.
func listOrders(query *gorm.DB, q model.OrderQuery) (*model.OrderListResponse, *utility.AppError) {
	if q.Page < 1 {
		q.Page = 1
	}
	if q.Limit < 1 {
		q.Limit = defaultLimit
	}
	q.Limit = min(q.Limit, maxLimit)
	switch q.Status {
	case "":
	case model.OrderPending, model.OrderPaid, model.OrderFulfilled, model.OrderCancelled, model.OrderExpired:
		query = query.Where("status = ?", q.Status)
	default:
		return nil, &utility.AppError{StatusCode: http.StatusBadRequest, Message: "status must be one of pending, paid, fulfilled, cancelled, expired", Err: errors.New("invalid status")}
	}

	var total int64
	if err := query.Session(&gorm.Session{}).Model(&model.Orders{}).Count(&total).Error; err != nil {
		return nil, internalErr(err)
	}
	var orders []model.Orders
	if err := query.Order("created_at DESC").Order("id").Offset((q.Page - 1) * q.Limit).Limit(q.Limit).Find(&orders).Error; err != nil {
		return nil, internalErr(err)
	}

	ids := make([]uuid.UUID, 0, len(orders))
	for _, o := range orders {
		ids = append(ids, o.Id)
	}
	byOrder := map[uuid.UUID][]model.OrderItems{}
	if len(ids) > 0 {
		var items []model.OrderItems
		if err := query.Session(&gorm.Session{NewDB: true}).Where("order_id IN ?", ids).Order("product_name").Find(&items).Error; err != nil {
			return nil, internalErr(err)
		}
		for _, it := range items {
			byOrder[it.OrderId] = append(byOrder[it.OrderId], it)
		}
	}

	resp := &model.OrderListResponse{Items: make([]model.OrderResponse, 0, len(orders)), Page: q.Page, Limit: q.Limit, Total: total}
	for _, o := range orders {
		resp.Items = append(resp.Items, *toResponse(o, byOrder[o.Id]))
	}
	return resp, nil
}

// GetStorefrontOrder lets a shopper see an order they placed in this store.
// The order id is random and long, so it works as the guest's receipt link.
func GetStorefrontOrder(db *gorm.DB, storeId uuid.UUID, publicOrderId string) (*model.OrderResponse, *utility.AppError) {
	return findOrder(db, storeId, publicOrderId)
}

// ListCustomerOrders lists a signed-in customer's own orders.
func ListCustomerOrders(db *gorm.DB, customerId uuid.UUID, q model.OrderQuery) (*model.OrderListResponse, *utility.AppError) {
	return listOrders(db.Where("customer_id = ?", customerId), q)
}

// ListSellerOrders lists a store's orders for its owner.
func ListSellerOrders(db *gorm.DB, publicStoreId string, adminId uuid.UUID, q model.OrderQuery) (*model.OrderListResponse, *utility.AppError) {
	store, appErr := services.OwnedStore(db, publicStoreId, adminId)
	if appErr != nil {
		return nil, appErr
	}
	return listOrders(db.Where("store_id = ?", store.ID), q)
}

func GetSellerOrder(db *gorm.DB, publicStoreId string, adminId uuid.UUID, publicOrderId string) (*model.OrderResponse, *utility.AppError) {
	store, appErr := services.OwnedStore(db, publicStoreId, adminId)
	if appErr != nil {
		return nil, appErr
	}
	return findOrder(db, store.ID, publicOrderId)
}

// allowedTransitions lists the status changes a seller can make. "paid" here
// is for payments the seller confirms themselves (bank transfer, cash).
// Paid orders cannot be cancelled until refunds exist.
var allowedTransitions = map[string][]string{
	model.OrderPending: {model.OrderPaid, model.OrderCancelled},
	model.OrderPaid:    {model.OrderFulfilled},
}

// UpdateOrderStatus applies a seller's status change. Cancelling a pending
// order puts its stock back on sale.
func UpdateOrderStatus(db *gorm.DB, publicStoreId string, adminId uuid.UUID, publicOrderId, status string) (*model.OrderResponse, *utility.AppError) {
	store, appErr := services.OwnedStore(db, publicStoreId, adminId)
	if appErr != nil {
		return nil, appErr
	}

	txErr := db.Transaction(func(tx *gorm.DB) error {
		var o model.Orders
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("public_id = ? AND store_id = ?", publicOrderId, store.ID).First(&o).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &utility.AppError{StatusCode: http.StatusNotFound, Message: "Order not found", Err: err}
		}
		if err != nil {
			return err
		}

		allowed := false
		for _, next := range allowedTransitions[o.Status] {
			allowed = allowed || next == status
		}
		if !allowed {
			return &utility.AppError{StatusCode: http.StatusConflict, Message: fmt.Sprintf("A %s order cannot be marked %s", o.Status, status), Err: errors.New("invalid status change")}
		}

		now := time.Now().UTC()
		updates := map[string]any{"status": status, "updated_at": now}
		if status == model.OrderPaid {
			updates["paid_at"] = now
		}
		if err := tx.Model(&model.Orders{}).Where("id = ?", o.Id).Updates(updates).Error; err != nil {
			return err
		}
		if status == model.OrderCancelled {
			return restoreStock(tx, []uuid.UUID{o.Id})
		}
		return nil
	})
	if txErr != nil {
		var appErr *utility.AppError
		if errors.As(txErr, &appErr) {
			return nil, appErr
		}
		return nil, internalErr(txErr)
	}
	return findOrder(db, store.ID, publicOrderId)
}

// ExpireOrders marks pending orders past their hold time as expired and puts
// their stock back on sale. Orders locked by a seller's status change are
// skipped and picked up on the next run. It returns how many expired.
func ExpireOrders(db *gorm.DB, now time.Time) (int, error) {
	var ids []uuid.UUID
	err := db.Transaction(func(tx *gorm.DB) error {
		err := tx.Raw(`UPDATE orders SET status = ?, updated_at = NOW()
			WHERE id IN (SELECT id FROM orders WHERE status = ? AND expires_at <= ?
			             ORDER BY expires_at LIMIT 500 FOR UPDATE SKIP LOCKED)
			RETURNING id`, model.OrderExpired, model.OrderPending, now).Scan(&ids).Error
		if err != nil {
			return err
		}
		return restoreStock(tx, ids)
	})
	return len(ids), err
}

// StartExpiryWorker runs ExpireOrders every interval until ctx is done.
func StartExpiryWorker(ctx context.Context, db *gorm.DB, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				n, err := ExpireOrders(db, time.Now().UTC())
				if err != nil {
					log.Printf("order expiry: %v", err)
				} else if n > 0 {
					log.Printf("order expiry: released stock for %d expired orders", n)
				}
			}
		}
	}()
}
