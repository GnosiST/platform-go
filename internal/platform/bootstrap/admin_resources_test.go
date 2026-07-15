package bootstrap

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"platform-go/internal/platform/adminresource"
	"platform-go/internal/platform/capability"
	"platform-go/internal/platform/config"
	"platform-go/internal/platform/core"
	"platform-go/internal/platform/dataprotection"
	"platform-go/internal/platform/organizationrbac"
	"platform-go/internal/platform/storage"
)

func TestAdminResourcesFromConfigUsesMemoryStoreByDefault(t *testing.T) {
	store, err := AdminResourcesFromConfig(config.Config{}, core.DefaultManifests(), nil)
	if err != nil {
		t.Fatalf("AdminResourcesFromConfig() error = %v", err)
	}

	if _, err := store.List("tenants"); err != nil {
		t.Fatalf("List(tenants) error = %v", err)
	}
}

func TestAdminResourcesFromConfigUsesFileBackedStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "admin-resources.json")
	store, err := AdminResourcesFromConfig(config.Config{AdminResourceFile: path}, core.DefaultManifests(), nil)
	if err != nil {
		t.Fatalf("AdminResourcesFromConfig() error = %v", err)
	}
	created, err := store.Create("tenants", adminresource.WriteInput{
		Code:   "persisted",
		Name:   "Persisted Tenant",
		Status: "enabled",
		Values: map[string]string{"isolation": "sandbox"},
	})
	if err != nil {
		t.Fatalf("Create(tenants) error = %v", err)
	}

	reloaded, err := AdminResourcesFromConfig(config.Config{AdminResourceFile: path}, core.DefaultManifests(), nil)
	if err != nil {
		t.Fatalf("reload AdminResourcesFromConfig() error = %v", err)
	}
	tenants, err := reloaded.List("tenants")
	if err != nil {
		t.Fatalf("List(tenants) error = %v", err)
	}
	for _, tenant := range tenants {
		if tenant.ID == created.ID {
			return
		}
	}
	t.Fatalf("reloaded tenants missing %q: %+v", created.ID, tenants)
}

func TestAdminResourcesFromConfigUsesSQLStore(t *testing.T) {
	cfg := config.Config{
		AdminResourceDriver: "platform_admin_resource_test",
		AdminResourceDSN:    "bootstrap-admin-resources",
	}
	store, err := AdminResourcesFromConfig(cfg, core.DefaultManifests(), nil)
	if err != nil {
		t.Fatalf("AdminResourcesFromConfig() error = %v", err)
	}
	if _, err := store.Update("roles", "role-operator", adminresource.WriteInput{
		Name:   "Operator",
		Status: "enabled",
		Values: map[string]string{"groupCode": "operations", "dataScope": "current_org", "permissions": "admin:user:read"},
	}); err != nil {
		t.Fatalf("Update(role-operator) error = %v", err)
	}

	reloaded, err := AdminResourcesFromConfig(cfg, core.DefaultManifests(), nil)
	if err != nil {
		t.Fatalf("reload AdminResourcesFromConfig() error = %v", err)
	}
	menus := reloaded.MenuItemsForPrincipal(reloaded.CurrentPrincipal("ops"))
	for _, menu := range menus {
		if menu.Route == "/users" {
			return
		}
	}
	t.Fatalf("menus after SQL reload = %+v, want /users", menus)
}

func TestAdminResourcesFromConfigUsesGORMStoreForSupportedDrivers(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "admin-resources.db")
	cfg := config.Config{
		AdminResourceDriver: "sqlite",
		AdminResourceDSN:    dsn,
	}
	store, err := AdminResourcesFromConfig(cfg, core.DefaultManifests(), nil)
	if err != nil {
		t.Fatalf("AdminResourcesFromConfig() error = %v", err)
	}
	if _, err := store.Update("roles", "role-operator", adminresource.WriteInput{
		Name:   "Operator",
		Status: "enabled",
		Values: map[string]string{"groupCode": "operations", "dataScope": "current_org", "permissions": "admin:user:read"},
	}); err != nil {
		t.Fatalf("Update(role-operator) error = %v", err)
	}

	reloaded, err := AdminResourcesFromConfig(cfg, core.DefaultManifests(), nil)
	if err != nil {
		t.Fatalf("reload AdminResourcesFromConfig() error = %v", err)
	}
	menus := reloaded.MenuItemsForPrincipal(reloaded.CurrentPrincipal("ops"))
	for _, menu := range menus {
		if menu.Route == "/users" {
			return
		}
	}
	t.Fatalf("menus after GORM reload = %+v, want /users", menus)
}

