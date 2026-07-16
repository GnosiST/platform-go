package main

import (
	"encoding/json"
	"fmt"

	"github.com/GnosiST/platform-go/pkg/platform/capability"
)

// This example is intentionally outside the default runtime. A downstream
// repository can define its own manifest and pass it to its composition root.
func main() {
	manifest := capability.Manifest{
		ID:      capability.ID("example.catalog"),
		Name:    "Example catalog",
		Version: "v1",
		Admin: capability.AdminSurface{Resources: []capability.AdminResource{{
			Resource:         "catalog-items",
			Title:            capability.LocalizedText{ZH: "目录项", EN: "Catalog items"},
			PermissionPrefix: "catalog-items",
			Menu: capability.AdminMenu{
				Route: "/catalog-items",
				Icon:  "appstore",
				Order: 100,
			},
		}}},
	}
	encoded, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		panic(err)
	}
	fmt.Println(string(encoded))
}
