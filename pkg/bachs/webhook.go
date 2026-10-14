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
