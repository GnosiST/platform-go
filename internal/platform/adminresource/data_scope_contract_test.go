package adminresource

import "testing"

func TestDataScopeForPrincipalExportsTrustedRoleScope(t *testing.T) {
	store := NewStore()
	scope := store.DataScopeForPrincipal(store.CurrentPrincipal("ops"))
	if scope.All || len(scope.OrgCodes) != 1 || scope.OrgCodes[0] != "platform-ops" || scope.Self {
		t.Fatalf("operator data scope = %+v, want current platform-ops only", scope)
	}
	if len(scope.ActorIdentifiers) != 2 || scope.ActorIdentifiers[0] != "ops" || scope.ActorIdentifiers[1] != "user-ops" {
		t.Fatalf("operator actor identifiers = %+v, want username and user id", scope.ActorIdentifiers)
	}

	adminScope := store.DataScopeForPrincipal(store.CurrentPrincipal("admin"))
	if !adminScope.All {
		t.Fatalf("admin data scope = %+v, want all", adminScope)
	}
}
