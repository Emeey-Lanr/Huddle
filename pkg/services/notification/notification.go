// Package notification emails sellers about their orders. An email is
// queued in the notifications table inside the transaction that changes the
// order, and a worker sends queued emails, retrying failures.
package notification

import (
	"context"
	"fmt"
	"huddle/pkg/mail"
	"huddle/pkg/model"
	"huddle/pkg/utility"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	batchSize   = 20
	maxAttempts = 8
	// A claimed email is tried again after this if the worker stops mid-send.
	lease = 10 * time.Minute
)

// Nigeria has no daylight saving, so a fixed zone avoids needing tzdata.
var wat = time.FixedZone("WAT", 60*60)

// QueueOrderPaid queues the "new paid order" email to the order's seller.
// Call it inside the transaction that marks the order paid. An order gets
// this email at most once.
func QueueOrderPaid(tx *gorm.DB, orderId uuid.UUID, now time.Time) error {
	return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.Notifications{
		Id: utility.GenerateUUID(), Kind: model.NotificationOrderPaid, OrderId: orderId,
		Status: model.NotificationPending, NextAttemptAt: now, CreatedAt: now, UpdatedAt: now,
	}).Error
}

// SendDue sends queued emails that are due and returns how many were sent.
// Rows are claimed with SKIP LOCKED, so two workers never send the same one.
func SendDue(ctx context.Context, db *gorm.DB, sender mail.Sender, now time.Time) (int, error) {
	var due []model.Notifications
	err := db.Raw(`UPDATE notifications SET next_attempt_at = ?, updated_at = ?
		WHERE id IN (SELECT id FROM notifications WHERE status = ? AND next_attempt_at <= ?
		             ORDER BY next_attempt_at LIMIT ? FOR UPDATE SKIP LOCKED)
		RETURNING *`, now.Add(lease), now, model.NotificationPending, now, batchSize).Scan(&due).Error
	if err != nil {
		return 0, err
	}

	sent := 0
	for _, n := range due {
		if ctx.Err() != nil {
			break // the rest are tried again once their lease ends
		}
		msg, sendErr := message(db, n)
		if sendErr == nil {
			sendErr = sender.Send(ctx, msg)
		}
		if err := record(db, n, sendErr, now); err != nil {
			return sent, err
		}
		if sendErr == nil {
			sent++
		}
	}
	return sent, nil
}

// record saves the outcome of one send attempt.
func record(db *gorm.DB, n model.Notifications, sendErr error, now time.Time) error {
	attempts := n.Attempts + 1
	updates := map[string]any{"attempts": attempts, "updated_at": now}
	switch {
	case sendErr == nil:
		updates["status"] = model.NotificationSent
		updates["sent_at"] = now
		updates["last_error"] = ""
	case attempts >= maxAttempts:
		log.Printf("notification %s (%s): giving up after %d attempts: %v", n.Id, n.Kind, attempts, sendErr)
		updates["status"] = model.NotificationFailed
		updates["last_error"] = truncate(sendErr.Error(), 500)
	default:
		log.Printf("notification %s (%s): attempt %d failed: %v", n.Id, n.Kind, attempts, sendErr)
		updates["next_attempt_at"] = now.Add(backoff(attempts))
		updates["last_error"] = truncate(sendErr.Error(), 500)
	}
	return db.Model(&model.Notifications{}).Where("id = ?", n.Id).Updates(updates).Error
}

// backoff is the wait after a failed attempt: 1m, 4m, 16m, ... up to 6h.
func backoff(attempts int) time.Duration {
	d := time.Minute
	for i := 1; i < attempts && d < 6*time.Hour; i++ {
		d *= 4
	}
	return min(d, 6*time.Hour)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func message(db *gorm.DB, n model.Notifications) (mail.Message, error) {
	switch n.Kind {
	case model.NotificationOrderPaid:
		return orderPaidMessage(db, n.OrderId)
	}
	return mail.Message{}, fmt.Errorf("unknown notification kind %q", n.Kind)
}

// orderPaidMessage tells the seller what was bought and who to deliver it to.
// It is built when sent, so it goes to the seller's current email.
func orderPaidMessage(db *gorm.DB, orderId uuid.UUID) (mail.Message, error) {
	var o model.Orders
	if err := db.Where("id = ?", orderId).First(&o).Error; err != nil {
		return mail.Message{}, err
	}
	var items []model.OrderItems
	if err := db.Where("order_id = ?", o.Id).Order("product_name").Find(&items).Error; err != nil {
		return mail.Message{}, err
	}
	var store model.Stores
	if err := db.Where("id = ?", o.StoreId).First(&store).Error; err != nil {
		return mail.Message{}, err
	}
	var admin model.Admins
	if err := db.Where("id = ?", store.AdminId).First(&admin).Error; err != nil {
		return mail.Message{}, err
	}

	var b strings.Builder
	fmt.Fprintf(&b, "You have a new paid order on %s.\n\n", store.StoreName)
	fmt.Fprintf(&b, "Order: %s\n", o.PublicId)
	if o.PaidAt != nil {
		fmt.Fprintf(&b, "Paid: %s\n", o.PaidAt.In(wat).Format("2 Jan 2006, 3:04 PM (WAT)"))
	}
	fmt.Fprintf(&b, "Total: %s\n\nItems:\n", money(o.Currency, o.Total))
	for _, it := range items {
		fmt.Fprintf(&b, "- %d x %s (%s each) = %s\n", it.Quantity, it.ProductName, money(o.Currency, it.UnitPrice), money(o.Currency, it.LineTotal))
	}
	fmt.Fprintf(&b, "\nCustomer: %s\nPhone: %s\n", o.CustomerName, o.CustomerPhone)
	if o.CustomerEmail != "" {
		fmt.Fprintf(&b, "Email: %s\n", o.CustomerEmail)
	}
	if o.DeliveryMethod == model.DeliveryMethodDelivery {
		fmt.Fprintf(&b, "Delivery to: %s, %s, %s\n", o.Address, o.City, o.State)
	} else {
		b.WriteString("Pickup: the customer will collect it\n")
	}
	if o.Note != "" {
		fmt.Fprintf(&b, "Note from the customer: %s\n", o.Note)
	}
	b.WriteString("\nOnce the customer has the order, mark it fulfilled in Huddle.\n")

	return mail.Message{
		To:      admin.Email,
		Subject: fmt.Sprintf("New paid order %s: %s (%s)", o.PublicId, money(o.Currency, o.Total), store.StoreName),
		Body:    b.String(),
	}, nil
}

// money formats an amount like ₦15,000.00 (other currencies as "USD 15.00").
func money(currency string, amount decimal.Decimal) string {
	s := amount.Abs().StringFixed(2)
	whole, frac := s[:len(s)-3], s[len(s)-3:]
	var g strings.Builder
	for i, r := range whole {
		if i > 0 && (len(whole)-i)%3 == 0 {
			g.WriteByte(',')
		}
		g.WriteRune(r)
	}
	sign := ""
	if amount.IsNegative() {
		sign = "-"
	}
	if strings.EqualFold(currency, "NGN") {
		return sign + "₦" + g.String() + frac
	}
	return sign + strings.ToUpper(currency) + " " + g.String() + frac
}

// StartWorker runs SendDue every interval until ctx is done.
func StartWorker(ctx context.Context, db *gorm.DB, sender mail.Sender, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				n, err := SendDue(ctx, db, sender, time.Now().UTC())
				if err != nil {
					log.Printf("notifications: %v", err)
				} else if n > 0 {
					log.Printf("notifications: sent %d emails", n)
				}
			}
		}
	}()
}
