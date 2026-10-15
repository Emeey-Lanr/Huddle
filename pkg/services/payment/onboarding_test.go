package payment

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"huddle/pkg/bachs"
	"huddle/pkg/model"
)

// fakeConnect answers the Bachs Connect endpoints. Accounts are keyed by
// idempotency key, as Bachs returns the same account for a retried request.
type fakeConnect struct {
	mu         sync.Mutex
	accounts   map[string]string // idempotency key -> account id
	created    []bachs.AccountRequest
	createKeys []string
	linkTypes  []string
	capStatus  string // status of ngn_card_collection; "active" when empty
	linkError  string // error_code for account links
	down       bool   // every request fails with 500
}

func (f *fakeConnect) client(t *testing.T) *bachs.Client {
	f.accounts = map[string]string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.down {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		status := f.capStatus
		if status == "" {
			status = "active"
		}
		caps := fmt.Sprintf(`{"ngn_card_collection":{"status":%q,"requested":true,"status_details":[{"code":"requirements_outstanding","resolution":"provide_info","message":"Upload a valid ID"}]},
			"payouts":{"status":"active","requested":true,"status_details":null},
			"crypto":{"status":"unrequested","requested":false,"status_details":null}}`, status)

		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/accounts":
			var body bachs.AccountRequest
			json.NewDecoder(r.Body).Decode(&body)
			key := r.Header.Get("Idempotency-Key")
			id, ok := f.accounts[key]
			if !ok {
				id = fmt.Sprintf("acct_%d", len(f.accounts)+1)
				f.accounts[key] = id
				f.created = append(f.created, body)
				f.createKeys = append(f.createKeys, key)
			}
			w.WriteHeader(http.StatusCreated)
			fmt.Fprintf(w, `{"id":%q,"capabilities":%s,"requirements":{"currently_due":[]}}`, id, caps)
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/account-links"):
			var body bachs.AccountLinkRequest
			json.NewDecoder(r.Body).Decode(&body)
			f.linkTypes = append(f.linkTypes, body.Type)
			if f.linkError != "" {
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprintf(w, `{"detail":"no","error_code":%q}`, f.linkError)
				return
			}
			w.WriteHeader(http.StatusCreated)
			fmt.Fprintf(w, `{"id":"alnk_1","url":"https://connect.bachs.io/onboarding/tok_%d","expires_at":"2030-01-01T00:00:00Z"}`, len(f.linkTypes))
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/accounts/"):
			fmt.Fprintf(w, `{"id":%q,"capabilities":%s,"requirements":{"currently_due":["representative.id_document"],"past_due":["representative.id_document"]}}`,
				strings.TrimPrefix(r.URL.Path, "/v1/accounts/"), caps)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return &bachs.Client{BaseURL: srv.URL, APIKey: "sk_sandbox_test", HTTP: srv.Client()}
}

var onboardReq = model.OnboardingRequest{ReturnURL: "https://huddle.ng/payments", RefreshURL: "https://huddle.ng/payments/retry"}

func (s *shop) reloadStore() *model.Stores {
	var store model.Stores
	s.db.Where("id = ?", s.store.ID).First(&store)
	return &store
}

func (s *shop) capability(account, name string) string {
	var c model.BachsCapabilities
	s.db.Where("account_id = ? AND name = ?", account, name).First(&c)
	return c.Status
}

func TestOnboard_CreatesAccountOnceAndLinks(t *testing.T) {
	s := newShop(t)
	fake := &fakeConnect{capStatus: "restricted"}
	client := fake.client(t)

	link, appErr := Onboard(s.db, client, s.reloadStore(), "chioma@example.com", onboardReq)
	if appErr != nil {
		t.Fatalf("onboard: %s %v", appErr.Message, appErr.Err)
	}
	if !strings.HasPrefix(link.URL, "https://connect.bachs.io/onboarding/") || link.ExpiresAt.IsZero() {
		t.Errorf("link = %+v", link)
	}
	if len(fake.created) != 1 {
		t.Fatalf("%d accounts created, want 1", len(fake.created))
	}
	got := fake.created[0]
	if got.ContactEmail != "chioma@example.com" || got.Country != "NG" || got.DisplayName != s.store.StoreName {
		t.Errorf("account request = %+v", got)
	}
	if !got.Configuration["merchant"].Capabilities["ngn_card_collection"].Requested || !got.Configuration["recipient"].Capabilities["payouts"].Requested {
		t.Errorf("capabilities requested = %+v", got.Configuration)
	}
	if fake.createKeys[0] != "huddle-store-"+s.store.ID.String() {
		t.Errorf("idempotency key = %q", fake.createKeys[0])
	}
	store := s.reloadStore()
	if store.BachsAccountId == nil || *store.BachsAccountId != "acct_1" {
		t.Fatalf("bachs_account_id = %v, want acct_1", store.BachsAccountId)
	}
	if got := s.capability("acct_1", "ngn_card_collection"); got != "restricted" {
		t.Errorf("saved ngn_card_collection = %q, want restricted", got)
	}
	if got := s.capability("acct_1", "crypto"); got != "" {
		t.Errorf("unrequested capability saved as %q", got)
	}

	// Again: same account, new onboarding link.
	if _, appErr := Onboard(s.db, client, store, "chioma@example.com", onboardReq); appErr != nil {
		t.Fatalf("second onboard: %s", appErr.Message)
	}
	if len(fake.created) != 1 {
		t.Errorf("%d accounts created after a second call, want 1", len(fake.created))
	}

	// Once the store can collect, the link is for updating details.
	saveCapability(s.db, "acct_1", "ngn_card_collection", "active", time.Now().UTC())
	if _, appErr := Onboard(s.db, client, store, "chioma@example.com", onboardReq); appErr != nil {
		t.Fatalf("third onboard: %s", appErr.Message)
	}
	if want := []string{"onboarding", "onboarding", "update"}; strings.Join(fake.linkTypes, ",") != strings.Join(want, ",") {
		t.Errorf("link types = %v, want %v", fake.linkTypes, want)
	}
}

func TestOnboard_ConcurrentCallsShareOneAccount(t *testing.T) {
	s := newShop(t)
	fake := &fakeConnect{}
	client := fake.client(t)

	var wg sync.WaitGroup
	errs := make(chan string, 5)
	for range 5 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, appErr := Onboard(s.db, client, s.reloadStore(), "chioma@example.com", onboardReq); appErr != nil {
				errs <- appErr.Message
			}
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Errorf("onboard: %s", e)
	}
	if len(fake.created) != 1 {
		t.Errorf("%d Bachs accounts created, want 1", len(fake.created))
	}
	if store := s.reloadStore(); store.BachsAccountId == nil || *store.BachsAccountId != "acct_1" {
		t.Errorf("bachs_account_id = %v", store.BachsAccountId)
	}
}

