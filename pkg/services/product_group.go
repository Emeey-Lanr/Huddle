package services

import (
	"errors"
	"fmt"
	"huddle/pkg/model"
	"huddle/pkg/utility"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	maxGroupName      = 40
	MaxGroupsPerStore = 30
)

func groupErr(status int, message string) *utility.AppError {
	return &utility.AppError{StatusCode: status, Message: message, Err: errors.New(strings.ToLower(message))}
}

func groupInternal(err error) *utility.AppError {
	return &utility.AppError{StatusCode: http.StatusInternalServerError, Message: "Internal Server Error", Err: err}
}

// groupName trims and checks a group's name.
func groupName(name string) (string, *utility.AppError) {
	name = strings.Join(strings.Fields(name), " ")
	if name == "" {
		return "", groupErr(http.StatusBadRequest, "Give the group a name")
	}
	if utf8.RuneCountInString(name) > maxGroupName {
		return "", groupErr(http.StatusBadRequest, fmt.Sprintf("A group name can be at most %d characters", maxGroupName))
	}
	return name, nil
}

// groupSlug picks the group's address name, unique in the store: "Ankara Bags"
// becomes ankara-bags, then ankara-bags-2 if that's taken.
func groupSlug(tx *gorm.DB, storeId, except uuid.UUID, name string) (string, error) {
	base := slugBase(name)
	if base == "" {
		base = "group"
	}
	for n := 1; ; n++ {
		slug := base
		if n > 1 {
			slug = fmt.Sprintf("%s-%d", base, n)
		}
		var count int64
		if err := tx.Model(&model.ProductGroups{}).Where("store_id = ? AND slug = ? AND id <> ?", storeId, slug, except).Count(&count).Error; err != nil {
			return "", err
		}
		if count == 0 {
			return slug, nil
		}
	}
}

func nameTaken(tx *gorm.DB, storeId, except uuid.UUID, name string) (bool, error) {
	var count int64
	err := tx.Model(&model.ProductGroups{}).Where("store_id = ? AND lower(name) = lower(?) AND id <> ?", storeId, name, except).Count(&count).Error
	return count > 0, err
}

// lockStore makes concurrent changes to one store's groups wait for each other.
func lockStore(tx *gorm.DB, storeId uuid.UUID) error {
	return tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").Where("id = ?", storeId).First(&model.Stores{}).Error
}

func ownedGroup(tx *gorm.DB, storeId uuid.UUID, publicGroupId string) (*model.ProductGroups, *utility.AppError) {
	var group model.ProductGroups
	if err := tx.Where("public_id = ? AND store_id = ?", publicGroupId, storeId).First(&group).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, groupErr(http.StatusNotFound, "Group not found")
		}
		return nil, groupInternal(err)
	}
	return &group, nil
}

// asAppError returns the AppError a transaction failed with, or wraps the error.
func asAppError(err error) *utility.AppError {
	var ae *utility.AppError
	if errors.As(err, &ae) {
		return ae
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return groupErr(http.StatusConflict, "This store already has a group with that name")
	}
	return groupInternal(err)
}

func groupResponses(db *gorm.DB, groups []model.ProductGroups) ([]model.ProductGroupResponse, error) {
	resp := make([]model.ProductGroupResponse, 0, len(groups))
	if len(groups) == 0 {
		return resp, nil
	}
	ids := make([]uuid.UUID, 0, len(groups))
	for _, g := range groups {
		ids = append(ids, g.Id)
	}
	var rows []struct {
		GroupId  uuid.UUID
		PublicId string
	}
	if err := db.Model(&model.Products{}).Select("group_id, public_id").Where("group_id IN ?", ids).Order("created_at DESC").Order("id").Scan(&rows).Error; err != nil {
		return nil, err
	}
	products := make(map[uuid.UUID][]string, len(groups))
	for _, r := range rows {
		products[r.GroupId] = append(products[r.GroupId], r.PublicId)
	}
	for _, g := range groups {
		ids := products[g.Id]
		if ids == nil {
			ids = []string{}
		}
		resp = append(resp, model.ProductGroupResponse{Id: g.PublicId, Name: g.Name, Slug: g.Slug, Position: g.Position, ProductIds: ids, CreatedAt: g.CreatedAt})
	}
	return resp, nil
}

