package payment

import (
	"context"
	"errors"
	"huddle/pkg/bachs"
	"huddle/pkg/model"
	"huddle/pkg/services/order"
	"huddle/pkg/utility"
	"log"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// refundable lists the payment statuses a seller can refund.
var refundable = []string{model.PaymentPaid, model.PaymentNeedsRefund, model.PaymentReview}

// Issues lists the store's payments the seller must act on, newest first.
func Issues(db *gorm.DB, storeId uuid.UUID) ([]model.PaymentIssue, *utility.AppError) {
	issues := []model.PaymentIssue{}
	err := db.Raw(`SELECT p.public_id AS payment_id, p.status, p.amount, p.currency, p.paid_at, p.created_at,
			o.public_id AS order_id,
			COALESCE((SELECT r.status FROM refunds r WHERE r.payment_id = p.id ORDER BY r.created_at DESC LIMIT 1), '') AS refund_status
		FROM payments p JOIN orders o ON o.id = p.order_id
		WHERE p.store_id = ? AND p.status IN ?
		ORDER BY p.created_at DESC, p.id`,
		storeId, []string{model.PaymentNeedsRefund, model.PaymentReview}).Scan(&issues).Error
	if err != nil {
		return nil, appErr(http.StatusInternalServerError, "Internal Server Error", err)
	}
	return issues, nil
}

// Refund returns a payment's money to the shopper from the seller's Bachs
// balance: all of it, or req.Amount. The refund is saved before Bachs is
// called, and its id is the reference and idempotency key, so asking again
// while it is unconfirmed re-sends the same refund instead of making a
// second one.
func Refund(db *gorm.DB, client *bachs.Client, storeId uuid.UUID, publicPaymentId string, req model.RefundRequest) (*model.RefundResponse, *utility.AppError) {
	if client == nil {
		return nil, appErr(http.StatusServiceUnavailable, "Online payment is not set up yet", bachs.ErrNotConfigured)
	}

	var p model.Payments
	var r model.Refunds
	txErr := db.Transaction(func(tx *gorm.DB) error {
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("public_id = ? AND store_id = ?", publicPaymentId, storeId).First(&p).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return appErr(http.StatusNotFound, "Payment not found", err)
		}
		if err != nil {
			return err
		}

		err = tx.Where("payment_id = ? AND status IN ?", p.Id, []string{model.RefundProcessing, model.RefundPaid}).First(&r).Error
		switch {
		case err == nil && r.Status == model.RefundPaid:
			return appErr(http.StatusConflict, "This payment is already refunded", errors.New("refunded"))
		case err == nil && r.BachsRefundId != nil:
			return appErr(http.StatusConflict, "A refund for this payment is already in progress", errors.New("refund processing"))
		case err == nil:
			return nil // not confirmed by Bachs yet: send it again
		case !errors.Is(err, gorm.ErrRecordNotFound):
			return err
		}

		if !slices.Contains(refundable, p.Status) {
			return appErr(http.StatusConflict, "A payment that is "+p.Status+" cannot be refunded", errors.New("not refundable"))
		}
		if p.ChargeId == nil {
			return appErr(http.StatusConflict, "Bachs has not confirmed this payment's charge yet. Please try again shortly", errors.New("no charge id"))
		}
		amount := p.Amount
		if req.Amount != nil {
			amount = *req.Amount
			switch {
			case !amount.IsPositive():
				return appErr(http.StatusBadRequest, "The refund amount must be more than 0", errors.New("bad amount"))
			case amount.GreaterThan(p.Amount):
				return appErr(http.StatusBadRequest, "The refund amount can't be more than the payment ("+p.Currency+" "+p.Amount.StringFixed(2)+")", errors.New("bad amount"))
			case !amount.Equal(amount.Round(2)):
				return appErr(http.StatusBadRequest, "The refund amount can have at most 2 decimal places", errors.New("bad amount"))
			}
		}
		now := time.Now().UTC()
		r = model.Refunds{
			Id: utility.GenerateUUID(), PublicId: "rfd_" + strings.ReplaceAll(utility.GenerateUUID().String(), "-", ""),
			PaymentId: p.Id, StoreId: storeId, Amount: amount, Currency: p.Currency,
			Reason: req.Reason, CancelOrder: !req.KeepOrder, Status: model.RefundProcessing, CreatedAt: now, UpdatedAt: now,
		}
		return tx.Create(&r).Error
	})
	if txErr != nil {
		var ae *utility.AppError
		if errors.As(txErr, &ae) {
			return nil, ae
		}
		return nil, appErr(http.StatusInternalServerError, "Internal Server Error", txErr)
	}

	accountId := ""
	if p.AccountId != nil {
		accountId = *p.AccountId
	}
	reason := r.Reason
	if reason == "" {
		reason = "Refunded by the seller"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	refundReq := bachs.RefundRequest{ChargeId: *p.ChargeId, Reference: r.PublicId, Reason: reason}
	if r.Amount.LessThan(p.Amount) {
		refundReq.Amount = r.Amount.StringFixed(2)
	}
	refund, err := client.CreateRefund(ctx, accountId, refundReq)
	if err != nil {
		var apiErr *bachs.APIError
		switch {
		case errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusConflict:
			// Bachs already has a refund for this charge; its webhook settles ours.
			return nil, appErr(http.StatusConflict, "A refund for this payment is already in progress", err)
		case errors.As(err, &apiErr) && apiErr.StatusCode < 500:
			// Refused, so nothing moved and the seller can try again later.
			if fErr := settleRefund(db, r.PublicId, "", model.RefundFailed, apiErr.Detail, time.Now().UTC()); fErr != nil {
				return nil, appErr(http.StatusInternalServerError, "Internal Server Error", fErr)
			}
			if strings.Contains(strings.ToLower(apiErr.Detail), "insufficient balance") {
				return nil, appErr(http.StatusConflict, lowBalanceMessage(client, accountId, r), err)
			}
			msg := "The payment provider refused this refund"
			if apiErr.Detail != "" {
				msg += ": " + apiErr.Detail
			}
			return nil, appErr(http.StatusBadGateway, msg, err)
		}
		// Unknown outcome: the refund stays processing; asking again re-sends it.
		return nil, appErr(http.StatusBadGateway, "Could not confirm the refund with the payment provider. Please try again", err)
	}

	status := model.RefundProcessing
	switch {
	case bachs.RefundSucceeded(refund.Status):
		status = model.RefundPaid
	case refund.Status == "failed":
		status = model.RefundFailed
	}
	if err := settleRefund(db, r.PublicId, refund.RefundId, status, "", time.Now().UTC()); err != nil {
		return nil, appErr(http.StatusInternalServerError, "Internal Server Error", err)
	}
	if err := db.Where("id = ?", r.Id).First(&r).Error; err != nil {
		return nil, appErr(http.StatusInternalServerError, "Internal Server Error", err)
	}
	return toRefundResponse(r, p.PublicId), nil
}

// lowBalanceMessage tells the seller their Bachs balance can't cover the
// refund, and how much it can. Bachs keeps its fee on a sale, so a seller
// whose balance is only that sale can't refund all of it.
func lowBalanceMessage(client *bachs.Client, accountId string, r model.Refunds) string {
	want := r.Currency + " " + r.Amount.StringFixed(2)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	balances, err := client.Balances(ctx, accountId)
	if err == nil {
		for _, b := range balances {
			available, err := decimal.NewFromString(b.AvailableBalance)
			if err == nil && strings.EqualFold(b.Currency, r.Currency) {
				have := r.Currency + " " + available.StringFixed(2)
				return "Your Bachs balance (" + have + ") is less than this refund (" + want + "). Refund up to " + have + ", or try again after more sales"
			}
		}
	} else {
		log.Printf("bachs: read balance of %q: %v", accountId, err)
	}
	return "Your Bachs balance is less than this refund (" + want + "). Refund a smaller amount, or try again after more sales"
}

func settleRefund(db *gorm.DB, reference, bachsRefundId, status, failureReason string, now time.Time) error {
	return db.Transaction(func(tx *gorm.DB) error {
		return applyRefund(tx, reference, bachsRefundId, status, failureReason, now)
	})
}

// applyRefund records what Bachs says about a refund, found by its
// reference (or Bachs id). Only a processing refund changes, so a late or
// repeated report can't undo a final one. A paid refund marks the payment
// refunded, and cancels the order if this payment is what paid it, unless
// the seller chose to keep the order.
func applyRefund(tx *gorm.DB, reference, bachsRefundId, status, failureReason string, now time.Time) error {
	var r model.Refunds
	q := tx.Clauses(clause.Locking{Strength: "UPDATE"})
	var err error
	if reference != "" {
		err = q.Where("public_id = ?", reference).First(&r).Error
	} else {
		err = q.Where("bachs_refund_id = ?", bachsRefundId).First(&r).Error
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		log.Printf("bachs refund %q (%s): not a Huddle refund, ignored", reference, bachsRefundId)
		return nil
	}
	if err != nil {
		return err
	}

	updates := map[string]any{"updated_at": now}
	if bachsRefundId != "" && r.BachsRefundId == nil {
		updates["bachs_refund_id"] = bachsRefundId
	}
	final := r.Status == model.RefundProcessing && status != model.RefundProcessing
	if final {
		updates["status"] = status
		updates["completed_at"] = now
		if failureReason != "" {
			updates["failure_reason"] = truncate(failureReason, 500)
		}
	}
	if err := tx.Model(&model.Refunds{}).Where("id = ?", r.Id).Updates(updates).Error; err != nil {
		return err
	}
	if !final || status != model.RefundPaid {
		return nil
	}

	var p model.Payments
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", r.PaymentId).First(&p).Error; err != nil {
		return err
	}
	if err := tx.Model(&model.Payments{}).Where("id = ?", p.Id).
		Updates(map[string]any{"status": model.PaymentRefunded, "updated_at": now}).Error; err != nil {
		return err
	}
	if p.Status == model.PaymentPaid && r.CancelOrder {
		return order.CancelRefunded(tx, p.OrderId, now)
	}
	return nil
}

// refundEvent applies refund.created, refund.paid and refund.failed.
func refundEvent(tx *gorm.DB, e *bachs.Event, now time.Time) error {
	status := model.RefundProcessing
	switch e.Type {
	case "refund.paid":
		status = model.RefundPaid
	case "refund.failed":
		status = model.RefundFailed
	}
	if e.Data.Reference == "" && e.Data.RefundId == "" {
		return nil
	}
	return applyRefund(tx, e.Data.Reference, e.Data.RefundId, status, "Bachs could not complete the refund", now)
}

func toRefundResponse(r model.Refunds, paymentId string) *model.RefundResponse {
	return &model.RefundResponse{
		RefundId: r.PublicId, PaymentId: paymentId, Status: r.Status, Amount: r.Amount, Currency: r.Currency,
		Reason: r.Reason, CancelOrder: r.CancelOrder, FailureReason: r.FailureReason, CreatedAt: r.CreatedAt, CompletedAt: r.CompletedAt,
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
