package router

import (
	"net/http"
	"regexp"
	"testing"

	"huddle/pkg/model"
	"huddle/pkg/utility"

	"github.com/google/uuid"
)

func TestStoreKey_RegenerateReplacesOldKey(t *testing.T) {
	s := newServer(t)
	chioma := s.signupAdmin("chioma")
	storeId, oldKey := s.createStoreWithKey(chioma, "Chioma's Bags")

	if w := s.request(http.MethodGet, "/storefront/store", map[string]string{"X-Store-Key": oldKey}, nil, nil); w.Code != http.StatusOK {
		t.Fatalf("old key before regenerating: %d", w.Code)
	}

	code, env := s.do(http.MethodPost, "/store/"+storeId+"/key", chioma, nil)
	var d struct {
		StoreId string `json:"store_id"`
		ApiKey  string `json:"api_key"`
	}
	s.data(env, &d)
	if code != http.StatusOK || d.StoreId != storeId {
		t.Fatalf("regenerate: %d %+v %s", code, d, env.Message)
	}
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(d.ApiKey) || d.ApiKey == oldKey {
		t.Fatalf("new key %q should be 64 hex chars and differ from the old one", d.ApiKey)
	}

	if w := s.request(http.MethodGet, "/storefront/store", map[string]string{"X-Store-Key": oldKey}, nil, nil); w.Code != http.StatusUnauthorized {
		t.Errorf("old key after regenerating: %d, want 401", w.Code)
	}
	if w := s.request(http.MethodGet, "/storefront/store", map[string]string{"X-Store-Key": d.ApiKey}, nil, nil); w.Code != http.StatusOK {
		t.Errorf("new key: %d, want 200", w.Code)
	}

	// Only the hash is stored.
	var store model.Stores
	s.db.Where("store_id = ?", storeId).First(&store)
	if store.ApiKey == d.ApiKey || store.ApiKey != utility.HashAPIKey(d.ApiKey) {
		t.Errorf("stored key is not the hash of the new key")
	}
}

func TestStoreKey_OnlyOwnerCanRegenerate(t *testing.T) {
	s := newServer(t)
	chioma := s.signupAdmin("chioma")
	storeId, key := s.createStoreWithKey(chioma, "Chioma's Bags")
	customerToken, _ := utility.CreateToken(uuid.New(), "shopper@example.com", utility.RoleCustomer)

	cases := []struct {
		name, path, token string
		want              int
	}{
		{"another admin", "/store/" + storeId + "/key", s.signupAdmin("tunde"), http.StatusForbidden},
		{"customer token", "/store/" + storeId + "/key", customerToken, http.StatusForbidden},
		{"no token", "/store/" + storeId + "/key", "", http.StatusUnauthorized},
		{"unknown store", "/store/str-missing/key", chioma, http.StatusNotFound},
	}
	for _, tc := range cases {
		if code, env := s.do(http.MethodPost, tc.path, tc.token, nil); code != tc.want {
			t.Errorf("%s: %d (%s), want %d", tc.name, code, env.Message, tc.want)
		}
	}

	// None of the rejected attempts changed the key.
	if w := s.request(http.MethodGet, "/storefront/store", map[string]string{"X-Store-Key": key}, nil, nil); w.Code != http.StatusOK {
		t.Errorf("original key stopped working after rejected attempts: %d", w.Code)
	}

	// The store key itself cannot be used to regenerate the key.
	w := s.request(http.MethodPost, "/store/"+storeId+"/key", map[string]string{"X-Store-Key": key}, nil, nil)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("store key used as auth: %d, want 401", w.Code)
	}
}
