package bachs

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// SignatureTolerance is how old a delivery may be, which limits replays.
const SignatureTolerance = 5 * time.Minute

var ErrBadSignature = errors.New("invalid webhook signature")

// Sign returns the hex HMAC-SHA256 of "{timestamp}.{body}", as Bachs signs it.
func Sign(secret string, timestamp int64, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strconv.FormatInt(timestamp, 10) + "."))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifySignature checks a delivery against the raw body. It reads
// X-Bachs-Signature-V2 ("t=...,v1=...,v1=..."; any v1 may match, which keeps
// secret rotation working) and falls back to X-Bachs-Timestamp with
// X-Bachs-Signature.
func VerifySignature(header http.Header, body []byte, secret string, now time.Time) error {
	if secret == "" {
		return errors.New("BACHS_WEBHOOK_SECRET is not set")
	}

	var timestamp string
	var signatures []string
	if v2 := header.Get("X-Bachs-Signature-V2"); v2 != "" {
		for _, part := range strings.Split(v2, ",") {
			k, v, ok := strings.Cut(strings.TrimSpace(part), "=")
			switch {
			case !ok:
			case k == "t":
				timestamp = v
			case k == "v1":
				signatures = append(signatures, v)
			}
		}
	} else {
		timestamp = header.Get("X-Bachs-Timestamp")
		if sig := header.Get("X-Bachs-Signature"); sig != "" {
			signatures = []string{sig}
		}
	}

	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil || len(signatures) == 0 {
		return ErrBadSignature
	}
	if age := now.Sub(time.Unix(ts, 0)); age > SignatureTolerance || age < -SignatureTolerance {
		return ErrBadSignature
	}

	expected := []byte(Sign(secret, ts, body))
	for _, sig := range signatures {
		if hmac.Equal(expected, []byte(sig)) {
			return nil
		}
	}
	return ErrBadSignature
}

// WebhookSecrets splits BACHS_WEBHOOK_SECRET. Bachs endpoints listen to one
// event source each (the platform's own account, or connected accounts), so
// Huddle has two endpoints with the same URL and a secret for each, given
// comma-separated.
func WebhookSecrets(value string) []string {
	var secrets []string
	for _, s := range strings.Split(value, ",") {
		if s = strings.TrimSpace(s); s != "" {
			secrets = append(secrets, s)
		}
	}
	return secrets
}

// VerifyAny accepts a delivery signed with any of the secrets.
func VerifyAny(header http.Header, body []byte, secrets []string, now time.Time) error {
	if len(secrets) == 0 {
		return errors.New("BACHS_WEBHOOK_SECRET is not set")
	}
	for _, secret := range secrets {
		if err := VerifySignature(header, body, secret, now); err == nil {
			return nil
		}
	}
	return ErrBadSignature
}

// Event is the webhook envelope. Fields Huddle does not use are ignored, as
// Bachs asks, since new fields may be added at any time.
type Event struct {
	Id             string    `json:"id"`
	Type           string    `json:"type"`
	CreatedAt      time.Time `json:"created_at"`
	OrganizationId string    `json:"organization_id"`
	// Account is set on events from a connected (seller) account.
	Account string    `json:"account"`
	Data    EventData `json:"data"`
}

type EventData struct {
	CheckoutId    string `json:"checkout_id"`
	Reference     string `json:"reference"`
	Status        string `json:"status"`
	PaymentStatus string `json:"payment_status"`
	Amount        string `json:"amount"`
	Currency      string `json:"currency"`
	// Set on capability.updated.
	Account    string `json:"account"`
	Capability string `json:"capability"`
	// The charge behind a payment: charge_id on collection.succeeded and
	// refund events, charge.id on checkout.completed.
	ChargeId string       `json:"charge_id"`
	Charge   *EventCharge `json:"charge"`
	// Set on refund events; Reference is then the refund's reference.
	RefundId string `json:"refund_id"`
}

type EventCharge struct {
	Id string `json:"id"`
}

// ChargeID returns the event's charge id, or "".
func (d EventData) ChargeID() string {
	if d.ChargeId != "" {
		return d.ChargeId
	}
	if d.Charge != nil {
		return d.Charge.Id
	}
	return ""
}

// SourceAccount is the account an event happened on: "account" when set,
// otherwise organization_id, as the Bachs docs say to read it.
func (e *Event) SourceAccount() string {
	if e.Account != "" {
		return e.Account
	}
	return e.OrganizationId
}

func ParseEvent(body []byte) (*Event, error) {
	var e Event
	if err := json.Unmarshal(body, &e); err != nil {
		return nil, err
	}
	if e.Id == "" || e.Type == "" {
		return nil, errors.New("webhook event has no id or type")
	}
	return &e, nil
}
