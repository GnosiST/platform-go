package bootstrap

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/GnosiST/platform-go/internal/platform/adminresource"
	"github.com/GnosiST/platform-go/internal/platform/config"
	"github.com/GnosiST/platform-go/internal/platform/httpapi"
	"github.com/GnosiST/platform-go/internal/platform/kernel"
	"github.com/GnosiST/platform-go/internal/platform/organizationrbac"
	"github.com/GnosiST/platform-go/internal/platform/rbac"
	"github.com/GnosiST/platform-go/internal/platform/serviceobject"
	"github.com/GnosiST/platform-go/internal/platform/storage"

	"gorm.io/gorm"
)

type OrganizationRBAC struct {
	Repository         *organizationrbac.GORMRepository
	ServiceObjects     *serviceobject.Runtime
	AdminMenus         httpapi.AdminMenuResolver
	MenuComparisonSink httpapi.AdminMenuComparisonSink
	close              func() error
}

type OrganizationRBACMigration struct {
	Repository *organizationrbac.GORMRepository
	close      func() error
}

func PrepareOrganizationRBAC(ctx context.Context, cfg config.Config) error {
	db, closeDB, err := openOrganizationRBACDB(cfg)
	if err != nil {
		return err
	}
	defer func() { _ = closeDB() }()
	if _, err := organizationrbac.PrepareGORMRepository(ctx, db); err != nil {
		return err
	}
	_, err = serviceobject.NewGORMIdempotencyStore(ctx, db, serviceobject.GORMIdempotencyStoreOptions{})
	return err
}

func OpenOrganizationRBAC(ctx context.Context, cfg config.Config) (*OrganizationRBAC, error) {
	if cfg.OrganizationRBACMode != config.OrganizationRBACModeTarget {
		return nil, errors.New("organization rbac target mode is required")
	}
	db, closeDB, err := openOrganizationRBACDB(cfg)
	if err != nil {
		return nil, err
	}
	repository, err := organizationrbac.OpenGORMRepository(ctx, db)
	if err != nil {
		_ = closeDB()
		return nil, err
	}
	if _, err := repository.ValidateCutover(ctx); err != nil {
		_ = closeDB()
		return nil, err
	}
	if _, err := repository.ValidateMenuPromotion(ctx, cfg.AdminMenuServingMode, cfg.AdminRoleMenuWriteEnabled); err != nil {
		_ = closeDB()
		return nil, err
	}
	idempotency, err := serviceobject.OpenGORMIdempotencyStore(ctx, db, serviceobject.GORMIdempotencyStoreOptions{})
	if err != nil {
		_ = closeDB()
		return nil, err
	}
	executor, err := organizationrbac.NewServiceObjectExecutorWithOptions(repository, nil, organizationrbac.ServiceObjectExecutorOptions{
		RoleMenuWriteEnabled: cfg.AdminRoleMenuWriteEnabled,
	})
	if err != nil {
		_ = closeDB()
		return nil, err
	}
	registry, err := serviceobject.NewRegistryWithDomainCommands(
		organizationrbac.OrganizationQueryDefinitions(), nil, organizationrbac.OrganizationDomainCommandDefinitions(),
	)
	if err != nil {
		_ = closeDB()
		return nil, err
	}
	runtime, err := serviceobject.NewRuntimeWithDomainCommands(
		registry,
		serviceobject.AuthorizerFunc(func(context.Context, kernel.ExecutionContext, string, string) bool { return false }),
		executor, nil, executor, idempotency,
	)
	if err != nil {
		_ = closeDB()
		return nil, err
	}
	return &OrganizationRBAC{
		Repository: repository, ServiceObjects: runtime,
		AdminMenus:         organizationRBACAdminMenuResolver{repository: repository},
		MenuComparisonSink: organizationRBACMenuComparisonSink{repository: repository}, close: closeDB,
	}, nil
}

type organizationRBACMenuComparisonSink struct {
	repository *organizationrbac.GORMRepository
}

func (s organizationRBACMenuComparisonSink) Record(ctx context.Context, principal rbac.Principal, comparison httpapi.AdminMenuComparison) {
	if s.repository == nil {
		return
	}
	_ = s.repository.RecordMenuDualReadComparison(ctx, strings.TrimSpace(principal.User.Username), comparison)
}

type organizationRBACAdminMenuResolver struct {
	repository *organizationrbac.GORMRepository
}