func TestOnboard_Errors(t *testing.T) {
	s := newShop(t)
	if _, appErr := Onboard(s.db, nil, s.reloadStore(), "chioma@example.com", onboardReq); appErr == nil || appErr.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("no Bachs key: %+v, want 503", appErr)
	}

	fake := &fakeConnect{down: true}
	client := fake.client(t)
	if _, appErr := Onboard(s.db, client, s.reloadStore(), "chioma@example.com", onboardReq); appErr == nil || appErr.StatusCode != http.StatusBadGateway {
		t.Errorf("Bachs down: %+v, want 502", appErr)
	}
	if store := s.reloadStore(); store.BachsAccountId != nil {
		t.Errorf("account saved after a failed create: %v", *store.BachsAccountId)
	}

	fake.down = false
	fake.linkError = "CONNECTED_ACCOUNT_NOT_ELIGIBLE_FOR_LINK"
	if _, appErr := Onboard(s.db, client, s.reloadStore(), "chioma@example.com", onboardReq); appErr == nil || appErr.StatusCode != http.StatusConflict ||
		!strings.Contains(appErr.Message, "suspended or rejected") {
		t.Errorf("rejected account: %+v, want 409", appErr)
	}
}

func TestSetup(t *testing.T) {
	s := newShop(t)
	fake := &fakeConnect{capStatus: "restricted"}
	client := fake.client(t)

	setup, appErr := Setup(s.db, client, s.reloadStore())
	if appErr != nil || setup.Connected || setup.Ready || len(setup.Capabilities) != 0 {
		t.Fatalf("store without account: %+v %v", setup, appErr)
	}

	s.connect("acct_chioma")
	setup, appErr = Setup(s.db, client, s.reloadStore())
	if appErr != nil {
		t.Fatalf("setup: %s", appErr.Message)
	}
	if !setup.Connected || setup.Ready {
		t.Errorf("restricted account: connected=%v ready=%v, want true/false", setup.Connected, setup.Ready)
	}
	if len(setup.Outstanding) != 1 || setup.Outstanding[0] != "representative.id_document" {
		t.Errorf("outstanding = %v", setup.Outstanding)
	}
	var card model.CapabilityStatus
	for _, c := range setup.Capabilities {
		if c.Name == "ngn_card_collection" {
			card = c
		}
	}
	if card.Status != "restricted" || card.Message != "Upload a valid ID" {
		t.Errorf("ngn_card_collection = %+v", card)
	}

	fake.capStatus = "active"
	if setup, _ = Setup(s.db, client, s.reloadStore()); !setup.Ready {
		t.Errorf("active account not ready: %+v", setup)
	}

	// Bachs unreachable: last known statuses.
	fake.down = true
	setup, appErr = Setup(s.db, client, s.reloadStore())
	if appErr != nil || !setup.Ready || len(setup.Capabilities) != 2 {
		t.Errorf("Bachs down: %+v %v, want stored statuses", setup, appErr)
	}
}

