package bachs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

// knownVector was computed with Python, following the Bachs docs' sample:
// hmac.new(b"whsec_test", b"1700000000." + b'{"id":"evt_1"}', hashlib.sha256).hexdigest()
const knownVector = "c89214b5b5da833daed6f0b8c5bb6bd58cea9022bd80ccc78230f3942d632925"

func TestSign_KnownVector(t *testing.T) {
	if got := Sign("whsec_test", 1700000000, []byte(`{"id":"evt_1"}`)); got != knownVector {
		t.Errorf("Sign = %s, want %s", got, knownVector)
	}
}

func headersV2(secret string, ts int64, body []byte, extra ...string) http.Header {
	h := http.Header{}
	v := "t=" + strconv.FormatInt(ts, 10)
	for _, s := range extra {
		v += ",v1=" + s
	}
	v += ",v1=" + Sign(secret, ts, body)
	h.Set("X-Bachs-Signature-V2", v)
	return h
}

func TestVerifySignature(t *testing.T) {
	body := []byte(`{"id":"evt_1","type":"checkout.completed"}`)
	now := time.Unix(1700000000, 0)
	ts := now.Unix()

	if err := VerifySignature(headersV2("whsec_new", ts, body, "deadbeef"), body, "whsec_new", now); err != nil {
		t.Errorf("valid v2 with an extra (rotated) signature: %v", err)
	}

	legacy := http.Header{}
	legacy.Set("X-Bachs-Timestamp", strconv.FormatInt(ts, 10))
	legacy.Set("X-Bachs-Signature", Sign("whsec_new", ts, body))
	if err := VerifySignature(legacy, body, "whsec_new", now); err != nil {
		t.Errorf("valid legacy headers: %v", err)
	}

	cases := map[string]struct {
		header http.Header
		body   []byte
		secret string
		now    time.Time
	}{
		"wrong secret":     {headersV2("whsec_other", ts, body), body, "whsec_new", now},
		"changed body":     {headersV2("whsec_new", ts, body), []byte(`{"id":"evt_1","type":"checkout.expired"}`), "whsec_new", now},
		"too old (replay)": {headersV2("whsec_new", ts, body), body, "whsec_new", now.Add(6 * time.Minute)},
		"from the future":  {headersV2("whsec_new", ts, body), body, "whsec_new", now.Add(-6 * time.Minute)},
		"no headers":       {http.Header{}, body, "whsec_new", now},
		"no secret set":    {headersV2("", ts, body), body, "", now},
	}
	for name, tc := range cases {
		if err := VerifySignature(tc.header, tc.body, tc.secret, tc.now); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestCreateCheckout(t *testing.T) {
	var gotHeaders http.Header
	var gotBody CheckoutRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/checkout-sessions" {
			t.Errorf("request %s %s", r.Method, r.URL.Path)
		}
		gotHeaders = r.Header.Clone()
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{"checkout_id":"chk_1","checkout_url":"https://pay.bachs.io/chk_1","status":"open","expires_at":"2026-10-14T12:30:00Z","reference":"pay_1","new_field":1}`)
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, APIKey: "sk_sandbox_x", HTTP: srv.Client()}
	req := CheckoutRequest{Pricing: Pricing{Currency: "NGN", Amount: "15000.00", PriceType: "fixed"}, SuccessURL: "https://shop.example/ok", Reference: "pay_1", ExpiresInMinutes: 25}
	checkout, err := c.CreateCheckout(context.Background(), "acct_seller", "pay_1", req)
	if err != nil {
		t.Fatal(err)
	}
	if checkout.CheckoutId != "chk_1" || checkout.CheckoutURL == "" {
		t.Errorf("checkout = %+v", checkout)
	}
	if gotHeaders.Get("Authorization") != "Bearer sk_sandbox_x" || gotHeaders.Get("X-Account-Id") != "acct_seller" || gotHeaders.Get("Idempotency-Key") != "pay_1" {
		t.Errorf("headers = %v", gotHeaders)
	}
	if gotBody.Pricing.Amount != "15000.00" || gotBody.ExpiresInMinutes != 25 || gotBody.PlatformFee != "" {
		t.Errorf("body = %+v", gotBody)
	}

	// Without an account the header is not sent at all.
	c.CreateCheckout(context.Background(), "", "pay_2", req)
	if _, sent := gotHeaders["X-Account-Id"]; sent {
		t.Errorf("X-Account-Id sent for a platform checkout")
	}
}

func TestCreateCheckout_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"detail":"Amount is below the minimum of 100 NGN","error_code":"AMOUNT_TOO_SMALL"}`)
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, APIKey: "sk", HTTP: srv.Client()}
	_, err := c.CreateCheckout(context.Background(), "", "k", CheckoutRequest{})
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 400 || apiErr.ErrorCode != "AMOUNT_TOO_SMALL" {
		t.Errorf("err = %v", err)
	}
}

func TestParseEvent_IgnoresUnknownFields(t *testing.T) {
	e, err := ParseEvent([]byte(`{"id":"evt_1","type":"checkout.completed","account":"acct_1","surprise":true,
		"data":{"checkout_id":"chk_1","payment_status":"paid","amount":"15000.00","currency":"NGN","charge":{"id":"ch_1"}}}`))
	if err != nil || e.Data.CheckoutId != "chk_1" || e.Account != "acct_1" || e.Data.Amount != "15000.00" {
		t.Errorf("%+v %v", e, err)
	}
	if _, err := ParseEvent([]byte(`{"type":"x"}`)); err == nil {
		t.Error("event without id accepted")
	}
}

func TestCapabilities_AllShapes(t *testing.T) {
	for name, body := range map[string]string{
		"keyed by name": `{"ngn_card_collection":{"status":"active","requested":true}}`,
		"list":          `[{"name":"ngn_card_collection","status":"active","requested":true}]`,
		"wrapped list":  `{"data":[{"name":"ngn_card_collection","status":"active","requested":true}]}`,
	} {
		var caps Capabilities
		if err := json.Unmarshal([]byte(body), &caps); err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if len(caps) != 1 || caps[0].Name != "ngn_card_collection" || caps[0].Status != CapabilityActive || !caps[0].Requested {
			t.Errorf("%s: %+v", name, caps)
		}
	}
}

func TestVerifyAny_EitherEndpointSecret(t *testing.T) {
	body := []byte(`{"id":"evt_1","type":"capability.updated"}`)
	now := time.Unix(1700000000, 0)
	secrets := WebhookSecrets(" whsec_account , whsec_connect,")
	if len(secrets) != 2 {
		t.Fatalf("secrets = %q, want 2", secrets)
	}
	for _, s := range []string{"whsec_account", "whsec_connect"} {
		if err := VerifyAny(headersV2(s, now.Unix(), body), body, secrets, now); err != nil {
			t.Errorf("signed with %s: %v", s, err)
		}
	}
	if err := VerifyAny(headersV2("whsec_other", now.Unix(), body), body, secrets, now); err == nil {
		t.Error("signed with an unknown secret was accepted")
	}
	if err := VerifyAny(headersV2("whsec_account", now.Unix(), body), body, nil, now); err == nil {
		t.Error("no secrets configured was accepted")
	}
}
