package router

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// withFakeBachsRefunds is a fake Bachs for checkouts and refunds.
func withFakeBachsRefunds(t *testing.T) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/checkout-sessions":
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, `{"checkout_id":"chk_refund","checkout_url":"https://checkout.bachs.io/chk_refund","status":"open","expires_at":"2030-01-01T00:00:00Z"}`)
		case "/v1/refunds":
			var body struct{ Reference string }
			json.NewDecoder(r.Body).Decode(&body)
			w.WriteHeader(http.StatusCreated)
			fmt.Fprintf(w, `{"refund_id":"ref_route","reference":%q,"status":"processing"}`, body.Reference)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("BACHS_BASE_URL", srv.URL)
	t.Setenv("BACHS_API_KEY", "sk_sandbox_test")
	t.Setenv("BACHS_WEBHOOK_SECRET", webhookSecret)
}

func TestPayments_RefundRoutes(t *testing.T) {
	withFakeBachsRefunds(t)
	s := newShop(t)
	_, order, _ := s.checkout(s.fillCart(s.bag, 2), "", delivery)

	w := s.request(http.MethodPost, "/storefront/orders/"+order.Id+"/pay", map[string]string{"X-Store-Key": s.key}, nil,
		map[string]string{"success_url": "https://chioma.shop/thanks"})
	var pay struct {
		PaymentId string `json:"payment_id"`
	}
	decode(t, w, &pay)
	paid := `{"id":"evt_refund_paid","type":"checkout.completed","data":{"checkout_id":"chk_refund","payment_status":"paid","amount":"30000.00","currency":"NGN","charge":{"id":"ch_route"}}}`
	if w := s.webhook(paid, webhookSecret); w.Code != http.StatusOK {
		t.Fatalf("paid webhook: %d %s", w.Code, w.Body.String())
	}

	orderPath := "/store/" + s.storeId + "/orders/" + order.Id
	var seen struct {
		Payment *struct {
			PaymentId string `json:"payment_id"`
			Status    string `json:"status"`
		} `json:"payment"`
	}
	code, env := s.do(http.MethodGet, orderPath, s.owner, nil)
	s.data(env, &seen)
	if code != http.StatusOK || seen.Payment == nil || seen.Payment.PaymentId != pay.PaymentId || seen.Payment.Status != "paid" {
		t.Fatalf("seller order: %d %s", code, env.Data)
	}
	if w := s.get("/storefront/orders/"+order.Id, s.key); w.Code != http.StatusOK || strings.Contains(w.Body.String(), pay.PaymentId) {
		t.Errorf("storefront order shows the payment: %s", w.Body.String())
	}

	refundPath := "/store/" + s.storeId + "/payments/" + pay.PaymentId + "/refund"
	if code, _ := s.do(http.MethodPost, refundPath, "", nil); code != http.StatusUnauthorized {
		t.Errorf("no token: %d, want 401", code)
	}
	if code, _ := s.do(http.MethodPost, refundPath, s.otherOwner, nil); code != http.StatusForbidden {
		t.Errorf("another seller: %d, want 403", code)
	}
	if code, _ := s.do(http.MethodPost, "/store/"+s.storeId+"/payments/pay_missing/refund", s.owner, nil); code != http.StatusNotFound {
		t.Errorf("unknown payment: %d, want 404", code)
	}
	if code, _ := s.do(http.MethodPost, refundPath, s.owner, map[string]string{"reason": strings.Repeat("a", 501)}); code != http.StatusBadRequest {
		t.Errorf("reason too long: %d, want 400", code)
	}

	var refund struct {
		RefundId string `json:"refund_id"`
		Status   string `json:"status"`
	}
	code, env = s.do(http.MethodPost, refundPath, s.owner, nil)
	s.data(env, &refund)
	if code != http.StatusOK || refund.Status != "processing" {
		t.Fatalf("refund: %d %s %s", code, env.Message, env.Data)
	}

	done := fmt.Sprintf(`{"id":"evt_refund_done","type":"refund.paid","data":{"refund_id":"ref_route","reference":%q,"status":"paid"}}`, refund.RefundId)
	if w := s.webhook(done, webhookSecret); w.Code != http.StatusOK {
		t.Fatalf("refund webhook: %d %s", w.Code, w.Body.String())
	}
	if got := s.orderStatus(order.Id); got != "cancelled" {
		t.Errorf("order = %s after refund, want cancelled", got)
	}

	code, env = s.do(http.MethodGet, "/store/"+s.storeId+"/payments/issues", s.owner, nil)
	if code != http.StatusOK || string(env.Data) != "[]" {
		t.Errorf("issues: %d %s, want an empty list", code, env.Data)
	}
	if code, _ := s.do(http.MethodGet, "/store/"+s.storeId+"/payments/issues", s.otherOwner, nil); code != http.StatusForbidden {
		t.Errorf("another seller's issues: %d, want 403", code)
	}
}