func TestAdminResourcesFromConfigRequiresPreparedOrganizationRBACTarget(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "organization-rbac.db")
	cfg := config.Config{
		AdminResourceDriver:  "sqlite",
		AdminResourceDSN:     dsn,
		OrganizationRBACMode: config.OrganizationRBACModeTarget,
	}
	if _, err := AdminResourcesFromConfig(cfg, core.DefaultManifests(), nil); err == nil {
		t.Fatal("AdminResourcesFromConfig(unprepared target) error = nil")
	}

	db, err := storage.OpenGORM(storage.Config{Driver: "sqlite", DSN: dsn})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	repository, err := adminresource.NewGORMAdminResourceRepository(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Save(context.Background(), adminresource.ResourceSnapshot{
		NextID: 1000,
		Resources: map[string][]adminresource.Record{
			"org-units": {{
				ID: "org-tenant-hq", Code: "tenant-hq", Name: "Tenant HQ", Status: "enabled",
				Values: map[string]string{"type": "company", "tenantCode": "tenant-a", "sortOrder": "1"},
			}},
			"role-groups": {{
				ID: "group-tenant-ops", Code: "tenant-ops", Name: "Tenant Ops", Status: "enabled",
				Values: map[string]string{"sortOrder": "1"},
			}},
			"roles": {{
				ID: "role-operator", Code: "tenant-operator", Name: "Tenant Operator", Status: "enabled",
				Values: map[string]string{"groupCode": "tenant-ops", "dataScope": "organization", "permissions": "admin:user:read"},
			}},
		},
	}); err != nil {
		t.Fatalf("seed minimal legacy snapshot: %v", err)
	}
	if _, err := organizationrbac.PrepareGORMRepository(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	if err := db.Table("platform_admin_role_groups").Where("code = ?", "tenant-ops").Updates(map[string]any{
		"scope_type": "tenant", "tenant_code": "tenant-a",
	}).Error; err != nil {
		t.Fatal(err)
	}

	store, err := AdminResourcesFromConfig(cfg, core.DefaultManifests(), nil)
	if err != nil {
		t.Fatalf("AdminResourcesFromConfig(prepared target) error = %v", err)
	}
	if _, err := store.Update("roles", "role-operator", adminresource.WriteInput{
		Name: "Changed Outside Domain", Status: "enabled",
		Values: map[string]string{"groupCode": "operations", "dataScope": "all", "permissions": "admin:*"},
	}); !errors.Is(err, adminresource.ErrDomainOwnedMutation) {
		t.Fatalf("Update(domain-owned role) error = %v, want ErrDomainOwnedMutation", err)
	}
	organizationRepository, err := organizationrbac.OpenGORMRepository(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	if report, err := organizationRepository.ValidateCutover(context.Background()); err != nil {
		t.Fatalf("ValidateCutover() error = %v, report = %+v", err, report)
	}
	if _, err := organizationRepository.ReplaceOrgUnitRoleGroups(context.Background(), organizationrbac.ReplaceOrgUnitRoleGroupsRequest{
		OrgUnitCode: "tenant-hq", RoleGroupCodes: []string{"tenant-ops"}, ExpectedRevision: 0, ActorID: "admin", ChangedAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	derived, err := organizationRepository.DeriveAndValidateUser(context.Background(), organizationrbac.User{
		Code: "tenant-user", ScopeType: organizationrbac.ScopeTenant, OrgUnitCode: "tenant-hq", Status: "enabled",
	}, []string{"tenant-operator"})
	if err != nil {
		t.Fatalf("DeriveAndValidateUser(target tenant user) error = %v", err)
	}
	if derived.TenantCode != "tenant-a" {
		t.Fatalf("DeriveAndValidateUser(target tenant user).TenantCode = %q, want tenant-a", derived.TenantCode)
	}
	created, err := store.Create("users", adminresource.WriteInput{
		Code: "tenant-user", Name: "Tenant User", Status: "enabled",
		Values: map[string]string{
			"tenantCode": "tenant-a", "orgUnitCode": "tenant-hq", "roles": "tenant-operator",
		},
	})
	if err != nil {
		t.Fatalf("Create(target tenant user) error = %v", err)
	}
	if _, err := store.Update("users", created.ID, adminresource.WriteInput{
		Code: created.Code, Name: created.Name, Status: created.Status,
		Values: map[string]string{
			"tenantCode": "tenant-a", "orgUnitCode": "tenant-hq", "roles": "",
		},
	}); !errors.Is(err, adminresource.ErrDomainOwnedMutation) {
		t.Fatalf("Update(target user authorization) error = %v, want ErrDomainOwnedMutation", err)
	}
}

func TestAdminResourcesFromConfigRejectsSQLStoreWithoutDSN(t *testing.T) {
	_, err := AdminResourcesFromConfig(config.Config{AdminResourceDriver: "platform_admin_resource_test"}, core.DefaultManifests(), nil)
	if err == nil {
		t.Fatalf("AdminResourcesFromConfig() error = nil, want missing DSN")
	}
	if !strings.Contains(err.Error(), "admin resource dsn is required") {
		t.Fatalf("AdminResourcesFromConfig() error = %v, want missing DSN", err)
	}
}

func TestAdminResourcesFromConfigInjectsProtectionBeforePersistentLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "protected-admin-resources.json")
	cfg := config.Config{AdminResourceFile: path}
	manifests := bootstrapProtectedManifests()
	runtime, err := DataProtectionRuntimeFromConfig(dataProtectionConfigForTest(config.RuntimeEnvironmentTest, dataprotection.ProviderLocalTest))
	if err != nil {
		t.Fatal(err)
	}
	store, err := AdminResourcesFromConfig(cfg, manifests, runtime)
	if err != nil {
		t.Fatalf("AdminResourcesFromConfig() error = %v", err)
	}
	if _, err := store.Create("protected-bootstrap-records", adminresource.WriteInput{
		Code: "protected-1", Name: "Protected", Values: map[string]string{"governmentReference": "bootstrap-secret-marker"},
	}); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(content), "bootstrap-secret-marker") || !strings.Contains(string(content), "pgo:enc:v1:") {
		t.Fatalf("protected repository content = %s", content)
	}
	if _, err := AdminResourcesFromConfig(cfg, manifests, runtime); err != nil {
		t.Fatalf("reload with historical keys error = %v", err)
	}
	replaced, err := DataProtectionRuntimeFromConfig(replacedDataProtectionConfigForTest())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AdminResourcesFromConfig(cfg, manifests, replaced); err == nil {
		t.Fatal("reload with replaced historical key error = nil")
	}
}

func bootstrapProtectedManifests() []capability.Manifest {
	return append(core.DefaultManifests(), capability.Manifest{
		ID: "protected-bootstrap-test",
		Admin: capability.AdminSurface{Resources: []capability.AdminResource{{
			Resource: "protected-bootstrap-records", Title: capability.Text("受保护记录", "Protected Records"), Description: capability.Text("测试。", "Test."),
			PermissionPrefix: "admin:protected-bootstrap", Deletion: &capability.AdminResourceDeletionPolicy{Mode: capability.AdminDeletionSoftDelete, PolicyVersion: 1},
			Protection: &capability.AdminResourceProtection{SchemaVersion: 1, Scope: "global"},
			Fields: []capability.AdminField{
				{Key: "code", Label: capability.Text("编码", "Code"), Type: "text", Source: "record", Required: true, InForm: true},
				{Key: "name", Label: capability.Text("名称", "Name"), Type: "text", Source: "record", Required: true, InForm: true},
				{Key: "governmentReference", Label: capability.Text("政府引用", "Government Reference"), Type: "text", Source: "values", InForm: true,
					Sensitivity: capability.FieldSensitivitySensitive, StorageMode: capability.FieldStorageEncrypted,
					ResponseMode: capability.FieldProjectionPrivileged, ExportMode: capability.FieldProjectionOmitted,
					Protection: &capability.AdminFieldProtection{Format: dataprotection.FormatAES256GCMV1, Normalization: dataprotection.NormalizationRawV1}},
			},
		}}},
	})
}

func replacedDataProtectionConfigForTest() config.Config {
	cfg := dataProtectionConfigForTest(config.RuntimeEnvironmentTest, dataprotection.ProviderLocalTest)
	encoded := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("x", 32)))
	cfg.DataEncryptionKeyringJSON = "{\"enc-v1\":\"" + encoded + "\"}"
	return cfg
}

