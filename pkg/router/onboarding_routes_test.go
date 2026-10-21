package router

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// withFakeConnect points the API at a fake Bachs that answers the Connect
// endpoints, before the router is built.
func withFakeConnect(t *testing.T) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		caps := `{"ngn_card_collection":{"status":"active","requested":true},"payouts":{"status":"active","requested":true}}`
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/accounts":
			w.WriteHeader(http.StatusCreated)
			fmt.Fprintf(w, `{"id":"acct_route","capabilities":%s}`, caps)
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/account-links"):
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, `{"url":"https://connect.bachs.io/onboarding/tok_route","expires_at":"2030-01-01T00:00:00Z"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/accounts/acct_route":
			fmt.Fprintf(w, `{"id":"acct_route","capabilities":%s,"requirements":{"currently_due":[]}}`, caps)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("BACHS_BASE_URL", srv.URL)
	t.Setenv("BACHS_API_KEY", "sk_sandbox_test")
	t.Setenv("BACHS_WEBHOOK_SECRET", webhookSecret)
}

func TestPayments_SellerOnboarding(t *testing.T) {
	withFakeConnect(t)
	s := newShop(t)
	setupPath := "/store/" + s.storeId + "/payments"
	onboardPath := setupPath + "/onboarding"
	urls := map[string]string{"return_url": "https://huddle.ng/payments", "refresh_url": "https://huddle.ng/payments/retry"}

	var setup struct {
		Connected bool `json:"connected"`
		Ready     bool `json:"ready"`
	}
	code, env := s.do(http.MethodGet, setupPath, s.owner, nil)
	s.data(env, &setup)
	if code != http.StatusOK || setup.Connected || setup.Ready {
		t.Fatalf("before onboarding: %d %+v", code, setup)
	}

	if code, _ := s.do(http.MethodPost, onboardPath, "", urls); code != http.StatusUnauthorized {
		t.Errorf("no token: %d, want 401", code)
	}
	if code, _ := s.do(http.MethodPost, onboardPath, s.otherOwner, urls); code != http.StatusForbidden {
		t.Errorf("another seller: %d, want 403", code)
	}
	if code, _ := s.do(http.MethodGet, setupPath, s.otherOwner, nil); code != http.StatusForbidden {
		t.Errorf("another seller reads setup: %d, want 403", code)
	}
	for name, body := range map[string]map[string]string{
		"no refresh_url":  {"return_url": "https://huddle.ng/payments"},
		"javascript link": {"return_url": "javascript:alert(1)", "refresh_url": "https://huddle.ng/payments/retry"},
	} {
		if code, _ := s.do(http.MethodPost, onboardPath, s.owner, body); code != http.StatusBadRequest {
			t.Errorf("%s: %d, want 400", name, code)
		}
	}

	var link struct {
		URL string `json:"url"`
	}
	code, env = s.do(http.MethodPost, onboardPath, s.owner, urls)
	s.data(env, &link)
	if code != http.StatusOK || link.URL != "https://connect.bachs.io/onboarding/tok_route" {
		t.Fatalf("onboard: %d %+v %s", code, link, env.Message)
	}

	code, env = s.do(http.MethodGet, setupPath, s.owner, nil)
	s.data(env, &setup)
	if code != http.StatusOK || !setup.Connected || !setup.Ready {
		t.Errorf("after onboarding: %d %+v", code, setup)
	}
	// The account id stays private.
	if code, env := s.do(http.MethodGet, "/store/"+s.storeId, s.owner, nil); code != http.StatusOK || strings.Contains(string(env.Data), "acct_route") {
		t.Errorf("store details leak the Bachs account: %s", env.Data)
	}
}

func TestPayments_CapabilityWebhook(t *testing.T) {
	withFakeConnect(t)
	s := newShop(t)
	s.db.Exec("UPDATE stores SET bachs_account_id = 'acct_route' WHERE store_id = ?", s.storeId)

	event := `{"id":"evt_cap_1","type":"capability.updated","created_at":"2030-01-01T00:00:00Z","organization_id":"acct_route","account":"acct_route",
		"data":{"account":"acct_route","capability":"ngn_card_collection","status":"restricted","requested":true}}`
	if w := s.webhook(event, webhookSecret); w.Code != http.StatusOK {
		t.Fatalf("capability webhook: %d %s", w.Code, w.Body.String())
	}
	var status string
	s.db.Raw("SELECT status FROM bachs_capabilities WHERE account_id = 'acct_route' AND name = 'ngn_card_collection'").Scan(&status)
	if status != "restricted" {
		t.Errorf("saved status = %q, want restricted", status)
	}
}
