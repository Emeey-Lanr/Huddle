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
