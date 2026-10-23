// Package storedomain decides which browser origins may call the API: those
// listed in CORS_ALLOWED_ORIGINS and https://<a store's custom domain>.
package storedomain

import (
	"errors"
	"net"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"huddle/pkg/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// MaxPerStore is how many custom domains one store can have.
const MaxPerStore = 5

// maxCacheEntries bounds the cache, which also remembers origins that belong
// to no store: requests from made-up origins must not grow it forever.
const maxCacheEntries = 10000

var hostname = regexp.MustCompile(`^([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+([a-z]{2,63}|xn--[a-z0-9-]{1,59})$`)

// Normalize turns what a seller typed ("ChiomasBags.com", "https://chiomasbags.com/")
// into a bare lowercase hostname, or says why it can't be a storefront domain.
func Normalize(input string) (string, error) {
	invalid := errors.New("Enter just the domain, like chiomasbags.com")
	s := strings.ToLower(strings.TrimSpace(input))
	if strings.Contains(s, "://") {
		u, err := url.Parse(s)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.Port() != "" ||
			(u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
			return "", invalid
		}
		s = u.Hostname()
	}
	s = strings.TrimSuffix(s, ".")
	if net.ParseIP(s) != nil {
		return "", errors.New("Use a domain name, not an IP address")
	}
	if len(s) > 253 || !hostname.MatchString(s) || strings.HasSuffix(s, ".localhost") {
		return "", invalid
	}
	return s, nil
}

// HTTPSHost returns the host of an https origin without a port, such as
// "chiomasbags.com" for "https://chiomasbags.com". Custom domains are https only.
func HTTPSHost(origin string) (string, bool) {
	host, ok := strings.CutPrefix(origin, "https://")
	if !ok || host == "" || strings.ContainsAny(host, ":/@?#") {
		return "", false
	}
	return strings.ToLower(host), true
}

// OriginAllowed reports whether origin is in allowList, a comma-separated list
// of exact origins ("https://chiomasbags.com") or subdomain wildcards
// ("https://*.huddle.ng"), as in CORS_ALLOWED_ORIGINS.
func OriginAllowed(origin, allowList string) bool {
	for _, entry := range strings.Split(allowList, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if entry == origin {
			return true
		}
		// "https://*.huddle.ng" matches "https://shop.huddle.ng" but not "https://huddle.ng"
		// or "https://evilhuddle.ng".
		if scheme, host, ok := strings.Cut(entry, "://*."); ok {
			prefix := scheme + "://"
			if strings.HasPrefix(origin, prefix) {
				sub := strings.TrimPrefix(origin, prefix)
				if strings.HasSuffix(sub, "."+host) && len(sub) > len(host)+1 && !strings.ContainsAny(sub, "/@") {
					return true
				}
			}
		}
	}
	return false
}

// UnderWildcard reports whether https://<domain> matches a wildcard in
// allowList. Such subdomains (e.g. of huddle.ng) belong to Huddle, not to one store.
func UnderWildcard(domain, allowList string) bool {
	var wildcards []string
	for _, entry := range strings.Split(allowList, ",") {
		if strings.Contains(entry, "*") {
			wildcards = append(wildcards, entry)
		}
	}
	return OriginAllowed("https://"+domain, strings.Join(wildcards, ","))
}

// Cache answers "which store has this domain?" from memory, so CORS checks
// don't query Postgres on every request. Answers, including "no store", last
// ttl. Forget makes a change visible on this server at once; other servers
// see it within ttl.
type Cache struct {
	db      *gorm.DB
	ttl     time.Duration
	mu      sync.Mutex
	entries map[string]cacheEntry
	gen     uint64 // bumped by Forget, so a lookup that raced with it isn't cached
}

type cacheEntry struct {
	storeId uuid.UUID
	expires time.Time
}

func NewCache(db *gorm.DB, ttl time.Duration) *Cache {
	return &Cache{db: db, ttl: ttl, entries: map[string]cacheEntry{}}
}

// StoreFor returns the internal id of the store that has domain, or uuid.Nil.
func (c *Cache) StoreFor(domain string) (uuid.UUID, error) {
	now := time.Now()
	c.mu.Lock()
	e, ok := c.entries[domain]
	gen := c.gen
	c.mu.Unlock()
	if ok && now.Before(e.expires) {
		return e.storeId, nil
	}

	var ids []uuid.UUID
	if err := c.db.Model(&model.StoreDomains{}).Where("domain = ?", domain).Limit(1).Pluck("store_id", &ids).Error; err != nil {
		return uuid.Nil, err
	}
	storeId := uuid.Nil
	if len(ids) > 0 {
		storeId = ids[0]
	}

	c.mu.Lock()
	if c.gen == gen {
		if len(c.entries) >= maxCacheEntries {
			c.entries = map[string]cacheEntry{}
		}
		c.entries[domain] = cacheEntry{storeId: storeId, expires: now.Add(c.ttl)}
	}
	c.mu.Unlock()
	return storeId, nil
}

// Forget drops what the cache knows about domain, after it is added or removed.
func (c *Cache) Forget(domain string) {
	c.mu.Lock()
	delete(c.entries, domain)
	c.gen++
	c.mu.Unlock()
}
