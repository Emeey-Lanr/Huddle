package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type productJSON struct {
	Id    string `json:"id"`
	Name  string `json:"name"`
	Price string `json:"price"`
}

type listJSON struct {
	Items []productJSON `json:"items"`
	Total int           `json:"total"`
}

// shop sets up Chioma's store with 3 products (one hidden) and Tunde's store
// with one product, all through the admin API.
type shop struct {
	*server
	key, otherKey             string
	bag, cap, hidden, foreign string
	owner, storeId            string // Chioma's admin token and store id
	otherOwner                string // Tunde's admin token
}

func newShop(t *testing.T) shop {
	t.Helper()
	s := newServer(t)
	chioma := s.signupAdmin("chioma")
	storeId, key := s.createStoreWithKey(chioma, "Chioma's Bags")
	bag := s.addProduct(chioma, storeId, "Ankara bag", 15000, 10)
	cap := s.addProduct(chioma, storeId, "Fila cap", 5000, 3)
	hidden := s.addProduct(chioma, storeId, "Draft item", 1000, 5)
	if code, env := s.do(http.MethodPut, "/product/"+hidden, chioma, map[string]any{"active": false}); code != http.StatusOK {
		t.Fatalf("hide product: %d %s", code, env.Message)
	}

	tunde := s.signupAdmin("tunde")
	tundeStore, tundeKey := s.createStoreWithKey(tunde, "Tunde Tech")
	foreign := s.addProduct(tunde, tundeStore, "Phone case", 3000, 5)

	return shop{server: s, key: key, otherKey: tundeKey, bag: bag, cap: cap, hidden: hidden, foreign: foreign,
		owner: chioma, storeId: storeId, otherOwner: tunde}
}

func (s shop) get(path, key string) *httptest.ResponseRecorder {
	return s.request(http.MethodGet, path, map[string]string{"X-Store-Key": key}, nil, nil)
}

func TestStorefront_RequiresValidKey(t *testing.T) {
	s := newShop(t)
	for _, path := range []string{"/storefront/store", "/storefront/products", "/storefront/products/" + s.bag, "/cart"} {
		if w := s.request(http.MethodGet, path, nil, nil, nil); w.Code != http.StatusUnauthorized {
			t.Errorf("%s without key: %d, want 401", path, w.Code)
		}
		if w := s.get(path, "not-a-real-key"); w.Code != http.StatusUnauthorized {
			t.Errorf("%s with wrong key: %d, want 401", path, w.Code)
		}
	}
}

func TestStorefront_StoreInfo(t *testing.T) {
	s := newShop(t)
	w := s.get("/storefront/store", s.key)
	var store struct {
		StoreName string `json:"store_name"`
	}
	decode(t, w, &store)
	if w.Code != http.StatusOK || store.StoreName != "Chioma's Bags" {
		t.Errorf("%d %+v", w.Code, store)
	}
	if body := w.Body.String(); strings.Contains(body, "api_key") || strings.Contains(body, "admin_id") {
		t.Errorf("store info leaks private fields: %s", body)
	}
}

func TestStorefront_ListsOnlyActiveProductsOfThisStore(t *testing.T) {
	s := newShop(t)
	w := s.get("/storefront/products?sort=price&order=asc", s.key)
	var list listJSON
	decode(t, w, &list)
	if w.Code != http.StatusOK || list.Total != 2 || len(list.Items) != 2 {
		t.Fatalf("%d total=%d items=%+v, want the 2 active products", w.Code, list.Total, list.Items)
	}
	if list.Items[0].Name != "Fila cap" || list.Items[1].Name != "Ankara bag" {
		t.Errorf("sorted by price: %+v", list.Items)
	}

	// ?active=false is not a storefront option; hidden products stay hidden.
	decode(t, s.get("/storefront/products?active=false", s.key), &list)
	if list.Total != 2 {
		t.Errorf("active=false exposed hidden products: total %d", list.Total)
	}

	if w := s.get("/storefront/products?sort=password", s.key); w.Code != http.StatusBadRequest {
		t.Errorf("bad sort: %d, want 400", w.Code)
	}
}

func TestStorefront_ProductDetail(t *testing.T) {
	s := newShop(t)
	w := s.get("/storefront/products/"+s.bag, s.key)
	var d struct {
		Product productJSON
		Images  []any
	}
	decode(t, w, &d)
	if w.Code != http.StatusOK || d.Product.Name != "Ankara bag" || d.Product.Price != "15000" || d.Images == nil {
		t.Errorf("%d %+v", w.Code, d)
	}

	for name, path := range map[string]string{
		"hidden product":          "/storefront/products/" + s.hidden,
		"another store's product": "/storefront/products/" + s.foreign,
		"unknown product":         "/storefront/products/prod_missing",
	} {
		if w := s.get(path, s.key); w.Code != http.StatusNotFound {
			t.Errorf("%s: %d, want 404", name, w.Code)
		}
	}
}

func TestStorefront_CartFlowWithCookie(t *testing.T) {
	s := newShop(t)
	headers := map[string]string{"X-Store-Key": s.key}

	w := s.request(http.MethodPost, "/cart", headers, nil, map[string]any{"product_id": s.bag, "quantity": 2})
	if w.Code != http.StatusOK {
		t.Fatalf("add: %d %s", w.Code, w.Body.String())
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "cart_session" {
		t.Fatalf("cart cookie not set: %+v", cookies)
	}

	var cart struct {
		Items []struct {
			ProductId string `json:"product_id"`
			Quantity  int    `json:"quantity"`
		}
	}
	w = s.request(http.MethodGet, "/cart", headers, cookies, nil)
	decode(t, w, &cart)
	if len(cart.Items) != 1 || cart.Items[0].Quantity != 2 {
		t.Errorf("cart = %+v", cart)
	}

	// Another store's product cannot go into this store's cart.
	if w := s.request(http.MethodPost, "/cart", headers, cookies, map[string]any{"product_id": s.foreign, "quantity": 1}); w.Code != http.StatusNotFound {
		t.Errorf("foreign product: %d, want 404", w.Code)
	}

	// The same session has a separate, empty cart at the other store.
	w = s.request(http.MethodGet, "/cart", map[string]string{"X-Store-Key": s.otherKey}, cookies, nil)
	decode(t, w, &cart)
	if len(cart.Items) != 0 {
		t.Errorf("other store's cart = %+v, want empty", cart.Items)
	}
}
