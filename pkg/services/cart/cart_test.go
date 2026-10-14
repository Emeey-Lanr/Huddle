package cart

import (
	"net/http"
	"testing"

	"huddle/pkg/model"
	"huddle/pkg/testutil"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type env struct {
	db  *gorm.DB
	mr  *miniredis.Miniredis
	rdb *redis.Client
	bag model.Products // ₦15,000, 10 in stock
	hat model.Products // ₦5,000, 3 in stock
}

func setup(t *testing.T) env {
	t.Helper()
	db := testutil.NewDB(t)
	mr, rdb := testutil.NewRedis(t)
	admin := testutil.SeedAdmin(t, db)
	store := testutil.SeedStore(t, db, admin.Id)
	return env{
		db:  db,
		mr:  mr,
		rdb: rdb,
		bag: testutil.SeedProduct(t, db, store.ID, "Ankara bag", 15000, 10),
		hat: testutil.SeedProduct(t, db, store.ID, "Fila cap", 5000, 3),
	}
}

func add(id string, qty uint) *model.AddCartItemRequest {
	return &model.AddCartItemRequest{ProductId: id, Quantity: qty}
}

func TestAddToCart_NewSession(t *testing.T) {
	e := setup(t)

	sid, resp, err := AddToCart(add(e.bag.PublicId, 2), "", e.db, e.rdb)
	if err != nil {
		t.Fatalf("AddToCart: %s %v", err.Message, err.Err)
	}
	if sid == "" || resp.SessionId != sid {
		t.Fatalf("session id not returned: %q", sid)
	}
	if len(resp.Items) != 1 || resp.Items[0].Quantity != 2 {
		t.Errorf("items = %+v", resp.Items)
	}
	if !e.mr.Exists("cart:" + sid) {
		t.Error("cart not persisted in redis")
	}
	if ttl := e.mr.TTL("cart:" + sid); ttl <= 0 {
		t.Errorf("cart has no expiry (ttl %v)", ttl)
	}
}

func TestAddToCart_PriceAndNameComeFromDatabase(t *testing.T) {
	e := setup(t)

	_, resp, err := AddToCart(add(e.bag.PublicId, 1), "", e.db, e.rdb)
	if err != nil {
		t.Fatalf("AddToCart: %s", err.Message)
	}
	got := resp.Items[0]
	if !got.Price.Equal(e.bag.Price) || got.ProductName != "Ankara bag" || got.Currency != "NGN" {
		t.Errorf("item = %+v, want price %s from database", got, e.bag.Price)
	}
}

func TestAddToCart_SameProductIncrementsQuantity(t *testing.T) {
	e := setup(t)
	sid, _, _ := AddToCart(add(e.bag.PublicId, 2), "", e.db, e.rdb)
	_, resp, _ := AddToCart(add(e.bag.PublicId, 3), sid, e.db, e.rdb)

	if len(resp.Items) != 1 || resp.Items[0].Quantity != 5 {
		t.Errorf("items = %+v, want one item with quantity 5", resp.Items)
	}
}

func TestAddToCart_DifferentProductsAppend(t *testing.T) {
	e := setup(t)
	sid, _, _ := AddToCart(add(e.bag.PublicId, 1), "", e.db, e.rdb)
	_, resp, _ := AddToCart(add(e.hat.PublicId, 1), sid, e.db, e.rdb)

	if len(resp.Items) != 2 {
		t.Errorf("items = %+v, want 2", resp.Items)
	}
}

func TestAddToCart_UnknownProduct(t *testing.T) {
	e := setup(t)
	if _, _, err := AddToCart(add("prod_missing", 1), "", e.db, e.rdb); err == nil || err.StatusCode != http.StatusNotFound {
		t.Errorf("err = %+v, want 404", err)
	}
}

func TestAddToCart_InactiveProduct(t *testing.T) {
	e := setup(t)
	e.db.Model(&model.Products{}).Where("id = ?", e.bag.Id).Update("active", false)

	if _, _, err := AddToCart(add(e.bag.PublicId, 1), "", e.db, e.rdb); err == nil || err.StatusCode != http.StatusNotFound {
		t.Errorf("err = %+v, want 404", err)
	}
}

func TestAddToCart_NotEnoughStock(t *testing.T) {
	e := setup(t)
	if _, _, err := AddToCart(add(e.hat.PublicId, 4), "", e.db, e.rdb); err == nil || err.StatusCode != http.StatusConflict {
		t.Errorf("4 of 3: err = %+v, want 409", err)
	}

	// Adding in two steps must also respect stock.
	sid, _, _ := AddToCart(add(e.hat.PublicId, 2), "", e.db, e.rdb)
	if _, _, err := AddToCart(add(e.hat.PublicId, 2), sid, e.db, e.rdb); err == nil || err.StatusCode != http.StatusConflict {
		t.Errorf("2+2 of 3: err = %+v, want 409", err)
	}
}

func TestGetCart(t *testing.T) {
	e := setup(t)

	resp, err := GetCart("", e.rdb)
	if err != nil || len(resp.Items) != 0 {
		t.Fatalf("empty session: %+v %v", resp, err)
	}

	sid, _, _ := AddToCart(add(e.bag.PublicId, 1), "", e.db, e.rdb)
	resp, _ = GetCart(sid, e.rdb)
	if len(resp.Items) != 1 {
		t.Errorf("items = %+v", resp.Items)
	}

	resp, _ = GetCart("cart_doesnotexist", e.rdb)
	if resp.Items == nil || len(resp.Items) != 0 {
		t.Errorf("unknown session should return empty items, got %+v", resp.Items)
	}
}

func TestUpdateCartItemQuantity(t *testing.T) {
	e := setup(t)
	sid, _, _ := AddToCart(add(e.bag.PublicId, 1), "", e.db, e.rdb)

	resp, err := UpdateCartItemQuantity(e.bag.PublicId, 7, sid, e.db, e.rdb)
	if err != nil || resp.Items[0].Quantity != 7 {
		t.Fatalf("update: %+v %v", resp, err)
	}

	if _, err := UpdateCartItemQuantity(e.bag.PublicId, 11, sid, e.db, e.rdb); err == nil || err.StatusCode != http.StatusConflict {
		t.Errorf("over stock: err = %+v, want 409", err)
	}
	if _, err := UpdateCartItemQuantity(e.hat.PublicId, 1, sid, e.db, e.rdb); err == nil || err.StatusCode != http.StatusNotFound {
		t.Errorf("item not in cart: err = %+v, want 404", err)
	}
	if _, err := UpdateCartItemQuantity(e.bag.PublicId, 1, "", e.db, e.rdb); err == nil || err.StatusCode != http.StatusNotFound {
		t.Errorf("no session: err = %+v, want 404", err)
	}
}

func TestRemoveCartItem(t *testing.T) {
	e := setup(t)
	sid, _, _ := AddToCart(add(e.bag.PublicId, 1), "", e.db, e.rdb)
	AddToCart(add(e.hat.PublicId, 1), sid, e.db, e.rdb)

	resp, err := RemoveCartItem(e.bag.PublicId, sid, e.rdb)
	if err != nil || len(resp.Items) != 1 || resp.Items[0].ProductId != e.hat.PublicId {
		t.Fatalf("remove: %+v %v", resp, err)
	}

	resp, err = RemoveCartItem(e.hat.PublicId, sid, e.rdb)
	if err != nil || resp.Items == nil || len(resp.Items) != 0 {
		t.Errorf("removing last item should leave empty (non-nil) list: %+v %v", resp, err)
	}

	if _, err := RemoveCartItem("prod_x", sid, e.rdb); err == nil || err.StatusCode != http.StatusNotFound {
		t.Errorf("missing item: err = %+v, want 404", err)
	}
}

func TestClearCart(t *testing.T) {
	e := setup(t)
	sid, _, _ := AddToCart(add(e.bag.PublicId, 1), "", e.db, e.rdb)

	if err := ClearCart(sid, e.rdb); err != nil {
		t.Fatalf("ClearCart: %v", err.Err)
	}
	if e.mr.Exists("cart:" + sid) {
		t.Error("cart still in redis after clear")
	}
	if err := ClearCart("", e.rdb); err != nil {
		t.Errorf("clearing empty session should be a no-op: %v", err.Err)
	}
}
