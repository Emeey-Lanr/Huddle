// Package payment takes online payments for orders through Bachs.
package payment

import (
	"context"
	"errors"
	"fmt"
	"huddle/pkg/bachs"
	"huddle/pkg/model"
	"huddle/pkg/services/order"
	"huddle/pkg/utility"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// A checkout must close before its order's stock hold ends, with this margin.
const holdMargin = time.Minute

func appErr(code int, message string, err error) *utility.AppError {
	return &utility.AppError{StatusCode: code, Message: message, Err: err}
}

// platformFee is Huddle's cut of a sale, from BACHS_PLATFORM_FEE_PERCENT
// (e.g. "1.5"). It is zero when unset.
func platformFee(total decimal.Decimal) decimal.Decimal {
	pct, err := decimal.NewFromString(os.Getenv("BACHS_PLATFORM_FEE_PERCENT"))
	if err != nil || !pct.IsPositive() {
		return decimal.Zero
	}
	return total.Mul(pct).Div(decimal.NewFromInt(100)).Round(2)
}

// StartPayment opens a Bachs checkout for a pending order and returns where
// to send the shopper. The checkout is charged to the store's Bachs
// sub-account, so the money lands in the seller's balance. An open checkout
// for the order is reused instead of creating another.
func StartPayment(db *gorm.DB, client *bachs.Client, storeId uuid.UUID, publicOrderId string, req model.PayRequest) (*model.PaymentResponse, *utility.AppError) {
	if client == nil {
		return nil, appErr(http.StatusServiceUnavailable, "Online payment is not set up yet", bachs.ErrNotConfigured)
	}

	var o model.Orders
	if err := db.Where("public_id = ? AND store_id = ?", publicOrderId, storeId).First(&o).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, appErr(http.StatusNotFound, "Order not found", err)
		}
		return nil, appErr(http.StatusInternalServerError, "Internal Server Error", err)
	}
	now := time.Now().UTC()
	switch {
	case o.Status == model.OrderPaid || o.Status == model.OrderFulfilled:
		return nil, appErr(http.StatusConflict, "This order is already paid", errors.New("order paid"))
	case o.Status != model.OrderPending:
		return nil, appErr(http.StatusConflict, "This order is "+o.Status+". Please place it again", errors.New("order not pending"))
	}
	minutes := int(o.ExpiresAt.Sub(now.Add(holdMargin)) / time.Minute)
	if minutes < 1 {
		return nil, appErr(http.StatusConflict, "This order is about to expire. Please place it again", errors.New("hold ending"))
	}

	var open model.Payments
	err := db.Where("order_id = ? AND status = ? AND expires_at > ?", o.Id, model.PaymentOpen, now.Add(holdMargin)).
		Order("created_at DESC").First(&open).Error
	if err == nil {
		return toResponse(open), nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, appErr(http.StatusInternalServerError, "Internal Server Error", err)
	}

	var store model.Stores
	if err := db.Where("id = ?", storeId).First(&store).Error; err != nil {
		return nil, appErr(http.StatusInternalServerError, "Internal Server Error", err)
	}
	accountId := ""
	if store.BachsAccountId != nil {
		accountId = *store.BachsAccountId
	}
	// Outside production a store without a sub-account is charged to the
	// platform account, so payments can be tried in the sandbox.
	if accountId == "" && os.Getenv("APP_ENV") == "production" {
		return nil, appErr(http.StatusConflict, "This store can't take online payments yet", errors.New("store has no Bachs account"))
	}

	publicId := "pay_" + strings.ReplaceAll(utility.GenerateUUID().String(), "-", "")
	checkoutReq := bachs.CheckoutRequest{
		Pricing:          bachs.Pricing{Currency: o.Currency, Amount: o.Total.StringFixed(2), PriceType: "fixed"},
		SuccessURL:       req.SuccessURL,
		CancelURL:        req.CancelURL,
		Reference:        publicId,
		ExpiresInMinutes: min(minutes, 1440),
		Metadata:         map[string]string{"order_id": o.PublicId, "payment_id": publicId},
	}
	if o.CustomerEmail != "" {
		checkoutReq.Customer = &bachs.Customer{Email: o.CustomerEmail, Name: o.CustomerName, PhoneNumber: o.CustomerPhone}
	}
	if fee := platformFee(o.Total); fee.IsPositive() {
		checkoutReq.PlatformFee = fee.StringFixed(2)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	checkout, err := client.CreateCheckout(ctx, accountId, publicId, checkoutReq)
	if err != nil {
		var apiErr *bachs.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode < 500 && apiErr.Detail != "" {
			return nil, appErr(http.StatusBadGateway, "The payment provider refused this payment: "+apiErr.Detail, err)
		}
		return nil, appErr(http.StatusBadGateway, "Could not reach the payment provider. Please try again", err)
	}

	expiresAt := checkout.ExpiresAt
	if expiresAt.IsZero() {
		expiresAt = now.Add(time.Duration(checkoutReq.ExpiresInMinutes) * time.Minute)
	}
	payment := model.Payments{
		Id: utility.GenerateUUID(), PublicId: publicId, OrderId: o.Id, StoreId: storeId, Provider: "bachs",
		CheckoutId: checkout.CheckoutId, CheckoutURL: checkout.CheckoutURL, Status: model.PaymentOpen,
		Amount: o.Total, Currency: o.Currency, ExpiresAt: expiresAt, CreatedAt: now, UpdatedAt: now,
	}
	if accountId != "" {
		payment.AccountId = &accountId
	}
	if err := db.Create(&payment).Error; err != nil {
		return nil, appErr(http.StatusInternalServerError, "Internal Server Error", err)
	}
	return toResponse(payment), nil
}

