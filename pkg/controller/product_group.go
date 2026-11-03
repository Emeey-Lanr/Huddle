package controller

import (
	"huddle/pkg/audit"
	"huddle/pkg/model"
	"huddle/pkg/response"
	"huddle/pkg/services"
	"huddle/pkg/utility"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Product groups ("Shoes", "Bags") are managed by the store's owner.

func (b *StoreBase) ListGroups(c *gin.Context) {
	adminId, ok := adminUUID(c)
	if !ok {
		return
	}
	groups, serviceErr := services.ListProductGroups(b.Db, c.Param("id"), adminId)
	if serviceErr != nil {
		response.Error(c, serviceErr.StatusCode, serviceErr.Message, serviceErr.Err)
		return
	}
	response.Success(c, "Groups retrieved successfully", groups)
}

func (b *StoreBase) CreateGroup(c *gin.Context) {
	adminId, ok := adminUUID(c)
	if !ok {
		return
	}
	var req model.ProductGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request payload", err)
		return
	}
	group, serviceErr := services.CreateProductGroup(b.Db, c.Param("id"), adminId, req)
	if serviceErr != nil {
		response.Error(c, serviceErr.StatusCode, serviceErr.Message, serviceErr.Err)
		return
	}
	audit.LogEvent(c.Request.Context(), "PRODUCT_GROUP_CREATED", adminId.String(), group.Id, "SUCCESS", map[string]interface{}{"store_id": c.Param("id"), "name": group.Name})
	response.Created(c, "Group created", group)
}

func (b *StoreBase) UpdateGroup(c *gin.Context) {
	adminId, ok := adminUUID(c)
	if !ok {
		return
	}
	var req model.ProductGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request payload", err)
		return
	}
	group, serviceErr := services.UpdateProductGroup(b.Db, c.Param("id"), c.Param("groupId"), adminId, req)
	if serviceErr != nil {
		response.Error(c, serviceErr.StatusCode, serviceErr.Message, serviceErr.Err)
		return
	}
	response.Success(c, "Group updated", group)
}

func (b *StoreBase) DeleteGroup(c *gin.Context) {
	adminId, ok := adminUUID(c)
	if !ok {
		return
	}
	if serviceErr := services.DeleteProductGroup(b.Db, c.Param("id"), c.Param("groupId"), adminId); serviceErr != nil {
		response.Error(c, serviceErr.StatusCode, serviceErr.Message, serviceErr.Err)
		return
	}
	audit.LogEvent(c.Request.Context(), "PRODUCT_GROUP_DELETED", adminId.String(), c.Param("groupId"), "SUCCESS", map[string]interface{}{"store_id": c.Param("id")})
	response.Success(c, "Group deleted; its products are now ungrouped", nil)
}

func (b *StoreBase) AddGroupProducts(c *gin.Context) {
	adminId, ok := adminUUID(c)
	if !ok {
		return
	}
	var req model.ProductGroupProductsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request payload", err)
		return
	}
	if err := utility.Validate.Struct(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Send product_ids: 1 to 100 product ids", utility.ValidationErrors(err))
		return
	}
	group, serviceErr := services.AddProductsToGroup(b.Db, c.Param("id"), c.Param("groupId"), adminId, req.ProductIds)
	if serviceErr != nil {
		response.Error(c, serviceErr.StatusCode, serviceErr.Message, serviceErr.Err)
		return
	}
	response.Success(c, "Products added to the group", group)
}

func (b *StoreBase) RemoveGroupProduct(c *gin.Context) {
	adminId, ok := adminUUID(c)
	if !ok {
		return
	}
	group, serviceErr := services.RemoveProductFromGroup(b.Db, c.Param("id"), c.Param("groupId"), c.Param("productId"), adminId)
	if serviceErr != nil {
		response.Error(c, serviceErr.StatusCode, serviceErr.Message, serviceErr.Err)
		return
	}
	response.Success(c, "Product removed from the group", group)
}

func (b *StorefrontBase) Groups(c *gin.Context) {
	groups, serviceErr := services.ListStorefrontGroups(b.Db, storeFromKey(c))
	if serviceErr != nil {
		response.Error(c, serviceErr.StatusCode, serviceErr.Message, serviceErr.Err)
		return
	}
	response.Success(c, "Groups retrieved successfully", groups)
}
