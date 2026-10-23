package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"huddle/pkg/testutil"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func healthz(t *testing.T, r *gin.Engine) (int, map[string]string) {
	t.Helper()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", w.Header().Get("Cache-Control"))
	}
	var checks map[string]string
	decode(t, w, &checks)
	return w.Code, checks
}

func TestHealthz_Healthy(t *testing.T) {
	s := newServer(t)
	code, checks := healthz(t, s.r)
	if code != http.StatusOK || checks["postgres"] != "ok" || checks["redis"] != "ok" {
		t.Errorf("healthz: %d %v, want 200 with both ok", code, checks)
	}
}

func TestHealthz_RedisDown(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mr, rdb := testutil.NewRedis(t)
	r := RoutesSetup(testutil.NewDB(t), rdb)
	mr.Close()

	code, checks := healthz(t, r)
	if code != http.StatusServiceUnavailable || checks["postgres"] != "ok" || checks["redis"] != "down" {
		t.Errorf("healthz: %d %v, want 503 with redis down", code, checks)
	}
}

func TestHealthz_PostgresDown(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// Nothing listens on port 1.
	db, err := gorm.Open(postgres.Open("postgres://huddle:x@127.0.0.1:1/huddle?sslmode=disable&connect_timeout=1"), &gorm.Config{DisableAutomaticPing: true})
	if err != nil {
		t.Fatal(err)
	}
	_, rdb := testutil.NewRedis(t)
	r := RoutesSetup(db, rdb)

	code, checks := healthz(t, r)
	if code != http.StatusServiceUnavailable || checks["postgres"] != "down" || checks["redis"] != "ok" {
		t.Errorf("healthz: %d %v, want 503 with postgres down", code, checks)
	}
}

func clientIP(t *testing.T, proxies, remoteAddr, forwardedFor string) string {
	t.Helper()
	t.Setenv("TRUSTED_PROXIES", proxies)
	gin.SetMode(gin.TestMode)
	r := RoutesSetup(nil, nil)
	r.GET("/test-client-ip", func(c *gin.Context) { c.String(http.StatusOK, c.ClientIP()) })

	req := httptest.NewRequest(http.MethodGet, "/test-client-ip", nil)
	req.RemoteAddr = remoteAddr
	req.Header.Set("X-Forwarded-For", forwardedFor)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Body.String()
}

func TestClientIP_OnlyTrustedProxiesMayForward(t *testing.T) {
	// Without TRUSTED_PROXIES a client can't pick its own IP (e.g. to dodge the rate limiter).
	if ip := clientIP(t, "", "203.0.113.7:5000", "1.2.3.4"); ip != "203.0.113.7" {
		t.Errorf("no trusted proxies: client IP %q, want the connecting address", ip)
	}
	// Behind a trusted load balancer, the forwarded address is the client's.
	if ip := clientIP(t, "10.0.0.0/8", "10.0.0.5:5000", "203.0.113.7"); ip != "203.0.113.7" {
		t.Errorf("trusted proxy: client IP %q, want the forwarded address", ip)
	}
	// Someone else forwarding is still not believed.
	if ip := clientIP(t, "10.0.0.0/8", "198.51.100.9:5000", "1.2.3.4"); ip != "198.51.100.9" {
		t.Errorf("untrusted sender: client IP %q, want the connecting address", ip)
	}
}
