package payment

import (
	"context"
	"errors"
	"huddle/pkg/bachs"
	"huddle/pkg/model"
	"huddle/pkg/utility"
	"log"
	"net/http"
	"slices"
	"time"

	"gorm.io/gorm"
)

// Every seller account asks for NGN cards and bank transfers, to sell, and
// payouts, to withdraw. In the sandbox they are active at once; live, Bachs
// reviews them after the seller finishes onboarding.
var sellerConfiguration = map[string]bachs.Persona{
	"merchant": {Capabilities: map[string]bachs.CapabilityRequest{
		"ngn_card_collection": {Requested: true},
		"bank_transfer":       {Requested: true},
	}},
	"recipient": {Capabilities: map[string]bachs.CapabilityRequest{
		"payouts": {Requested: true},
	}},
}

// collectCapabilities let shoppers pay; a store is ready when one is active.
var collectCapabilities = []string{"ngn_card_collection", "bank_transfer"}

// Onboard returns a link to Bachs' hosted onboarding for the store, creating
// the store's Bachs account first if it has none. The link is single use and
// replaces any earlier one.
func Onboard(db *gorm.DB, client *bachs.Client, store *model.Stores, email string, req model.OnboardingRequest) (*model.OnboardingLink, *utility.AppError) {
	if client == nil {
		return nil, appErr(http.StatusServiceUnavailable, "Online payment is not set up yet", bachs.ErrNotConfigured)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()

	accountId, appError := ensureAccount(ctx, db, client, store, email)
	if appError != nil {
		return nil, appError
	}

	ready, err := Ready(db, accountId)
	if err != nil {
		return nil, appErr(http.StatusInternalServerError, "Internal Server Error", err)
	}
	linkType := bachs.LinkOnboarding
	if ready {
		linkType = bachs.LinkUpdate
	}
	link, err := client.CreateAccountLink(ctx, accountId, utility.GenerateUUID().String(), bachs.AccountLinkRequest{
		Type: linkType, ReturnURL: req.ReturnURL, RefreshURL: req.RefreshURL,
	})
	if err != nil {
		var apiErr *bachs.APIError
		if errors.As(err, &apiErr) {
			switch apiErr.ErrorCode {
			case "CONNECTED_ACCOUNT_REQUIREMENTS_NOT_FOUND":
				return nil, appErr(http.StatusConflict, "Your payment account is set up. There is nothing to update", err)
			case "CONNECTED_ACCOUNT_NOT_ELIGIBLE_FOR_LINK":
				return nil, appErr(http.StatusConflict, "Bachs has suspended or rejected this payment account. Please contact Bachs support", err)
			}
		}
		return nil, providerErr(err)
	}
	return &model.OnboardingLink{URL: link.URL, ExpiresAt: link.ExpiresAt}, nil
}

// ensureAccount returns the store's Bachs account id, creating the account
// if needed. The idempotency key is the store's id, so two requests at once
// get the same Bachs account.
func ensureAccount(ctx context.Context, db *gorm.DB, client *bachs.Client, store *model.Stores, email string) (string, *utility.AppError) {
	if store.BachsAccountId != nil && *store.BachsAccountId != "" {
		return *store.BachsAccountId, nil
	}
	account, err := client.CreateAccount(ctx, "huddle-store-"+store.ID.String(), bachs.AccountRequest{
		ContactEmail:  email,
		DisplayName:   store.StoreName,
		Country:       "NG",
		Configuration: sellerConfiguration,
	})
	if err != nil {
		return "", providerErr(err)
	}

	err = db.Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&model.Stores{}).Where("id = ? AND bachs_account_id IS NULL", store.ID).Update("bachs_account_id", account.Id)
		if res.Error != nil {
			return res.Error
		}
		return saveCapabilities(tx, account.Id, account.Capabilities, time.Now().UTC())
	})
	if err != nil {
		return "", appErr(http.StatusInternalServerError, "Internal Server Error", err)
	}

	// Another request may have saved the account first; use what is stored.
	var saved model.Stores
	if err := db.Where("id = ?", store.ID).First(&saved).Error; err != nil {
		return "", appErr(http.StatusInternalServerError, "Internal Server Error", err)
	}
	if saved.BachsAccountId == nil {
		return "", appErr(http.StatusInternalServerError, "Internal Server Error", errors.New("bachs account not saved"))
	}
	store.BachsAccountId = saved.BachsAccountId
	return *saved.BachsAccountId, nil
}

