package adminresource

import (
	"context"
	"testing"

	"platform-go/internal/platform/core"

	"gorm.io/gorm"
)

type legacyGORMAdminPermission struct {
	ID          string `gorm:"column:id;primaryKey"`
	Code        string `gorm:"column:code;uniqueIndex;not null"`
	Name        string `gorm:"column:name;not null"`
	Status      string `gorm:"column:status;not null"`
	Description string `gorm:"column:description;not null"`
	UpdatedAt   string `gorm:"column:updated_at;not null"`
	Capability  string `gorm:"column:capability;not null"`
	Resource    string `gorm:"column:resource;not null"`
	Action      string `gorm:"column:action;not null"`
	Prefix      string `gorm:"column:prefix;not null"`
	ValuesJSON  string `gorm:"column:values_json;not null"`
}

func (legacyGORMAdminPermission) TableName() string { return adminPermissionsTable }

func TestGORMPermissionResourceTypeMigrationBackfillsLegacyRowsAsAPI(t *testing.T) {
	db := openAdminResourceGORMDB(t)
	if err := db.AutoMigrate(&legacyGORMAdminPermission{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&legacyGORMAdminPermission{
		ID: "permission-user-read", Code: "admin:user:read", Name: "Read Users", Status: "enabled",
		Description: "Read users.", UpdatedAt: "2026-07-15T00:00:00Z", Capability: "identity",
		Resource: "users", Action: "read", Prefix: "admin:user", ValuesJSON: `{}`,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := NewGORMAdminResourceRepository(context.Background(), db); err != nil {
		t.Fatalf("NewGORMAdminResourceRepository() error = %v", err)
	}
	var permission gormAdminPermission
	if err := db.Where("code = ?", "admin:user:read").Take(&permission).Error; err != nil {
		t.Fatal(err)
	}
	if permission.ResourceType != "api" {
		t.Fatalf("legacy permission resource type = %q, want api", permission.ResourceType)
	}
}

func TestPermissionCatalogAndPersistenceRequireKnownResourceType(t *testing.T) {
	permissions := permissionCatalogFromCapabilities(core.DefaultManifests(), "2026-07-15T00:00:00Z")
	if len(permissions) == 0 {
		t.Fatal("permission catalog is empty")
	}
	for _, permission := range permissions {
		if permission.Values["resourceType"] != "api" {
			t.Fatalf("permission %q resource type = %q, want api", permission.Code, permission.Values["resourceType"])
		}
	}
	db := openAdminResourceGORMDB(t)
	if err := db.AutoMigrate(&gormAdminPermission{}); err != nil {
		t.Fatal(err)
	}
	err := db.Transaction(func(tx *gorm.DB) error {
		return savePermissions(tx, []Record{{
			ID: "permission-invalid", Code: "invalid", Name: "Invalid", Status: "enabled", UpdatedAt: "2026-07-15T00:00:00Z",
			Values: map[string]string{"resourceType": "unknown"},
		}})
	})
	if err == nil {
		t.Fatal("savePermissions(invalid resource type) error = nil")
	}
}
