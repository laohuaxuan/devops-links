package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"devops-links/internal/store"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func linkToJSON(item store.Link, urlPrefix string, canManage bool) gin.H {
	iconURL := ""
	if p := strings.TrimSpace(item.IconPath); p != "" {
		iconURL = urlPrefix + "/" + p
	}
	return gin.H{
		"id":          item.ID,
		"category_id": item.CategoryID,
		"name":        item.Name,
		"url":         item.URL,
		"icon_url":    iconURL,
		"icon_path":   item.IconPath,
		"maintainer":  strings.TrimSpace(item.Maintainer),
		"remark":      strings.TrimSpace(item.Remark),
		"sort_order":  item.SortOrder,
		"created_by":  item.CreatedBy,
		"can_manage":  canManage,
	}
}

func (h *Handler) currentUserID(c *gin.Context) uint {
	userID, _ := c.Get("user_id")
	id, _ := userID.(uint)
	return id
}

func (h *Handler) currentRole(c *gin.Context) string {
	role, _ := c.Get("role")
	r, _ := role.(string)
	return r
}

func (h *Handler) isSuperAdmin(c *gin.Context) bool {
	return store.IsSuperAdminRole(h.currentRole(c))
}

func normalizeCategoryScope(raw string) string {
	scope := strings.TrimSpace(strings.ToLower(raw))
	if scope == "" {
		return store.CategoryScopeShared
	}
	return scope
}

func categoryCanManage(role string, userID uint, cat *store.Category) bool {
	if cat == nil {
		return false
	}
	if cat.IsPersonal() {
		return cat.OwnerID == userID
	}
	if store.IsSuperAdminRole(role) {
		return true
	}
	if !store.IsPrivilegedRole(role) {
		return false
	}
	if cat.IsRoot() {
		return false
	}
	return cat.CreatedBy == userID
}

func linkCanManage(role string, userID uint, link *store.Link, cat *store.Category) bool {
	if link == nil || cat == nil {
		return false
	}
	if cat.IsPersonal() {
		return cat.OwnerID == userID
	}
	if store.IsSuperAdminRole(role) {
		return true
	}
	if !store.IsPrivilegedRole(role) {
		return false
	}
	if cat.IsRoot() {
		return false
	}
	return cat.CreatedBy == userID
}

func (h *Handler) buildCategoryTree(categories []store.Category, role string, userID uint) []gin.H {
	linksByCategory := make(map[uint][]store.Link)
	for _, cat := range categories {
		links, err := h.store.ListLinksByCategoryID(cat.ID)
		if err != nil {
			continue
		}
		linksByCategory[cat.ID] = links
	}

	buildLinks := func(cat store.Category) []gin.H {
		items := linksByCategory[cat.ID]
		out := make([]gin.H, 0, len(items))
		for _, l := range items {
			out = append(out, linkToJSON(l, h.cfg.Upload.URLPrefix, linkCanManage(role, userID, &l, &cat)))
		}
		return out
	}

	buildCategory := func(cat store.Category) gin.H {
		return gin.H{
			"id":         cat.ID,
			"parent_id":  cat.ParentID,
			"name":       cat.Name,
			"scope":      cat.Scope,
			"owner_id":   cat.OwnerID,
			"sort_order": cat.SortOrder,
			"created_by": cat.CreatedBy,
			"can_manage": categoryCanManage(role, userID, &cat),
			"links":      buildLinks(cat),
		}
	}

	childrenByParent := make(map[uint][]store.Category)
	for _, cat := range categories {
		if cat.ParentID == 0 {
			continue
		}
		childrenByParent[cat.ParentID] = append(childrenByParent[cat.ParentID], cat)
	}

	out := make([]gin.H, 0)
	for _, cat := range categories {
		if !cat.IsRoot() {
			continue
		}
		children := childrenByParent[cat.ID]
		childOut := make([]gin.H, 0, len(children))
		totalLinks := len(linksByCategory[cat.ID])
		for _, child := range children {
			childLinks := linksByCategory[child.ID]
			totalLinks += len(childLinks)
			childOut = append(childOut, buildCategory(child))
		}
		root := buildCategory(cat)
		root["children"] = childOut
		root["link_count"] = totalLinks
		out = append(out, root)
	}
	return out
}

func (h *Handler) listCategories(c *gin.Context) {
	userID := h.currentUserID(c)
	role := h.currentRole(c)

	sharedCategories, err := h.store.ListCategoriesByScope(store.CategoryScopeShared, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	personalCategories, err := h.store.ListCategoriesByScope(store.CategoryScopePersonal, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"categories":          h.buildCategoryTree(sharedCategories, role, userID),
		"personal_categories": h.buildCategoryTree(personalCategories, role, userID),
		"is_admin":            h.isPrivileged(c),
		"is_super_admin":      h.isSuperAdmin(c),
		"user_id":             userID,
	})
}

