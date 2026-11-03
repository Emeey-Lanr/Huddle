package router

import (
	"net/http"
	"testing"
)

func (s shop) atHost(host string, extra map[string]string) int {
	headers := map[string]string{"X-Store-Host": host}
	for k, v := range extra {
		headers[k] = v
	}
	return s.request(http.MethodGet, "/storefront/store", headers, nil, nil).Code
}

func TestStoreAddress_NewStoreGetsSlugFromName(t *testing.T) {
	s := newShop(t)
	_, env := s.do(http.MethodGet, "/store/"+s.storeId, s.owner, nil)
	var store struct {
		Slug string `json:"slug"`
	}
	s.data(env, &store)
	if store.Slug != "chiomas-bags" {
		t.Errorf("slug = %q, want chiomas-bags", store.Slug)
	}

	// A second store with the same name still gets its own address.
	other := s.createStore(s.otherOwner, "Chioma's Bags")
	_, env = s.do(http.MethodGet, "/store/"+other, s.otherOwner, nil)
	var second struct {
		Slug string `json:"slug"`
	}
	s.data(env, &second)
	if second.Slug == "" || second.Slug == "chiomas-bags" {
		t.Errorf("second store's slug = %q", second.Slug)
	}
}

func TestStoreAddress_HostFindsTheStore(t *testing.T) {
	s := newShop(t)
	t.Setenv("STOREFRONT_BASE_DOMAIN", "huddle.ng")

	cases := []struct {
		host string
		want int
	}{
		{"chiomas-bags.huddle.ng", http.StatusOK},
		{"Chiomas-Bags.HUDDLE.ng:4321", http.StatusOK},
		{"nobody.huddle.ng", http.StatusNotFound},
		{"a.chiomas-bags.huddle.ng", http.StatusNotFound},
		{"chiomas-bags.evil.ng", http.StatusNotFound},
		{"huddle.ng", http.StatusNotFound},
	}
	for _, tc := range cases {
		if got := s.atHost(tc.host, nil); got != tc.want {
			t.Errorf("%s: %d, want %d", tc.host, got, tc.want)
		}
	}

	var store struct {
		StoreName string `json:"store_name"`
	}
	decode(t, s.request(http.MethodGet, "/storefront/store", map[string]string{"X-Store-Host": "chiomas-bags.huddle.ng"}, nil, nil), &store)
	if store.StoreName != "Chioma's Bags" {
		t.Errorf("found %q", store.StoreName)
	}
}

func TestStoreAddress_CustomDomainFindsTheStore(t *testing.T) {
	s := newShop(t)
	s.addDomain("chiomasbags.com")
	if got := s.atHost("chiomasbags.com", nil); got != http.StatusOK {
		t.Errorf("custom domain: %d, want 200", got)
	}
	if got := s.atHost("chiomas-bags.huddle.ng", nil); got != http.StatusNotFound {
		t.Errorf("no base domain set: %d, want 404", got)
	}
}

// A page on one store's custom domain must not act for another store just by
// naming its host.
func TestStoreAddress_OtherStoresDomainCannotBorrowAHost(t *testing.T) {
	s := newShop(t)
	t.Setenv("STOREFRONT_BASE_DOMAIN", "huddle.ng")
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://*.huddle.ng")
	s.addDomain("chiomasbags.com")

	if got := s.atHost("tunde-tech.huddle.ng", map[string]string{"Origin": "https://chiomasbags.com"}); got != http.StatusForbidden {
		t.Errorf("another store's domain: %d, want 403", got)
	}
	if got := s.atHost("chiomas-bags.huddle.ng", map[string]string{"Origin": "https://attacker.com"}); got != http.StatusForbidden {
		t.Errorf("unknown site: %d, want 403", got)
	}
	if got := s.atHost("chiomas-bags.huddle.ng", map[string]string{"Origin": "https://chiomas-bags.huddle.ng"}); got != http.StatusOK {
		t.Errorf("the store's own address: %d, want 200", got)
	}
}

func TestStoreAddress_CartWorksByHost(t *testing.T) {
	s := newShop(t)
	t.Setenv("STOREFRONT_BASE_DOMAIN", "huddle.ng")
	w := s.request(http.MethodPost, "/cart", map[string]string{"X-Store-Host": "chiomas-bags.huddle.ng"}, nil,
		map[string]any{"product_id": s.bag, "quantity": 1})
	if w.Code != http.StatusOK && w.Code != http.StatusCreated {
		t.Errorf("add to cart: %d %s", w.Code, w.Body.String())
	}
}

func TestStoreAddress_SellerCanChangeIt(t *testing.T) {
	s := newShop(t)
	t.Setenv("STOREFRONT_BASE_DOMAIN", "huddle.ng")
	path := "/store/" + s.storeId

	if code, env := s.do(http.MethodPut, path, s.owner, map[string]any{"slug": "chioma"}); code != http.StatusOK {
		t.Fatalf("change: %d %s", code, env.Message)
	}
	if got := s.atHost("chioma.huddle.ng", nil); got != http.StatusOK {
		t.Errorf("new address: %d, want 200", got)
	}
	if got := s.atHost("chiomas-bags.huddle.ng", nil); got != http.StatusNotFound {
		t.Errorf("old address: %d, want 404", got)
	}

	if code, _ := s.do(http.MethodPut, "/store/"+s.storeId, s.owner, map[string]any{"slug": "chioma"}); code != http.StatusOK {
		t.Errorf("keeping the same address: %d, want 200", code)
	}

	bad := map[string]int{
		"tunde-tech": http.StatusConflict,
		"www":        http.StatusBadRequest,
		"Has Spaces": http.StatusBadRequest,
		"-dash":      http.StatusBadRequest,
		"dash-":      http.StatusBadRequest,
		"a.b":        http.StatusBadRequest,
		"this-name-is-far-too-long-for-a-store-address-honestly": http.StatusBadRequest,
	}
	for slug, want := range bad {
		if code, _ := s.do(http.MethodPut, path, s.owner, map[string]any{"slug": slug}); code != want {
			t.Errorf("%q: %d, want %d", slug, code, want)
		}
	}
}
