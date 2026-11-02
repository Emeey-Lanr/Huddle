package model

import (
	"time"

	"github.com/google/uuid"
)

type Stores struct {
	ID               uuid.UUID `json:"id" gorm:"uuid"`
	AdminId          uuid.UUID `json:"admin_id" gorm:"column:admin_id"`
	StoreName        string    `json:"store_name" validate:"required" gorm:"column:store_name"`
	StoreDescription string    `json:"store_description" validate:"required,min=10" gorm:"column:store_description"`
	ApiKey           string    `json:"api_key" gorm:"column:api_key"`
	StoreId          string    `json:"store_id" gorm:"column:store_id"`
	// Slug is the name in the store's web address; never set when creating a store.
	Slug string `json:"-" gorm:"column:slug;default:(-)"`
	// BachsAccountId is the seller's Bachs sub-account; never set from requests.
	BachsAccountId *string `json:"-" gorm:"column:bachs_account_id"`
	// Theme is JSON (StoreTheme). Read-only here so creating a store keeps the
	// column's default; services.UpdateStoreTheme writes it.
	Theme     string    `json:"-" gorm:"column:theme;->"`
	CreatedAt time.Time `json:"created_at" gorm:"column:created_at"`
	UpdatedAt time.Time `json:"updated_at" gorm:"column:updated_at"`
}

// StoreResponse is what admins see; it never includes the API key or internal ids.
type StoreResponse struct {
	StoreId          string    `json:"store_id" gorm:"column:store_id"`
	StoreName        string    `json:"store_name" gorm:"column:store_name"`
	StoreDescription string    `json:"store_description" gorm:"column:store_description"`
	Slug             string    `json:"slug" gorm:"column:slug"`
	CreatedAt        time.Time `json:"created_at" gorm:"column:created_at"`
	UpdatedAt        time.Time `json:"updated_at" gorm:"column:updated_at"`
}

// StorefrontStore is the public store info shoppers see.
type StorefrontStore struct {
	StoreId          string     `json:"store_id"`
	StoreName        string     `json:"store_name"`
	StoreDescription string     `json:"store_description"`
	Theme            StoreTheme `json:"theme"`
}

// StoreTheme is how a store's customer website looks. Every store runs the same
// template; these settings are all that differ, so a bad value can't break
// checkout. They end up in a public page, so services.UpdateStoreTheme checks
// each one against a strict format.
type StoreTheme struct {
	LogoUrl      string             `json:"logo_url"`
	BannerUrl    string             `json:"banner_url"`
	PrimaryColor string             `json:"primary_color"`
	AccentColor  string             `json:"accent_color"`
	Font         string             `json:"font"`
	Sections     StoreThemeSections `json:"sections"`
}

// StoreThemeSections says which optional parts of the shop home page show.
type StoreThemeSections struct {
	Banner bool `json:"banner"`
	About  bool `json:"about"`
}

// StoreThemeFonts are the fonts the template ships with.
var StoreThemeFonts = []string{"anton", "bricolage", "fraunces", "system", "inter", "poppins", "lora"}

// DefaultStoreTheme is the look of a store nobody has themed yet.
func DefaultStoreTheme() StoreTheme {
	return StoreTheme{
		PrimaryColor: "#111827",
		AccentColor:  "#2563eb",
		Font:         "anton",
		Sections:     StoreThemeSections{Banner: true, About: true},
	}
}

// StoreThemeUpdate request. Missing fields are left unchanged; an empty
// logo_url or banner_url removes the image.
type StoreThemeUpdate struct {
	LogoUrl      *string                   `json:"logo_url"`
	BannerUrl    *string                   `json:"banner_url"`
	PrimaryColor *string                   `json:"primary_color"`
	AccentColor  *string                   `json:"accent_color"`
	Font         *string                   `json:"font"`
	Sections     *StoreThemeSectionsUpdate `json:"sections"`
}

// StoreThemeSectionsUpdate leaves missing fields unchanged.
type StoreThemeSectionsUpdate struct {
	Banner *bool `json:"banner"`
	About  *bool `json:"about"`
}

// StoreDomains is a custom domain a store's storefront runs on.
type StoreDomains struct {
	Id        uuid.UUID
	StoreId   uuid.UUID // stores.id, not the public store id
	Domain    string    // bare lowercase hostname, e.g. "chiomasbags.com"
	CreatedAt time.Time
}

// StoreDomainRequest adds a domain; a full link such as "https://chiomasbags.com/" is accepted.
type StoreDomainRequest struct {
	Domain string `json:"domain" validate:"required,max=300"`
}

// StoreDomainResponse is a store domain as sellers see it.
type StoreDomainResponse struct {
	Domain    string    `json:"domain"`
	Origin    string    `json:"origin"`
	CreatedAt time.Time `json:"created_at"`
}

// StoreUpdate request; empty fields are left unchanged.
type StoreUpdate struct {
	StoreName        string `json:"store_name" validate:"omitempty,max=150"`
	StoreDescription string `json:"store_description" validate:"omitempty,min=10"`
	// Slug changes the store's web address; the old address stops working.
	Slug string `json:"slug"`
}