type adminMenuParameter struct {
	Key   string `json:"key"`
	Type  string `json:"type"`
	Value any    `json:"value"`
}

func (r organizationRBACAdminMenuResolver) Revision(ctx context.Context, principal rbac.Principal) (httpapi.AdminMenuRevision, error) {
	snapshot, err := r.repository.LoadRoleMenuRevisionSnapshot(ctx, principal.Roles)
	if err != nil {
		return httpapi.AdminMenuRevision{}, err
	}
	roleRevisions := make([]httpapi.AdminMenuRoleRevision, 0, len(snapshot.RoleRevisions))
	for _, role := range snapshot.RoleRevisions {
		roleRevisions = append(roleRevisions, httpapi.AdminMenuRoleRevision{RoleCode: role.RoleCode, Revision: role.Revision})
	}
	return httpapi.AdminMenuRevision{GlobalRevision: snapshot.GlobalRevision, RoleRevisions: roleRevisions}, nil
}

func (r organizationRBACAdminMenuResolver) Resolve(ctx context.Context, principal rbac.Principal, expected httpapi.AdminMenuRevision) ([]adminresource.MenuItem, error) {
	resolved, err := r.repository.ResolveRoleMenuNodes(ctx, principal.Roles)
	if err != nil {
		return nil, err
	}
	if resolved.Revision != expected.GlobalRevision {
		return nil, errors.New("organization rbac menu revision changed during resolution")
	}
	return adminMenuItemsFromOrganizationNodes(resolved.Nodes)
}

func adminMenuItemsFromOrganizationNodes(nodes []organizationrbac.MenuNode) ([]adminresource.MenuItem, error) {
	items := make([]adminresource.MenuItem, 0, len(nodes))
	for _, node := range nodes {
		parametersValue := make([]adminMenuParameter, 0, len(node.Parameters))
		for _, parameter := range node.Parameters {
			parametersValue = append(parametersValue, adminMenuParameter{Key: parameter.Key, Type: string(parameter.Type), Value: parameter.Value})
		}
		parameters, err := json.Marshal(parametersValue)
		if err != nil {
			return nil, err
		}
		items = append(items, adminresource.MenuItem{
			Name: node.Code, NodeType: string(node.NodeType), Route: node.Route, Parent: node.ParentCode, ParentCode: node.ParentCode,
			ComponentKey: node.ComponentKey, ResourceCode: node.ResourceCode, IsExternal: node.External, ExternalURL: node.ExternalURL,
			OpenMode: string(node.OpenMode), Parameters: string(parameters), CacheEnabled: node.CacheEnabled, Hidden: node.Hidden,
			ActiveMenuCode: node.ActiveMenuCode, BreadcrumbVisible: node.BreadcrumbVisible, Resource: node.ResourceCode,
			Title:       adminresource.LocalizedText{ZH: node.TitleZH, EN: node.TitleEN},
			Description: adminresource.LocalizedText{ZH: node.DescriptionZH, EN: node.DescriptionEN},
			Icon:        node.Icon, Order: node.SortOrder,
		})
	}
	return items, nil
}

func OpenOrganizationRBACMigration(ctx context.Context, cfg config.Config) (*OrganizationRBACMigration, error) {
	db, closeDB, err := openOrganizationRBACDB(cfg)
	if err != nil {
		return nil, err
	}
	repository, err := organizationrbac.OpenGORMRepository(ctx, db)
	if err != nil {
		_ = closeDB()
		return nil, err
	}
	return &OrganizationRBACMigration{Repository: repository, close: closeDB}, nil
}

func (r *OrganizationRBAC) Close() error {
	if r == nil || r.close == nil {
		return nil
	}
	return r.close()
}

func (r *OrganizationRBACMigration) Close() error {
	if r == nil || r.close == nil {
		return nil
	}
	return r.close()
}

func openOrganizationRBACDB(cfg config.Config) (*gorm.DB, func() error, error) {
	if !isGORMAdminResourceDriver(cfg.AdminResourceDriver) || cfg.AdminResourceDSN == "" || cfg.AdminResourceFile != "" {
		return nil, nil, errors.New("organization rbac requires persistent GORM Admin resource storage")
	}
	db, err := storage.OpenGORM(storage.Config{Driver: cfg.AdminResourceDriver, DSN: cfg.AdminResourceDSN})
	if err != nil {
		return nil, nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, nil, err
	}
	return db, sqlDB.Close, nil
}
