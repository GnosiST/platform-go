package organizationrbac

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"sort"
	"strings"
	"time"

	"platform-go/internal/platform/adminresource"
	"platform-go/internal/platform/serviceobject"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	menusTable             = "platform_admin_menus"
	roleMenusTable         = "platform_admin_role_menus"
	roleMenuRevisionsTable = "platform_admin_role_menu_revisions"
	pageButtonsTable       = "platform_admin_page_buttons"
)

type gormMenu struct {
	ID                string `gorm:"column:id;size:191;primaryKey"`
	Code              string `gorm:"column:code;size:191;uniqueIndex;not null"`
	Name              string `gorm:"column:name;size:191;not null;default:''"`
	Status            string `gorm:"column:status;size:32;index;not null;default:'disabled'"`
	Description       string `gorm:"column:description;type:text;not null;default:''"`
	UpdatedAt         string `gorm:"column:updated_at;size:35;not null;default:''"`
	NodeType          string `gorm:"column:node_type;size:32;index;not null;default:'page'"`
	ParentCode        string `gorm:"column:parent_code;size:191;index;not null;default:''"`
	Route             string `gorm:"column:route;size:512;not null;default:''"`
	ComponentKey      string `gorm:"column:component_key;size:191;not null;default:''"`
	ResourceCode      string `gorm:"column:resource_code;size:191;not null;default:''"`
	External          bool   `gorm:"column:is_external;not null;default:false"`
	ExternalURL       string `gorm:"column:external_url;size:2048;not null;default:''"`
	OpenMode          string `gorm:"column:open_mode;size:32;not null;default:''"`
	ParametersJSON    string `gorm:"column:parameters_json;type:text;not null;default:'[]'"`
	CacheEnabled      bool   `gorm:"column:cache_enabled;not null;default:true"`
	Hidden            bool   `gorm:"column:hidden;not null;default:false"`
	ActiveMenuCode    string `gorm:"column:active_menu_code;size:191;not null;default:''"`
	BreadcrumbVisible bool   `gorm:"column:breadcrumb_visible;not null;default:true"`
	Icon              string `gorm:"column:icon;size:191;not null;default:''"`
	SortOrder         int    `gorm:"column:sort_order;index;not null;default:0"`
	TitleZH           string `gorm:"column:title_zh;size:191;not null;default:''"`
	TitleEN           string `gorm:"column:title_en;size:191;not null;default:''"`
	DescriptionZH     string `gorm:"column:description_zh;type:text;not null;default:''"`
	DescriptionEN     string `gorm:"column:description_en;type:text;not null;default:''"`
	LegacyPermission  string `gorm:"column:permission;size:191;not null;default:''"`
	Parent            string `gorm:"column:parent;size:191;not null;default:''"`
	Resource          string `gorm:"column:resource;size:191;not null;default:''"`
	Group             string `gorm:"column:group_name;size:191;not null;default:''"`
	ValuesJSON        string `gorm:"column:values_json;type:text;not null;default:'{}'"`
}

type gormRoleMenu struct {
	RoleCode  string    `gorm:"column:role_code;size:191;primaryKey"`
	MenuCode  string    `gorm:"column:menu_code;size:191;primaryKey"`
	Revision  uint64    `gorm:"column:revision;not null"`
	ActorID   string    `gorm:"column:actor_id;size:191;not null"`
	CreatedAt time.Time `gorm:"column:created_at;not null"`
	UpdatedAt time.Time `gorm:"column:updated_at;not null"`
}

type gormRoleMenuRevision struct {
	RoleCode  string    `gorm:"column:role_code;size:191;primaryKey"`
	Revision  uint64    `gorm:"column:revision;not null"`
	UpdatedAt time.Time `gorm:"column:updated_at;not null"`
}

type gormPageButton struct {
	MenuCode       string `gorm:"column:menu_code;size:191;primaryKey"`
	ButtonKey      string `gorm:"column:button_key;size:191;primaryKey"`
	LabelZH        string `gorm:"column:label_zh;size:191;not null"`
	LabelEN        string `gorm:"column:label_en;size:191;not null"`
	Action         string `gorm:"column:action;size:191;not null"`
	SortOrder      int    `gorm:"column:sort_order;index;not null"`
	Status         string `gorm:"column:status;size:32;index;not null"`
	PermissionCode string `gorm:"column:permission_code;size:191;uniqueIndex;not null"`
}

func (gormMenu) TableName() string             { return menusTable }
func (gormRoleMenu) TableName() string         { return roleMenusTable }
func (gormRoleMenuRevision) TableName() string { return roleMenuRevisionsTable }
func (gormPageButton) TableName() string       { return pageButtonsTable }

type MenuPermission struct {
	Code         string
	Status       string
	ResourceType string
	MenuCode     string
	ButtonKey    string
	Action       string
}

type RoleMenuSet struct {
	RoleCode  string
	MenuCodes []string
	Revision  uint64
}

type ReplaceRoleMenusRequest struct {
	RoleCode         string
	MenuCodes        []string
	ExpectedRevision uint64
	ActorID          string
	ChangedAt        time.Time
}

