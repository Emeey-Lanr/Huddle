package payment

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"huddle/pkg/bachs"
	"huddle/pkg/model"
	"huddle/pkg/services/order"
	"huddle/pkg/testutil"

	"github.com/shopspring/decimal"
)

// fakeRefunds answers POST /v1/refunds. mode is "processing", "success",
// "refuse" (400 with detail) or "down" (500).
type fakeRefunds struct {
	mu       sync.Mutex
	mode     string
	detail   string
	requests []refundCall
}

type refundCall struct {
	accountId, idempotencyKey string
	body                      bachs.RefundRequest
}

func (f *fakeRefunds) client(t *testing.T) *bachs.Client {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/balances" {
			fmt.Fprint(w, `{"balances":[{"currency":"USD","available_balance":"0.00"},{"currency":"NGN","available_balance":"14700.00"}]}`)
			return
		}
		var body bachs.RefundRequest
		json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		f.requests = append(f.requests, refundCall{r.Header.Get("X-Account-Id"), r.Header.Get("Idempotency-Key"), body})
		mode := f.mode
		f.mu.Unlock()
		switch mode {
		case "refuse":
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"detail":%q,"error_code":"BAD_REQUEST"}`, f.detail)
		case "down":
			w.WriteHeader(http.StatusInternalServerError)
		default:
			status := "processing"
			if mode == "success" {
				status = "success"
			}
			w.WriteHeader(http.StatusCreated)
			fmt.Fprintf(w, `{"refund_id":"ref_%s","charge_id":%q,"reference":%q,"status":%q}`, body.Reference, body.ChargeId, body.Reference, status)
		}
	}))
	t.Cleanup(srv.Close)
	return &bachs.Client{BaseURL: srv.URL, APIKey: "sk_sandbox_test", HTTP: srv.Client()}
}

// paidOrder places and pays an order through the seller's account, the way
// Bachs reports it, and returns the order and payment ids.
func (s *shop) paidOrder(quantity uint, charge string) (string, string) {
	s.t.Helper()
	o := s.placeOrder(quantity, "")
	p := s.pay((&fakeBachs{}).client(s.t), o.Id)
	e := event("evt_paid_"+p.PaymentId, "checkout.completed", s.payment(p.PaymentId).CheckoutId, p.Amount.StringFixed(2), "acct_chioma")
	e.Data.Charge = &bachs.EventCharge{Id: charge}
	s.handle(e)
	return o.Id, p.PaymentId
}

func (s *shop) refund(client *bachs.Client, paymentId string) (*model.RefundResponse, int, string) {
	s.t.Helper()
	r, appErr := Refund(s.db, client, s.store.ID, paymentId, model.RefundRequest{Reason: "Out of stock"})
	if appErr != nil {
		return nil, appErr.StatusCode, appErr.Message
	}
	return r, http.StatusOK, ""
}

func refundEventFor(id, typ, reference string) *bachs.Event {
	return &bachs.Event{Id: id, Type: typ, Account: "acct_chioma", Data: bachs.EventData{Reference: reference, RefundId: "ref_" + reference}}
}

func TestRefund_PaidOrderIsCancelledWhenRefundIsPaid(t *testing.T) {
	s := newShop(t)
	s.connect("acct_chioma")
	orderId, paymentId := s.paidOrder(2, "ch_1")
	if got := s.payment(paymentId).ChargeId; got == nil || *got != "ch_1" {
		t.Fatalf("charge id = %v, want ch_1", got)
	}
	if s.stock() != 3 {
		t.Fatalf("stock = %d, want 3", s.stock())
	}

	fake := &fakeRefunds{mode: "processing"}
	client := fake.client(t)
	r, code, msg := s.refund(client, paymentId)
	if code != http.StatusOK {
		t.Fatalf("refund: %d %s", code, msg)
	}
	if r.Status != "processing" || !r.Amount.Equal(s.payment(paymentId).Amount) || r.Reason != "Out of stock" {
		t.Errorf("refund = %+v", r)
	}
	call := fake.requests[0]
	if call.accountId != "acct_chioma" || call.body.ChargeId != "ch_1" || call.body.Reference != r.RefundId ||
		call.idempotencyKey != r.RefundId || call.body.IdempotencyKey != r.RefundId || call.body.Amount != "" {
		t.Errorf("Bachs request = %+v", call)
	}
	if got := s.orderStatus(orderId); got != "paid" {
		t.Errorf("order = %s while the refund is processing, want paid", got)
	}

	if _, code, msg := s.refund(client, paymentId); code != http.StatusConflict || !strings.Contains(msg, "in progress") {
		t.Errorf("second refund: %d %s, want 409 in progress", code, msg)
	}

	s.handle(refundEventFor("evt_r1", "refund.created", r.RefundId))
	s.handle(refundEventFor("evt_r2", "refund.paid", r.RefundId))
	if got := s.payment(paymentId).Status; got != "refunded" {
		t.Errorf("payment = %s, want refunded", got)
	}
	if got := s.orderStatus(orderId); got != "cancelled" {
		t.Errorf("order = %s, want cancelled", got)
	}
	if s.stock() != 5 {
		t.Errorf("stock = %d after refund, want 5", s.stock())
	}

	// A late failure report can't undo a paid refund.
	s.handle(refundEventFor("evt_r3", "refund.failed", r.RefundId))
	var saved model.Refunds
	s.db.Where("public_id = ?", r.RefundId).First(&saved)
	if saved.Status != "paid" || saved.BachsRefundId == nil || saved.CompletedAt == nil {
		t.Errorf("refund row = %+v", saved)
	}
	if _, code, msg := s.refund(client, paymentId); code != http.StatusConflict || msg != "This payment is already refunded" {
		t.Errorf("refund again: %d %s", code, msg)
	}
}

func TestRefund_RefusedCanBeTriedAgain(t *testing.T) {
	s := newShop(t)
	s.connect("acct_chioma")
	orderId, paymentId := s.paidOrder(1, "ch_2")

	fake := &fakeRefunds{mode: "refuse", detail: "Insufficient balance to refund"}
	client := fake.client(t)
	if _, code, msg := s.refund(client, paymentId); code != http.StatusConflict ||
		msg != "Your Bachs balance (NGN 14700.00) is less than this refund (NGN 15000.00). Refund up to NGN 14700.00, or try again after more sales" {
		t.Errorf("refused refund: %d %s, want 409 with the balance", code, msg)
	}
	var failed model.Refunds
	s.db.Where("status = ?", "failed").First(&failed)
	if failed.FailureReason != "Insufficient balance to refund" {
		t.Errorf("failed refund = %+v", failed)
	}

	fake.mode = "success"
	r, code, msg := s.refund(client, paymentId)
	if code != http.StatusOK || r.Status != "paid" {
		t.Fatalf("retry: %d %s %+v", code, msg, r)
	}
	if r.RefundId == failed.PublicId {
		t.Error("retry reused the failed refund's reference")
	}
	if got := s.orderStatus(orderId); got != "cancelled" {
		t.Errorf("order = %s, want cancelled", got)
	}
}

func TestRefund_UnconfirmedRefundIsResentNotDuplicated(t *testing.T) {
	s := newShop(t)
	s.connect("acct_chioma")
	_, paymentId := s.paidOrder(1, "ch_3")

	fake := &fakeRefunds{mode: "down"}
	client := fake.client(t)
	if _, code, _ := s.refund(client, paymentId); code != http.StatusBadGateway {
		t.Errorf("Bachs down: %d, want 502", code)
	}
	fake.mode = "processing"
	r, code, msg := s.refund(client, paymentId)
	if code != http.StatusOK || r.Status != "processing" {
		t.Fatalf("retry: %d %s", code, msg)
	}
	if len(fake.requests) != 2 || fake.requests[0].body.Reference != fake.requests[1].body.Reference {
		t.Errorf("requests = %+v, want the same refund sent twice", fake.requests)
	}
	var n int64
	s.db.Model(&model.Refunds{}).Count(&n)
	if n != 1 {
		t.Errorf("%d refunds saved, want 1", n)
	}
}

func TestRefund_Rejects(t *testing.T) {
	s := newShop(t)
	s.connect("acct_chioma")
	client := (&fakeRefunds{}).client(t)
	_, paymentId := s.paidOrder(1, "ch_4")

	if _, code, _ := s.refund(nil, paymentId); code != http.StatusServiceUnavailable {
		t.Errorf("no Bachs key: %d, want 503", code)
	}
	if _, code, _ := s.refund(client, "pay_missing"); code != http.StatusNotFound {
		t.Errorf("unknown payment: %d, want 404", code)
	}
	other := testutil.SeedStore(t, s.db, s.admin.Id)
	if _, appErr := Refund(s.db, client, other.ID, paymentId, model.RefundRequest{}); appErr == nil || appErr.StatusCode != http.StatusNotFound {
		t.Errorf("another store's payment: %+v, want 404", appErr)
	}

	open := s.pay((&fakeBachs{}).client(t), s.placeOrder(1, "").Id)
	if _, code, _ := s.refund(client, open.PaymentId); code != http.StatusConflict {
		t.Errorf("open payment: %d, want 409", code)
	}
	s.db.Model(&model.Payments{}).Where("public_id = ?", open.PaymentId).Update("status", "paid")
	if _, code, msg := s.refund(client, open.PaymentId); code != http.StatusConflict || !strings.Contains(msg, "not confirmed") {
		t.Errorf("paid without a charge id: %d %s", code, msg)
	}
}

func TestRefund_NeedsRefundLeavesTheOrderAlone(t *testing.T) {
	s := newShop(t)
	s.connect("acct_chioma")
	o := s.placeOrder(1, "")
	p := s.pay((&fakeBachs{}).client(t), o.Id)
	// The seller took a bank transfer first, so this payment is a duplicate.
	order.UpdateOrderStatus(s.db, s.store.StoreId, s.admin.Id, o.Id, model.OrderPaid)
	e := event("evt_dup", "collection.succeeded", s.payment(p.PaymentId).CheckoutId, p.Amount.StringFixed(2), "acct_chioma")
	e.Data.ChargeId = "ch_5"
	s.handle(e)
	if got := s.payment(p.PaymentId).Status; got != "needs_refund" {
		t.Fatalf("payment = %s, want needs_refund", got)
	}

	issues, appErr := Issues(s.db, s.store.ID)
	if appErr != nil || len(issues) != 1 || issues[0].PaymentId != p.PaymentId || issues[0].OrderId != o.Id || issues[0].Status != "needs_refund" {
		t.Fatalf("issues = %+v %v", issues, appErr)
	}

	r, code, msg := s.refund((&fakeRefunds{mode: "success"}).client(t), p.PaymentId)
	if code != http.StatusOK || r.Status != "paid" {
		t.Fatalf("refund: %d %s", code, msg)
	}
	if got := s.orderStatus(o.Id); got != "paid" {
		t.Errorf("order = %s, want still paid (by the seller's transfer)", got)
	}
	if issues, _ := Issues(s.db, s.store.ID); len(issues) != 0 {
		t.Errorf("issues after refund = %+v, want none", issues)
	}
}

func TestSellerOrder_ShowsPaymentAndRefund(t *testing.T) {
	s := newShop(t)
	s.connect("acct_chioma")
	orderId, paymentId := s.paidOrder(1, "ch_6")
	// An expired second checkout must not hide the payment that took money.
	s.db.Exec("INSERT INTO payments (id, public_id, order_id, store_id, checkout_id, checkout_url, status, amount, currency, expires_at, created_at, updated_at) SELECT gen_random_uuid(), 'pay_later', order_id, store_id, 'chk_later', 'https://x', 'expired', amount, currency, NOW(), NOW() + interval '1 minute', NOW() FROM payments WHERE public_id = ?", paymentId)

	got, appErr := order.GetSellerOrder(s.db, s.store.StoreId, s.admin.Id, orderId)
	if appErr != nil || got.Payment == nil || got.Payment.PaymentId != paymentId || got.Payment.Status != "paid" || got.Payment.PaidAt == nil {
		t.Fatalf("seller order payment = %+v %v", got.Payment, appErr)
	}

	s.refund((&fakeRefunds{mode: "processing"}).client(t), paymentId)
	list, appErr := order.ListSellerOrders(s.db, s.store.StoreId, s.admin.Id, model.OrderQuery{})
	if appErr != nil || len(list.Items) != 1 || list.Items[0].Payment == nil || list.Items[0].Payment.RefundStatus != "processing" {
		t.Fatalf("listed payment = %+v %v", list.Items, appErr)
	}

	shopper, _ := order.GetStorefrontOrder(s.db, s.store.ID, orderId)
	if shopper.Payment != nil {
		t.Errorf("shopper sees the payment: %+v", shopper.Payment)
	}
}

func TestRefund_PartialAmount(t *testing.T) {
	s := newShop(t)
	s.connect("acct_chioma")
	fake := &fakeRefunds{mode: "success"}
	client := fake.client(t)

	// What the seller received after Bachs' fee; the order is cancelled.
	orderId, paymentId := s.paidOrder(1, "ch_p1")
	amount := decimal.RequireFromString("14700")
	r, appErr := Refund(s.db, client, s.store.ID, paymentId, model.RefundRequest{Amount: &amount})
	if appErr != nil || r.Status != "paid" || !r.Amount.Equal(amount) || !r.CancelOrder {
		t.Fatalf("partial refund: %+v %v", r, appErr)
	}
	if got := fake.requests[0].body.Amount; got != "14700.00" {
		t.Errorf("Bachs amount = %q, want 14700.00", got)
	}
	if got := s.orderStatus(orderId); got != "cancelled" {
		t.Errorf("order = %s, want cancelled", got)
	}

	// One missing item: refund part, keep the order.
	keptId, keptPayment := s.paidOrder(2, "ch_p2")
	part := decimal.RequireFromString("15000")
	r, appErr = Refund(s.db, client, s.store.ID, keptPayment, model.RefundRequest{Amount: &part, KeepOrder: true})
	if appErr != nil || r.Status != "paid" || r.CancelOrder {
		t.Fatalf("keep-order refund: %+v %v", r, appErr)
	}
	if got := s.orderStatus(keptId); got != "paid" {
		t.Errorf("kept order = %s, want paid", got)
	}
	if got := s.payment(keptPayment).Status; got != "refunded" {
		t.Errorf("payment = %s, want refunded", got)
	}
}

func TestRefund_BadAmounts(t *testing.T) {
	s := newShop(t)
	s.connect("acct_chioma")
	client := (&fakeRefunds{mode: "success"}).client(t)
	_, paymentId := s.paidOrder(1, "ch_b1")
	for _, a := range []string{"0", "-5", "15000.01", "100.005"} {
		amount := decimal.RequireFromString(a)
		if _, appErr := Refund(s.db, client, s.store.ID, paymentId, model.RefundRequest{Amount: &amount}); appErr == nil || appErr.StatusCode != http.StatusBadRequest {
			t.Errorf("amount %s: %+v, want 400", a, appErr)
		}
	}
	var n int64
	s.db.Model(&model.Refunds{}).Count(&n)
	if n != 0 {
		t.Errorf("%d refunds saved for bad amounts", n)
	}
}
