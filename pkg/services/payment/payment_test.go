package payment

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"huddle/pkg/bachs"
	"huddle/pkg/model"
	"huddle/pkg/services/cart"
	"huddle/pkg/services/order"
	"huddle/pkg/testutil"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// fakeBachs records checkout requests and answers like Bachs does.
type fakeBachs struct {
	mu       sync.Mutex
	requests []fakeRequest
	fail     string // when set, respond 400 with this detail
}

type fakeRequest struct {
	header http.Header
	body   bachs.CheckoutRequest
}

func (f *fakeBachs) client(t *testing.T) *bachs.Client {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body bachs.CheckoutRequest
		json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		f.requests = append(f.requests, fakeRequest{r.Header.Clone(), body})
		n := len(f.requests)
		f.mu.Unlock()
		if f.fail != "" {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"detail":%q,"error_code":"INVALID"}`, f.fail)
			return
		}
		w.WriteHeader(http.StatusCreated)
		expires := time.Now().UTC().Add(time.Duration(body.ExpiresInMinutes) * time.Minute).Format(time.RFC3339)
		fmt.Fprintf(w, `{"checkout_id":"chk_%d","checkout_url":"https://checkout.bachs.io/chk_%d","status":"open","expires_at":%q,"reference":%q}`, n, n, expires, body.Reference)
	}))
	t.Cleanup(srv.Close)
	return &bachs.Client{BaseURL: srv.URL, APIKey: "sk_sandbox_test", HTTP: srv.Client()}
}

type shop struct {
	t     *testing.T
	db    *gorm.DB
	rdb   *redis.Client
	admin model.Admins
	store model.Stores
	bag   model.Products
}

func newShop(t *testing.T) *shop {
	t.Helper()
	db := testutil.NewDB(t)
	_, rdb := testutil.NewRedis(t)
	admin := testutil.SeedAdmin(t, db)
	store := testutil.SeedStore(t, db, admin.Id)
	return &shop{t: t, db: db, rdb: rdb, admin: admin, store: store, bag: testutil.SeedProduct(t, db, store.ID, "Ankara Bag", 15000, 5)}
}

func (s *shop) connect(accountId string) {
	s.db.Model(&model.Stores{}).Where("id = ?", s.store.ID).Update("bachs_account_id", accountId)
}

// placeOrder checks out quantity bags and returns the order.
func (s *shop) placeOrder(quantity uint, email string) *model.OrderResponse {
	s.t.Helper()
	session, _, appErr := cart.AddToCart(&model.AddCartItemRequest{ProductId: s.bag.PublicId, Quantity: quantity}, s.store.StoreId, "", s.db, s.rdb)
	if appErr != nil {
		s.t.Fatalf("add to cart: %s", appErr.Message)
	}
	o, appErr := order.Checkout(s.db, s.rdb, s.store.ID, s.store.StoreId, session, nil, "",
		model.CheckoutRequest{Name: "Mama Nkechi", Phone: "08031234567", Email: email, DeliveryMethod: "pickup"})
	if appErr != nil {
		s.t.Fatalf("checkout: %s", appErr.Message)
	}
	return o
}

func (s *shop) pay(client *bachs.Client, orderId string) *model.PaymentResponse {
	s.t.Helper()
	p, appErr := StartPayment(s.db, client, s.store.ID, orderId, model.PayRequest{SuccessURL: "https://chioma.shop/thanks", CancelURL: "https://chioma.shop/cart"})
	if appErr != nil {
		s.t.Fatalf("start payment: %s %v", appErr.Message, appErr.Err)
	}
	return p
}

func (s *shop) orderStatus(id string) string {
	var o model.Orders
	s.db.Where("public_id = ?", id).First(&o)
	return o.Status
}

func (s *shop) payment(id string) model.Payments {
	var p model.Payments
	s.db.Where("public_id = ?", id).First(&p)
	return p
}

func (s *shop) stock() int {
	var p model.Products
	s.db.Where("id = ?", s.bag.Id).First(&p)
	return p.StockQuantity
}

func event(id, typ, checkoutId, amount, account string) *bachs.Event {
	return &bachs.Event{Id: id, Type: typ, Account: account, Data: bachs.EventData{
		CheckoutId: checkoutId, PaymentStatus: "paid", Status: "completed", Amount: amount, Currency: "NGN",
	}}
}

func (s *shop) handle(e *bachs.Event) {
	s.t.Helper()
	if err := HandleEvent(s.db, e, time.Now().UTC()); err != nil {
		s.t.Fatalf("handle %s: %v", e.Type, err)
	}
}

func TestStartPayment_DirectChargeToSeller(t *testing.T) {
	s := newShop(t)
	s.connect("acct_chioma")
	fake := &fakeBachs{}
	client := fake.client(t)
	o := s.placeOrder(2, "ada@example.com")

	p := s.pay(client, o.Id)
	if !strings.HasPrefix(p.CheckoutURL, "https://checkout.bachs.io/") || p.Amount.String() != "30000" {
		t.Fatalf("payment = %+v", p)
	}
	if len(fake.requests) != 1 {
		t.Fatalf("%d requests to Bachs, want 1", len(fake.requests))
	}
	req := fake.requests[0]
	if req.header.Get("X-Account-Id") != "acct_chioma" || req.header.Get("Idempotency-Key") != p.PaymentId {
		t.Errorf("headers: account %q, idempotency %q", req.header.Get("X-Account-Id"), req.header.Get("Idempotency-Key"))
	}
	b := req.body
	if b.Pricing.Amount != "30000.00" || b.Pricing.Currency != "NGN" || b.Reference != p.PaymentId || b.PlatformFee != "" {
		t.Errorf("body = %+v", b)
	}
	if b.ExpiresInMinutes < 27 || b.ExpiresInMinutes > 29 {
		t.Errorf("checkout lasts %d minutes; it must end before the 30 minute stock hold", b.ExpiresInMinutes)
	}
	if b.Customer == nil || b.Customer.Email != "ada@example.com" || b.Metadata["order_id"] != o.Id {
		t.Errorf("customer %+v metadata %v", b.Customer, b.Metadata)
	}
	if saved := s.payment(p.PaymentId); saved.Status != model.PaymentOpen || saved.AccountId == nil || *saved.AccountId != "acct_chioma" {
		t.Errorf("saved payment = %+v", saved)
	}

	// Asking again reuses the open checkout.
	if again := s.pay(client, o.Id); again.PaymentId != p.PaymentId || len(fake.requests) != 1 {
		t.Errorf("second pay created another checkout: %s, %d requests", again.PaymentId, len(fake.requests))
	}

	// A guest without email is sent without a customer; Bachs collects it.
	s.pay(client, s.placeOrder(1, "").Id)
	if c := fake.requests[1].body.Customer; c != nil {
		t.Errorf("customer sent without email: %+v", c)
	}
}

func TestStartPayment_PlatformFee(t *testing.T) {
	t.Setenv("BACHS_PLATFORM_FEE_PERCENT", "1.5")
	s := newShop(t)
	fake := &fakeBachs{}
	s.pay(fake.client(t), s.placeOrder(2, "").Id)
	if fee := fake.requests[0].body.PlatformFee; fee != "450.00" {
		t.Errorf("platform fee = %q, want 450.00 (1.5%% of 30000)", fee)
	}
}

func TestStartPayment_Rejects(t *testing.T) {
	s := newShop(t)
	fake := &fakeBachs{}
	client := fake.client(t)
	req := model.PayRequest{SuccessURL: "https://chioma.shop/thanks"}
	code := func(c *bachs.Client, orderId string) (int, string) {
		_, appErr := StartPayment(s.db, c, s.store.ID, orderId, req)
		if appErr == nil {
			return http.StatusOK, ""
		}
		return appErr.StatusCode, appErr.Message
	}

	o := s.placeOrder(1, "")
	if c, _ := code(nil, o.Id); c != http.StatusServiceUnavailable {
		t.Errorf("no Bachs key: %d, want 503", c)
	}
	if c, _ := code(client, "ord_missing"); c != http.StatusNotFound {
		t.Errorf("unknown order: %d, want 404", c)
	}
	other := testutil.SeedStore(t, s.db, s.admin.Id)
	if _, appErr := StartPayment(s.db, client, other.ID, o.Id, req); appErr == nil || appErr.StatusCode != http.StatusNotFound {
		t.Errorf("order of another store: %+v, want 404", appErr)
	}

	t.Setenv("APP_ENV", "production")
	if c, m := code(client, o.Id); c != http.StatusConflict || m != "This store can't take online payments yet" {
		t.Errorf("production store without account: %d %s", c, m)
	}
	t.Setenv("APP_ENV", "")

	fake.fail = "Amount is below the minimum of 100 NGN"
	if c, m := code(client, o.Id); c != http.StatusBadGateway || !strings.Contains(m, "below the minimum") {
		t.Errorf("Bachs refusal: %d %s, want 502 with its reason", c, m)
	}
	fake.fail = ""

	s.db.Model(&model.Orders{}).Where("public_id = ?", o.Id).Update("expires_at", time.Now().Add(90*time.Second))
	if c, _ := code(client, o.Id); c != http.StatusConflict {
		t.Errorf("order about to expire: %d, want 409", c)
	}

	paid := s.placeOrder(1, "")
	order.UpdateOrderStatus(s.db, s.store.StoreId, s.admin.Id, paid.Id, model.OrderPaid)
	if c, m := code(client, paid.Id); c != http.StatusConflict || m != "This order is already paid" {
		t.Errorf("paid order: %d %s", c, m)
	}
	var saved int64
	s.db.Model(&model.Payments{}).Count(&saved)
	if saved != 0 {
		t.Errorf("%d payments saved by rejected attempts", saved)
	}
}

func TestHandleEvent_PaysOrderOnce(t *testing.T) {
	s := newShop(t)
	s.connect("acct_chioma")
	o := s.placeOrder(2, "")
	p := s.pay((&fakeBachs{}).client(t), o.Id)
	chk := s.payment(p.PaymentId).CheckoutId

	s.handle(event("evt_1", "checkout.completed", chk, "30000.00", "acct_chioma"))
	if st := s.orderStatus(o.Id); st != model.OrderPaid {
		t.Fatalf("order = %s, want paid", st)
	}
	saved := s.payment(p.PaymentId)
	if saved.Status != model.PaymentPaid || saved.PaidAt == nil {
		t.Errorf("payment = %s", saved.Status)
	}

	// The matching collection.succeeded and a redelivery change nothing.
	s.handle(event("evt_2", "collection.succeeded", chk, "30000.00", "acct_chioma"))
	s.handle(event("evt_1", "checkout.completed", chk, "30000.00", "acct_chioma"))
	if st := s.payment(p.PaymentId).Status; st != model.PaymentPaid {
		t.Errorf("payment after repeats = %s, want paid", st)
	}
	if got := s.stock(); got != 3 {
		t.Errorf("stock = %d, want 3", got)
	}
	var events int64
	s.db.Model(&model.WebhookEvents{}).Count(&events)
	if events != 2 {
		t.Errorf("%d events recorded, want 2", events)
	}

	// Events that aren't ours, or aren't payments, are ignored.
	s.handle(event("evt_3", "collection.succeeded", "chk_someone_else", "1.00", ""))
	s.handle(&bachs.Event{Id: "evt_4", Type: "payout.paid"})
	unpaid := event("evt_5", "checkout.completed", chk, "30000.00", "acct_chioma")
	unpaid.Data.PaymentStatus = "no_payment_required"
	s.handle(unpaid)
}

func TestHandleEvent_MismatchNeedsReview(t *testing.T) {
	s := newShop(t)
	s.connect("acct_chioma")
	s.db.Model(&model.Products{}).Where("id = ?", s.bag.Id).Update("stock_quantity", 100)
	client := (&fakeBachs{}).client(t)

	cases := map[string]*bachs.Event{
		"wrong amount":   event("evt_a", "collection.succeeded", "", "100.00", "acct_chioma"),
		"wrong currency": event("evt_b", "collection.succeeded", "", "30000.00", "acct_chioma"),
		"wrong account":  event("evt_c", "collection.succeeded", "", "30000.00", "acct_mallam"),
		"no account":     event("evt_d", "collection.succeeded", "", "30000.00", ""),
	}
	cases["wrong currency"].Data.Currency = "USD"
	for name, e := range cases {
		o := s.placeOrder(2, "")
		p := s.pay(client, o.Id)
		e.Data.CheckoutId = s.payment(p.PaymentId).CheckoutId
		s.handle(e)
		if st := s.payment(p.PaymentId).Status; st != model.PaymentReview {
			t.Errorf("%s: payment = %s, want review", name, st)
		}
		if st := s.orderStatus(o.Id); st != model.OrderPending {
			t.Errorf("%s: order = %s, want pending", name, st)
		}
	}
}

func TestHandleEvent_LatePayment(t *testing.T) {
	s := newShop(t)
	client := (&fakeBachs{}).client(t)

	// The order expired, but the stock is still there: the order is revived.
	o := s.placeOrder(2, "")
	p := s.pay(client, o.Id)
	chk := s.payment(p.PaymentId).CheckoutId
	s.handle(&bachs.Event{Id: "evt_x", Type: "checkout.expired", Data: bachs.EventData{CheckoutId: chk}})
	if st := s.payment(p.PaymentId).Status; st != model.PaymentExpired {
		t.Fatalf("payment = %s, want expired", st)
	}
	order.ExpireOrders(s.db, time.Now().Add(time.Hour))
	if got := s.stock(); got != 5 {
		t.Fatalf("stock = %d, want 5 after expiry", got)
	}
	s.handle(event("evt_late", "collection.succeeded", chk, "30000.00", ""))
	if st := s.orderStatus(o.Id); st != model.OrderPaid {
		t.Errorf("order = %s, want paid", st)
	}
	if got := s.stock(); got != 3 {
		t.Errorf("stock = %d, want 3 (taken again)", got)
	}

	// The order expired and the stock was sold to someone else: refund.
	gone := s.placeOrder(2, "")
	p = s.pay(client, gone.Id)
	chk = s.payment(p.PaymentId).CheckoutId
	order.ExpireOrders(s.db, time.Now().Add(time.Hour))
	s.db.Model(&model.Products{}).Where("id = ?", s.bag.Id).Update("stock_quantity", 1)
	s.handle(event("evt_gone", "collection.succeeded", chk, "30000.00", ""))
	if st := s.payment(p.PaymentId).Status; st != model.PaymentNeedsRefund {
		t.Errorf("payment = %s, want needs_refund", st)
	}
	if st := s.orderStatus(gone.Id); st != model.OrderExpired {
		t.Errorf("order = %s, want expired", st)
	}
	if got := s.stock(); got != 1 {
		t.Errorf("stock = %d, want 1 (untouched)", got)
	}
}

func TestHandleEvent_SellerAlreadyMarkedPaid(t *testing.T) {
	s := newShop(t)
	o := s.placeOrder(1, "")
	p := s.pay((&fakeBachs{}).client(t), o.Id)
	order.UpdateOrderStatus(s.db, s.store.StoreId, s.admin.Id, o.Id, model.OrderPaid)

	s.handle(event("evt_1", "checkout.completed", s.payment(p.PaymentId).CheckoutId, "15000.00", ""))
	if st := s.payment(p.PaymentId).Status; st != model.PaymentNeedsRefund {
		t.Errorf("payment = %s, want needs_refund (paid twice)", st)
	}
}
