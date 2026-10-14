package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"huddle/pkg/utility"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func init() { gin.SetMode(gin.TestMode) }

// knownBug skips a test that documents an existing defect.
// Run with HUDDLE_KNOWN_BUGS=1 to see them fail until fixed.
func knownBug(t *testing.T, why string) {
	t.Helper()
	if os.Getenv("HUDDLE_KNOWN_BUGS") != "1" {
		t.Skip("known bug: " + why)
	}
}

func authRouter() *gin.Engine {
	r := gin.New()
	r.GET("/p", AuthMiddleware, func(c *gin.Context) {
		id, _ := c.Get("adminId")
		c.String(http.StatusOK, id.(string))
	})
	return r
}

func doAuth(header string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/p", nil)
	if header != "" {
		req.Header.Set("Authorization", header)
	}
	w := httptest.NewRecorder()
	authRouter().ServeHTTP(w, req)
	return w
}

func TestAuthMiddleware_MissingHeader(t *testing.T) {
	if w := doAuth(""); w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestAuthMiddleware_BadFormat(t *testing.T) {
	for _, h := range []string{"Token abc", "Bearer", "Bearer a b"} {
		if w := doAuth(h); w.Code != http.StatusUnauthorized {
			t.Errorf("%q: status = %d, want 401", h, w.Code)
		}
	}
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	if w := doAuth("Bearer not.a.jwt"); w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestAuthMiddleware_ValidToken(t *testing.T) {
	id := uuid.New()
	token, err := utility.CreateToken(id, "a@b.com")
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}
	w := doAuth("Bearer " + token)
	if w.Code != http.StatusOK || w.Body.String() != id.String() {
		t.Errorf("status = %d body = %q", w.Code, w.Body.String())
	}
}

// SECRET_KEY is read into a package variable before main() calls godotenv.Load(),
// so unless it is set in the OS environment, tokens are signed with an empty key
// and anyone can forge one.
func TestAuthMiddleware_ForgedTokenWithEmptyKeyRejected(t *testing.T) {
	knownBug(t, "jwt secret is empty when SECRET_KEY only comes from .env")
	claims := jwt.MapClaims{
		"admin_id": uuid.NewString(),
		"email":    "attacker@example.com",
		"exp":      time.Now().Add(time.Hour).Unix(),
	}
	forged, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(""))
	if w := doAuth("Bearer " + forged); w.Code != http.StatusUnauthorized {
		t.Errorf("forged token accepted: status = %d", w.Code)
	}
}

func TestStoreContextMiddleware(t *testing.T) {
	r := gin.New()
	r.GET("/s", StoreContextMiddleware, func(c *gin.Context) {
		v, _ := c.Get("X-Store-Id")
		c.String(http.StatusOK, v.(string))
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/s", nil))
	if w.Code != http.StatusUnauthorized {
		t.Errorf("missing header: status = %d, want 401", w.Code)
	}

	req := httptest.NewRequest(http.MethodGet, "/s", nil)
	req.Header.Set("X-Store-Id", "str-123")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK || w.Body.String() != "str-123" {
		t.Errorf("status = %d body = %q", w.Code, w.Body.String())
	}
}

func TestRateLimiter_BlocksAfterLimit(t *testing.T) {
	r := gin.New()
	r.GET("/r", RateLimiter, func(c *gin.Context) { c.Status(http.StatusOK) })

	hit := func(ip string) int {
		req := httptest.NewRequest(http.MethodGet, "/r", nil)
		req.RemoteAddr = ip + ":1234"
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w.Code
	}

	for i := 0; i < defaultLimiter.maxRequests; i++ {
		if code := hit("10.0.0.1"); code != http.StatusOK {
			t.Fatalf("request %d: status = %d, want 200", i+1, code)
		}
	}
	if code := hit("10.0.0.1"); code != http.StatusTooManyRequests {
		t.Errorf("over limit: status = %d, want 429", code)
	}
	if code := hit("10.0.0.2"); code != http.StatusOK {
		t.Errorf("other client: status = %d, want 200", code)
	}
}
