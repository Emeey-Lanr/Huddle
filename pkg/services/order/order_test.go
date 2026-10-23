package order

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"huddle/pkg/model"
	"huddle/pkg/services/cart"
	"huddle/pkg/testutil"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type shop struct {
	t     *testing.T
	db    *gorm.DB
	rdb   *redis.Client
	admin model.Admins
	store model.Stores
}

func newShop(t *testing.T) *shop {
	t.Helper()
	db := testutil.NewDB(t)
	_, rdb := testutil.NewRedis(t)
	admin := testutil.SeedAdmin(t, db)
	return &shop{t: t, db: db, rdb: rdb, admin: admin, store: testutil.SeedStore(t, db, admin.Id)}
}

// addToCart puts quantity of product into session's cart and returns the session.
func (s *shop) addToCart(session string, p model.Products, quantity uint) string {
	s.t.Helper()
	session, _, appErr := cart.AddToCart(&model.AddCartItemRequest{ProductId: p.PublicId, Quantity: quantity}, s.store.StoreId, session, s.db, s.rdb)
	if appErr != nil {
		s.t.Fatalf("add to cart: %s %v", appErr.Message, appErr.Err)
	}
	return session
}

func pickup() model.CheckoutRequest {
	return model.CheckoutRequest{Name: "Mama Nkechi", Phone: "0803 123 4567", DeliveryMethod: model.DeliveryMethodPickup}
}

func (s *shop) stock(p model.Products) int {
	s.t.Helper()
	var fresh model.Products
	if err := s.db.Where("id = ?", p.Id).First(&fresh).Error; err != nil {
		s.t.Fatalf("reload product: %v", err)
	}
	return fresh.StockQuantity
}

func TestCheckout_ReservesStockAndCopiesPrices(t *testing.T) {
	s := newShop(t)
	bag := testutil.SeedProduct(t, s.db, s.store.ID, "Ankara Bag", 15000, 5)
	session := s.addToCart("", bag, 2)

	// The price changes after the item was added; checkout charges today's price.
	s.db.Model(&model.Products{}).Where("id = ?", bag.Id).Update("price", decimal.NewFromInt(16000))

	order, appErr := Checkout(s.db, s.rdb, s.store.ID, s.store.StoreId, session, nil, "", pickup())
	if appErr != nil {
		t.Fatalf("checkout: %s %v", appErr.Message, appErr.Err)
	}
	if order.Status != model.OrderPending || !order.Total.Equal(decimal.NewFromInt(32000)) || order.Currency != "NGN" {
		t.Fatalf("order = %s %s %s, want pending 32000 NGN", order.Status, order.Total, order.Currency)
	}
	if order.Phone != "08031234567" {
		t.Errorf("phone = %q, want spaces removed", order.Phone)
	}
	if got := order.ExpiresAt.Sub(order.CreatedAt); got != 30*time.Minute {
		t.Errorf("hold = %v, want 30m", got)
	}
	if got := s.stock(bag); got != 3 {
		t.Errorf("stock = %d, want 3", got)
	}

	c, _ := cart.GetCart(s.store.StoreId, session, s.rdb)
	if len(c.Items) != 0 {
		t.Errorf("cart still has %d items after checkout", len(c.Items))
	}

	// Later product edits do not change the order.
	s.db.Model(&model.Products{}).Where("id = ?", bag.Id).Updates(map[string]any{"name": "Renamed", "price": decimal.NewFromInt(99)})
	saved, _ := GetStorefrontOrder(s.db, s.store.ID, order.Id)
	if it := saved.Items[0]; it.ProductName != "Ankara Bag" || !it.UnitPrice.Equal(decimal.NewFromInt(16000)) || it.Quantity != 2 {
		t.Errorf("saved item changed: %+v", it)
	}
}

