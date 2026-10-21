package response

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() { gin.SetMode(gin.TestMode) }

func run(fn func(c *gin.Context)) (*httptest.ResponseRecorder, ResponseEnvelope) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	fn(c)
	var env ResponseEnvelope
	_ = json.Unmarshal(w.Body.Bytes(), &env)
	return w, env
}

func TestSuccess(t *testing.T) {
	w, env := run(func(c *gin.Context) { Success(c, "ok", gin.H{"a": 1}) })
	if w.Code != http.StatusOK || !env.Success || env.Message != "ok" || env.Data == nil {
		t.Errorf("unexpected response %d %+v", w.Code, env)
	}
}

func TestCreated(t *testing.T) {
	w, env := run(func(c *gin.Context) { Created(c, "made", nil) })
	if w.Code != http.StatusCreated || !env.Success {
		t.Errorf("unexpected response %d %+v", w.Code, env)
	}
}

func TestError(t *testing.T) {
	w, env := run(func(c *gin.Context) { Error(c, http.StatusBadRequest, "bad", errors.New("boom")) })
	if w.Code != http.StatusBadRequest || env.Success || env.Error != "boom" {
		t.Errorf("unexpected response %d %+v", w.Code, env)
	}
}

func TestError_NilErr(t *testing.T) {
	_, env := run(func(c *gin.Context) { Error(c, http.StatusNotFound, "missing", nil) })
	if env.Error != "" {
		t.Errorf("Error = %q, want empty", env.Error)
	}
}
