package router

import (
	"net/http"
	"strings"
	"testing"

	"huddle/pkg/utility"

	"github.com/google/uuid"
)

type orderJSON struct {
	Id     string `json:"id"`
	Status string `json:"status"`
	Total  string `json:"total"`
	Email  string `json:"email"`
	Items  []struct {
		ProductId string `json:"product_id"`
		Quantity  int    `json:"quantity"`
		LineTotal string `json:"line_total"`
	} `json:"items"`
}

type orderListJSON struct {
	Items []orderJSON `json:"items"`
	Total int         `json:"total"`
}

var delivery = map[string]any{
	"name": "Mama Nkechi", "phone": "+234 803 123 4567", "email": "",
	"delivery_method": "delivery", "address": "12 Balogun Market Road", "city": "Lagos Island", "state": "Lagos",
}

// fillCart adds quantity of product to a new cart and returns its cookies.
func (s shop) fillCart(product string, quantity int) []*http.Cookie {
	s.t.Helper()
	w := s.request(http.MethodPost, "/cart", map[string]string{"X-Store-Key": s.key}, nil, map[string]any{"product_id": product, "quantity": quantity})
	if w.Code != http.StatusOK {
		s.t.Fatalf("add to cart: %d %s", w.Code, w.Body.String())
	}
	return w.Result().Cookies()
}

// checkout places an order with the cart cookies; token may be empty for a guest.
func (s shop) checkout(cookies []*http.Cookie, token string, body any) (int, orderJSON, envelope) {
	s.t.Helper()
	headers := map[string]string{"X-Store-Key": s.key}
	if token != "" {
		headers["Authorization"] = "Bearer " + token
	}
	w := s.request(http.MethodPost, "/storefront/checkout", headers, cookies, body)
	var o orderJSON
	env := decode(s.t, w, nil)
	if w.Code == http.StatusCreated {
		s.data(env, &o)
	}
	return w.Code, o, env
}

func (s shop) signupCustomer(email string) string {
	s.t.Helper()
	code, env := s.do(http.MethodPost, "/customer/signup", "", map[string]string{
		"store_id": s.storeId, "name": "Ada", "email": email, "password": "secret123",
	})
	if code != http.StatusCreated {
		s.t.Fatalf("customer signup: %d %s", code, env.Message)
	}
	var d struct{ Token string }
	s.data(env, &d)
	return d.Token
}

func TestOrders_GuestCheckout(t *testing.T) {
	s := newShop(t)
	cookies := s.fillCart(s.bag, 2)

	code, order, env := s.checkout(cookies, "", delivery)
	if code != http.StatusCreated {
		t.Fatalf("checkout: %d %s %s", code, env.Message, env.Error)
	}
	if order.Status != "pending" || order.Total != "30000" || len(order.Items) != 1 || order.Items[0].Quantity != 2 {
		t.Fatalf("order = %+v", order)
	}
	if !strings.HasPrefix(order.Id, "ord_") {
		t.Errorf("order id %q", order.Id)
	}

	// The cart is empty and a second checkout has nothing to buy.
	var cart struct{ Items []any }
	decode(t, s.request(http.MethodGet, "/cart", map[string]string{"X-Store-Key": s.key}, cookies, nil), &cart)
	if len(cart.Items) != 0 {
		t.Errorf("cart has %d items after checkout", len(cart.Items))
	}
	if code, _, env := s.checkout(cookies, "", delivery); code != http.StatusBadRequest || env.Message != "Your cart is empty" {
		t.Errorf("second checkout: %d %s", code, env.Message)
	}

	// The shopper can open the order with the store key; another store can't.
	w := s.get("/storefront/orders/"+order.Id, s.key)
	var got orderJSON
	decode(t, w, &got)
	if w.Code != http.StatusOK || got.Id != order.Id {
		t.Errorf("view order: %d %+v", w.Code, got)
	}
	if w := s.get("/storefront/orders/"+order.Id, s.otherKey); w.Code != http.StatusNotFound {
		t.Errorf("view with another store's key: %d, want 404", w.Code)
	}

	// Stock was reserved: 10 - 2.
	var detail struct {
		Product struct {
			StockQuantity int `json:"stock_quantity"`
		} `json:"product"`
	}
	decode(t, s.get("/storefront/products/"+s.bag, s.key), &detail)
	if detail.Product.StockQuantity != 8 {
		t.Errorf("stock = %d, want 8", detail.Product.StockQuantity)
	}
}

func TestOrders_CheckoutValidation(t *testing.T) {
	s := newShop(t)
	cookies := s.fillCart(s.bag, 1)

	cases := []struct {
		name string
		body map[string]any
		want string
	}{
		{"no name", map[string]any{"phone": "08031234567", "delivery_method": "pickup"}, "name is required"},
		{"bad method", map[string]any{"name": "Ada", "phone": "08031234567", "delivery_method": "drone"}, "delivery_method must be one of: delivery, pickup"},
		{"delivery without address", map[string]any{"name": "Ada", "phone": "08031234567", "delivery_method": "delivery", "city": "Ikeja", "state": "Lagos"}, "address is required for delivery"},
		{"bad phone", map[string]any{"name": "Ada", "phone": "my number", "delivery_method": "pickup"}, "phone must be a valid phone number"},
	}
	for _, tc := range cases {
		code, _, env := s.checkout(cookies, "", tc.body)
		if code != http.StatusBadRequest || !strings.Contains(env.Message+env.Error, tc.want) {
			t.Errorf("%s: %d %s %s, want 400 %q", tc.name, code, env.Message, env.Error, tc.want)
		}
	}

	// Pickup needs no address.
	if code, _, env := s.checkout(cookies, "", map[string]any{"name": "Ada", "phone": "08031234567", "delivery_method": "pickup"}); code != http.StatusCreated {
		t.Errorf("pickup: %d %s %s", code, env.Message, env.Error)
	}
}