func TestCheckout_OnlyOneShopperGetsTheLastItem(t *testing.T) {
	s := newShop(t)
	bag := testutil.SeedProduct(t, s.db, s.store.ID, "Last Bag", 15000, 1)

	const shoppers = 10
	sessions := make([]string, shoppers)
	for i := range sessions {
		sessions[i] = s.addToCart("", bag, 1)
	}

	var wg sync.WaitGroup
	codes := make([]int, shoppers)
	for i, session := range sessions {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, appErr := Checkout(s.db, s.rdb, s.store.ID, s.store.StoreId, session, nil, "", pickup()); appErr != nil {
				codes[i] = appErr.StatusCode
			} else {
				codes[i] = http.StatusCreated
			}
		}()
	}
	wg.Wait()

	won, soldOut := 0, 0
	for _, c := range codes {
		switch c {
		case http.StatusCreated:
			won++
		case http.StatusConflict:
			soldOut++
		}
	}
	if won != 1 || soldOut != shoppers-1 {
		t.Fatalf("codes = %v, want 1 order and %d sold out", codes, shoppers-1)
	}
	if got := s.stock(bag); got != 0 {
		t.Errorf("stock = %d, want 0", got)
	}
}

func TestCheckout_FailedItemRollsBackTheWholeOrder(t *testing.T) {
	s := newShop(t)
	bag := testutil.SeedProduct(t, s.db, s.store.ID, "Bag", 15000, 5)
	shoe := testutil.SeedProduct(t, s.db, s.store.ID, "Shoe", 9000, 5)
	session := s.addToCart("", bag, 2)
	s.addToCart(session, shoe, 3)

	// Someone else bought the shoes in the meantime.
	s.db.Model(&model.Products{}).Where("id = ?", shoe.Id).Update("stock_quantity", 1)

	_, appErr := Checkout(s.db, s.rdb, s.store.ID, s.store.StoreId, session, nil, "", pickup())
	if appErr == nil || appErr.StatusCode != http.StatusConflict || appErr.Message != "Only 1 of Shoe left in stock" {
		t.Fatalf("checkout = %+v, want 409 Only 1 of Shoe left in stock", appErr)
	}
	if got := s.stock(bag); got != 5 {
		t.Errorf("bag stock = %d, want 5 (rolled back)", got)
	}
	var orders int64
	s.db.Model(&model.Orders{}).Count(&orders)
	if orders != 0 {
		t.Errorf("%d orders saved, want 0", orders)
	}
	if c, _ := cart.GetCart(s.store.StoreId, session, s.rdb); len(c.Items) != 2 {
		t.Errorf("cart was changed by a failed checkout: %d items", len(c.Items))
	}

	// A product the seller has hidden can't be bought either.
	s.db.Model(&model.Products{}).Where("id = ?", shoe.Id).Updates(map[string]any{"stock_quantity": 5, "active": false})
	if _, appErr := Checkout(s.db, s.rdb, s.store.ID, s.store.StoreId, session, nil, "", pickup()); appErr == nil || appErr.StatusCode != http.StatusConflict {
		t.Errorf("inactive product: %+v, want 409", appErr)
	}
}

