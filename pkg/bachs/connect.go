package bachs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"time"
)

// Capability statuses. Only active allows the action.
const CapabilityActive = "active"

// CapabilityRequest turns a capability on or off when creating an account.
type CapabilityRequest struct {
	Requested bool `json:"requested"`
}

type Persona struct {
	Capabilities map[string]CapabilityRequest `json:"capabilities"`
}

// AccountRequest creates a connected (seller) account under the platform.
type AccountRequest struct {
	ContactEmail  string             `json:"contact_email"`
	DisplayName   string             `json:"display_name,omitempty"`
	Country       string             `json:"country,omitempty"`
	Configuration map[string]Persona `json:"configuration,omitempty"`
}

type StatusDetail struct {
	Code       string `json:"code"`
	Resolution string `json:"resolution"`
	Message    string `json:"message"`
}

type Capability struct {
	Name          string         `json:"name"`
	Status        string         `json:"status"`
	Requested     bool           `json:"requested"`
	StatusDetails []StatusDetail `json:"status_details"`
}

// Capabilities reads the shapes Bachs uses for capabilities: an object keyed
// by name (on an account), a list, or a list wrapped in "data".
type Capabilities []Capability

func (c *Capabilities) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		*c = nil
		return nil
	}
	var list []Capability
	if data[0] == '[' {
		if err := json.Unmarshal(data, &list); err != nil {
			return err
		}
		*c = list
		return nil
	}
	var wrapped struct {
		Data *[]Capability `json:"data"`
	}
	if err := json.Unmarshal(data, &wrapped); err == nil && wrapped.Data != nil {
		*c = *wrapped.Data
		return nil
	}
	var byName map[string]Capability
	if err := json.Unmarshal(data, &byName); err != nil {
		return err
	}
	list = make([]Capability, 0, len(byName))
	for name, cap := range byName {
		cap.Name = name
		list = append(list, cap)
	}
	*c = list
	return nil
}

type Requirements struct {
	CurrentlyDue        []string `json:"currently_due"`
	PastDue             []string `json:"past_due"`
	PendingVerification []string `json:"pending_verification"`
}

type Account struct {
	Id           string       `json:"id"`
	Capabilities Capabilities `json:"capabilities"`
	Requirements Requirements `json:"requirements"`
}

// AccountLink is a single-use link to Bachs' hosted onboarding.
type AccountLink struct {
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Account link types: onboarding for a new account, update to collect more
// details from an account that is already live.
const (
	LinkOnboarding = "onboarding"
	LinkUpdate     = "update"
)

type AccountLinkRequest struct {
	Type       string `json:"type"`
	RefreshURL string `json:"refresh_url"`
	ReturnURL  string `json:"return_url"`
}

// CreateAccount creates a connected account. idempotencyKey makes a retry
// return the same account instead of creating a second one.
func (c *Client) CreateAccount(ctx context.Context, idempotencyKey string, req AccountRequest) (*Account, error) {
	var account Account
	if err := c.do(ctx, http.MethodPost, "/v1/accounts", "", idempotencyKey, req, &account); err != nil {
		return nil, err
	}
	if account.Id == "" {
		return nil, errors.New("bachs: account response has no id")
	}
	return &account, nil
}

func (c *Client) GetAccount(ctx context.Context, accountId string) (*Account, error) {
	var account Account
	if err := c.do(ctx, http.MethodGet, "/v1/accounts/"+url.PathEscape(accountId), "", "", nil, &account); err != nil {
		return nil, err
	}
	return &account, nil
}

func (c *Client) ListCapabilities(ctx context.Context, accountId string) (Capabilities, error) {
	var caps Capabilities
	if err := c.do(ctx, http.MethodGet, "/v1/accounts/"+url.PathEscape(accountId)+"/capabilities", "", "", nil, &caps); err != nil {
		return nil, err
	}
	return caps, nil
}

// CreateAccountLink makes a new onboarding link. It supersedes any earlier
// link, so create one only when the seller is about to open it.
func (c *Client) CreateAccountLink(ctx context.Context, accountId, idempotencyKey string, req AccountLinkRequest) (*AccountLink, error) {
	var link AccountLink
	if err := c.do(ctx, http.MethodPost, "/v1/accounts/"+url.PathEscape(accountId)+"/account-links", "", idempotencyKey, req, &link); err != nil {
		return nil, err
	}
	if link.URL == "" {
		return nil, errors.New("bachs: account link response has no url")
	}
	return &link, nil
}