func oneGroupResponse(db *gorm.DB, group model.ProductGroups) (*model.ProductGroupResponse, *utility.AppError) {
	resp, err := groupResponses(db, []model.ProductGroups{group})
	if err != nil {
		return nil, groupInternal(err)
	}
	return &resp[0], nil
}

// ListProductGroups returns the store's groups in the order shoppers see them.
func ListProductGroups(db *gorm.DB, publicStoreId string, adminId uuid.UUID) ([]model.ProductGroupResponse, *utility.AppError) {
	store, appErr := OwnedStore(db, publicStoreId, adminId)
	if appErr != nil {
		return nil, appErr
	}
	var groups []model.ProductGroups
	if err := db.Where("store_id = ?", store.ID).Order("position").Order("created_at").Order("id").Find(&groups).Error; err != nil {
		return nil, groupInternal(err)
	}
	resp, err := groupResponses(db, groups)
	if err != nil {
		return nil, groupInternal(err)
	}
	return resp, nil
}

// CreateProductGroup adds an empty group at the end of the store's list.
func CreateProductGroup(db *gorm.DB, publicStoreId string, adminId uuid.UUID, req model.ProductGroupRequest) (*model.ProductGroupResponse, *utility.AppError) {
	store, appErr := OwnedStore(db, publicStoreId, adminId)
	if appErr != nil {
		return nil, appErr
	}
	name, appErr := groupName(req.Name)
	if appErr != nil {
		return nil, appErr
	}

	var group model.ProductGroups
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := lockStore(tx, store.ID); err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&model.ProductGroups{}).Where("store_id = ?", store.ID).Count(&count).Error; err != nil {
			return err
		}
		if count >= MaxGroupsPerStore {
			return groupErr(http.StatusConflict, fmt.Sprintf("A store can have at most %d groups", MaxGroupsPerStore))
		}
		if taken, err := nameTaken(tx, store.ID, uuid.Nil, name); err != nil {
			return err
		} else if taken {
			return groupErr(http.StatusConflict, "This store already has a group with that name")
		}
		slug, err := groupSlug(tx, store.ID, uuid.Nil, name)
		if err != nil {
			return err
		}
		position := int(count)
		if req.Position != nil {
			position = *req.Position
		}
		now := time.Now().UTC()
		group = model.ProductGroups{
			Id: utility.GenerateUUID(), PublicId: "grp_" + strings.ReplaceAll(utility.GenerateUUID().String(), "-", ""),
			StoreId: store.ID, Name: name, Slug: slug, Position: position, CreatedAt: now, UpdatedAt: now,
		}
		return tx.Create(&group).Error
	})
	if err != nil {
		return nil, asAppError(err)
	}
	return oneGroupResponse(db, group)
}

// UpdateProductGroup renames or moves a group. A new name gives it a new
// address, so old links to the group stop working.
func UpdateProductGroup(db *gorm.DB, publicStoreId, publicGroupId string, adminId uuid.UUID, req model.ProductGroupRequest) (*model.ProductGroupResponse, *utility.AppError) {
	store, appErr := OwnedStore(db, publicStoreId, adminId)
	if appErr != nil {
		return nil, appErr
	}
	var name string
	if strings.TrimSpace(req.Name) != "" {
		if name, appErr = groupName(req.Name); appErr != nil {
			return nil, appErr
		}
	}

	var group *model.ProductGroups
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := lockStore(tx, store.ID); err != nil {
			return err
		}
		var ae *utility.AppError
		if group, ae = ownedGroup(tx, store.ID, publicGroupId); ae != nil {
			return ae
		}
		updates := map[string]any{}
		if name != "" && name != group.Name {
			if taken, err := nameTaken(tx, store.ID, group.Id, name); err != nil {
				return err
			} else if taken {
				return groupErr(http.StatusConflict, "This store already has a group with that name")
			}
			slug, err := groupSlug(tx, store.ID, group.Id, name)
			if err != nil {
				return err
			}
			updates["name"], updates["slug"] = name, slug
			group.Name, group.Slug = name, slug
		}
		if req.Position != nil {
			updates["position"] = *req.Position
			group.Position = *req.Position
		}
		if len(updates) == 0 {
			return nil
		}
		updates["updated_at"] = time.Now().UTC()
		return tx.Model(&model.ProductGroups{}).Where("id = ?", group.Id).Updates(updates).Error
	})
	if err != nil {
		return nil, asAppError(err)
	}
	return oneGroupResponse(db, *group)
}

