package services

import (
	"errors"
	"fmt"
	"huddle/pkg/model"
	"huddle/pkg/storedomain"
	"huddle/pkg/utility"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func toStoreDomainResponse(d model.StoreDomains) model.StoreDomainResponse {
	return model.StoreDomainResponse{Domain: d.Domain, Origin: "https://" + d.Domain, CreatedAt: d.CreatedAt}
}

func ListStoreDomains(db *gorm.DB, publicStoreId string, adminId uuid.UUID) ([]model.StoreDomainResponse, *utility.AppError) {
	store, appErr := OwnedStore(db, publicStoreId, adminId)
	if appErr != nil {
		return nil, appErr
	}

	var rows []model.StoreDomains
	if err := db.Where("store_id = ?", store.ID).Order("created_at").Order("domain").Find(&rows).Error; err != nil {
		return nil, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Internal Server Error", Err: err}
	}
	resp := make([]model.StoreDomainResponse, 0, len(rows))
	for _, d := range rows {
		resp = append(resp, toStoreDomainResponse(d))
	}
	return resp, nil
}

// AddStoreDomain connects a custom domain to the store, so its storefront on
// https://<domain> can call the API. Adding a domain the store already has
// returns it with created = false.
func AddStoreDomain(db *gorm.DB, domains *storedomain.Cache, publicStoreId string, adminId uuid.UUID, input string) (*model.StoreDomainResponse, bool, *utility.AppError) {
	store, appErr := OwnedStore(db, publicStoreId, adminId)
	if appErr != nil {
		return nil, false, appErr
	}

	domain, err := storedomain.Normalize(input)
	if err != nil {
		return nil, false, &utility.AppError{StatusCode: http.StatusBadRequest, Message: err.Error(), Err: err}
	}
	if storedomain.UnderWildcard(domain, os.Getenv("CORS_ALLOWED_ORIGINS")) {
		return nil, false, &utility.AppError{StatusCode: http.StatusBadRequest, Message: "Huddle subdomains don't need to be added", Err: errors.New("domain is under a CORS wildcard")}
	}

	var row model.StoreDomains
	created := false
	txErr := db.Transaction(func(tx *gorm.DB) error {
		// Locking the store makes concurrent adds count each other.
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").Where("id = ?", store.ID).First(&model.Stores{}).Error; err != nil {
			return err
		}
		err := tx.Where("store_id = ? AND domain = ?", store.ID, domain).First(&row).Error
		if err == nil {
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		var count int64
		if err := tx.Model(&model.StoreDomains{}).Where("store_id = ?", store.ID).Count(&count).Error; err != nil {
			return err
		}
		if count >= storedomain.MaxPerStore {
			return &utility.AppError{StatusCode: http.StatusConflict, Message: fmt.Sprintf("A store can have at most %d domains. Remove one first.", storedomain.MaxPerStore), Err: errors.New("too many domains")}
		}

		row = model.StoreDomains{Id: utility.GenerateUUID(), StoreId: store.ID, Domain: domain, CreatedAt: time.Now().UTC()}
		if err := tx.Create(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return &utility.AppError{StatusCode: http.StatusConflict, Message: "This domain is already connected to another store", Err: err}
			}
			return err
		}
		created = true
		return nil
	})
	if txErr != nil {
		var ae *utility.AppError
		if errors.As(txErr, &ae) {
			return nil, false, ae
		}
		return nil, false, &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Could not add domain", Err: txErr}
	}

	domains.Forget(domain)
	resp := toStoreDomainResponse(row)
	return &resp, created, nil
}

// RemoveStoreDomain disconnects a domain; its storefront can no longer call the API.
func RemoveStoreDomain(db *gorm.DB, domains *storedomain.Cache, publicStoreId string, adminId uuid.UUID, input string) *utility.AppError {
	store, appErr := OwnedStore(db, publicStoreId, adminId)
	if appErr != nil {
		return appErr
	}

	notFound := &utility.AppError{StatusCode: http.StatusNotFound, Message: "Domain not found", Err: errors.New("domain not found")}
	domain, err := storedomain.Normalize(input)
	if err != nil {
		return notFound
	}
	res := db.Where("store_id = ? AND domain = ?", store.ID, domain).Delete(&model.StoreDomains{})
	if res.Error != nil {
		return &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Could not remove domain", Err: res.Error}
	}
	if res.RowsAffected == 0 {
		return notFound
	}

	domains.Forget(domain)
	return nil
}
