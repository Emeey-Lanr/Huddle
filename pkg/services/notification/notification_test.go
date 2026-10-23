package notification

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"huddle/pkg/mail"
	"huddle/pkg/model"
	"huddle/pkg/testutil"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// fakeSender records emails, or fails with err when set.
type fakeSender struct {
	mu   sync.Mutex
	sent []mail.Message
	err  error
}

func (f *fakeSender) Send(_ context.Context, m mail.Message) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.sent = append(f.sent, m)
	return nil
}

// paidOrder seeds a seller, store and paid delivery order of 2 bags.
func paidOrder(t *testing.T, db *gorm.DB, paidAt time.Time) (model.Admins, model.Orders) {
	t.Helper()
	admin := testutil.SeedAdmin(t, db)
	store := testutil.SeedStore(t, db, admin.Id)
	bag := testutil.SeedProduct(t, db, store.ID, "Ankara Bag", 15000, 5)
	o := model.Orders{
		Id: uuid.New(), PublicId: "ord_" + uuid.NewString()[:8], StoreId: store.ID, Status: model.OrderPaid,
		Currency: "NGN", Total: decimal.NewFromInt(30000), CustomerName: "Mama Nkechi", CustomerPhone: "08031234567",
		DeliveryMethod: model.DeliveryMethodDelivery, Address: "12 Allen Avenue", City: "Ikeja", State: "Lagos",
		Note: "Call before you come", ExpiresAt: paidAt.Add(30 * time.Minute), PaidAt: &paidAt, CreatedAt: paidAt, UpdatedAt: paidAt,
	}
	if err := db.Create(&o).Error; err != nil {
		t.Fatalf("seed order: %v", err)
	}
	item := model.OrderItems{Id: uuid.New(), OrderId: o.Id, ProductId: &bag.Id, ProductPublicId: bag.PublicId, ProductName: bag.Name,
		UnitPrice: decimal.NewFromInt(15000), Quantity: 2, LineTotal: decimal.NewFromInt(30000)}
	if err := db.Create(&item).Error; err != nil {
		t.Fatalf("seed item: %v", err)
	}
	return admin, o
}

func notificationFor(t *testing.T, db *gorm.DB, orderId uuid.UUID) model.Notifications {
	t.Helper()
	var n model.Notifications
	if err := db.Where("order_id = ?", orderId).First(&n).Error; err != nil {
		t.Fatalf("notification: %v", err)
	}
	return n
}

func TestSendDue_EmailsSellerOnce(t *testing.T) {
	db := testutil.NewDB(t)
	now := time.Date(2026, 10, 21, 14, 4, 0, 0, time.UTC)
	admin, o := paidOrder(t, db, now)

	// Queuing twice (e.g. two payment events) still gives one email.
	for range 2 {
		if err := QueueOrderPaid(db, o.Id, now); err != nil {
			t.Fatalf("queue: %v", err)
		}
	}

	f := &fakeSender{}
	n, err := SendDue(context.Background(), db, f, now)
	if err != nil || n != 1 || len(f.sent) != 1 {
		t.Fatalf("SendDue = %d, %v; %d emails, want 1", n, err, len(f.sent))
	}
	m := f.sent[0]
	if m.To != admin.Email {
		t.Errorf("sent to %q, want the seller %q", m.To, admin.Email)
	}
	if !strings.Contains(m.Subject, o.PublicId) || !strings.Contains(m.Subject, "₦30,000.00") {
		t.Errorf("subject = %q", m.Subject)
	}
	for _, want := range []string{
		"Order: " + o.PublicId, "Paid: 21 Oct 2026, 3:04 PM (WAT)", "Total: ₦30,000.00",
		"- 2 x Ankara Bag (₦15,000.00 each) = ₦30,000.00", "Customer: Mama Nkechi", "Phone: 08031234567",
		"Delivery to: 12 Allen Avenue, Ikeja, Lagos", "Note from the customer: Call before you come",
	} {
		if !strings.Contains(m.Body, want) {
			t.Errorf("body is missing %q:\n%s", want, m.Body)
		}
	}

	saved := notificationFor(t, db, o.Id)
	if saved.Status != model.NotificationSent || saved.Attempts != 1 || saved.SentAt == nil {
		t.Errorf("saved = %+v", saved)
	}
	if n, _ := SendDue(context.Background(), db, f, now.Add(time.Hour)); n != 0 || len(f.sent) != 1 {
		t.Errorf("sent again: %d", n)
	}
}