func TestCheckout_Rejects(t *testing.T) {
	s := newShop(t)
	bag := testutil.SeedProduct(t, s.db, s.store.ID, "Bag", 15000, 5)
	session := s.addToCart("", bag, 1)

	badPhone := pickup()
	badPhone.Phone = "call me"
	if _, appErr := Checkout(s.db, s.rdb, s.store.ID, s.store.StoreId, session, nil, "", badPhone); appErr == nil || appErr.StatusCode != http.StatusBadRequest {
		t.Errorf("bad phone: %+v, want 400", appErr)
	}
	if _, appErr := Checkout(s.db, s.rdb, s.store.ID, s.store.StoreId, "", nil, "", pickup()); appErr == nil || appErr.StatusCode != http.StatusBadRequest {
		t.Errorf("no session: %+v, want 400", appErr)
	}
	if _, appErr := Checkout(s.db, s.rdb, s.store.ID, s.store.StoreId, "cart_unknown", nil, "", pickup()); appErr == nil || appErr.Message != "Your cart is empty" {
		t.Errorf("empty cart: %+v, want Your cart is empty", appErr)
	}

	// A customer of another store can't check out here.
	other := testutil.SeedStore(t, s.db, s.admin.Id)
	stranger := model.Customers{Id: uuid.New(), StoreId: other.ID, Name: "Ada", Email: "ada@example.com", Password: "x"}
	s.db.Create(&stranger)
	if _, appErr := Checkout(s.db, s.rdb, s.store.ID, s.store.StoreId, session, &stranger.Id, stranger.Email, pickup()); appErr == nil || appErr.StatusCode != http.StatusForbidden {
		t.Errorf("customer of another store: %+v, want 403", appErr)
	}

	// A second checkout of the same cart while one is running is refused.
	s.rdb.Set(t.Context(), fmt.Sprintf("checkout-lock:%s:%s", session, s.store.StoreId), "1", time.Minute)
	if _, appErr := Checkout(s.db, s.rdb, s.store.ID, s.store.StoreId, session, nil, "", pickup()); appErr == nil || appErr.StatusCode != http.StatusConflict {
		t.Errorf("concurrent checkout: %+v, want 409", appErr)
	}
	if got := s.stock(bag); got != 5 {
		t.Errorf("stock = %d after rejected checkouts, want 5", got)
	}
}

func TestCheckout_HoldTimeFromEnv(t *testing.T) {
	t.Setenv("ORDER_HOLD_MINUTES", "45")
	if got := HoldDuration(); got != 45*time.Minute {
		t.Errorf("hold = %v, want 45m", got)
	}
	t.Setenv("ORDER_HOLD_MINUTES", "nonsense")
	if got := HoldDuration(); got != 30*time.Minute {
		t.Errorf("hold = %v, want default 30m", got)
	}
}

func TestExpireOrders_ReleasesStockOnce(t *testing.T) {
	s := newShop(t)
	bag := testutil.SeedProduct(t, s.db, s.store.ID, "Bag", 15000, 5)
	expiring, _ := Checkout(s.db, s.rdb, s.store.ID, s.store.StoreId, s.addToCart("", bag, 2), nil, "", pickup())
	paid, _ := Checkout(s.db, s.rdb, s.store.ID, s.store.StoreId, s.addToCart("", bag, 1), nil, "", pickup())
	if _, appErr := UpdateOrderStatus(s.db, s.store.StoreId, s.admin.Id, paid.Id, model.OrderPaid); appErr != nil {
		t.Fatalf("mark paid: %s", appErr.Message)
	}
	if got := s.stock(bag); got != 2 {
		t.Fatalf("stock = %d, want 2", got)
	}

	if n, err := ExpireOrders(s.db, time.Now().UTC()); err != nil || n != 0 {
		t.Fatalf("before hold ends: %d %v, want 0", n, err)
	}

	later := time.Now().UTC().Add(31 * time.Minute)
	if n, err := ExpireOrders(s.db, later); err != nil || n != 1 {
		t.Fatalf("after hold ends: %d %v, want 1", n, err)
	}
	if got := s.stock(bag); got != 4 {
		t.Errorf("stock = %d, want 4 (2 released, paid order keeps 1)", got)
	}
	o, _ := GetStorefrontOrder(s.db, s.store.ID, expiring.Id)
	if o.Status != model.OrderExpired {
		t.Errorf("status = %s, want expired", o.Status)
	}

	if n, _ := ExpireOrders(s.db, later); n != 0 {
		t.Errorf("second run expired %d, want 0", n)
	}
	if got := s.stock(bag); got != 4 {
		t.Errorf("stock = %d after second run, want 4", got)
	}

	if _, appErr := UpdateOrderStatus(s.db, s.store.StoreId, s.admin.Id, expiring.Id, model.OrderPaid); appErr == nil || appErr.StatusCode != http.StatusConflict {
		t.Errorf("paying an expired order: %+v, want 409", appErr)
	}
}

