package controller

import (
	"errors"
	"huddle/pkg/audit"
	"huddle/pkg/bachs"
	"huddle/pkg/model"
	"huddle/pkg/response"
	"huddle/pkg/services"
	"huddle/pkg/services/payment"
	"huddle/pkg/utility"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const maxWebhookBody = 1 << 20

type PaymentController struct {
	Db *gorm.DB
	// Bachs is nil when BACHS_API_KEY is not set.
	Bachs *bachs.Client
}

// Pay opens a Bachs checkout for an order and returns the URL to send the
// shopper to.
func (b *PaymentController) Pay(c *gin.Context) {
	var req model.PayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request payload", err)
		return
	}
	if err := utility.Validate.Struct(&req); err != nil {
		if validationErr := utility.ValidationErrors(err); validationErr != nil {
			response.Error(c, http.StatusBadRequest, "Validation failed", validationErr)
			return
		}
	}

	resp, serviceErr := payment.StartPayment(b.Db, b.Bachs, storeFromKey(c), c.Param("id"), req)
	if serviceErr != nil {
		response.Error(c, serviceErr.StatusCode, serviceErr.Message, serviceErr.Err)
		return
	}
	audit.LogEvent(c.Request.Context(), "PAYMENT_STARTED", c.GetString("X-Store-Id"), c.Param("id"), "SUCCESS", map[string]interface{}{"payment_id": resp.PaymentId})
	response.Success(c, "Checkout ready", resp)
}

// BachsWebhook receives Bachs events. Only signed deliveries are accepted.
// 400 tells Bachs not to retry; 5xx makes it retry.
func (b *PaymentController) BachsWebhook(c *gin.Context) {
	secrets := bachs.WebhookSecrets(os.Getenv("BACHS_WEBHOOK_SECRET"))
	if len(secrets) == 0 {
		response.Error(c, http.StatusServiceUnavailable, "Webhooks are not set up", errors.New("BACHS_WEBHOOK_SECRET is not set"))
		return
	}

	body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxWebhookBody))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Could not read body", err)
		return
	}
	if err := bachs.VerifyAny(c.Request.Header, body, secrets, time.Now()); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid signature", err)
		return
	}
	event, err := bachs.ParseEvent(body)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid event", err)
		return
	}

	if err := payment.HandleEvent(b.Db, event, time.Now().UTC()); err != nil {
		log.Printf("bachs webhook %s (%s): %v", event.Id, event.Type, err)
		response.Error(c, http.StatusInternalServerError, "Could not process event", err)
		return
	}
	response.Success(c, "Event received", nil)
}

// Onboard returns a link to Bachs onboarding so the seller can take online
// payments. Owner only.
func (b *PaymentController) Onboard(c *gin.Context) {
	adminId, ok := adminUUID(c)
	if !ok {
		return
	}
	storeId := c.Param("id")
	var req model.OnboardingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request payload", err)
		return
	}
	if err := utility.Validate.Struct(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Validation failed", utility.ValidationErrors(err))
		return
	}

	store, serviceErr := services.OwnedStore(b.Db, storeId, adminId)
	if serviceErr != nil {
		response.Error(c, serviceErr.StatusCode, serviceErr.Message, serviceErr.Err)
		return
	}
	link, serviceErr := payment.Onboard(b.Db, b.Bachs, store, c.GetString("email"), req)
	if serviceErr != nil {
		audit.LogEvent(c.Request.Context(), "PAYMENT_ONBOARDING", adminId.String(), storeId, "FAILED", map[string]interface{}{"error": serviceErr.Message})
		response.Error(c, serviceErr.StatusCode, serviceErr.Message, serviceErr.Err)
		return
	}
	audit.LogEvent(c.Request.Context(), "PAYMENT_ONBOARDING", adminId.String(), storeId, "SUCCESS", nil)
	response.Success(c, "Open the link to set up payments", link)
}

// Setup tells the seller whether their store can take online payments.
func (b *PaymentController) Setup(c *gin.Context) {
	adminId, ok := adminUUID(c)
	if !ok {
		return
	}
	store, serviceErr := services.OwnedStore(b.Db, c.Param("id"), adminId)
	if serviceErr != nil {
		response.Error(c, serviceErr.StatusCode, serviceErr.Message, serviceErr.Err)
		return
	}
	setup, serviceErr := payment.Setup(b.Db, b.Bachs, store)
	if serviceErr != nil {
		response.Error(c, serviceErr.StatusCode, serviceErr.Message, serviceErr.Err)
		return
	}
	response.Success(c, "Payment setup retrieved", setup)
}
