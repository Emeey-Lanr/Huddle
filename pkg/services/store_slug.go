package services

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"huddle/pkg/model"
	"huddle/pkg/utility"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"github.com/google/uuid"
	"golang.org/x/text/unicode/norm"
	"gorm.io/gorm"
)

const (
	maxSlug = 40
	// maxSlugBase leaves room for "-" and a 6-character suffix.
	maxSlugBase = 33
)

var (
	slugFormat    = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)
	slugSeparator = regexp.MustCompile(`[^a-z0-9]+`)
	// reservedSlugs are subdomains the platform may need for itself. The same
	// list is in migration 000015.
	reservedSlugs = []string{"admin", "api", "app", "assets", "blog", "cdn", "dashboard", "docs", "ftp", "help", "mail", "seller", "static", "status", "store", "stores", "support", "www"}
)

// withoutAccents splits accented letters and drops the marks: "Ìyá Ṣadé"
// becomes "Iya Sade", so Yoruba and Igbo names keep their letters.
func withoutAccents(s string) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(s) {
		if !unicode.Is(unicode.Mn, r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// slugBase turns a store name into its web address form: "Chioma's Bags"
// becomes "chiomas-bags". A name with no usable letters gives "".
func slugBase(name string) string {
	s := strings.NewReplacer("'", "", "’", "").Replace(strings.ToLower(withoutAccents(name)))
	s = strings.Trim(slugSeparator.ReplaceAllString(s, "-"), "-")
	if len(s) > maxSlugBase {
		s = strings.TrimRight(s[:maxSlugBase], "-")
	}
	return s
}

func slugSuffix() (string, error) {
	b := make([]byte, 3)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func slugTaken(db *gorm.DB, slug string, except uuid.UUID) (bool, error) {
	var n int64
	err := db.Model(&model.Stores{}).Where("slug = ? AND id <> ?", slug, except).Count(&n).Error
	return n > 0, err
}

// newStoreSlug picks the address for a new store: its name if free, otherwise
// the name plus a short suffix, so two "Chioma's Bags" can both exist.
func newStoreSlug(db *gorm.DB, name string) (string, error) {
	base := slugBase(name)
	if base != "" && !slices.Contains(reservedSlugs, base) {
		taken, err := slugTaken(db, base, uuid.Nil)
		if err != nil {
			return "", err
		}
		if !taken {
			return base, nil
		}
	}

	suffix, err := slugSuffix()
	if err != nil {
		return "", err
	}
	if base == "" {
		base = "store"
	}
	return base + "-" + suffix, nil
}

// checkStoreSlug validates a slug a seller chose for their store.
func checkStoreSlug(db *gorm.DB, slug string, storeId uuid.UUID) *utility.AppError {
	if len(slug) > maxSlug || !slugFormat.MatchString(slug) {
		message := "Store address can only use lowercase letters, numbers and dashes, up to 40 characters"
		return &utility.AppError{StatusCode: http.StatusBadRequest, Message: message, Err: errors.New(message)}
	}
	if slices.Contains(reservedSlugs, slug) {
		message := "That store address is reserved"
		return &utility.AppError{StatusCode: http.StatusBadRequest, Message: message, Err: errors.New(message)}
	}
	taken, err := slugTaken(db, slug, storeId)
	if err != nil {
		return &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Internal Server Error", Err: err}
	}
	if taken {
		message := "That store address is taken"
		return &utility.AppError{StatusCode: http.StatusConflict, Message: message, Err: errors.New(message)}
	}
	return nil
}
