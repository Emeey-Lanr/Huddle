package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func cartCookie(t *testing.T) *http.Cookie {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	setCartCookie(c, "cart_abc")

	cookies := w.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "cart_session" || cookies[0].Value != "cart_abc" {
		t.Fatalf("cookies = %+v", cookies)
	}
	return cookies[0]
}

func TestCartCookie_Production(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	ck := cartCookie(t)
	if !ck.Secure || ck.SameSite != http.SameSiteNoneMode || !ck.HttpOnly {
		t.Errorf("secure=%v samesite=%v httponly=%v, want Secure, SameSite=None, HttpOnly", ck.Secure, ck.SameSite, ck.HttpOnly)
	}
	if ck.MaxAge != 7*24*3600 || ck.Path != "/" {
		t.Errorf("max-age=%d path=%q", ck.MaxAge, ck.Path)
	}
}

func TestCartCookie_Development(t *testing.T) {
	t.Setenv("APP_ENV", "")
	ck := cartCookie(t)
	if ck.Secure || ck.SameSite != http.SameSiteLaxMode || !ck.HttpOnly {
		t.Errorf("secure=%v samesite=%v httponly=%v, want not Secure, SameSite=Lax, HttpOnly", ck.Secure, ck.SameSite, ck.HttpOnly)
	}
}