type MenuDefinition struct {
	ID          string
	Name        string
	Description string
	UpdatedAt   string
	Node        MenuNode
	Buttons     []PageButton
}

type ReplaceMenuDefinitionRequest struct {
	Definition       MenuDefinition
	ExpectedRevision uint64
	ActorID          string
	ChangedAt        time.Time
}

type RoleMenuImpact struct {
	RoleCode          string
	TenantCode        string
	CurrentMenuCodes  []string
	ProposedMenuCodes []string
	ExpectedRevision  uint64
	Changed           bool
}

func ValidateMenuSnapshot(nodes []MenuNode, buttons []PageButton, permissions []MenuPermission) error {
	byCode := make(map[string]MenuNode, len(nodes))
	for _, node := range nodes {
		node.Code = strings.TrimSpace(node.Code)
		node.ParentCode = strings.TrimSpace(node.ParentCode)
		if !validCode(node.Code) || strings.TrimSpace(node.TitleZH) == "" || strings.TrimSpace(node.TitleEN) == "" ||
			(node.Status != StatusEnabled && node.Status != "disabled") {
			return ErrInvalid
		}
		if _, duplicate := byCode[node.Code]; duplicate {
			return ErrInvalid
		}
		switch node.NodeType {
		case MenuNodeTypeDirectory:
			if node.Route != "" || node.ComponentKey != "" || node.ResourceCode != "" || node.External || node.ExternalURL != "" || node.OpenMode != "" || len(node.Parameters) != 0 {
				return ErrInvalid
			}
		case MenuNodeTypePage:
			if err := validatePageNode(node); err != nil {
				return err
			}
		default:
			return ErrInvalid
		}
		byCode[node.Code] = node
	}
	for _, node := range nodes {
		if node.ParentCode == "" {
			// Root nodes still need active-menu validation below.
		} else {
			parent, exists := byCode[node.ParentCode]
			legacyMissingParent := !exists && node.LegacyPermission != ""
			if !legacyMissingParent && (!exists || parent.NodeType != MenuNodeTypeDirectory || node.ParentCode == node.Code) {
				return ErrInvalid
			}
		}
		activeMenuCode := strings.TrimSpace(node.ActiveMenuCode)
		if activeMenuCode != "" {
			target, exists := byCode[activeMenuCode]
			if !exists || activeMenuCode == node.Code || target.NodeType != MenuNodeTypePage || target.Status != StatusEnabled {
				return ErrInvalid
			}
		}
	}
	if menuTreeHasCycle(byCode) {
		return ErrInvalid
	}

	buttonByPermission := make(map[string]PageButton, len(buttons))
	buttonKeys := make(map[string]struct{}, len(buttons))
	for _, button := range buttons {
		menu, exists := byCode[strings.TrimSpace(button.MenuCode)]
		key := strings.TrimSpace(button.MenuCode) + "\x00" + strings.TrimSpace(button.ButtonKey)
		if !exists || menu.NodeType != MenuNodeTypePage || !validCode(button.ButtonKey) || strings.TrimSpace(button.LabelZH) == "" ||
			strings.TrimSpace(button.LabelEN) == "" || !validCode(button.Action) || !validCode(button.PermissionCode) ||
			(button.Status != StatusEnabled && button.Status != "disabled") {
			return ErrInvalid
		}
		if _, duplicate := buttonKeys[key]; duplicate {
			return ErrInvalid
		}
		if _, duplicate := buttonByPermission[button.PermissionCode]; duplicate {
			return ErrInvalid
		}
		buttonKeys[key] = struct{}{}
		buttonByPermission[button.PermissionCode] = button
	}
	matched := make(map[string]struct{}, len(buttonByPermission))
	for _, permission := range permissions {
		if permission.ResourceType != PermissionResourceTypePageButton {
			continue
		}
		button, exists := buttonByPermission[permission.Code]
		if !exists || permission.Status != button.Status || permission.MenuCode != button.MenuCode ||
			permission.ButtonKey != button.ButtonKey || permission.Action != button.Action {
			return ErrInvalid
		}
		matched[permission.Code] = struct{}{}
	}
	if len(matched) != len(buttonByPermission) {
		return ErrInvalid
	}
	return nil
}