func toResponse(p model.Payments) *model.PaymentResponse {
	return &model.PaymentResponse{PaymentId: p.PublicId, CheckoutURL: p.CheckoutURL, Amount: p.Amount, Currency: p.Currency, ExpiresAt: p.ExpiresAt}
}

// HandleEvent applies a verified Bachs webhook event. Each event id is
// handled once; a returned error rolls everything back so Bachs retries.
func HandleEvent(db *gorm.DB, e *bachs.Event, now time.Time) error {
	return db.Transaction(func(tx *gorm.DB) error {
		res := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.WebhookEvents{
			EventId: e.Id, Provider: "bachs", Type: e.Type, ReceivedAt: now,
		})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return nil // already handled
		}

		switch e.Type {
		case "checkout.completed":
			if e.Data.PaymentStatus != "paid" {
				return nil
			}
			return paid(tx, e, now)
		case "collection.succeeded":
			return paid(tx, e, now)
		case "checkout.expired":
			return tx.Model(&model.Payments{}).Where("checkout_id = ? AND status = ?", e.Data.CheckoutId, model.PaymentOpen).
				Updates(map[string]any{"status": model.PaymentExpired, "updated_at": now}).Error
		}
		return nil
	})
}

// paid records a successful payment. Bachs sends both checkout.completed and
// collection.succeeded for one payment, so the second is a no-op.
func paid(tx *gorm.DB, e *bachs.Event, now time.Time) error {
	if e.Data.CheckoutId == "" {
		return nil // not from a checkout, e.g. a virtual account deposit
	}
	var p model.Payments
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("checkout_id = ?", e.Data.CheckoutId).First(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		log.Printf("bachs webhook %s: unknown checkout %s, ignored", e.Id, e.Data.CheckoutId)
		return nil
	}
	if err != nil {
		return err
	}
	if p.Status != model.PaymentOpen && p.Status != model.PaymentExpired {
		return nil
	}

	setStatus := func(status string) error {
		updates := map[string]any{"status": status, "updated_at": now}
		if status == model.PaymentPaid {
			updates["paid_at"] = now
		}
		return tx.Model(&model.Payments{}).Where("id = ?", p.Id).Updates(updates).Error
	}

	if reason := mismatch(p, e); reason != "" {
		log.Printf("bachs webhook %s: payment %s needs review: %s", e.Id, p.PublicId, reason)
		return setStatus(model.PaymentReview)
	}

	outcome, err := order.ApplyPayment(tx, p.OrderId, now)
	if err != nil {
		return err
	}
	if outcome != order.PaymentApplied {
		log.Printf("bachs webhook %s: payment %s needs a refund (order could not take it)", e.Id, p.PublicId)
		return setStatus(model.PaymentNeedsRefund)
	}
	return setStatus(model.PaymentPaid)
}

// mismatch says why a payment event does not match what was asked for, or "".
func mismatch(p model.Payments, e *bachs.Event) string {
	amount, err := decimal.NewFromString(e.Data.Amount)
	if err != nil || !amount.Equal(p.Amount) {
		return fmt.Sprintf("amount %q, expected %s", e.Data.Amount, p.Amount.StringFixed(2))
	}
	if !strings.EqualFold(e.Data.Currency, p.Currency) {
		return fmt.Sprintf("currency %q, expected %s", e.Data.Currency, p.Currency)
	}
	// Events from a seller's sub-account carry its id in "account".
	want := ""
	if p.AccountId != nil {
		want = *p.AccountId
	}
	if e.Account != want {
		return fmt.Sprintf("account %q, expected %q", e.Account, want)
	}
	return ""
}