func TestOrders_CheckoutTooMany(t *testing.T) {
	s := newShop(t)
	cookies := s.fillCart(s.cap, 3)
	other := s.fillCart(s.cap, 2)

	if code, _, env := s.checkout(cookies, "", delivery); code != http.StatusCreated {
		t.Fatalf("first: %d %s", code, env.Message)
	}
	if code, _, env := s.checkout(other, "", delivery); code != http.StatusConflict || env.Message != "Fila cap is sold out" {
		t.Errorf("second: %d %s, want 409", code, env.Message)
	}
}

func TestOrders_SignedInCustomer(t *testing.T) {
	s := newShop(t)
	ada := s.signupCustomer("ada@example.com")

	code, order, env := s.checkout(s.fillCart(s.bag, 1), ada, delivery)
	if code != http.StatusCreated {
		t.Fatalf("checkout: %d %s", code, env.Message)
	}
	if order.Email != "ada@example.com" {
		t.Errorf("email = %q, want the account email", order.Email)
	}
	s.checkout(s.fillCart(s.cap, 1), "", delivery) // a guest order

	code, env = s.do(http.MethodGet, "/customer/orders", ada, nil)
	var list orderListJSON
	s.data(env, &list)
	if code != http.StatusOK || list.Total != 1 || list.Items[0].Id != order.Id {
		t.Errorf("customer orders: %d %+v", code, list)
	}

	bola := s.signupCustomer("bola@example.com")
	code, env = s.do(http.MethodGet, "/customer/orders", bola, nil)
	s.data(env, &list)
	if code != http.StatusOK || list.Total != 0 {
		t.Errorf("another customer sees %d orders", list.Total)
	}
	if code, _ := s.do(http.MethodGet, "/customer/orders", s.owner, nil); code != http.StatusForbidden {
		t.Errorf("admin token on customer orders: %d, want 403", code)
	}

	// A sent token must be a valid customer token; it is never ignored.
	cookies := s.fillCart(s.bag, 1)
	if code, _, _ := s.checkout(cookies, "not-a-token", delivery); code != http.StatusUnauthorized {
		t.Errorf("bad token: %d, want 401", code)
	}
	if code, _, _ := s.checkout(cookies, s.owner, delivery); code != http.StatusForbidden {
		t.Errorf("admin token: %d, want 403", code)
	}
	unknown, _ := utility.CreateToken(uuid.New(), "ghost@example.com", utility.RoleCustomer)
	if code, _, _ := s.checkout(cookies, unknown, delivery); code != http.StatusForbidden {
		t.Errorf("customer of no store: %d, want 403", code)
	}
}

func TestOrders_SellerManagesOrders(t *testing.T) {
	s := newShop(t)
	_, first, _ := s.checkout(s.fillCart(s.bag, 1), "", delivery)
	_, second, _ := s.checkout(s.fillCart(s.cap, 2), "", delivery)
	base := "/store/" + s.storeId + "/orders"

	code, env := s.do(http.MethodGet, base, s.owner, nil)
	var list orderListJSON
	s.data(env, &list)
	if code != http.StatusOK || list.Total != 2 || list.Items[0].Id != second.Id {
		t.Fatalf("list: %d %+v (want newest first)", code, list)
	}

	code, env = s.do(http.MethodPatch, base+"/"+first.Id, s.owner, map[string]string{"status": "paid"})
	var o orderJSON
	s.data(env, &o)
	if code != http.StatusOK || o.Status != "paid" {
		t.Fatalf("mark paid: %d %s", code, env.Message)
	}

	code, env = s.do(http.MethodGet, base+"?status=paid", s.owner, nil)
	s.data(env, &list)
	if list.Total != 1 || list.Items[0].Id != first.Id {
		t.Errorf("filter paid: %+v", list)
	}
	if code, _ := s.do(http.MethodGet, base+"?status=lost", s.owner, nil); code != http.StatusBadRequest {
		t.Errorf("bad status filter: %d, want 400", code)
	}

	if code, env := s.do(http.MethodPatch, base+"/"+second.Id, s.owner, map[string]string{"status": "fulfilled"}); code != http.StatusConflict {
		t.Errorf("fulfil unpaid: %d %s, want 409", code, env.Message)
	}
	if code, env := s.do(http.MethodPatch, base+"/"+second.Id, s.owner, map[string]string{"status": "expired"}); code != http.StatusBadRequest {
		t.Errorf("seller sets expired: %d %s, want 400", code, env.Message)
	}
	if code, env := s.do(http.MethodGet, base+"/"+second.Id, s.owner, nil); code != http.StatusOK {
		t.Errorf("get one: %d %s", code, env.Message)
	}

	// Other sellers and customers are kept out.
	for name, tc := range map[string]struct {
		token string
		want  int
	}{
		"other seller": {s.otherOwner, http.StatusForbidden},
		"customer":     {s.signupCustomer("ada@example.com"), http.StatusForbidden},
		"no token":     {"", http.StatusUnauthorized},
	} {
		if code, _ := s.do(http.MethodGet, base, tc.token, nil); code != tc.want {
			t.Errorf("%s list: %d, want %d", name, code, tc.want)
		}
		if code, _ := s.do(http.MethodPatch, base+"/"+second.Id, tc.token, map[string]string{"status": "cancelled"}); code != tc.want {
			t.Errorf("%s update: %d, want %d", name, code, tc.want)
		}
	}
}
