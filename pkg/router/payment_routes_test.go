package router

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"huddle/pkg/bachs"
)

const webhookSecret = "whsec_test_secret"

// withFakeBachs points the API at a fake Bachs before the router is built.
func withFakeBachs(t *testing.T) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{"checkout_id":"chk_route","checkout_url":"https://checkout.bachs.io/chk_route","status":"open","expires_at":"2030-01-01T00:00:00Z"}`)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("BACHS_BASE_URL", srv.URL)
	t.Setenv("BACHS_API_KEY", "sk_sandbox_test")
	t.Setenv("BACHS_WEBHOOK_SECRET", webhookSecret)
}

func (s shop) webhook(body string, signWith string) *httptest.ResponseRecorder {
	ts := time.Now().Unix()
	req := httptest.NewRequest(http.MethodPost, "/webhooks/bachs", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Bachs-Signature-V2", "t="+strconv.FormatInt(ts, 10)+",v1="+bachs.Sign(signWith, ts, []byte(body)))
	w := httptest.NewRecorder()
	s.r.ServeHTTP(w, req)
	return w
}

func TestPayments_PayAndWebhook(t *testing.T) {
	withFakeBachs(t)
	s := newShop(t)
	_, order, _ := s.checkout(s.fillCart(s.bag, 2), "", delivery)

	payPath := "/storefront/orders/" + order.Id + "/pay"
	w := s.request(http.MethodPost, payPath, map[string]string{"X-Store-Key": s.key}, nil,
		map[string]string{"success_url": "https://chioma.shop/thanks"})
	var pay struct {
		CheckoutURL string `json:"checkout_url"`
		Amount      string `json:"amount"`
	}
	decode(t, w, &pay)
	if w.Code != http.StatusOK || pay.CheckoutURL != "https://checkout.bachs.io/chk_route" || pay.Amount != "30000" {
		t.Fatalf("pay: %d %+v %s", w.Code, pay, w.Body.String())
	}

	for name, body := range map[string]map[string]string{
		"no success_url":  {},
		"javascript link": {"success_url": "javascript:alert(1)"},
	} {
		w := s.request(http.MethodPost, payPath, map[string]string{"X-Store-Key": s.key}, nil, body)
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s: %d, want 400", name, w.Code)
		}
	}
	if w := s.request(http.MethodPost, payPath, map[string]string{"X-Store-Key": s.otherKey}, nil,
		map[string]string{"success_url": "https://chioma.shop/thanks"}); w.Code != http.StatusNotFound {
		t.Errorf("pay with another store's key: %d, want 404", w.Code)
	}

	event := `{"id":"evt_route_1","type":"checkout.completed","data":{"checkout_id":"chk_route","payment_status":"paid","amount":"30000.00","currency":"NGN"}}`

	// Unsigned or wrongly signed deliveries are refused with 400 (no retry).
	if w := s.webhook(event, "whsec_wrong"); w.Code != http.StatusBadRequest {
		t.Errorf("bad signature: %d, want 400", w.Code)
	}
	unsigned := httptest.NewRequest(http.MethodPost, "/webhooks/bachs", bytes.NewBufferString(event))
	uw := httptest.NewRecorder()
	s.r.ServeHTTP(uw, unsigned)
	if uw.Code != http.StatusBadRequest {
		t.Errorf("unsigned: %d, want 400", uw.Code)
	}
	if got := s.orderStatus(order.Id); got != "pending" {
		t.Fatalf("order = %s after rejected webhooks, want pending", got)
	}

	if w := s.webhook(event, webhookSecret); w.Code != http.StatusOK {
		t.Fatalf("signed webhook: %d %s", w.Code, w.Body.String())
	}
	if got := s.orderStatus(order.Id); got != "paid" {
		t.Errorf("order = %s, want paid", got)
	}
	if w := s.webhook(event, webhookSecret); w.Code != http.StatusOK {
		t.Errorf("redelivery: %d, want 200", w.Code)
	}

	// Paying a paid order is refused.
	if w := s.request(http.MethodPost, payPath, map[string]string{"X-Store-Key": s.key}, nil,
		map[string]string{"success_url": "https://chioma.shop/thanks"}); w.Code != http.StatusConflict {
		t.Errorf("pay a paid order: %d, want 409", w.Code)
	}
}

func TestPayments_NotConfigured(t *testing.T) {
	t.Setenv("BACHS_API_KEY", "")
	t.Setenv("BACHS_WEBHOOK_SECRET", "")
	s := newShop(t)
	_, order, _ := s.checkout(s.fillCart(s.bag, 1), "", delivery)

	w := s.request(http.MethodPost, "/storefront/orders/"+order.Id+"/pay", map[string]string{"X-Store-Key": s.key}, nil,
		map[string]string{"success_url": "https://chioma.shop/thanks"})
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("pay without Bachs key: %d, want 503", w.Code)
	}
	// 503 makes Bachs retry once the secret is configured.
	if w := s.webhook(`{"id":"evt_1","type":"checkout.completed"}`, ""); w.Code != http.StatusServiceUnavailable {
		t.Errorf("webhook without secret: %d, want 503", w.Code)
	}
}

func (s shop) orderStatus(id string) string {
	s.t.Helper()
	code, env := s.do(http.MethodGet, "/store/"+s.storeId+"/orders/"+id, s.owner, nil)
	var o orderJSON
	s.data(env, &o)
	if code != http.StatusOK {
		s.t.Fatalf("get order: %d %s", code, env.Message)
	}
	return o.Status
}
