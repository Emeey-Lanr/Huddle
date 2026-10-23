package config

import (
	"strings"
	"testing"
)

// env returns a getenv over a valid development setup, with overrides.
func env(overrides map[string]string) func(string) string {
	base := map[string]string{
		"SECRET_KEY": "dev-secret", "DB_HOST": "localhost", "DB_USER": "huddle", "DB_NAME": "huddle",
	}
	for k, v := range overrides {
		base[k] = v
	}
	return func(key string) string { return base[key] }
}

// prodEnv is a complete production setup.
func prodEnv(overrides map[string]string) func(string) string {
	base := map[string]string{
		"APP_ENV": "production", "SECRET_KEY": strings.Repeat("s", 40), "DB_SSLMODE": "require",
		"BACHS_API_KEY": "sk", "BACHS_WEBHOOK_SECRET": "whsec", "TRUSTED_PROXIES": "10.0.0.0/8",
		"CORS_ALLOWED_ORIGINS": "https://*.huddle.ng", "SMTP_HOST": "smtp.example.com", "SMTP_FROM": "Huddle <orders@huddle.ng>",
		"CLOUDINARY_URL": "cloudinary://k:s@cloud",
	}
	for k, v := range overrides {
		base[k] = v
	}
	return env(base)
}

func hasMessage(list []string, part string) bool {
	for _, m := range list {
		if strings.Contains(m, part) {
			return true
		}
	}
	return false
}

func TestCheck_ValidSetups(t *testing.T) {
	for name, getenv := range map[string]func(string) string{"development": env(nil), "production": prodEnv(nil)} {
		problems, warnings := Check(getenv)
		if len(problems) != 0 || len(warnings) != 0 {
			t.Errorf("%s: problems %v, warnings %v; want none", name, problems, warnings)
		}
	}
}

func TestCheck_Problems(t *testing.T) {
	cases := []struct {
		name   string
		getenv func(string) string
		want   string
	}{
		{"APP_ENV typo", env(map[string]string{"APP_ENV": "prod"}), "APP_ENV"},
		{"port not a number", env(map[string]string{"PORT": "eighty"}), "PORT"},
		{"port out of range", env(map[string]string{"PORT": "70000"}), "PORT"},
		{"no secret", env(map[string]string{"SECRET_KEY": ""}), "SECRET_KEY"},
		{"short production secret", prodEnv(map[string]string{"SECRET_KEY": "short"}), "SECRET_KEY"},
		{"no database host", env(map[string]string{"DB_HOST": ""}), "DB_HOST"},
		{"bad ssl mode", env(map[string]string{"DB_SSLMODE": "on"}), "DB_SSLMODE"},
		{"hold minutes not a number", env(map[string]string{"ORDER_HOLD_MINUTES": "half an hour"}), "ORDER_HOLD_MINUTES"},
		{"hold minutes zero", env(map[string]string{"ORDER_HOLD_MINUTES": "0"}), "ORDER_HOLD_MINUTES"},
		{"fee not a number", env(map[string]string{"BACHS_PLATFORM_FEE_PERCENT": "1.5%"}), "BACHS_PLATFORM_FEE_PERCENT"},
		{"fee too high", env(map[string]string{"BACHS_PLATFORM_FEE_PERCENT": "100"}), "BACHS_PLATFORM_FEE_PERCENT"},
		{"bad proxy", env(map[string]string{"TRUSTED_PROXIES": "10.0.0.1, load-balancer"}), "TRUSTED_PROXIES"},
		{"production without Bachs", prodEnv(map[string]string{"BACHS_WEBHOOK_SECRET": ""}), "BACHS_API_KEY"},
	}
	for _, tc := range cases {
		problems, _ := Check(tc.getenv)
		if !hasMessage(problems, tc.want) {
			t.Errorf("%s: problems %v, want one about %s", tc.name, problems, tc.want)
		}
	}
}

func TestCheck_ProductionWarnings(t *testing.T) {
	for _, key := range []string{"DB_SSLMODE", "TRUSTED_PROXIES", "CORS_ALLOWED_ORIGINS", "SMTP_HOST", "CLOUDINARY_URL"} {
		problems, warnings := Check(prodEnv(map[string]string{key: ""}))
		if len(problems) != 0 || !hasMessage(warnings, key) {
			t.Errorf("without %s: problems %v, warnings %v; want only a warning about it", key, problems, warnings)
		}
	}
	// Outside production these are fine to leave out.
	if _, warnings := Check(env(nil)); len(warnings) != 0 {
		t.Errorf("development warnings: %v", warnings)
	}
}

func TestCheck_NeverShowsSecrets(t *testing.T) {
	secret := "super-secret-value-123"
	problems, warnings := Check(prodEnv(map[string]string{"SECRET_KEY": secret[:10], "BACHS_API_KEY": secret, "BACHS_WEBHOOK_SECRET": ""}))
	for _, m := range append(problems, warnings...) {
		if strings.Contains(m, secret[:10]) {
			t.Errorf("message shows a secret: %q", m)
		}
	}
}

func TestPortAndProxies(t *testing.T) {
	if p := Port(env(nil)); p != "8080" {
		t.Errorf("default port %q", p)
	}
	if p := Port(env(map[string]string{"PORT": " 3000 "})); p != "3000" {
		t.Errorf("PORT 3000 gave %q", p)
	}
	if got := TrustedProxies(env(map[string]string{"TRUSTED_PROXIES": " 10.0.0.1, ,192.168.0.0/16 "})); strings.Join(got, "|") != "10.0.0.1|192.168.0.0/16" {
		t.Errorf("proxies %v", got)
	}
	if got := TrustedProxies(env(nil)); got != nil {
		t.Errorf("no proxies: %v", got)
	}
}