func validatePageNode(node MenuNode) error {
	if len(node.Parameters) > MaximumMenuParameters {
		return ErrInvalid
	}
	parameterKeys := make(map[string]struct{}, len(node.Parameters))
	for _, parameter := range node.Parameters {
		if !validCode(parameter.Key) || forbiddenMenuParameterKey(parameter.Key) {
			return ErrInvalid
		}
		if _, duplicate := parameterKeys[parameter.Key]; duplicate {
			return ErrInvalid
		}
		parameterKeys[parameter.Key] = struct{}{}
		switch parameter.Type {
		case MenuParameterTypeString:
			value, ok := parameter.Value.(string)
			if !ok || serviceobject.IsForbiddenMenuParameterStringValue(value) {
				return ErrInvalid
			}
		case MenuParameterTypeNumber:
			switch parameter.Value.(type) {
			case float64, float32, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			default:
				return ErrInvalid
			}
		case MenuParameterTypeBoolean:
			if _, ok := parameter.Value.(bool); !ok {
				return ErrInvalid
			}
		default:
			return ErrInvalid
		}
	}
	if node.External {
		parsed, err := url.Parse(strings.TrimSpace(node.ExternalURL))
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" || node.Route != "" || node.ComponentKey != "" || node.ResourceCode != "" ||
			(node.OpenMode != MenuOpenModeSameTab && node.OpenMode != MenuOpenModeNewTab) {
			return ErrInvalid
		}
		return nil
	}
	if !strings.HasPrefix(node.Route, "/") || strings.HasPrefix(node.Route, "//") || strings.ContainsAny(node.Route, "{}*") || strings.Contains(node.Route, ":") || strings.TrimSpace(node.ComponentKey) == "" ||
		node.ExternalURL != "" || node.OpenMode != "" {
		return ErrInvalid
	}
	return nil
}

func forbiddenMenuParameterKey(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "datasource", "shard", "database", "schema", "sql", "script", "expression", "route-template", "physical-routing":
		return true
	default:
		return false
	}
}

func (r *GORMRepository) ReplaceMenuDefinition(ctx context.Context, request ReplaceMenuDefinitionRequest) (uint64, error) {
	definition := request.Definition
	if !r.ready(ctx) || strings.TrimSpace(definition.ID) == "" || strings.TrimSpace(definition.Name) == "" ||
		!validCode(definition.Node.Code) || !validCode(request.ActorID) || request.ChangedAt.IsZero() {
		return 0, ErrInvalid
	}
	for index := range definition.Buttons {
		if definition.Buttons[index].MenuCode == "" {
			definition.Buttons[index].MenuCode = definition.Node.Code
		}
	}
	var committed uint64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		currentRevision, err := loadGlobalRevision(tx)
		if err != nil {
			return err
		}
		if currentRevision != request.ExpectedRevision {
			return &RevisionConflictError{Expected: request.ExpectedRevision, Actual: currentRevision}
		}
		if err := validateMenuDefinitionIdentity(tx, definition); err != nil {
			return err
		}
		nodes, buttons, permissions, err := loadMenuValidationSnapshot(tx, definition)
		if err != nil {
			return err
		}
		if err := ValidateMenuSnapshot(nodes, buttons, permissions); err != nil {
			return err
		}
		changed, err := menuDefinitionChanged(tx, definition)
		if err != nil {
			return err
		}
		if !changed {
			committed = currentRevision
			return nil
		}
		row, err := gormMenuFromDefinition(definition, request.ChangedAt)
		if err != nil {
			return err
		}
		cacheEnabled := row.CacheEnabled
		if err := tx.Select("*").Clauses(clause.OnConflict{UpdateAll: true}).Create(&row).Error; err != nil {
			return repositoryError(err)
		}
		if err := tx.Model(&gormMenu{}).Where("code = ?", row.Code).UpdateColumn("cache_enabled", cacheEnabled).Error; err != nil {
			return repositoryError(err)
		}
		if err := replaceMenuButtonsAndPermissions(tx, definition.Node.Code, definition.Buttons); err != nil {
			return err
		}
		committed, err = advanceGlobalRevision(tx, request.ExpectedRevision)
		return err
	})
	if err != nil {
		return 0, repositoryError(err)
	}
	return committed, nil
}

func validateMenuDefinitionIdentity(db *gorm.DB, definition MenuDefinition) error {
	var byCode gormMenu
	err := db.Where("code = ?", definition.Node.Code).Take(&byCode).Error
	if err == nil && byCode.ID != definition.ID {
		return ErrInvalid
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return repositoryError(err)
	}
	var byID gormMenu
	err = db.Where("id = ?", definition.ID).Take(&byID).Error
	if err == nil && byID.Code != definition.Node.Code {
		return ErrInvalid
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return repositoryError(err)
	}
	return nil
}

func (r *GORMRepository) LoadMenuDefinition(ctx context.Context, menuCode string) (MenuDefinition, uint64, error) {
	if !r.ready(ctx) || !validCode(menuCode) {
		return MenuDefinition{}, 0, ErrInvalid
	}
	db := r.db.WithContext(ctx)
	var row gormMenu
	if err := db.Where("code = ?", menuCode).Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return MenuDefinition{}, 0, ErrNotFound
		}
		return MenuDefinition{}, 0, repositoryError(err)
	}
	node, err := menuNodeFromGORM(row)
	if err != nil {
		return MenuDefinition{}, 0, err
	}
	var buttonRows []gormPageButton
	if err := db.Where("menu_code = ?", menuCode).Order("sort_order, button_key").Find(&buttonRows).Error; err != nil {
		return MenuDefinition{}, 0, repositoryError(err)
	}
	buttons := make([]PageButton, 0, len(buttonRows))
	for _, button := range buttonRows {
		buttons = append(buttons, pageButtonFromGORM(button))
	}
	revision, err := loadGlobalRevision(db)
	if err != nil {
		return MenuDefinition{}, 0, err
	}
	return MenuDefinition{ID: row.ID, Name: row.Name, Description: row.Description, UpdatedAt: row.UpdatedAt, Node: node, Buttons: buttons}, revision, nil
}

