package router

import (
	"net/http"
	"strings"
	"testing"
)

func TestCORS_PreflightFromAllowedOrigin(t *testing.T) {
	s := newShop(t)
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://chiomasbags.com,https://*.huddle.ng")

	for _, path := range []string{"/cart", "/cart/items/" + s.bag, "/storefront/products"} {
		w := s.request(http.MethodOptions, path, map[string]string{
			"Origin":                         "https://shop.huddle.ng",
			"Access-Control-Request-Method":  "POST",
			"Access-Control-Request-Headers": "content-type, x-store-key",
		}, nil, nil)

		h := w.Header()
		if w.Code != http.StatusNoContent {
			t.Errorf("%s: status %d, want 204", path, w.Code)
		}
		if h.Get("Access-Control-Allow-Origin") != "https://shop.huddle.ng" || h.Get("Access-Control-Allow-Credentials") != "true" {
			t.Errorf("%s: allow-origin=%q credentials=%q", path, h.Get("Access-Control-Allow-Origin"), h.Get("Access-Control-Allow-Credentials"))
		}
		if !strings.Contains(h.Get("Access-Control-Allow-Headers"), "X-Store-Key") {
			t.Errorf("%s: allow-headers %q missing X-Store-Key", path, h.Get("Access-Control-Allow-Headers"))
		}
	}
}

func TestCORS_PreflightFromOtherOriginRejected(t *testing.T) {
	s := newShop(t)
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://chiomasbags.com")

	w := s.request(http.MethodOptions, "/cart", map[string]string{
		"Origin": "https://attacker.com", "Access-Control-Request-Method": "POST",
	}, nil, nil)
	if w.Code != http.StatusForbidden || w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("status %d allow-origin %q, want 403 and none", w.Code, w.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestCORS_ActualRequestGetsHeaders(t *testing.T) {
	s := newShop(t)
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://chiomasbags.com")

	w := s.request(http.MethodGet, "/storefront/products", map[string]string{"Origin": "https://chiomasbags.com", "X-Store-Key": s.key}, nil, nil)
	if w.Code != http.StatusOK || w.Header().Get("Access-Control-Allow-Origin") != "https://chiomasbags.com" {
		t.Errorf("status %d allow-origin %q", w.Code, w.Header().Get("Access-Control-Allow-Origin"))
	}

	// Requests without an Origin (curl, server-to-server) are unaffected.
	w = s.request(http.MethodGet, "/storefront/products", map[string]string{"X-Store-Key": s.key}, nil, nil)
	if w.Code != http.StatusOK || w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("no origin: status %d allow-origin %q", w.Code, w.Header().Get("Access-Control-Allow-Origin"))
	}
}
