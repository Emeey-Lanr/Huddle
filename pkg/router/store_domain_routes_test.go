package router

import (
	"fmt"
	"net/http"
	"testing"

	"huddle/pkg/utility"

	"github.com/google/uuid"
)

type storeDomain struct {
	Domain string `json:"domain"`
	Origin string `json:"origin"`
}

func (s shop) listDomains() []storeDomain {
	s.t.Helper()
	code, env := s.do(http.MethodGet, "/store/"+s.storeId+"/domains", s.owner, nil)
	if code != http.StatusOK {
		s.t.Fatalf("list domains: %d %s", code, env.Message)
	}
	var d []storeDomain
	s.data(env, &d)
	return d
}

func (s shop) addDomain(domain string) {
	s.t.Helper()
	if code, env := s.do(http.MethodPost, "/store/"+s.storeId+"/domains", s.owner, map[string]string{"domain": domain}); code != http.StatusCreated {
		s.t.Fatalf("add domain %s: %d %s", domain, code, env.Message)
	}
}

func (s shop) preflight(origin string) int {
	return s.request(http.MethodOptions, "/cart", map[string]string{
		"Origin": origin, "Access-Control-Request-Method": "POST", "Access-Control-Request-Headers": "content-type, x-store-key",
	}, nil, nil).Code
}

func TestStoreDomains_AddListRemove(t *testing.T) {
	s := newShop(t)

	if d := s.listDomains(); len(d) != 0 {
		t.Fatalf("new store has domains: %+v", d)
	}

	code, env := s.do(http.MethodPost, "/store/"+s.storeId+"/domains", s.owner, map[string]string{"domain": " https://ChiomasBags.com/ "})
	var added storeDomain
	s.data(env, &added)
	if code != http.StatusCreated || added.Domain != "chiomasbags.com" || added.Origin != "https://chiomasbags.com" {
		t.Fatalf("add: %d %+v %s", code, added, env.Message)
	}

	// Adding it again is harmless.
	if code, env := s.do(http.MethodPost, "/store/"+s.storeId+"/domains", s.owner, map[string]string{"domain": "chiomasbags.com"}); code != http.StatusOK {
		t.Errorf("add again: %d %s, want 200", code, env.Message)
	}
	s.addDomain("www.chiomasbags.com")
	if d := s.listDomains(); len(d) != 2 || d[0].Domain != "chiomasbags.com" || d[1].Domain != "www.chiomasbags.com" {
		t.Errorf("list: %+v", d)
	}

	if code, env := s.do(http.MethodDelete, "/store/"+s.storeId+"/domains/ChiomasBags.com", s.owner, nil); code != http.StatusOK {
		t.Fatalf("remove: %d %s", code, env.Message)
	}
	if d := s.listDomains(); len(d) != 1 || d[0].Domain != "www.chiomasbags.com" {
		t.Errorf("after remove: %+v", d)
	}
	for _, domain := range []string{"chiomasbags.com", "not_a_domain"} {
		if code, _ := s.do(http.MethodDelete, "/store/"+s.storeId+"/domains/"+domain, s.owner, nil); code != http.StatusNotFound {
			t.Errorf("remove %q: %d, want 404", domain, code)
		}
	}
}

func TestStoreDomains_Rejected(t *testing.T) {
	s := newShop(t)
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://*.huddle.ng")

	for _, domain := range []string{"", "chiomasbags", "chiomasbags.com/shop", "https://chiomasbags.com:8443", "192.168.0.1", "localhost", "chioma.huddle.ng"} {
		if code, env := s.do(http.MethodPost, "/store/"+s.storeId+"/domains", s.owner, map[string]string{"domain": domain}); code != http.StatusBadRequest {
			t.Errorf("add %q: %d %s, want 400", domain, code, env.Message)
		}
	}

	// A domain belongs to one store.
	s.addDomain("chiomasbags.com")
	tundeStore := s.createStore(s.otherOwner, "Tunde Gadgets")
	if code, env := s.do(http.MethodPost, "/store/"+tundeStore+"/domains", s.otherOwner, map[string]string{"domain": "CHIOMASBAGS.COM"}); code != http.StatusConflict {
		t.Errorf("another store's domain: %d %s, want 409", code, env.Message)
	}

	// At most 5 per store.
	for i := 2; i <= 5; i++ {
		s.addDomain(fmt.Sprintf("shop%d.chiomasbags.com", i))
	}
	if code, env := s.do(http.MethodPost, "/store/"+s.storeId+"/domains", s.owner, map[string]string{"domain": "shop6.chiomasbags.com"}); code != http.StatusConflict {
		t.Errorf("sixth domain: %d %s, want 409", code, env.Message)
	}
}

