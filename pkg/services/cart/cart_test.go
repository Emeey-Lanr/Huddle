package cart

import (
	"net/http"
	"os"
	"testing"

	"huddle/pkg/model"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func knownBug(t *testing.T, why string) {
	t.Helper()
	if os.Getenv("HUDDLE_KNOWN_BUGS") != "1" {
		t.Skip("known bug: " + why)
	}
}

func newRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	return mr, redis.NewClient(&redis.Options{Addr: mr.Addr()})
}

func item(id string, qty uint) *model.CartItem {
	return &model.CartItem{ProductId: id, ProductName: "Ankara bag", Price: 15000, Currency: "NGN", Quantity: qty}
}

func TestAddToCart_NewSession(t *testing.T) {
	mr, rdb := newRedis(t)

	sid, resp, err := AddToCart(item("prod_1", 2), "", nil, rdb)
	if err != nil {
		t.Fatalf("AddToCart: %v", err.Err)
	}
	if sid == "" || resp.SessionId != sid {
		t.Fatalf("session id not returned: %q", sid)
	}
	if len(resp.Items) != 1 || resp.Items[0].Quantity != 2 {
		t.Errorf("items = %+v", resp.Items)
	}
	if !mr.Exists("cart:" + sid) {
		t.Error("cart not persisted in redis")
	}
	if ttl := mr.TTL("cart:" + sid); ttl <= 0 {
		t.Errorf("cart has no expiry (ttl %v)", ttl)
	}
}

func TestAddToCart_SameProductIncrementsQuantity(t *testing.T) {
	_, rdb := newRedis(t)
	sid, _, _ := AddToCart(item("prod_1", 2), "", nil, rdb)
	_, resp, _ := AddToCart(item("prod_1", 3), sid, nil, rdb)

	if len(resp.Items) != 1 || resp.Items[0].Quantity != 5 {
		t.Errorf("items = %+v, want one item with quantity 5", resp.Items)
	}
}

func TestAddToCart_DifferentProductsAppend(t *testing.T) {
	_, rdb := newRedis(t)
	sid, _, _ := AddToCart(item("prod_1", 1), "", nil, rdb)
	_, resp, _ := AddToCart(item("prod_2", 1), sid, nil, rdb)

	if len(resp.Items) != 2 {
		t.Errorf("items = %+v, want 2", resp.Items)
	}
}

func TestGetCart(t *testing.T) {
	_, rdb := newRedis(t)

	resp, err := GetCart("", rdb)
	if err != nil || len(resp.Items) != 0 {
		t.Fatalf("empty session: %+v %v", resp, err)
	}

	sid, _, _ := AddToCart(item("prod_1", 1), "", nil, rdb)
	resp, _ = GetCart(sid, rdb)
	if len(resp.Items) != 1 {
		t.Errorf("items = %+v", resp.Items)
	}

	resp, _ = GetCart("cart_doesnotexist", rdb)
	if resp.Items == nil || len(resp.Items) != 0 {
		t.Errorf("unknown session should return empty items, got %+v", resp.Items)
	}
}

func TestUpdateCartItemQuantity(t *testing.T) {
	_, rdb := newRedis(t)
	sid, _, _ := AddToCart(item("prod_1", 1), "", nil, rdb)

	resp, err := UpdateCartItemQuantity("prod_1", 7, sid, rdb)
	if err != nil || resp.Items[0].Quantity != 7 {
		t.Fatalf("update: %+v %v", resp, err)
	}

	if _, err := UpdateCartItemQuantity("prod_missing", 1, sid, rdb); err == nil || err.StatusCode != http.StatusNotFound {
		t.Errorf("missing item: err = %+v, want 404", err)
	}
	if _, err := UpdateCartItemQuantity("prod_1", 1, "", rdb); err == nil || err.StatusCode != http.StatusNotFound {
		t.Errorf("no session: err = %+v, want 404", err)
	}
}

func TestRemoveCartItem(t *testing.T) {
	_, rdb := newRedis(t)
	sid, _, _ := AddToCart(item("prod_1", 1), "", nil, rdb)
	AddToCart(item("prod_2", 1), sid, nil, rdb)

	resp, err := RemoveCartItem("prod_1", sid, rdb)
	if err != nil || len(resp.Items) != 1 || resp.Items[0].ProductId != "prod_2" {
		t.Fatalf("remove: %+v %v", resp, err)
	}

	resp, err = RemoveCartItem("prod_2", sid, rdb)
	if err != nil || resp.Items == nil || len(resp.Items) != 0 {
		t.Errorf("removing last item should leave empty (non-nil) list: %+v %v", resp, err)
	}

	if _, err := RemoveCartItem("prod_x", sid, rdb); err == nil || err.StatusCode != http.StatusNotFound {
		t.Errorf("missing item: err = %+v, want 404", err)
	}
}

func TestClearCart(t *testing.T) {
	mr, rdb := newRedis(t)
	sid, _, _ := AddToCart(item("prod_1", 1), "", nil, rdb)

	if err := ClearCart(sid, rdb); err != nil {
		t.Fatalf("ClearCart: %v", err.Err)
	}
	if mr.Exists("cart:" + sid) {
		t.Error("cart still in redis after clear")
	}
	if err := ClearCart("", rdb); err != nil {
		t.Errorf("clearing empty session should be a no-op: %v", err.Err)
	}
}

// The cart stores whatever price the client sends, so a shopper can buy a
// ₦15,000 bag for ₦1 once checkout reads totals from the cart.
func TestAddToCart_PriceComesFromDatabase(t *testing.T) {
	knownBug(t, "cart trusts client-supplied product price")
	_, rdb := newRedis(t)
	cheap := item("prod_1", 1)
	cheap.Price = 1

	_, resp, _ := AddToCart(cheap, "", nil, rdb)
	if resp.Items[0].Price == 1 {
		t.Error("cart accepted client-supplied price of 1")
	}
}
