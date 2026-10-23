package storedomain

import (
	"testing"
	"time"

	"huddle/pkg/model"
	"huddle/pkg/testutil"

	"github.com/google/uuid"
)

func TestOriginAllowed(t *testing.T) {
	list := "https://chiomasbags.com, https://*.huddle.ng"
	cases := map[string]bool{
		"https://chiomasbags.com":          true,
		"https://shop.huddle.ng":           true,
		"https://a.b.huddle.ng":            true,
		"https://huddle.ng":                false, // wildcard needs a subdomain
		"https://evilhuddle.ng":            false,
		"https://shop.huddle.ng.evil.com":  false,
		"http://shop.huddle.ng":            false, // scheme must match
		"https://chiomasbags.com.evil.com": false,
		"https://attacker.com":             false,
		"":                                 false,
	}
	for origin, want := range cases {
		if got := OriginAllowed(origin, list); got != want {
			t.Errorf("OriginAllowed(%q) = %v, want %v", origin, got, want)
		}
	}
	if OriginAllowed("https://chiomasbags.com", "") {
		t.Error("empty allow list allowed an origin")
	}
}

func TestNormalize(t *testing.T) {
	valid := map[string]string{
		"chiomasbags.com":                "chiomasbags.com",
		"  ChiomasBags.COM  ":            "chiomasbags.com",
		"https://chiomasbags.com":        "chiomasbags.com",
		"https://www.chiomasbags.com/":   "www.chiomasbags.com",
		"http://shop.chiomasbags.com.ng": "shop.chiomasbags.com.ng",
		"chiomasbags.com.":               "chiomasbags.com",
		"xn--bcher-kva.example":          "xn--bcher-kva.example",
		"my-shop.example.xn--p1ai":       "my-shop.example.xn--p1ai",
	}
	for in, want := range valid {
		if got, err := Normalize(in); err != nil || got != want {
			t.Errorf("Normalize(%q) = %q, %v; want %q", in, got, err, want)
		}
	}

	invalid := []string{
		"", "chiomasbags", "localhost", "shop.localhost", "chiomasbags.com/shop", "https://chiomasbags.com/shop",
		"https://chiomasbags.com:8443", "chiomasbags.com:8443", "ftp://chiomasbags.com", "https://user@chiomasbags.com",
		"https://chiomasbags.com?x=1", "192.168.0.1", "https://[::1]", "-shop.com", "shop-.com", "shop..com",
		"chioma bags.com", "bücher.example", "chiomasbags.c0m", "*.chiomasbags.com",
	}
	for _, in := range invalid {
		if got, err := Normalize(in); err == nil {
			t.Errorf("Normalize(%q) = %q, want an error", in, got)
		}
	}
}

func TestHTTPSHost(t *testing.T) {
	cases := map[string]string{
		"https://chiomasbags.com":      "chiomasbags.com",
		"https://ChiomasBags.com":      "chiomasbags.com",
		"http://chiomasbags.com":       "", // custom domains are https only
		"https://chiomasbags.com:8443": "",
		"https://chiomasbags.com/x":    "",
		"https://":                     "",
		"null":                         "",
	}
	for origin, want := range cases {
		got, ok := HTTPSHost(origin)
		if got != want || ok != (want != "") {
			t.Errorf("HTTPSHost(%q) = %q, %v; want %q", origin, got, ok, want)
		}
	}
}

func TestUnderWildcard(t *testing.T) {
	list := "https://chiomasbags.com, https://*.huddle.ng, http://localhost:3000"
	cases := map[string]bool{
		"shop.huddle.ng":  true,
		"huddle.ng":       false,
		"chiomasbags.com": false, // an exact entry is not a wildcard
		"tundetech.com":   false,
	}
	for domain, want := range cases {
		if got := UnderWildcard(domain, list); got != want {
			t.Errorf("UnderWildcard(%q) = %v, want %v", domain, got, want)
		}
	}
}

func TestCache_RemembersUntilForgotten(t *testing.T) {
	db := testutil.NewDB(t)
	store := testutil.SeedStore(t, db, testutil.SeedAdmin(t, db).Id)
	cache := NewCache(db, time.Hour)

	if id, err := cache.StoreFor("chiomasbags.com"); err != nil || id != uuid.Nil {
		t.Fatalf("before adding: %v, %v; want no store", id, err)
	}

	// Added without Forget (as another server would): the cached answer stays.
	if err := db.Create(&model.StoreDomains{Id: uuid.New(), StoreId: store.ID, Domain: "chiomasbags.com"}).Error; err != nil {
		t.Fatal(err)
	}
	if id, _ := cache.StoreFor("chiomasbags.com"); id != uuid.Nil {
		t.Errorf("cached 'no store' was not kept: %v", id)
	}

	cache.Forget("chiomasbags.com")
	if id, err := cache.StoreFor("chiomasbags.com"); err != nil || id != store.ID {
		t.Errorf("after Forget: %v, %v; want %v", id, err, store.ID)
	}

	// An expired answer is looked up again.
	short := NewCache(db, time.Millisecond)
	short.StoreFor("tundetech.com")
	db.Create(&model.StoreDomains{Id: uuid.New(), StoreId: store.ID, Domain: "tundetech.com"})
	time.Sleep(5 * time.Millisecond)
	if id, _ := short.StoreFor("tundetech.com"); id != store.ID {
		t.Errorf("expired entry not refreshed: %v", id)
	}
}