type categoryRequest struct {
	Name      string `json:"name"`
	SortOrder int    `json:"sort_order"`
	ParentID  uint   `json:"parent_id"`
	Scope     string `json:"scope"`
}

func (h *Handler) createCategory(c *gin.Context) {
	var req categoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "分类名称不能为空"})
		return
	}

	userID := h.currentUserID(c)
	role := h.currentRole(c)
	scope := normalizeCategoryScope(req.Scope)
	ownerID := uint(0)

	switch scope {
	case store.CategoryScopePersonal:
		ownerID = userID
		if req.ParentID == 0 {
			break
		}
		parent, err := h.store.GetCategory(req.ParentID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "父分类不存在"})
			return
		}
		if !parent.IsPersonal() || parent.OwnerID != userID {
			c.JSON(http.StatusForbidden, gin.H{"error": "无权在该分类下创建子分类"})
			return
		}
		if !parent.IsRoot() {
			c.JSON(http.StatusBadRequest, gin.H{"error": "只能在顶级分类下创建子分类"})
			return
		}
	case store.CategoryScopeShared:
		if req.ParentID == 0 {
			if !store.IsSuperAdminRole(role) {
				c.JSON(http.StatusForbidden, gin.H{"error": "请联系超级管理员创建大分类"})
				return
			}
		} else {
			if !store.IsPrivilegedRole(role) {
				c.JSON(http.StatusForbidden, gin.H{"error": "需要管理员权限"})
				return
			}
			parent, err := h.store.GetCategory(req.ParentID)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "父分类不存在"})
				return
			}
			if parent.IsPersonal() {
				c.JSON(http.StatusBadRequest, gin.H{"error": "不能在个人空间分类下创建共享分类"})
				return
			}
			if !parent.IsRoot() {
				c.JSON(http.StatusBadRequest, gin.H{"error": "只能在顶级分类下创建子分类"})
				return
			}
		}
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的分类范围"})
		return
	}

	if existing, err := h.store.GetCategoryByParentAndName(req.ParentID, name, scope, ownerID); err == nil && existing != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "同级分类名称已存在"})
		return
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	item := &store.Category{
		Name:      name,
		SortOrder: req.SortOrder,
		ParentID:  req.ParentID,
		Scope:     scope,
		OwnerID:   ownerID,
		CreatedBy: userID,
	}
	if err := h.store.CreateCategory(item); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.writeAudit(c, "create_category", "success", fmt.Sprintf("created category %s scope=%s parent=%d", name, scope, req.ParentID))
	c.JSON(http.StatusCreated, gin.H{"id": item.ID, "message": "created"})
}