func menuDefinitionChanged(db *gorm.DB, definition MenuDefinition) (bool, error) {
	var current gormMenu
	err := db.Where("code = ?", definition.Node.Code).Take(&current).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return true, nil
	}
	if err != nil {
		return false, repositoryError(err)
	}
	currentNode, err := menuNodeFromGORM(current)
	if err != nil {
		return false, err
	}
	if current.ID != definition.ID || current.Name != definition.Name || current.Description != definition.Description || !reflect.DeepEqual(currentNode, definition.Node) {
		return true, nil
	}
	var rows []gormPageButton
	if err := db.Where("menu_code = ?", definition.Node.Code).Find(&rows).Error; err != nil {
		return false, repositoryError(err)
	}
	currentButtons := make([]PageButton, 0, len(rows))
	for _, row := range rows {
		currentButtons = append(currentButtons, pageButtonFromGORM(row))
	}
	if !reflect.DeepEqual(canonicalPageButtons(currentButtons), canonicalPageButtons(definition.Buttons)) {
		return true, nil
	}
	for _, button := range definition.Buttons {
		var permission gormPermission
		if err := db.Where("code = ?", button.PermissionCode).Take(&permission).Error; err != nil {
			return true, nil
		}
		metadata, err := menuPermissionFromGORM(permission)
		if err != nil || metadata.ResourceType != PermissionResourceTypePageButton || metadata.Status != button.Status ||
			metadata.MenuCode != button.MenuCode || metadata.ButtonKey != button.ButtonKey || metadata.Action != button.Action {
			return true, nil
		}
	}
	var permissionCount int64
	if err := db.Model(&gormPermission{}).Where("resource_type = ? AND resource = ?", PermissionResourceTypePageButton, definition.Node.Code).Count(&permissionCount).Error; err != nil {
		return false, repositoryError(err)
	}
	return permissionCount != int64(len(definition.Buttons)), nil
}

func canonicalPageButtons(buttons []PageButton) []PageButton {
	result := append([]PageButton(nil), buttons...)
	sort.Slice(result, func(i, j int) bool {
		if result[i].MenuCode != result[j].MenuCode {
			return result[i].MenuCode < result[j].MenuCode
		}
		return result[i].ButtonKey < result[j].ButtonKey
	})
	return result
}

func loadMenuValidationSnapshot(db *gorm.DB, definition MenuDefinition) ([]MenuNode, []PageButton, []MenuPermission, error) {
	var menuRows []gormMenu
	if err := db.Order("sort_order, code").Find(&menuRows).Error; err != nil {
		return nil, nil, nil, repositoryError(err)
	}
	var deletedMenuIDs []string
	if err := db.Model(&gormResourceLifecycle{}).
		Where("resource = ? AND deleted_at <> ''", "menus").
		Pluck("record_id", &deletedMenuIDs).Error; err != nil {
		return nil, nil, nil, repositoryError(err)
	}
	deletedMenus := stringSet(deletedMenuIDs)
	nodes := make([]MenuNode, 0, len(menuRows)+1)
	found := false
	for _, row := range menuRows {
		if _, deleted := deletedMenus[row.ID]; deleted {
			if row.Code == definition.Node.Code {
				return nil, nil, nil, ErrInvalid
			}
			continue
		}
		if row.Code == definition.Node.Code {
			nodes = append(nodes, definition.Node)
			found = true
			continue
		}
		node, err := menuNodeFromGORM(row)
		if err != nil {
			return nil, nil, nil, err
		}
		nodes = append(nodes, node)
	}
	if !found {
		nodes = append(nodes, definition.Node)
	}
	var buttonRows []gormPageButton
	if err := db.Where("menu_code <> ?", definition.Node.Code).Order("menu_code, sort_order, button_key").Find(&buttonRows).Error; err != nil {
		return nil, nil, nil, repositoryError(err)
	}
	buttons := make([]PageButton, 0, len(buttonRows)+len(definition.Buttons))
	for _, row := range buttonRows {
		buttons = append(buttons, pageButtonFromGORM(row))
	}
	buttons = append(buttons, definition.Buttons...)
	var permissionRows []gormPermission
	if err := db.Order("code").Find(&permissionRows).Error; err != nil {
		return nil, nil, nil, repositoryError(err)
	}
	permissions := make([]MenuPermission, 0, len(permissionRows)+len(definition.Buttons))
	for _, row := range permissionRows {
		if row.ResourceType == PermissionResourceTypePageButton && row.Resource == definition.Node.Code {
			continue
		}
		permission, err := menuPermissionFromGORM(row)
		if err != nil {
			return nil, nil, nil, err
		}
		permissions = append(permissions, permission)
	}
	for _, button := range definition.Buttons {
		permissions = append(permissions, MenuPermission{Code: button.PermissionCode, Status: button.Status, ResourceType: PermissionResourceTypePageButton, MenuCode: button.MenuCode, ButtonKey: button.ButtonKey, Action: button.Action})
	}
	return nodes, buttons, permissions, nil
}

