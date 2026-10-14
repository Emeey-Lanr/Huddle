package router

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"huddle/pkg/testutil"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// server is the full Huddle router backed by a fresh Postgres schema and an
// in-memory Redis.
type server struct {
	t  *testing.T
	r  *gin.Engine
	db *gorm.DB
}

func newServer(t *testing.T) *server {
	t.Helper()
	gin.SetMode(gin.TestMode)
	t.Setenv("SECRET_KEY", "test-secret")
	db := testutil.NewDB(t)
	_, rdb := testutil.NewRedis(t)
	return &server{t: t, r: RoutesSetup(db, rdb), db: db}
}

type envelope struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
	Error   string          `json:"error"`
}

// do sends a JSON request; token may be empty. It returns the status and envelope.
func (s *server) do(method, path, token string, body any) (int, envelope) {
	s.t.Helper()
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	s.r.ServeHTTP(w, req)

	var env envelope
	json.Unmarshal(w.Body.Bytes(), &env)
	return w.Code, env
}

// data decodes the envelope's data field into v.
func (s *server) data(env envelope, v any) {
	s.t.Helper()
	if err := json.Unmarshal(env.Data, v); err != nil {
		s.t.Fatalf("decode data %s: %v", env.Data, err)
	}
}

// signupAdmin registers an admin through the API and returns its token.
func (s *server) signupAdmin(username string) string {
	s.t.Helper()
	code, env := s.do(http.MethodPost, "/admin/signup", "", map[string]string{
		"name": "Seller " + username, "username": username, "email": username + "@example.com", "password": "secret123",
	})
	if code != http.StatusCreated {
		s.t.Fatalf("signup %s: %d %s %s", username, code, env.Message, env.Error)
	}
	var d struct{ Token string }
	s.data(env, &d)
	return d.Token
}

// createStore creates a store through the API and returns its public id.
func (s *server) createStore(token, name string) string {
	s.t.Helper()
	code, env := s.do(http.MethodPost, "/store/create", token, map[string]string{
		"store_name": name, "store_description": "Handmade goods from Lagos",
	})
	if code != http.StatusCreated {
		s.t.Fatalf("create store %s: %d %s %s", name, code, env.Message, env.Error)
	}
	var d struct {
		StoreId string `json:"store_id"`
	}
	s.data(env, &d)
	return d.StoreId
}
