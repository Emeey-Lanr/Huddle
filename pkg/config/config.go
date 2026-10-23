// Package config checks the API's settings (environment variables) at
// startup, so a typo or a missing secret stops it with a clear message
// instead of misbehaving later.
package config

import (
	"fmt"
	"net"
	"slices"
	"strconv"
	"strings"

	"github.com/shopspring/decimal"
)

const (
	Production  = "production"
	Development = "development"

	defaultPort = "8080"

	// minProductionSecretLen keeps production tokens from being signed with a guessable key.
	minProductionSecretLen = 32
)

var sslModes = []string{"disable", "allow", "prefer", "require", "verify-ca", "verify-full"}

// IsProduction reports whether APP_ENV is "production".
func IsProduction(getenv func(string) string) bool {
	return getenv("APP_ENV") == Production
}

// Port is the port the API listens on: PORT, or 8080.
func Port(getenv func(string) string) string {
	if p := strings.TrimSpace(getenv("PORT")); p != "" {
		return p
	}
	return defaultPort
}

// TrustedProxies lists the proxies (IPs or CIDRs, from TRUSTED_PROXIES) whose
// X-Forwarded-For header is believed. Empty means none: the client IP is the
// address that connected, so clients can't fake it to dodge the rate limiter.
func TrustedProxies(getenv func(string) string) []string {
	var proxies []string
	for _, p := range strings.Split(getenv("TRUSTED_PROXIES"), ",") {
		if p = strings.TrimSpace(p); p != "" {
			proxies = append(proxies, p)
		}
	}
	return proxies
}

// Check returns problems that must stop the API from starting, and warnings
// worth logging. Messages name settings but never include secret values.
func Check(getenv func(string) string) (problems, warnings []string) {
	fail := func(format string, a ...any) { problems = append(problems, fmt.Sprintf(format, a...)) }
	warn := func(format string, a ...any) { warnings = append(warnings, fmt.Sprintf(format, a...)) }

	env := getenv("APP_ENV")
	if env != "" && env != Production && env != Development {
		fail("APP_ENV must be %q or %q (got %q)", Production, Development, env)
	}
	prod := env == Production

	if p, err := strconv.Atoi(Port(getenv)); err != nil || p < 1 || p > 65535 {
		fail("PORT must be a number from 1 to 65535")
	}

	secret := getenv("SECRET_KEY")
	switch {
	case secret == "":
		fail("SECRET_KEY must be set to sign auth tokens")
	case prod && len(secret) < minProductionSecretLen:
		fail("SECRET_KEY must be at least %d characters in production", minProductionSecretLen)
	}

	for _, key := range []string{"DB_HOST", "DB_USER", "DB_NAME"} {
		if getenv(key) == "" {
			fail("%s must be set", key)
		}
	}
	mode := getenv("DB_SSLMODE")
	if mode != "" && !slices.Contains(sslModes, mode) {
		fail("DB_SSLMODE must be one of %s", strings.Join(sslModes, ", "))
	}
	if prod && (mode == "" || mode == "disable") {
		warn("DB_SSLMODE is disable: use require (or verify-full) unless Postgres is on the same private network")
	}

	if v := getenv("ORDER_HOLD_MINUTES"); v != "" {
		if m, err := strconv.Atoi(v); err != nil || m <= 0 {
			fail("ORDER_HOLD_MINUTES must be a whole number of minutes above 0")
		}
	}
	if v := getenv("BACHS_PLATFORM_FEE_PERCENT"); v != "" {
		if pct, err := decimal.NewFromString(v); err != nil || pct.IsNegative() || pct.GreaterThanOrEqual(decimal.NewFromInt(100)) {
			fail("BACHS_PLATFORM_FEE_PERCENT must be a number from 0 to below 100")
		}
	}

	proxies := TrustedProxies(getenv)
	for _, p := range proxies {
		if net.ParseIP(p) == nil {
			if _, _, err := net.ParseCIDR(p); err != nil {
				fail("TRUSTED_PROXIES must be IP addresses or ranges like 10.0.0.0/8, comma-separated")
				break
			}
		}
	}

	if prod {
		if getenv("BACHS_API_KEY") == "" || getenv("BACHS_WEBHOOK_SECRET") == "" {
			fail("BACHS_API_KEY and BACHS_WEBHOOK_SECRET must be set in production: shoppers can't pay without them")
		}
		if len(proxies) == 0 {
			warn("TRUSTED_PROXIES is not set: behind a load balancer or proxy every client shares its IP, so the rate limiter blocks them together; set it to the proxy's address")
		}
		if getenv("CORS_ALLOWED_ORIGINS") == "" {
			warn("CORS_ALLOWED_ORIGINS is not set: only stores' custom domains can call the API from a browser (add e.g. https://*.huddle.ng)")
		}
		if getenv("SMTP_HOST") == "" || getenv("SMTP_FROM") == "" {
			warn("SMTP_HOST or SMTP_FROM is not set: sellers get no order emails")
		}
		if getenv("CLOUDINARY_URL") == "" {
			warn("CLOUDINARY_URL is not set: product image uploads will fail")
		}
	}
	return problems, warnings
}