var bootstrapAdminResourceDriverState sync.Map

func init() {
	sql.Register("platform_admin_resource_test", bootstrapAdminResourceDriver{})
}

type bootstrapAdminResourceDriver struct{}

func (bootstrapAdminResourceDriver) Open(name string) (driver.Conn, error) {
	stateValue, _ := bootstrapAdminResourceDriverState.LoadOrStore(name, &bootstrapAdminResourceState{})
	return &bootstrapAdminResourceConn{state: stateValue.(*bootstrapAdminResourceState)}, nil
}

type bootstrapAdminResourceState struct {
	mu       sync.Mutex
	nextID   string
	revision string
	records  []bootstrapAdminResourceRecord
}

type bootstrapAdminResourceRecord struct {
	Resource    string
	ID          string
	Code        string
	Name        string
	Status      string
	Description string
	UpdatedAt   string
	ValuesJSON  string
}

type bootstrapAdminResourceConn struct {
	state *bootstrapAdminResourceState
}

func (c *bootstrapAdminResourceConn) Prepare(query string) (driver.Stmt, error) {
	return nil, driver.ErrSkip
}

func (c *bootstrapAdminResourceConn) Close() error {
	return nil
}

func (c *bootstrapAdminResourceConn) Begin() (driver.Tx, error) {
	return bootstrapAdminResourceTx{}, nil
}