func TestExpireOrders_SameProductInManyOrders(t *testing.T) {
	s := newShop(t)
	bag := testutil.SeedProduct(t, s.db, s.store.ID, "Bag", 15000, 10)
	for _, qty := range []uint{1, 2, 3} {
		if _, appErr := Checkout(s.db, s.rdb, s.store.ID, s.store.StoreId, s.addToCart("", bag, qty), nil, "", pickup()); appErr != nil {
			t.Fatalf("checkout: %s", appErr.Message)
		}
	}
	if n, err := ExpireOrders(s.db, time.Now().UTC().Add(time.Hour)); err != nil || n != 3 {
		t.Fatalf("expired %d %v, want 3", n, err)
	}
	if got := s.stock(bag); got != 10 {
		t.Errorf("stock = %d, want all 10 back", got)
	}
}

func TestUpdateOrderStatus(t *testing.T) {
	s := newShop(t)
	bag := testutil.SeedProduct(t, s.db, s.store.ID, "Bag", 15000, 5)
	place := func() string {
		o, appErr := Checkout(s.db, s.rdb, s.store.ID, s.store.StoreId, s.addToCart("", bag, 2), nil, "", pickup())
		if appErr != nil {
			t.Fatalf("checkout: %s", appErr.Message)
		}
		return o.Id
	}
	update := func(id, status string) (*model.OrderResponse, int) {
		o, appErr := UpdateOrderStatus(s.db, s.store.StoreId, s.admin.Id, id, status)
		if appErr != nil {
			return nil, appErr.StatusCode
		}
		return o, http.StatusOK
	}

	// Cancelling a pending order puts its stock back.
	cancelled := place()
	if _, code := update(cancelled, model.OrderCancelled); code != http.StatusOK {
		t.Fatalf("cancel: %d", code)
	}
	if got := s.stock(bag); got != 5 {
		t.Errorf("stock after cancel = %d, want 5", got)
	}
	if _, code := update(cancelled, model.OrderPaid); code != http.StatusConflict {
		t.Errorf("paying a cancelled order: %d, want 409", code)
	}
	if _, code := update(cancelled, model.OrderCancelled); code != http.StatusConflict {
		t.Errorf("cancelling twice: %d, want 409", code)
	}
	if got := s.stock(bag); got != 5 {
		t.Errorf("stock after repeated cancel = %d, want 5", got)
	}

	// pending -> paid -> fulfilled; a paid order keeps its stock.
	sold := place()
	if _, code := update(sold, model.OrderFulfilled); code != http.StatusConflict {
		t.Errorf("fulfilling an unpaid order: %d, want 409", code)
	}
	o, code := update(sold, model.OrderPaid)
	if code != http.StatusOK || o.Status != model.OrderPaid || o.PaidAt == nil {
		t.Fatalf("mark paid: %d %+v", code, o)
	}
	if _, code := update(sold, model.OrderCancelled); code != http.StatusConflict {
		t.Errorf("cancelling a paid order: %d, want 409", code)
	}
	if o, code := update(sold, model.OrderFulfilled); code != http.StatusOK || o.Status != model.OrderFulfilled {
		t.Errorf("fulfil: %d", code)
	}
	if got := s.stock(bag); got != 3 {
		t.Errorf("stock = %d, want 3", got)
	}

	// Only the owner, and only orders of that store.
	stranger := testutil.SeedAdmin(t, s.db)
	if _, appErr := UpdateOrderStatus(s.db, s.store.StoreId, stranger.Id, sold, model.OrderPaid); appErr == nil || appErr.StatusCode != http.StatusForbidden {
		t.Errorf("other admin: %+v, want 403", appErr)
	}
	otherStore := testutil.SeedStore(t, s.db, s.admin.Id)
	if _, appErr := UpdateOrderStatus(s.db, otherStore.StoreId, s.admin.Id, sold, model.OrderPaid); appErr == nil || appErr.StatusCode != http.StatusNotFound {
		t.Errorf("order of another store: %+v, want 404", appErr)
	}
}

func TestStartExpiryWorker_StopsWithContext(t *testing.T) {
	db := testutil.NewDB(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := StartExpiryWorker(ctx, db, time.Hour)
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("expiry worker did not stop")
	}
}