func replaceMenuButtonsAndPermissions(db *gorm.DB, menuCode string, buttons []PageButton) error {
	var current []gormPageButton
	if err := db.Where("menu_code = ?", menuCode).Find(&current).Error; err != nil {
		return repositoryError(err)
	}
	targetByKey := make(map[string]PageButton, len(buttons))
	for _, button := range buttons {
		targetByKey[button.ButtonKey] = button
	}
	for _, row := range current {
		target, keep := targetByKey[row.ButtonKey]
		if keep && target.PermissionCode == row.PermissionCode {
			continue
		}
		var references int64
		if err := db.Model(&gormRolePermission{}).Where("permission = ?", row.PermissionCode).Count(&references).Error; err != nil {
			return repositoryError(err)
		}
		if references != 0 {
			return ErrRolePoolViolation
		}
		if err := db.Where("code = ? AND resource_type = ?", row.PermissionCode, PermissionResourceTypePageButton).Delete(&gormPermission{}).Error; err != nil {
			return repositoryError(err)
		}
		if !keep {
			if err := db.Where("menu_code = ? AND button_key = ?", menuCode, row.ButtonKey).Delete(&gormPageButton{}).Error; err != nil {
				return repositoryError(err)
			}
		}
	}
	for _, button := range buttons {
		metadata := map[string]string{"menuCode": button.MenuCode, "buttonKey": button.ButtonKey, "action": button.Action}
		encoded, err := json.Marshal(metadata)
		if err != nil {
			return ErrInvalid
		}
		var existing gormPermission
		err = db.Where("code = ?", button.PermissionCode).Take(&existing).Error
		if err == nil && existing.ResourceType != PermissionResourceTypePageButton {
			return ErrInvalid
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return repositoryError(err)
		}
		permissionID := existing.ID
		if permissionID == "" {
			permissionID = "permission-" + strings.NewReplacer(":", "-", "/", "-").Replace(button.PermissionCode)
		}
		permission := gormPermission{ID: permissionID, Code: button.PermissionCode, Name: button.LabelEN, Status: button.Status, Description: button.LabelEN,
			ResourceType: PermissionResourceTypePageButton, Resource: button.MenuCode, Action: button.Action, Prefix: "page:" + button.MenuCode, ValuesJSON: string(encoded)}
		if err := db.Clauses(clause.OnConflict{UpdateAll: true}).Create(&permission).Error; err != nil {
			return repositoryError(err)
		}
		row := gormPageButton{MenuCode: button.MenuCode, ButtonKey: button.ButtonKey, LabelZH: button.LabelZH, LabelEN: button.LabelEN, Action: button.Action, SortOrder: button.SortOrder, Status: button.Status, PermissionCode: button.PermissionCode}
		if err := db.Clauses(clause.OnConflict{UpdateAll: true}).Create(&row).Error; err != nil {
			return repositoryError(err)
		}
	}
	return nil
}

func gormMenuFromDefinition(definition MenuDefinition, changedAt time.Time) (gormMenu, error) {
	parameters, err := json.Marshal(definition.Node.Parameters)
	if err != nil {
		return gormMenu{}, ErrInvalid
	}
	values := map[string]string{
		"nodeType": string(definition.Node.NodeType), "parentCode": definition.Node.ParentCode, "route": definition.Node.Route,
		"componentKey": definition.Node.ComponentKey, "resourceCode": definition.Node.ResourceCode, "external": boolText(definition.Node.External),
		"externalUrl": definition.Node.ExternalURL, "openMode": string(definition.Node.OpenMode), "parameters": string(parameters),
		"cacheEnabled": boolText(definition.Node.CacheEnabled), "hidden": boolText(definition.Node.Hidden), "activeMenuCode": definition.Node.ActiveMenuCode,
		"breadcrumbVisible": boolText(definition.Node.BreadcrumbVisible), "icon": definition.Node.Icon, "sortOrder": fmt.Sprint(definition.Node.SortOrder),
		"titleZh": definition.Node.TitleZH, "titleEn": definition.Node.TitleEN, "descriptionZh": definition.Node.DescriptionZH, "descriptionEn": definition.Node.DescriptionEN,
	}
	valuesJSON, err := json.Marshal(values)
	if err != nil {
		return gormMenu{}, ErrInvalid
	}
	updatedAt := definition.UpdatedAt
	if strings.TrimSpace(updatedAt) == "" {
		updatedAt = changedAt.UTC().Format(time.RFC3339)
	}
	return gormMenu{ID: definition.ID, Code: definition.Node.Code, Name: definition.Name, Status: definition.Node.Status, Description: definition.Description, UpdatedAt: updatedAt,
		NodeType: string(definition.Node.NodeType), ParentCode: definition.Node.ParentCode, Route: definition.Node.Route, ComponentKey: definition.Node.ComponentKey,
		ResourceCode: definition.Node.ResourceCode, External: definition.Node.External, ExternalURL: definition.Node.ExternalURL, OpenMode: string(definition.Node.OpenMode),
		ParametersJSON: string(parameters), CacheEnabled: definition.Node.CacheEnabled, Hidden: definition.Node.Hidden, ActiveMenuCode: definition.Node.ActiveMenuCode,
		BreadcrumbVisible: definition.Node.BreadcrumbVisible, Icon: definition.Node.Icon, SortOrder: definition.Node.SortOrder, TitleZH: definition.Node.TitleZH,
		TitleEN: definition.Node.TitleEN, DescriptionZH: definition.Node.DescriptionZH, DescriptionEN: definition.Node.DescriptionEN,
		Parent: definition.Node.ParentCode, Resource: definition.Node.ResourceCode, ValuesJSON: string(valuesJSON)}, nil
}

