package router

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestSwaggerMatchesRoutes fails when a route is added or removed without
// updating doc/swagger.json.
func TestSwaggerMatchesRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := RoutesSetup(nil, nil) // registering routes does not touch the DB or Redis

	param := regexp.MustCompile(`:(\w+)`)
	inCode := map[string]bool{}
	for _, route := range r.Routes() {
		if route.Path == "/" || strings.HasPrefix(route.Path, "/swagger") || strings.HasPrefix(route.Path, "/doc") {
			continue
		}
		inCode[route.Method+" "+param.ReplaceAllString(route.Path, "{$1}")] = true
	}

	raw, err := os.ReadFile(filepath.Join("..", "..", "doc", "swagger.json"))
	if err != nil {
		t.Fatal(err)
	}
	var spec struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatalf("doc/swagger.json is not valid JSON: %v", err)
	}
	inDocs := map[string]bool{}
	for path, ops := range spec.Paths {
		for method := range ops {
			inDocs[strings.ToUpper(method)+" "+path] = true
		}
	}

	for route := range inCode {
		if !inDocs[route] {
			t.Errorf("route %s is not documented in doc/swagger.json", route)
		}
	}
	for route := range inDocs {
		if !inCode[route] {
			t.Errorf("doc/swagger.json documents %s, which no longer exists", route)
		}
	}
}