// DeleteProductGroup removes a group; its products stay, ungrouped.
func DeleteProductGroup(db *gorm.DB, publicStoreId, publicGroupId string, adminId uuid.UUID) *utility.AppError {
	store, appErr := OwnedStore(db, publicStoreId, adminId)
	if appErr != nil {
		return appErr
	}
	res := db.Where("public_id = ? AND store_id = ?", publicGroupId, store.ID).Delete(&model.ProductGroups{})
	if res.Error != nil {
		return groupInternal(res.Error)
	}
	if res.RowsAffected == 0 {
		return groupErr(http.StatusNotFound, "Group not found")
	}
	return nil
}

// AddProductsToGroup puts the store's products into the group, moving them out
// of any other group. Every product must belong to the store, or none move.
func AddProductsToGroup(db *gorm.DB, publicStoreId, publicGroupId string, adminId uuid.UUID, productIds []string) (*model.ProductGroupResponse, *utility.AppError) {
	store, appErr := OwnedStore(db, publicStoreId, adminId)
	if appErr != nil {
		return nil, appErr
	}
	unique := make([]string, 0, len(productIds))
	seen := make(map[string]bool, len(productIds))
	for _, id := range productIds {
		if id = strings.TrimSpace(id); id != "" && !seen[id] {
			seen[id] = true
			unique = append(unique, id)
		}
	}
	if len(unique) == 0 {
		return nil, groupErr(http.StatusBadRequest, "Choose at least one product")
	}

	var group *model.ProductGroups
	err := db.Transaction(func(tx *gorm.DB) error {
		var ae *utility.AppError
		if group, ae = ownedGroup(tx, store.ID, publicGroupId); ae != nil {
			return ae
		}
		res := tx.Model(&model.Products{}).Where("store_id = ? AND public_id IN ?", store.ID, unique).
			Updates(map[string]any{"group_id": group.Id, "updated_at": time.Now().UTC()})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != int64(len(unique)) {
			return groupErr(http.StatusNotFound, "Some of these products are not in this store")
		}
		return nil
	})
	if err != nil {
		return nil, asAppError(err)
	}
	return oneGroupResponse(db, *group)
}

// RemoveProductFromGroup takes one product out of the group.
func RemoveProductFromGroup(db *gorm.DB, publicStoreId, publicGroupId, publicProductId string, adminId uuid.UUID) (*model.ProductGroupResponse, *utility.AppError) {
	store, appErr := OwnedStore(db, publicStoreId, adminId)
	if appErr != nil {
		return nil, appErr
	}
	group, appErr := ownedGroup(db, store.ID, publicGroupId)
	if appErr != nil {
		return nil, appErr
	}
	res := db.Model(&model.Products{}).Where("store_id = ? AND public_id = ? AND group_id = ?", store.ID, publicProductId, group.Id).
		Updates(map[string]any{"group_id": nil, "updated_at": time.Now().UTC()})
	if res.Error != nil {
		return nil, groupInternal(res.Error)
	}
	if res.RowsAffected == 0 {
		return nil, groupErr(http.StatusNotFound, "This product is not in this group")
	}
	return oneGroupResponse(db, *group)
}

// ListStorefrontGroups returns the groups shoppers can browse: those with at
// least one product on sale, in the seller's order.
func ListStorefrontGroups(db *gorm.DB, storeId uuid.UUID) ([]model.StorefrontGroup, *utility.AppError) {
	groups := []model.StorefrontGroup{}
	err := db.Raw(`SELECT g.name, g.slug, count(p.id) AS products
		FROM product_groups g JOIN products p ON p.group_id = g.id AND p.active
		WHERE g.store_id = ?
		GROUP BY g.id
		ORDER BY g.position, g.created_at, g.id`, storeId).Scan(&groups).Error
	if err != nil {
		return nil, groupInternal(err)
	}
	return groups, nil
}

// storefrontGroupId finds a group by its slug for the shop's product filter.
func storefrontGroupId(db *gorm.DB, storeId uuid.UUID, slug string) (uuid.UUID, *utility.AppError) {
	var group model.ProductGroups
	if err := db.Select("id").Where("store_id = ? AND slug = ?", storeId, strings.ToLower(strings.TrimSpace(slug))).First(&group).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return uuid.Nil, groupErr(http.StatusNotFound, "Group not found")
		}
		return uuid.Nil, groupInternal(err)
	}
	return group.Id, nil
}
