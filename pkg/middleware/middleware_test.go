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

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Setenv("SECRET_KEY", "test-secret")
	os.Exit(m.Run())
}

func authRouter() *gin.Engine {
	r := gin.New()
	r.GET("/admin", AuthMiddleware, func(c *gin.Context) {
		c.String(http.StatusOK, c.GetString("adminId"))
	})
	r.GET("/customer", CustomerAuthMiddleware, func(c *gin.Context) {
		c.String(http.StatusOK, c.GetString("customerId"))
	})
	return r
}

func doAuthPath(path, header string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if header != "" {
		req.Header.Set("Authorization", header)
	}
	w := httptest.NewRecorder()
	authRouter().ServeHTTP(w, req)
	return w
}

func doAuth(header string) *httptest.ResponseRecorder { return doAuthPath("/admin", header) }

func token(t *testing.T, id uuid.UUID, role string) string {
	t.Helper()
	tok, err := utility.CreateToken(id, "a@b.com", role)
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}
	return "Bearer " + tok
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

func TestAuthMiddleware_ValidAdminToken(t *testing.T) {
	id := uuid.New()
	w := doAuth(token(t, id, utility.RoleAdmin))
	if w.Code != http.StatusOK || w.Body.String() != id.String() {
		t.Errorf("status = %d body = %q", w.Code, w.Body.String())
	}
}

func TestAuthMiddleware_CustomerTokenRejected(t *testing.T) {
	if w := doAuth(token(t, uuid.New(), utility.RoleCustomer)); w.Code != http.StatusForbidden {
		t.Errorf("customer token on admin route: status = %d, want 403", w.Code)
	}
}

func TestCustomerAuthMiddleware(t *testing.T) {
	id := uuid.New()
	w := doAuthPath("/customer", token(t, id, utility.RoleCustomer))
	if w.Code != http.StatusOK || w.Body.String() != id.String() {
		t.Errorf("customer token: status = %d body = %q", w.Code, w.Body.String())
	}
	if w := doAuthPath("/customer", token(t, uuid.New(), utility.RoleAdmin)); w.Code != http.StatusForbidden {
		t.Errorf("admin token on customer route: status = %d, want 403", w.Code)
	}
}

func TestAuthMiddleware_ForgedTokenWithEmptyKeyRejected(t *testing.T) {
	claims := jwt.MapClaims{
		"user_id": uuid.NewString(),
		"email":   "attacker@example.com",
		"role":    utility.RoleAdmin,
		"exp":     time.Now().Add(time.Hour).Unix(),
	}
	forged, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(""))
	if w := doAuth("Bearer " + forged); w.Code != http.StatusUnauthorized {
		t.Errorf("forged token accepted: status = %d", w.Code)
	}
}

func TestAuthMiddleware_TokenWithoutRoleRejected(t *testing.T) {
	claims := jwt.MapClaims{"user_id": uuid.NewString(), "exp": time.Now().Add(time.Hour).Unix()}
	old, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("test-secret"))
	if w := doAuth("Bearer " + old); w.Code != http.StatusForbidden {
		t.Errorf("token without role: status = %d, want 403", w.Code)
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
