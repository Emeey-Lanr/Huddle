package services

import (
	"encoding/json"
	"errors"
	"huddle/pkg/model"
	"huddle/pkg/utility"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const maxThemeImageUrl = 500

var themeColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// parseStoreTheme lays saved settings over the defaults, so a new setting added
// to the template later gets its default on stores saved before it existed.
func parseStoreTheme(saved string) (model.StoreTheme, error) {
	theme := model.DefaultStoreTheme()
	if saved == "" {
		return theme, nil
	}
	err := json.Unmarshal([]byte(saved), &theme)
	return theme, err
}

func invalidTheme(message string) *utility.AppError {
	return &utility.AppError{StatusCode: http.StatusBadRequest, Message: message, Err: errors.New(message)}
}

// themeImageUrl accepts an empty value (no image) or a plain https link.
func themeImageUrl(field, value string) (string, *utility.AppError) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	u, err := url.Parse(value)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || len(value) > maxThemeImageUrl {
		return "", invalidTheme(field + " must be an https link of at most 500 characters")
	}
	return value, nil
}

func themeColorValue(field, value string) (string, *utility.AppError) {
	value = strings.TrimSpace(value)
	if !themeColor.MatchString(value) {
		return "", invalidTheme(field + " must be a colour like #1a2b3c")
	}
	return strings.ToLower(value), nil
}

// applyThemeUpdate changes only the fields present in the request.
func applyThemeUpdate(theme *model.StoreTheme, req model.StoreThemeUpdate) *utility.AppError {
	var appErr *utility.AppError
	if req.LogoUrl != nil {
		if theme.LogoUrl, appErr = themeImageUrl("logo_url", *req.LogoUrl); appErr != nil {
			return appErr
		}
	}
	if req.BannerUrl != nil {
		if theme.BannerUrl, appErr = themeImageUrl("banner_url", *req.BannerUrl); appErr != nil {
			return appErr
		}
	}
	if req.PrimaryColor != nil {
		if theme.PrimaryColor, appErr = themeColorValue("primary_color", *req.PrimaryColor); appErr != nil {
			return appErr
		}
	}
	if req.AccentColor != nil {
		if theme.AccentColor, appErr = themeColorValue("accent_color", *req.AccentColor); appErr != nil {
			return appErr
		}
	}
	if req.Font != nil {
		if !slices.Contains(model.StoreThemeFonts, *req.Font) {
			return invalidTheme("font must be one of: " + strings.Join(model.StoreThemeFonts, ", "))
		}
		theme.Font = *req.Font
	}
	if req.Sections != nil {
		if req.Sections.Banner != nil {
			theme.Sections.Banner = *req.Sections.Banner
		}
		if req.Sections.About != nil {
			theme.Sections.About = *req.Sections.About
		}
	}
	return nil
}

func GetStoreTheme(db *gorm.DB, publicStoreId string, adminId uuid.UUID) (*model.StoreTheme, *utility.AppError) {
	store, appErr := OwnedStore(db, publicStoreId, adminId)
	if appErr != nil {
		return nil, appErr
	}
	theme, err := parseStoreTheme(store.Theme)
	if err != nil {
		return nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Internal Server Error", Err: err}
	}
	return &theme, nil
}

func UpdateStoreTheme(db *gorm.DB, publicStoreId string, adminId uuid.UUID, req model.StoreThemeUpdate) (*model.StoreTheme, *utility.AppError) {
	store, appErr := OwnedStore(db, publicStoreId, adminId)
	if appErr != nil {
		return nil, appErr
	}
	theme, err := parseStoreTheme(store.Theme)
	if err != nil {
		return nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Internal Server Error", Err: err}
	}
	if appErr := applyThemeUpdate(&theme, req); appErr != nil {
		return nil, appErr
	}

	saved, err := json.Marshal(theme)
	if err != nil {
		return nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Internal Server Error", Err: err}
	}
	err = db.Exec("UPDATE stores SET theme = ?::jsonb, updated_at = ? WHERE id = ?", string(saved), time.Now().UTC(), store.ID).Error
	if err != nil {
		return nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Could not update theme", Err: err}
	}
	return &theme, nil
}