func menuNodeFromGORM(row gormMenu) (MenuNode, error) {
	var parameters []MenuParameter
	if strings.TrimSpace(row.ParametersJSON) != "" {
		if err := json.Unmarshal([]byte(row.ParametersJSON), &parameters); err != nil {
			return MenuNode{}, ErrInvalid
		}
	}
	return MenuNode{Code: row.Code, ParentCode: row.ParentCode, NodeType: MenuNodeType(row.NodeType), TitleZH: row.TitleZH, TitleEN: row.TitleEN,
		DescriptionZH: row.DescriptionZH, DescriptionEN: row.DescriptionEN, Status: row.Status, Icon: row.Icon, SortOrder: row.SortOrder,
		Route: row.Route, ComponentKey: row.ComponentKey, ResourceCode: row.ResourceCode, External: row.External, ExternalURL: row.ExternalURL,
		OpenMode: MenuOpenMode(row.OpenMode), Parameters: parameters, CacheEnabled: row.CacheEnabled, Hidden: row.Hidden, ActiveMenuCode: row.ActiveMenuCode,
		BreadcrumbVisible: row.BreadcrumbVisible, LegacyPermission: row.LegacyPermission}, nil
}

func pageButtonFromGORM(row gormPageButton) PageButton {
	return PageButton{MenuCode: row.MenuCode, ButtonKey: row.ButtonKey, LabelZH: row.LabelZH, LabelEN: row.LabelEN, Action: row.Action, SortOrder: row.SortOrder, Status: row.Status, PermissionCode: row.PermissionCode}
}

func menuPermissionFromGORM(row gormPermission) (MenuPermission, error) {
	permission := MenuPermission{Code: row.Code, Status: row.Status, ResourceType: row.ResourceType, Action: row.Action}
	if row.ResourceType != PermissionResourceTypePageButton {
		return permission, nil
	}
	var metadata map[string]string
	if err := json.Unmarshal([]byte(row.ValuesJSON), &metadata); err != nil {
		return MenuPermission{}, ErrInvalid
	}
	permission.MenuCode, permission.ButtonKey, permission.Action = metadata["menuCode"], metadata["buttonKey"], metadata["action"]
	return permission, nil
}