type bootstrapAdminResourceTx struct{}

func (bootstrapAdminResourceTx) Commit() error   { return nil }
func (bootstrapAdminResourceTx) Rollback() error { return nil }

func (c *bootstrapAdminResourceConn) Exec(query string, args []driver.Value) (driver.Result, error) {
	c.state.mu.Lock()
	defer c.state.mu.Unlock()
	switch {
	case strings.Contains(query, "CREATE TABLE"):
		return driver.RowsAffected(0), nil
	case strings.Contains(query, "DELETE FROM platform_admin_resource_records"):
		c.state.records = nil
		return driver.RowsAffected(1), nil
	case strings.Contains(query, "DELETE FROM platform_admin_resource_lifecycle"):
		return driver.RowsAffected(0), nil
	case strings.Contains(query, "DELETE FROM platform_admin_resource_state"):
		if len(args) > 0 {
			switch fmt.Sprint(args[0]) {
			case "next_id":
				c.state.nextID = ""
			case "revision":
				c.state.revision = ""
			}
		}
		return driver.RowsAffected(1), nil
	case strings.Contains(query, "UPDATE platform_admin_resource_state"):
		if len(args) != 3 || fmt.Sprint(args[1]) != "revision" || c.state.revision != fmt.Sprint(args[2]) {
			return driver.RowsAffected(0), nil
		}
		c.state.revision = fmt.Sprint(args[0])
		return driver.RowsAffected(1), nil
	case strings.Contains(query, "INSERT INTO platform_admin_resource_state"):
		switch fmt.Sprint(args[0]) {
		case "next_id":
			c.state.nextID = fmt.Sprint(args[1])
		case "revision":
			c.state.revision = fmt.Sprint(args[1])
		}
		return driver.RowsAffected(1), nil
	case strings.Contains(query, "INSERT INTO platform_admin_resource_records"):
		c.state.records = append(c.state.records, bootstrapAdminResourceRecord{
			Resource:    fmt.Sprint(args[0]),
			ID:          fmt.Sprint(args[1]),
			Code:        fmt.Sprint(args[2]),
			Name:        fmt.Sprint(args[3]),
			Status:      fmt.Sprint(args[4]),
			Description: fmt.Sprint(args[5]),
			UpdatedAt:   fmt.Sprint(args[6]),
			ValuesJSON:  fmt.Sprint(args[7]),
		})
		return driver.RowsAffected(1), nil
	default:
		return driver.RowsAffected(0), nil
	}
}

func (c *bootstrapAdminResourceConn) Query(query string, args []driver.Value) (driver.Rows, error) {
	c.state.mu.Lock()
	defer c.state.mu.Unlock()
	if strings.Contains(query, "platform_admin_resource_state") {
		values := [][]driver.Value{}
		if len(args) > 0 {
			var value string
			switch fmt.Sprint(args[0]) {
			case "next_id":
				value = c.state.nextID
			case "revision":
				value = c.state.revision
			}
			if value != "" {
				values = append(values, []driver.Value{value})
			}
		}
		return &bootstrapAdminResourceRows{columns: []string{"value"}, values: values}, nil
	}
	if strings.Contains(query, "platform_admin_resource_lifecycle") {
		return &bootstrapAdminResourceRows{
			columns: []string{"resource", "record_id", "deleted_at", "deleted_by", "delete_reason", "purge_after", "deletion_policy_version"},
		}, nil
	}
	values := make([][]driver.Value, 0, len(c.state.records))
	for _, record := range c.state.records {
		values = append(values, []driver.Value{
			record.Resource,
			record.ID,
			record.Code,
			record.Name,
			record.Status,
			record.Description,
			record.UpdatedAt,
			record.ValuesJSON,
		})
	}
	return &bootstrapAdminResourceRows{
		columns: []string{"resource", "id", "code", "name", "status", "description", "updated_at", "values_json"},
		values:  values,
	}, nil
}

type bootstrapAdminResourceRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func (r *bootstrapAdminResourceRows) Columns() []string {
	return r.columns
}

func (r *bootstrapAdminResourceRows) Close() error {
	return nil
}

func (r *bootstrapAdminResourceRows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.index])
	r.index++
	return nil
}