// Setup reports whether the store can take online payments. It reads the
// account from Bachs when it can, and falls back to the last known statuses.
func Setup(db *gorm.DB, client *bachs.Client, store *model.Stores) (*model.PaymentSetup, *utility.AppError) {
	setup := &model.PaymentSetup{Capabilities: []model.CapabilityStatus{}, Outstanding: []string{}}
	if store.BachsAccountId == nil || *store.BachsAccountId == "" {
		return setup, nil
	}
	accountId := *store.BachsAccountId
	setup.Connected = true

	messages := map[string]string{}
	if client != nil {
		if account, err := readAccount(client, accountId); err != nil {
			log.Printf("bachs: read account %s: %v", accountId, err)
		} else {
			if err := saveCapabilities(db, accountId, account.Capabilities, time.Now().UTC()); err != nil {
				return nil, appErr(http.StatusInternalServerError, "Internal Server Error", err)
			}
			for _, c := range account.Capabilities {
				if len(c.StatusDetails) > 0 {
					messages[c.Name] = c.StatusDetails[0].Message
				}
			}
			for _, field := range append(account.Requirements.PastDue, account.Requirements.CurrentlyDue...) {
				if !slices.Contains(setup.Outstanding, field) {
					setup.Outstanding = append(setup.Outstanding, field)
				}
			}
		}
	}

	var caps []model.BachsCapabilities
	if err := db.Where("account_id = ?", accountId).Order("name").Find(&caps).Error; err != nil {
		return nil, appErr(http.StatusInternalServerError, "Internal Server Error", err)
	}
	for _, c := range caps {
		setup.Capabilities = append(setup.Capabilities, model.CapabilityStatus{Name: c.Name, Status: c.Status, Message: messages[c.Name]})
		if c.Status == bachs.CapabilityActive && slices.Contains(collectCapabilities, c.Name) {
			setup.Ready = true
		}
	}
	return setup, nil
}

// readAccount reads the account, and its capabilities separately when the
// account read leaves them out.
func readAccount(client *bachs.Client, accountId string) (*bachs.Account, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	account, err := client.GetAccount(ctx, accountId)
	if err != nil {
		return nil, err
	}
	if len(account.Capabilities) == 0 {
		if account.Capabilities, err = client.ListCapabilities(ctx, accountId); err != nil {
			return nil, err
		}
	}
	return account, nil
}

// Ready says whether shoppers can pay the account online.
func Ready(db *gorm.DB, accountId string) (bool, error) {
	var n int64
	err := db.Model(&model.BachsCapabilities{}).
		Where("account_id = ? AND name IN ? AND status = ?", accountId, collectCapabilities, bachs.CapabilityActive).
		Count(&n).Error
	return n > 0, err
}

// saveCapabilities stores the capabilities the account asked for or has, as
// they were at changedAt. A status already saved from later is kept.
func saveCapabilities(db *gorm.DB, accountId string, caps bachs.Capabilities, changedAt time.Time) error {
	for _, c := range caps {
		if c.Name == "" || (!c.Requested && c.Status != bachs.CapabilityActive) {
			continue
		}
		if err := saveCapability(db, accountId, c.Name, c.Status, changedAt); err != nil {
			return err
		}
	}
	return nil
}

func saveCapability(db *gorm.DB, accountId, name, status string, changedAt time.Time) error {
	return db.Exec(`INSERT INTO bachs_capabilities (account_id, name, status, changed_at) VALUES (?, ?, ?, ?)
		ON CONFLICT (account_id, name) DO UPDATE SET status = EXCLUDED.status, changed_at = EXCLUDED.changed_at
		WHERE bachs_capabilities.changed_at <= EXCLUDED.changed_at`,
		accountId, name, status, changedAt).Error
}

// providerErr turns a failed Bachs call into a 502 for the seller.
func providerErr(err error) *utility.AppError {
	var apiErr *bachs.APIError
	if errors.As(err, &apiErr) && apiErr.StatusCode < 500 && apiErr.Detail != "" {
		return appErr(http.StatusBadGateway, "The payment provider refused this request: "+apiErr.Detail, err)
	}
	return appErr(http.StatusBadGateway, "Could not reach the payment provider. Please try again", err)
}