func TestSendDue_RetriesThenGivesUp(t *testing.T) {
	db := testutil.NewDB(t)
	now := time.Now().UTC().Truncate(time.Second)
	_, o := paidOrder(t, db, now)
	QueueOrderPaid(db, o.Id, now)

	f := &fakeSender{err: errors.New("421 try again later")}
	if n, err := SendDue(context.Background(), db, f, now); n != 0 || err != nil {
		t.Fatalf("SendDue = %d, %v", n, err)
	}
	saved := notificationFor(t, db, o.Id)
	if saved.Status != model.NotificationPending || saved.Attempts != 1 || saved.LastError != "421 try again later" {
		t.Errorf("after a failure: %+v", saved)
	}
	if !saved.NextAttemptAt.Equal(now.Add(time.Minute)) {
		t.Errorf("next attempt %v, want a minute after %v", saved.NextAttemptAt, now)
	}
	// Not due again until the wait has passed.
	if SendDue(context.Background(), db, f, now.Add(30*time.Second)); notificationFor(t, db, o.Id).Attempts != 1 {
		t.Error("retried before its wait was over")
	}

	at := now
	for i := 2; i <= maxAttempts; i++ {
		at = at.Add(7 * time.Hour)
		SendDue(context.Background(), db, f, at)
	}
	saved = notificationFor(t, db, o.Id)
	if saved.Status != model.NotificationFailed || saved.Attempts != maxAttempts {
		t.Errorf("after %d failures: %+v", maxAttempts, saved)
	}

	// A failed email is not tried again.
	f.err = nil
	if n, _ := SendDue(context.Background(), db, f, at.Add(24*time.Hour)); n != 0 {
		t.Errorf("failed email sent later")
	}
}

func TestSendDue_OnlyDueAndPending(t *testing.T) {
	db := testutil.NewDB(t)
	now := time.Now().UTC()
	_, o := paidOrder(t, db, now)
	QueueOrderPaid(db, o.Id, now.Add(time.Hour)) // queued for later

	f := &fakeSender{}
	if n, _ := SendDue(context.Background(), db, f, now); n != 0 {
		t.Errorf("sent %d emails that are not due", n)
	}
	if n, _ := SendDue(context.Background(), db, f, now.Add(time.Hour)); n != 1 {
		t.Errorf("sent %d due emails, want 1", n)
	}
}

func TestBackoff(t *testing.T) {
	want := []time.Duration{time.Minute, 4 * time.Minute, 16 * time.Minute, 64 * time.Minute, 256 * time.Minute, 6 * time.Hour, 6 * time.Hour}
	for i, w := range want {
		if got := backoff(i + 1); got != w {
			t.Errorf("backoff(%d) = %v, want %v", i+1, got, w)
		}
	}
}

func TestMoney(t *testing.T) {
	cases := map[string]string{
		"0": "₦0.00", "5.5": "₦5.50", "999": "₦999.00", "1000": "₦1,000.00",
		"15000": "₦15,000.00", "1234567.891": "₦1,234,567.89", "-2500": "-₦2,500.00",
	}
	for in, want := range cases {
		if got := money("NGN", decimal.RequireFromString(in)); got != want {
			t.Errorf("money(%s) = %q, want %q", in, got, want)
		}
	}
	if got := money("usd", decimal.NewFromInt(1500)); got != "USD 1,500.00" {
		t.Errorf("money(usd) = %q", got)
	}
}
