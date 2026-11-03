package router

import (
	"net/http"
	"strings"
	"testing"

	"huddle/pkg/model"
)

func TestStorefront_ListGivesEachProductOnePhoto(t *testing.T) {
	s := newShop(t)
	var bagId string
	if err := s.db.Raw("SELECT id FROM products WHERE public_id = ?", s.bag).Scan(&bagId).Error; err != nil {
		t.Fatal(err)
	}
	for _, url := range []string{"https://img.test/a.jpg", "https://img.test/b.jpg"} {
		err := s.db.Exec("INSERT INTO images (id, product_id, public_id, image_url) VALUES (gen_random_uuid(), ?, gen_random_uuid()::text, ?)", bagId, url).Error
		if err != nil {
			t.Fatal(err)
		}
	}

	var list struct {
		Items []struct {
			Id       string `json:"id"`
			ImageUrl string `json:"image_url"`
		} `json:"items"`
	}
	decode(t, s.get("/storefront/products", s.key), &list)
	if len(list.Items) != 2 {
		t.Fatalf("%d items, want 2", len(list.Items))
	}
	for _, item := range list.Items {
		if item.Id == s.bag && !strings.HasPrefix(item.ImageUrl, "https://img.test/") {
			t.Errorf("bag has no photo: %+v", item)
		}
		if item.Id == s.cap && item.ImageUrl != "" {
			t.Errorf("cap has no images but got %q", item.ImageUrl)
		}
	}
}

func TestTheme_NewStoreGetsDefaults(t *testing.T) {
	s := newShop(t)

	var store struct {
		Theme model.StoreTheme `json:"theme"`
	}
	w := s.get("/storefront/store", s.key)
	decode(t, w, &store)
	if w.Code != http.StatusOK || store.Theme != model.DefaultStoreTheme() {
		t.Errorf("%d %+v, want the default theme", w.Code, store.Theme)
	}

	code, env := s.do(http.MethodGet, "/store/"+s.storeId+"/theme", s.owner, nil)
	var theme model.StoreTheme
	s.data(env, &theme)
	if code != http.StatusOK || theme != model.DefaultStoreTheme() {
		t.Errorf("seller view: %d %+v", code, theme)
	}
}

func TestTheme_SavedAndShownToShoppers(t *testing.T) {
	s := newShop(t)
	path := "/store/" + s.storeId + "/theme"

	code, env := s.do(http.MethodPut, path, s.owner, map[string]any{
		"logo_url":      "https://res.cloudinary.com/huddle/logo.png",
		"primary_color": "#AA1122",
		"font":          "poppins",
		"sections":      map[string]any{"about": false},
	})
	if code != http.StatusOK {
		t.Fatalf("save theme: %d %s", code, env.Message)
	}

	var store struct {
		Theme model.StoreTheme `json:"theme"`
	}
	decode(t, s.get("/storefront/store", s.key), &store)
	want := model.DefaultStoreTheme()
	want.LogoUrl = "https://res.cloudinary.com/huddle/logo.png"
	want.PrimaryColor = "#aa1122"
	want.Font = "poppins"
	want.Sections.About = false
	if store.Theme != want {
		t.Errorf("got %+v, want %+v", store.Theme, want)
	}

	// Later changes keep what was saved before; an empty link removes the image.
	if code, env := s.do(http.MethodPut, path, s.owner, map[string]any{"accent_color": "#00ff00", "logo_url": ""}); code != http.StatusOK {
		t.Fatalf("second save: %d %s", code, env.Message)
	}
	decode(t, s.get("/storefront/store", s.key), &store)
	want.AccentColor = "#00ff00"
	want.LogoUrl = ""
	if store.Theme != want {
		t.Errorf("after second save got %+v, want %+v", store.Theme, want)
	}
}

func TestTheme_RejectsBadValuesAndSavesNothing(t *testing.T) {
	s := newShop(t)
	path := "/store/" + s.storeId + "/theme"

	bad := []map[string]any{
		{"primary_color": "red"},
		{"primary_color": "#fff"},
		{"accent_color": "#000000;}body{display:none"},
		{"font": "comic-sans"},
		{"logo_url": "http://example.com/logo.png"},
		{"logo_url": "javascript:alert(1)"},
		{"banner_url": "https://user:pass@example.com/banner.png"},
		{"banner_url": "https://example.com/" + strings.Repeat("a", 500)},
		{"primary_color": "#123456", "font": "comic-sans"},
	}
	for _, body := range bad {
		if code, _ := s.do(http.MethodPut, path, s.owner, body); code != http.StatusBadRequest {
			t.Errorf("%v: %d, want 400", body, code)
		}
	}

	_, env := s.do(http.MethodGet, path, s.owner, nil)
	var theme model.StoreTheme
	s.data(env, &theme)
	if theme != model.DefaultStoreTheme() {
		t.Errorf("a rejected request changed the theme: %+v", theme)
	}
}

func TestTheme_OwnerOnly(t *testing.T) {
	s := newShop(t)
	path := "/store/" + s.storeId + "/theme"

	if code, _ := s.do(http.MethodGet, path, s.otherOwner, nil); code != http.StatusForbidden {
		t.Errorf("other seller reading: %d, want 403", code)
	}
	if code, _ := s.do(http.MethodPut, path, s.otherOwner, map[string]any{"font": "lora"}); code != http.StatusForbidden {
		t.Errorf("other seller saving: %d, want 403", code)
	}
	if code, _ := s.do(http.MethodPut, path, "", map[string]any{"font": "lora"}); code != http.StatusUnauthorized {
		t.Errorf("no token: %d, want 401", code)
	}
}