func TestStoreDomains_OnlyOwner(t *testing.T) {
	s := newShop(t)
	s.addDomain("chiomasbags.com")
	customerToken, _ := utility.CreateToken(uuid.New(), "shopper@example.com", utility.RoleCustomer)

	cases := []struct {
		name, token, store string
		want               int
	}{
		{"no token", "", s.storeId, http.StatusUnauthorized},
		{"customer token", customerToken, s.storeId, http.StatusForbidden},
		{"other seller", s.otherOwner, s.storeId, http.StatusForbidden},
		{"unknown store", s.owner, "str-unknown", http.StatusNotFound},
	}
	for _, tc := range cases {
		for _, req := range []struct{ method, path string }{
			{http.MethodGet, "/store/" + tc.store + "/domains"},
			{http.MethodPost, "/store/" + tc.store + "/domains"},
			{http.MethodDelete, "/store/" + tc.store + "/domains/chiomasbags.com"},
		} {
			if code, _ := s.do(req.method, req.path, tc.token, map[string]string{"domain": "tundetech.com"}); code != tc.want {
				t.Errorf("%s: %s %s = %d, want %d", tc.name, req.method, req.path, code, tc.want)
			}
		}
	}
	if d := s.listDomains(); len(d) != 1 || d[0].Domain != "chiomasbags.com" {
		t.Errorf("domains changed by others: %+v", d)
	}
}

func TestStoreDomains_CORSFollowsDomains(t *testing.T) {
	s := newShop(t)
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://*.huddle.ng")

	if code := s.preflight("https://chiomasbags.com"); code != http.StatusForbidden {
		t.Fatalf("before adding: %d, want 403", code)
	}

	// Seen at once, though "no store" was just cached.
	s.addDomain("chiomasbags.com")
	if code := s.preflight("https://chiomasbags.com"); code != http.StatusNoContent {
		t.Errorf("after adding: %d, want 204", code)
	}
	for _, origin := range []string{"http://chiomasbags.com", "https://chiomasbags.com:8443", "https://www.chiomasbags.com", "https://chiomasbags.com.evil.com"} {
		if code := s.preflight(origin); code != http.StatusForbidden {
			t.Errorf("%s: %d, want 403", origin, code)
		}
	}

	if code, _ := s.do(http.MethodDelete, "/store/"+s.storeId+"/domains/chiomasbags.com", s.owner, nil); code != http.StatusOK {
		t.Fatalf("remove: %d", code)
	}
	if code := s.preflight("https://chiomasbags.com"); code != http.StatusForbidden {
		t.Errorf("after removing: %d, want 403", code)
	}
}

func TestStoreDomains_KeyMustMatchDomain(t *testing.T) {
	s := newShop(t)
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://*.huddle.ng,http://localhost:3000")
	s.addDomain("chiomasbags.com")

	cases := []struct {
		name, origin, key string
		want              int
	}{
		{"own domain", "https://chiomasbags.com", s.key, http.StatusOK},
		{"another store's key", "https://chiomasbags.com", s.otherKey, http.StatusForbidden},
		{"unknown site", "https://attacker.com", s.key, http.StatusForbidden},
		{"http version of the domain", "http://chiomasbags.com", s.key, http.StatusForbidden},
		{"Huddle subdomain", "https://tunde.huddle.ng", s.otherKey, http.StatusOK},
		{"listed dev origin", "http://localhost:3000", s.otherKey, http.StatusOK},
		{"same origin (Swagger UI)", "http://example.com", s.key, http.StatusOK},
		{"no origin (curl)", "", s.otherKey, http.StatusOK},
	}
	for _, tc := range cases {
		headers := map[string]string{"X-Store-Key": tc.key}
		if tc.origin != "" {
			headers["Origin"] = tc.origin
		}
		for _, req := range []struct{ method, path string }{
			{http.MethodGet, "/storefront/products"},
			{http.MethodGet, "/cart"},
		} {
			if w := s.request(req.method, req.path, headers, nil, nil); w.Code != tc.want {
				t.Errorf("%s: %s %s = %d, want %d", tc.name, req.method, req.path, w.Code, tc.want)
			}
		}
	}
}
