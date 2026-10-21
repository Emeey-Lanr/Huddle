package middleware

import "testing"

func TestOriginAllowed(t *testing.T) {
	list := "https://chiomasbags.com, https://*.huddle.ng"
	cases := map[string]bool{
		"https://chiomasbags.com":          true,
		"https://shop.huddle.ng":           true,
		"https://a.b.huddle.ng":            true,
		"https://huddle.ng":                false, // wildcard needs a subdomain
		"https://evilhuddle.ng":            false,
		"https://shop.huddle.ng.evil.com":  false,
		"http://shop.huddle.ng":            false, // scheme must match
		"https://chiomasbags.com.evil.com": false,
		"https://attacker.com":             false,
		"":                                 false,
	}
	for origin, want := range cases {
		if got := originAllowed(origin, list); got != want {
			t.Errorf("originAllowed(%q) = %v, want %v", origin, got, want)
		}
	}
	if originAllowed("https://chiomasbags.com", "") {
		t.Error("empty allow list allowed an origin")
	}
}