func (h *Handler) updateCategory(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req categoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "分类名称不能为空"})
		return
	}

	cat, err := h.store.GetCategory(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "分类不存在"})
		return
	}
	if cat.IsPersonal() && cat.OwnerID != h.currentUserID(c) {
		c.JSON(http.StatusNotFound, gin.H{"error": "分类不存在"})
		return
	}
	if !categoryCanManage(h.currentRole(c), h.currentUserID(c), cat) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权编辑该分类"})
		return
	}

	ownerID := cat.OwnerID
	if !cat.IsPersonal() {
		ownerID = 0
	}
	if existing, err := h.store.GetCategoryByParentAndName(cat.ParentID, name, cat.Scope, ownerID); err == nil && existing.ID != id {
		c.JSON(http.StatusBadRequest, gin.H{"error": "同级分类名称已存在"})
		return
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	item := &store.Category{ID: id, Name: name, SortOrder: req.SortOrder}
	if err := h.store.UpdateCategory(item); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.writeAudit(c, "update_category", "success", fmt.Sprintf("updated category %s", name))
	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

func (h *Handler) deleteCategory(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	cat, err := h.store.GetCategory(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "分类不存在"})
		return
	}
	if cat.IsPersonal() && cat.OwnerID != h.currentUserID(c) {
		c.JSON(http.StatusNotFound, gin.H{"error": "分类不存在"})
		return
	}
	if !categoryCanManage(h.currentRole(c), h.currentUserID(c), cat) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权删除该分类"})
		return
	}
	if err := h.store.DeleteCategory(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.writeAudit(c, "delete_category", "success", fmt.Sprintf("deleted category %s", cat.Name))
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

type linkRequest struct {
	CategoryID uint   `json:"category_id"`
	Name       string `json:"name"`
	URL        string `json:"url"`
	IconPath   string `json:"icon_path"`
	Maintainer string `json:"maintainer"`
	Remark     string `json:"remark"`
	SortOrder  int    `json:"sort_order"`
}

func (h *Handler) ensureLinkCategoryManageable(c *gin.Context, categoryID uint) (*store.Category, error) {
	cat, err := h.store.GetCategory(categoryID)
	if err != nil {
		return nil, err
	}
	if cat.IsPersonal() && cat.OwnerID != h.currentUserID(c) {
		return nil, gorm.ErrRecordNotFound
	}
	if !linkCanManage(h.currentRole(c), h.currentUserID(c), &store.Link{CategoryID: categoryID}, cat) {
		return nil, errForbiddenLink
	}
	return cat, nil
}

var errForbiddenLink = errMsg("无权操作该分类下的链接")

func (h *Handler) createLink(c *gin.Context) {
	var req linkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if err := validateLinkRequest(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if _, err := h.ensureLinkCategoryManageable(c, req.CategoryID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "分类不存在"})
			return
		}
		if err == errForbiddenLink {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	item := &store.Link{
		CategoryID: req.CategoryID,
		Name:       strings.TrimSpace(req.Name),
		URL:        strings.TrimSpace(req.URL),
		IconPath:   strings.TrimSpace(req.IconPath),
		Maintainer: strings.TrimSpace(req.Maintainer),
		Remark:     strings.TrimSpace(req.Remark),
		SortOrder:  req.SortOrder,
		CreatedBy:  h.currentUserID(c),
	}
	if err := h.store.CreateLink(item); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.writeAudit(c, "create_link", "success", fmt.Sprintf("created link %s", item.Name))
	c.JSON(http.StatusCreated, gin.H{"id": item.ID, "message": "created"})
}

func (h *Handler) updateLink(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req linkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if err := validateLinkRequest(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	link, err := h.store.GetLink(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "链接不存在"})
		return
	}
	cat, err := h.store.GetCategory(link.CategoryID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "链接不存在"})
		return
	}
	if cat.IsPersonal() && cat.OwnerID != h.currentUserID(c) {
		c.JSON(http.StatusNotFound, gin.H{"error": "链接不存在"})
		return
	}
	if !linkCanManage(h.currentRole(c), h.currentUserID(c), link, cat) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权编辑该链接"})
		return
	}
	targetCat, err := h.store.GetCategory(req.CategoryID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "分类不存在"})
		return
	}
	if targetCat.IsPersonal() && targetCat.OwnerID != h.currentUserID(c) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "分类不存在"})
		return
	}
	if !linkCanManage(h.currentRole(c), h.currentUserID(c), link, targetCat) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权将链接移动到该分类"})
		return
	}

	item := &store.Link{
		ID:         id,
		CategoryID: req.CategoryID,
		Name:       strings.TrimSpace(req.Name),
		URL:        strings.TrimSpace(req.URL),
		IconPath:   strings.TrimSpace(req.IconPath),
		Maintainer: strings.TrimSpace(req.Maintainer),
		Remark:     strings.TrimSpace(req.Remark),
		SortOrder:  req.SortOrder,
	}
	if err := h.store.UpdateLink(item); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.writeAudit(c, "update_link", "success", fmt.Sprintf("updated link %s", item.Name))
	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

func (h *Handler) deleteLink(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	link, err := h.store.GetLink(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "链接不存在"})
		return
	}
	cat, err := h.store.GetCategory(link.CategoryID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "链接不存在"})
		return
	}
	if cat.IsPersonal() && cat.OwnerID != h.currentUserID(c) {
		c.JSON(http.StatusNotFound, gin.H{"error": "链接不存在"})
		return
	}
	if !linkCanManage(h.currentRole(c), h.currentUserID(c), link, cat) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权删除该链接"})
		return
	}
	if err := h.store.DeleteLink(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.writeAudit(c, "delete_link", "success", fmt.Sprintf("deleted link %s", link.Name))
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func validateLinkRequest(req *linkRequest) error {
	if req.CategoryID == 0 {
		return errMsg("请选择分类")
	}
	if strings.TrimSpace(req.Name) == "" {
		return errMsg("网站名称不能为空")
	}
	url := strings.TrimSpace(req.URL)
	if url == "" {
		return errMsg("网站地址不能为空")
	}
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return errMsg("网站地址需以 http:// 或 https:// 开头")
	}
	return nil
}

type errMsg string

func (e errMsg) Error() string { return string(e) }

func parseID(raw string) (uint, error) {
	n, err := strconv.ParseUint(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		return 0, err
	}
	return uint(n), nil
}
