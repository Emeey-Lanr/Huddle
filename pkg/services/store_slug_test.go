package services

import (
	"strings"
	"testing"

	"huddle/pkg/model"
	"huddle/pkg/testutil"
)

func TestSlugBase(t *testing.T) {
	cases := map[string]string{
		"Chioma's Bags":         "chiomas-bags",
		"Chioma’s  Bags!!":      "chiomas-bags",
		"  --Mama & Sons-- ":    "mama-sons",
		"Ìyá Ṣadé":              "iya-sade",
		"Ọ̀ṣun Beads":           "osun-beads",
		"محل":                   "",
		strings.Repeat("a", 50): strings.Repeat("a", 33),
	}
	for name, want := range cases {
		if got := slugBase(name); got != want {
			t.Errorf("slugBase(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestCreateStore_ReservedOrUnusableNameStillGetsAnAddress(t *testing.T) {
	db := testutil.NewDB(t)
	admin := testutil.SeedAdmin(t, db)

	for name, prefix := range map[string]string{"Admin": "admin-", "محل": "store-"} {
		store, _, err := CreateStore(db, model.Stores{StoreName: name, StoreDescription: "Handmade bags"}, admin.Id, admin.Email)
		if err != nil {
			t.Fatalf("%s: %s %v", name, err.Message, err.Err)
		}
		if !strings.HasPrefix(store.Slug, prefix) {
			t.Errorf("%s: slug %q, want prefix %q", name, store.Slug, prefix)
		}
	}
}

// Stores made before slugs existed must each get a valid, unique address.
func TestSlugMigration_FillsExistingStores(t *testing.T) {
	db := testutil.NewDB(t)
	admin := testutil.SeedAdmin(t, db)
	other := testutil.SeedAdmin(t, db)

	type seed struct {
		name   string
		exact  string // the slug it must get, if known
		prefix string
	}
	seeds := []seed{
		{name: "Chioma's Bags", exact: "chiomas-bags"},
		{name: "Support", prefix: "support-"},
		{name: "محل", prefix: "store-"},
		{name: strings.Repeat("Long name ", 8), prefix: "long-name-long-name"},
	}
	ids := map[string]string{}
	for _, s := range seeds {
		store, _, err := CreateStore(db, model.Stores{StoreName: s.name, StoreDescription: "Handmade bags"}, admin.Id, admin.Email)
		if err != nil {
			t.Fatalf("%s: %s %v", s.name, err.Message, err.Err)
		}
		ids[s.name] = store.ID.String()
	}
	twin, _, err := CreateStore(db, model.Stores{StoreName: "Chioma's Bags", StoreDescription: "Handmade bags"}, other.Id, other.Email)
	if err != nil {
		t.Fatalf("twin: %s", err.Message)
	}

	testutil.MigrateFile(t, db, "000015_store_slug.down.sql")
	testutil.MigrateFile(t, db, "000015_store_slug.up.sql")

	slugOf := func(id string) string {
		var slug string
		db.Raw("SELECT slug FROM stores WHERE id = ?", id).Scan(&slug)
		return slug
	}
	for _, s := range seeds {
		got := slugOf(ids[s.name])
		if s.exact != "" && got != s.exact {
			t.Errorf("%q: slug %q, want %q", s.name, got, s.exact)
		}
		if s.prefix != "" && !strings.HasPrefix(got, s.prefix) {
			t.Errorf("%q: slug %q, want prefix %q", s.name, got, s.prefix)
		}
		if len(got) > maxSlug || !slugFormat.MatchString(got) {
			t.Errorf("%q: invalid slug %q", s.name, got)
		}
	}
	if got := slugOf(twin.ID.String()); !strings.HasPrefix(got, "chiomas-bags-") {
		t.Errorf("second store with the same name: slug %q", got)
	}
}