func boolText(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func menuTreeHasCycle(nodes map[string]MenuNode) bool {
	for code := range nodes {
		seen := map[string]struct{}{}
		for current := code; current != ""; current = nodes[current].ParentCode {
			if _, duplicate := seen[current]; duplicate {
				return true
			}
			seen[current] = struct{}{}
			if _, exists := nodes[current]; !exists {
				break
			}
		}
	}
	return false
}

type AdminMenuPermissionSnapshotWriter struct{}

func NewAdminMenuPermissionSnapshotWriter() AdminMenuPermissionSnapshotWriter {
	return AdminMenuPermissionSnapshotWriter{}
}

func (AdminMenuPermissionSnapshotWriter) ApplyMenuPermissionSnapshot(ctx context.Context, tx *gorm.DB, currentMenus, proposedMenus, currentPermissions, proposedPermissions []adminresource.Record) error {
	if ctx == nil || tx == nil {
		return adminresource.ErrInvalidRecord
	}
	if err := validateOwnedSnapshotIdentity(currentMenus, proposedMenus); err != nil {
		return err
	}
	if err := validateOwnedSnapshotIdentity(currentPermissions, proposedPermissions); err != nil {
		return err
	}
	currentMenusByID := make(map[string]adminresource.Record, len(currentMenus))
	for _, record := range currentMenus {
		currentMenusByID[record.ID] = record
	}
	for _, record := range proposedMenus {
		if previous, exists := currentMenusByID[record.ID]; exists {
			if strings.TrimSpace(previous.Values["permission"]) != strings.TrimSpace(record.Values["permission"]) ||
				(strings.TrimSpace(previous.Values["parent"]) != strings.TrimSpace(record.Values["parent"]) && strings.TrimSpace(previous.Values["parentCode"]) == strings.TrimSpace(record.Values["parentCode"])) {
				return adminresource.ErrDomainOwnedMutation
			}
		}
	}
	nodes, buttons, err := menuNodesFromRecords(proposedMenus)
	if err != nil {
		return adminresource.ErrInvalidRecord
	}
	permissions, err := menuPermissionsFromRecords(proposedPermissions)
	if err != nil || ValidateMenuSnapshot(nodes, buttons, permissions) != nil {
		return adminresource.ErrInvalidRecord
	}
	if err := applyMenuRecords(tx.WithContext(ctx), proposedMenus); err != nil {
		return err
	}
	if err := applyPermissionRecords(tx.WithContext(ctx), proposedPermissions); err != nil {
		return err
	}
	return applyPageButtons(tx.WithContext(ctx), buttons)
}

func validateOwnedSnapshotIdentity(current, proposed []adminresource.Record) error {
	currentByID := make(map[string]adminresource.Record, len(current))
	for _, record := range current {
		currentByID[record.ID] = record
	}
	proposedIDs := make(map[string]struct{}, len(proposed))
	proposedCodes := make(map[string]struct{}, len(proposed))
	for _, record := range proposed {
		if strings.TrimSpace(record.ID) == "" || !validCode(record.Code) || strings.TrimSpace(record.Name) == "" || authorizationLifecycleProjectionChanged(adminresource.Record{}, record, false) {
			return adminresource.ErrInvalidRecord
		}
		if _, duplicate := proposedIDs[record.ID]; duplicate {
			return adminresource.ErrInvalidRecord
		}
		if _, duplicate := proposedCodes[record.Code]; duplicate {
			return adminresource.ErrInvalidRecord
		}
		if previous, exists := currentByID[record.ID]; exists && (previous.Code != record.Code || authorizationLifecycleProjectionChanged(previous, record, true)) {
			return adminresource.ErrDomainOwnedMutation
		}
		proposedIDs[record.ID] = struct{}{}
		proposedCodes[record.Code] = struct{}{}
	}
	for _, record := range current {
		if _, exists := proposedIDs[record.ID]; !exists {
			return adminresource.ErrDomainOwnedMutation
		}
	}
	return nil
}

func menuNodesFromRecords(records []adminresource.Record) ([]MenuNode, []PageButton, error) {
	nodes := make([]MenuNode, 0, len(records))
	buttons := make([]PageButton, 0)
	for _, record := range records {
		var parameters []MenuParameter
		if raw := strings.TrimSpace(record.Values["parameters"]); raw != "" {
			if err := json.Unmarshal([]byte(raw), &parameters); err != nil {
				return nil, nil, err
			}
		}
		var nested []PageButton
		if raw := strings.TrimSpace(record.Values["pageButtons"]); raw != "" {
			if err := json.Unmarshal([]byte(raw), &nested); err != nil {
				return nil, nil, err
			}
		}
		parentCode := strings.TrimSpace(record.Values["parentCode"])
		if parentCode == "" {
			parentCode = strings.TrimSpace(record.Values["parent"])
		}
		nodeType := MenuNodeType(strings.TrimSpace(record.Values["nodeType"]))
		if nodeType == "" {
			nodeType = MenuNodeTypePage
		}
		nodes = append(nodes, MenuNode{
			Code: record.Code, ParentCode: parentCode, NodeType: nodeType,
			TitleZH: valueWithFallback(record.Values["titleZh"], record.Values["nameZh"]), TitleEN: valueWithFallback(record.Values["titleEn"], record.Values["nameEn"]),
			DescriptionZH: valueWithFallback(record.Values["descriptionZh"], record.Description), DescriptionEN: valueWithFallback(record.Values["descriptionEn"], record.Description),
			Status: record.Status, Icon: record.Values["icon"], SortOrder: parseInt(record.Values["sortOrder"], record.Values["order"]),
			Route: record.Values["route"], ComponentKey: valueWithFallback(record.Values["componentKey"], record.Values["resource"]), ResourceCode: valueWithFallback(record.Values["resourceCode"], record.Values["resource"]),
			External: parseRecordBool(record.Values["external"], record.Values["isExternal"]), ExternalURL: record.Values["externalUrl"], OpenMode: MenuOpenMode(record.Values["openMode"]),
			Parameters: parameters, CacheEnabled: parseRecordBoolDefault(true, record.Values["cacheEnabled"]), Hidden: parseRecordBool(record.Values["hidden"]),
			ActiveMenuCode: record.Values["activeMenuCode"], BreadcrumbVisible: parseRecordBoolDefault(true, record.Values["breadcrumbVisible"]), LegacyPermission: record.Values["permission"],
		})
		buttons = append(buttons, nested...)
	}
	return nodes, buttons, nil
}

func menuPermissionsFromRecords(records []adminresource.Record) ([]MenuPermission, error) {
	result := make([]MenuPermission, 0, len(records))
	for _, record := range records {
		resourceType := strings.TrimSpace(record.Values["resourceType"])
		if resourceType == "" {
			resourceType = PermissionResourceTypeAPI
		}
		if !validPermissionResourceType(resourceType) {
			return nil, ErrInvalid
		}
		result = append(result, MenuPermission{Code: record.Code, Status: record.Status, ResourceType: resourceType, MenuCode: record.Values["menuCode"], ButtonKey: record.Values["buttonKey"], Action: record.Values["action"]})
	}
	return result, nil
}

func applyMenuRecords(tx *gorm.DB, records []adminresource.Record) error {
	for _, record := range records {
		row, err := gormMenuFromRecord(record)
		if err != nil {
			return adminresource.ErrInvalidRecord
		}
		cacheEnabled := row.CacheEnabled
		if err := tx.Select("*").Clauses(clause.OnConflict{UpdateAll: true}).Create(&row).Error; err != nil {
			return err
		}
		if err := tx.Model(&gormMenu{}).Where("code = ?", row.Code).UpdateColumn("cache_enabled", cacheEnabled).Error; err != nil {
			return err
		}
	}
	return nil
}

func applyPermissionRecords(tx *gorm.DB, records []adminresource.Record) error {
	for _, record := range records {
		valuesJSON, err := json.Marshal(record.Values)
		if err != nil {
			return adminresource.ErrInvalidRecord
		}
		resourceType := strings.TrimSpace(record.Values["resourceType"])
		if resourceType == "" {
			resourceType = PermissionResourceTypeAPI
		}
		row := gormPermission{ID: record.ID, Code: record.Code, Name: record.Name, Status: record.Status, Description: record.Description, UpdatedAt: record.UpdatedAt,
			Capability: record.Values["capability"], Resource: record.Values["resource"], Action: record.Values["action"], Prefix: record.Values["prefix"], ResourceType: resourceType, ValuesJSON: string(valuesJSON)}
		if err := tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}

func applyPageButtons(tx *gorm.DB, buttons []PageButton) error {
	targets := make(map[string]PageButton, len(buttons))
	for _, button := range buttons {
		targets[button.MenuCode+"\x00"+button.ButtonKey] = button
	}
	var current []gormPageButton
	if err := tx.Find(&current).Error; err != nil {
		return err
	}
	for _, row := range current {
		if _, keep := targets[row.MenuCode+"\x00"+row.ButtonKey]; keep {
			continue
		}
		if err := tx.Where("menu_code = ? AND button_key = ?", row.MenuCode, row.ButtonKey).Delete(&gormPageButton{}).Error; err != nil {
			return err
		}
	}
	for _, button := range buttons {
		row := gormPageButton{MenuCode: button.MenuCode, ButtonKey: button.ButtonKey, LabelZH: button.LabelZH, LabelEN: button.LabelEN, Action: button.Action, SortOrder: button.SortOrder, Status: button.Status, PermissionCode: button.PermissionCode}
		if err := tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}

func gormMenuFromRecord(record adminresource.Record) (gormMenu, error) {
	nodes, _, err := menuNodesFromRecords([]adminresource.Record{record})
	if err != nil || len(nodes) != 1 {
		return gormMenu{}, ErrInvalid
	}
	node := nodes[0]
	parameters, err := json.Marshal(node.Parameters)
	if err != nil {
		return gormMenu{}, err
	}
	values, err := json.Marshal(record.Values)
	if err != nil {
		return gormMenu{}, err
	}
	return gormMenu{
		ID: record.ID, Code: record.Code, Name: record.Name, Status: record.Status, Description: record.Description, UpdatedAt: record.UpdatedAt,
		NodeType: string(node.NodeType), ParentCode: node.ParentCode, Route: node.Route, ComponentKey: node.ComponentKey, ResourceCode: node.ResourceCode,
		External: node.External, ExternalURL: node.ExternalURL, OpenMode: string(node.OpenMode), ParametersJSON: string(parameters), CacheEnabled: node.CacheEnabled,
		Hidden: node.Hidden, ActiveMenuCode: node.ActiveMenuCode, BreadcrumbVisible: node.BreadcrumbVisible, Icon: node.Icon, SortOrder: node.SortOrder,
		TitleZH: node.TitleZH, TitleEN: node.TitleEN, DescriptionZH: node.DescriptionZH, DescriptionEN: node.DescriptionEN, LegacyPermission: node.LegacyPermission,
		Parent: node.ParentCode, Resource: valueWithFallback(record.Values["resource"], node.ResourceCode), Group: record.Values["group"], ValuesJSON: string(values),
	}, nil
}

func parseRecordBool(values ...string) bool {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value == "1" || strings.EqualFold(value, "true")
		}
	}
	return false
}

func parseRecordBoolDefault(fallback bool, values ...string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return parseRecordBool(value)
		}
	}
	return fallback
}

func parseInt(values ...string) int {
	for _, value := range values {
		var result int
		if _, err := fmt.Sscan(strings.TrimSpace(value), &result); err == nil {
			return result
		}
	}
	return 0
}

func valueWithFallback(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}
