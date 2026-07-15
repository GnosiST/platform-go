package organizationrbac

import "testing"

func TestMenuModelFreezesExecutableContractVocabulary(t *testing.T) {
	if MenuNodeTypeDirectory != MenuNodeType("directory") || MenuNodeTypePage != MenuNodeType("page") {
		t.Fatalf("menu node types = %q, %q", MenuNodeTypeDirectory, MenuNodeTypePage)
	}
	if MenuParameterTypeString != MenuParameterType("string") ||
		MenuParameterTypeNumber != MenuParameterType("number") ||
		MenuParameterTypeBoolean != MenuParameterType("boolean") {
		t.Fatalf("menu parameter types = %q, %q, %q", MenuParameterTypeString, MenuParameterTypeNumber, MenuParameterTypeBoolean)
	}
	if MenuOpenModeSameTab != MenuOpenMode("same-tab") || MenuOpenModeNewTab != MenuOpenMode("new-tab") {
		t.Fatalf("menu open modes = %q, %q", MenuOpenModeSameTab, MenuOpenModeNewTab)
	}
	if MenuServingModeLegacy != MenuServingMode("legacy") ||
		MenuServingModeDualRead != MenuServingMode("dual-read") ||
		MenuServingModeTarget != MenuServingMode("target") {
		t.Fatalf("menu serving modes = %q, %q, %q", MenuServingModeLegacy, MenuServingModeDualRead, MenuServingModeTarget)
	}
	if MaximumMenuParameters != 32 {
		t.Fatalf("MaximumMenuParameters = %d, want 32", MaximumMenuParameters)
	}
	if RoleMenuStoredNodeType != MenuNodeTypePage {
		t.Fatalf("RoleMenuStoredNodeType = %q, want page", RoleMenuStoredNodeType)
	}
	if DefaultMenuServingMode != MenuServingModeLegacy {
		t.Fatalf("DefaultMenuServingMode = %q, want legacy", DefaultMenuServingMode)
	}
}

func TestMenuModelCarriesTypedPagesButtonsAndPageOnlyBindings(t *testing.T) {
	page := MenuNode{
		Code:         "users",
		ParentCode:   "access",
		NodeType:     MenuNodeTypePage,
		Route:        "/users",
		ComponentKey: "users",
		Parameters: []MenuParameter{
			{Key: "tab", Type: MenuParameterTypeString, Value: "active"},
			{Key: "page", Type: MenuParameterTypeNumber, Value: float64(1)},
			{Key: "compact", Type: MenuParameterTypeBoolean, Value: true},
		},
	}
	button := PageButton{
		MenuCode:       page.Code,
		ButtonKey:      "create",
		LabelZH:        "新建",
		LabelEN:        "Create",
		Action:         "create",
		PermissionCode: "admin:user:create-button",
		SortOrder:      10,
		Status:         StatusEnabled,
	}
	binding := RoleMenuBinding{RoleCode: "tenant-admin", MenuCode: page.Code}

	if page.NodeType != MenuNodeTypePage || len(page.Parameters) != 3 {
		t.Fatalf("page = %+v", page)
	}
	if button.MenuCode != page.Code || button.ButtonKey == "" || button.PermissionCode == "" {
		t.Fatalf("button = %+v", button)
	}
	if binding.MenuCode != page.Code {
		t.Fatalf("binding = %+v", binding)
	}
}
