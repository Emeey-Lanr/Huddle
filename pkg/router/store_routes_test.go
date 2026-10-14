package router

import (
	"net/http"
	"strings"
	"testing"

	"huddle/pkg/utility"

	"github.com/google/uuid"
)

type storeJSON struct {
	StoreId          string `json:"store_id"`
	StoreName        string `json:"store_name"`
	StoreDescription string `json:"store_description"`
}

func TestStoreRoutes_ListOnlyMine(t *testing.T) {
	s := newServer(t)
	chioma := s.signupAdmin("chioma")
	tunde := s.signupAdmin("tunde")
	s.createStore(chioma, "Chioma's Bags")
	s.createStore(chioma, "Chioma's Shoes")
	s.createStore(tunde, "Tunde Tech")

	code, env := s.do(http.MethodGet, "/store/stores", chioma, nil)
	if code != http.StatusOK {
		t.Fatalf("status %d: %s", code, env.Message)
	}
	var stores []storeJSON
	s.data(env, &stores)
	if len(stores) != 2 {
		t.Fatalf("got %d stores, want 2: %+v", len(stores), stores)
	}
	if strings.Contains(string(env.Data), "api_key") || strings.Contains(string(env.Data), "admin_id") {
		t.Errorf("list leaks private fields: %s", env.Data)
	}

	// A new admin with no stores gets an empty list, not null.
	code, env = s.do(http.MethodGet, "/store/stores", s.signupAdmin("newbie"), nil)
	if code != http.StatusOK || string(env.Data) != "[]" {
		t.Errorf("empty list: %d %s", code, env.Data)
	}
}

func TestStoreRoutes_GetOne(t *testing.T) {
	s := newServer(t)
	chioma := s.signupAdmin("chioma")
	storeId := s.createStore(chioma, "Chioma's Bags")

	code, env := s.do(http.MethodGet, "/store/"+storeId, chioma, nil)
	var got storeJSON
	s.data(env, &got)
	if code != http.StatusOK || got.StoreId != storeId || got.StoreName != "Chioma's Bags" {
		t.Errorf("owner: %d %+v", code, got)
	}

	customerToken, _ := utility.CreateToken(uuid.New(), "shopper@example.com", utility.RoleCustomer)
	cases := []struct {
		name, path, token string
		want              int
	}{
		{"another admin", "/store/" + storeId, s.signupAdmin("tunde"), http.StatusForbidden},
		{"unknown store", "/store/str-missing", chioma, http.StatusNotFound},
		{"no token", "/store/" + storeId, "", http.StatusUnauthorized},
		{"customer token", "/store/" + storeId, customerToken, http.StatusForbidden},
	}
	for _, tc := range cases {
		if code, env := s.do(http.MethodGet, tc.path, tc.token, nil); code != tc.want {
			t.Errorf("%s: status %d (%s), want %d", tc.name, code, env.Message, tc.want)
		}
	}
}

func TestStoreRoutes_Update(t *testing.T) {
	s := newServer(t)
	chioma := s.signupAdmin("chioma")
	bags := s.createStore(chioma, "Chioma's Bags")
	s.createStore(chioma, "Chioma's Shoes")

	// Rename only; description stays.
	code, env := s.do(http.MethodPut, "/store/"+bags, chioma, map[string]string{"store_name": "Chioma Couture"})
	var got storeJSON
	s.data(env, &got)
	if code != http.StatusOK || got.StoreName != "Chioma Couture" || got.StoreDescription != "Handmade goods from Lagos" {
		t.Fatalf("rename: %d %+v", code, got)
	}

	cases := []struct {
		name  string
		body  map[string]string
		token string
		want  int
	}{
		{"name taken by own store (any case)", map[string]string{"store_name": "CHIOMA'S SHOES"}, chioma, http.StatusConflict},
		{"description too short", map[string]string{"store_description": "short"}, chioma, http.StatusBadRequest},
		{"another admin", map[string]string{"store_name": "Hijacked"}, s.signupAdmin("tunde"), http.StatusForbidden},
	}
	for _, tc := range cases {
		if code, env := s.do(http.MethodPut, "/store/"+bags, tc.token, tc.body); code != tc.want {
			t.Errorf("%s: status %d (%s %s), want %d", tc.name, code, env.Message, env.Error, tc.want)
		}
	}

	// Nothing above changed the store.
	_, env = s.do(http.MethodGet, "/store/"+bags, chioma, nil)
	s.data(env, &got)
	if got.StoreName != "Chioma Couture" {
		t.Errorf("store changed by a rejected update: %+v", got)
	}
}