func TestHandleEvent_CapabilityUpdated(t *testing.T) {
	s := newShop(t)
	t0 := time.Now().UTC().Add(-time.Hour)
	capEvent := func(id, status string, at time.Time) *bachs.Event {
		return &bachs.Event{Id: id, Type: "capability.updated", CreatedAt: at, OrganizationId: "acct_chioma", Account: "acct_chioma",
			Data: bachs.EventData{Account: "acct_chioma", Capability: "ngn_card_collection", Status: status}}
	}

	s.handle(capEvent("evt_c1", "restricted", t0))
	s.handle(capEvent("evt_c2", "active", t0.Add(time.Minute)))
	if got := s.capability("acct_chioma", "ngn_card_collection"); got != "active" {
		t.Errorf("after activation = %q, want active", got)
	}
	// An older event delivered late does not undo the newer status.
	s.handle(capEvent("evt_c0", "restricted", t0.Add(-time.Minute)))
	if got := s.capability("acct_chioma", "ngn_card_collection"); got != "active" {
		t.Errorf("after a late older event = %q, want active", got)
	}
	s.handle(capEvent("evt_c3", "restricted", t0.Add(2*time.Minute)))
	if got := s.capability("acct_chioma", "ngn_card_collection"); got != "restricted" {
		t.Errorf("after restriction = %q, want restricted", got)
	}
}

func TestStartPayment_ProductionNeedsActiveAccount(t *testing.T) {
	s := newShop(t)
	fake := &fakeBachs{}
	client := fake.client(t)
	s.connect("acct_chioma")
	o := s.placeOrder(1, "")
	t.Setenv("APP_ENV", "production")

	saveCapability(s.db, "acct_chioma", "ngn_card_collection", "restricted", time.Now().UTC())
	if _, appErr := StartPayment(s.db, client, s.store.ID, o.Id, model.PayRequest{SuccessURL: "https://chioma.shop/thanks"}); appErr == nil ||
		appErr.StatusCode != http.StatusConflict || appErr.Message != "This store can't take online payments yet" {
		t.Errorf("restricted account in production: %+v, want 409", appErr)
	}
	if len(fake.requests) != 0 {
		t.Errorf("%d checkouts created for a store that can't collect", len(fake.requests))
	}

	saveCapability(s.db, "acct_chioma", "ngn_card_collection", "active", time.Now().UTC())
	if p := s.pay(client, o.Id); p.CheckoutURL == "" {
		t.Errorf("active account: no checkout")
	}
}
