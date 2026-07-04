package core

import "platform-go/internal/platform/capability"

func DefaultManifests() []capability.Manifest {
	return []capability.Manifest{
		{ID: "tenant", Name: "Tenant", Version: "0.1.0"},
		{ID: "identity", Name: "Identity", Version: "0.1.0", Dependencies: []capability.ID{"tenant"}},
		{ID: "session", Name: "Session", Version: "0.1.0", Dependencies: []capability.ID{"identity"}},
		{ID: "rbac", Name: "RBAC", Version: "0.1.0", Dependencies: []capability.ID{"tenant", "identity"}},
		{ID: "menu", Name: "Menu", Version: "0.1.0", Dependencies: []capability.ID{"rbac"}},
		{ID: "api-resource", Name: "API Resource", Version: "0.1.0", Dependencies: []capability.ID{"rbac"}},
		{ID: "audit", Name: "Audit", Version: "0.1.0", Dependencies: []capability.ID{"tenant", "identity"}},
		{ID: "dictionary", Name: "Dictionary", Version: "0.1.0", Dependencies: []capability.ID{"tenant"}},
		{ID: "parameter", Name: "Parameter", Version: "0.1.0", Dependencies: []capability.ID{"tenant", "audit"}},
		{ID: "admin-shell", Name: "Admin Shell", Version: "0.1.0", Dependencies: []capability.ID{"identity", "session", "rbac", "menu"}},
		{ID: "system-admin", Name: "System Admin", Version: "0.1.0", Dependencies: []capability.ID{"admin-shell", "api-resource", "dictionary", "parameter", "audit"}},
	}
}
