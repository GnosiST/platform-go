package datasource

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/GnosiST/platform-go/internal/platform/storage"
	"gorm.io/gorm"
)

var (
	ErrInvalidSpec       = errors.New("invalid datasource registry specification")
	ErrCapabilityUnbound = errors.New("capability is not bound to a datasource group")
)

var identifierPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)

// Spec is deliberately reference-only: connection material stays outside the
// registry and is resolved by the process-owned Resolver at startup.
type Spec struct {
	Version     string       `json:"version"`
	Datasources []Datasource `json:"datasources"`
	Groups      []Group      `json:"groups"`
}

type Datasource struct {
	ID        string `json:"id"`
	ConfigRef string `json:"configRef"`
}

type Group struct {
	ID                   string   `json:"id"`
	PrimaryDatasourceID  string   `json:"primaryDatasourceId"`
	ReplicaDatasourceIDs []string `json:"replicaDatasourceIds,omitempty"`
	Capabilities         []string `json:"capabilities"`
}

type Resolver interface {
	ResolveDatasource(context.Context, string) (storage.Config, error)
}

type ResolverFunc func(context.Context, string) (storage.Config, error)

func (f ResolverFunc) ResolveDatasource(ctx context.Context, ref string) (storage.Config, error) {
	return f(ctx, ref)
}

type DB struct {
	*gorm.DB
	datasourceID string
}

func (db *DB) DatasourceID() string {
	if db == nil {
		return ""
	}
	return db.datasourceID
}

type Health struct {
	DatasourceID string `json:"datasourceId"`
	Ready        bool   `json:"ready"`
}

type Runtime struct {
	databases    map[string]*gorm.DB
	capabilities map[string]string
	groups       map[string]groupBinding
}

type groupBinding struct {
	PrimaryDatasourceID  string
	ReplicaDatasourceIDs []string
}

func LoadSpecFile(path string) (Spec, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return Spec{}, fmt.Errorf("read datasource registry: %w", err)
	}
	var spec Spec
	if err := json.Unmarshal(contents, &spec); err != nil {
		return Spec{}, fmt.Errorf("decode datasource registry: %w", err)
	}
	if err := validateSpec(spec); err != nil {
		return Spec{}, err
	}
	return spec, nil
}

func Open(ctx context.Context, spec Spec, resolver Resolver) (*Runtime, error) {
	if err := validateSpec(spec); err != nil {
		return nil, err
	}
	if ctx == nil {
		return nil, fmt.Errorf("%w: context is required", ErrInvalidSpec)
	}
	if resolver == nil {
		return nil, fmt.Errorf("%w: datasource resolver is required", ErrInvalidSpec)
	}

	runtime := &Runtime{databases: make(map[string]*gorm.DB, len(spec.Datasources)), capabilities: capabilityBindings(spec.Groups), groups: groupBindings(spec.Groups)}
	for _, datasource := range spec.Datasources {
		config, err := resolver.ResolveDatasource(ctx, datasource.ConfigRef)
		if err != nil {
			runtime.close()
			return nil, fmt.Errorf("resolve datasource %q: %w", datasource.ID, err)
		}
		db, err := storage.OpenGORM(config)
		if err != nil {
			runtime.close()
			return nil, fmt.Errorf("open datasource %q: %w", datasource.ID, err)
		}
		sqlDB, err := db.DB()
		if err != nil {
			runtime.close()
			return nil, fmt.Errorf("get datasource %q database handle: %w", datasource.ID, err)
		}
		if err := sqlDB.PingContext(ctx); err != nil {
			_ = sqlDB.Close()
			runtime.close()
			return nil, fmt.Errorf("health check datasource %q: %w", datasource.ID, err)
		}
		runtime.databases[datasource.ID] = db
	}
	return runtime, nil
}

func (r *Runtime) DatasourceID(capability string) (string, error) {
	if r == nil {
		return "", ErrCapabilityUnbound
	}
	datasourceID, found := r.capabilities[capability]
	if !found {
		return "", fmt.Errorf("%w: %s", ErrCapabilityUnbound, capability)
	}
	return datasourceID, nil
}

func (r *Runtime) DatasourceIDForGroup(groupID string) (string, error) {
	if r == nil {
		return "", ErrCapabilityUnbound
	}
	group, found := r.groups[groupID]
	if !found {
		return "", fmt.Errorf("%w: datasource group %s", ErrCapabilityUnbound, groupID)
	}
	return group.PrimaryDatasourceID, nil
}

func (r *Runtime) ReplicaDatasourceIDsForGroup(groupID string) ([]string, error) {
	if r == nil {
		return nil, ErrCapabilityUnbound
	}
	group, found := r.groups[groupID]
	if !found {
		return nil, fmt.Errorf("%w: datasource group %s", ErrCapabilityUnbound, groupID)
	}
	return append([]string(nil), group.ReplicaDatasourceIDs...), nil
}

func (r *Runtime) ValidateCapabilityBindings(enabled []string) error {
	if r == nil {
		return fmt.Errorf("%w: runtime is unavailable", ErrInvalidSpec)
	}
	allowed := make(map[string]struct{}, len(enabled))
	for _, capability := range enabled {
		allowed[capability] = struct{}{}
	}
	for capability := range r.capabilities {
		if _, found := allowed[capability]; !found {
			return fmt.Errorf("%w: bound capability %q is not enabled", ErrInvalidSpec, capability)
		}
	}
	return nil
}

func (r *Runtime) DB(capability string) (*DB, error) {
	datasourceID, err := r.DatasourceID(capability)
	if err != nil {
		return nil, err
	}
	db := r.databases[datasourceID]
	if db == nil {
		return nil, fmt.Errorf("%w: bound datasource %q is unavailable", ErrCapabilityUnbound, datasourceID)
	}
	return &DB{DB: db, datasourceID: datasourceID}, nil
}

func (r *Runtime) DBForGroup(groupID string) (*DB, error) {
	datasourceID, err := r.DatasourceIDForGroup(groupID)
	if err != nil {
		return nil, err
	}
	db := r.databases[datasourceID]
	if db == nil {
		return nil, fmt.Errorf("%w: datasource group %q is unavailable", ErrCapabilityUnbound, groupID)
	}
	return &DB{DB: db, datasourceID: datasourceID}, nil
}

func (r *Runtime) Transaction(ctx context.Context, capability string, fn func(*DB) error) error {
	db, err := r.DB(capability)
	if err != nil {
		return err
	}
	return transaction(ctx, db, fn)
}

func (r *Runtime) TransactionForGroup(ctx context.Context, groupID string, fn func(*DB) error) error {
	db, err := r.DBForGroup(groupID)
	if err != nil {
		return err
	}
	return transaction(ctx, db, fn)
}

func transaction(ctx context.Context, db *DB, fn func(*DB) error) error {
	if ctx == nil || db == nil || fn == nil {
		return fmt.Errorf("%w: transaction context, database and callback are required", ErrInvalidSpec)
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&DB{DB: tx, datasourceID: db.datasourceID})
	})
}

func (r *Runtime) Health() []Health {
	if r == nil {
		return nil
	}
	ids := make([]string, 0, len(r.databases))
	for id := range r.databases {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	health := make([]Health, 0, len(ids))
	for _, id := range ids {
		ready := r.databases[id] != nil
		if ready {
			sqlDB, err := r.databases[id].DB()
			ready = err == nil && sqlDB.PingContext(context.Background()) == nil
		}
		health = append(health, Health{DatasourceID: id, Ready: ready})
	}
	return health
}

func (r *Runtime) Close() error {
	if r == nil {
		return nil
	}
	return r.close()
}

func (r *Runtime) close() error {
	var errs []error
	for _, db := range r.databases {
		if db == nil {
			continue
		}
		sqlDB, err := db.DB()
		if err == nil {
			errs = append(errs, sqlDB.Close())
		} else {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func validateSpec(spec Spec) error {
	if spec.Version != "v1" {
		return fmt.Errorf("%w: version must be v1", ErrInvalidSpec)
	}
	if len(spec.Datasources) == 0 || len(spec.Groups) == 0 {
		return fmt.Errorf("%w: datasources and groups are required", ErrInvalidSpec)
	}
	datasources := make(map[string]struct{}, len(spec.Datasources))
	for _, datasource := range spec.Datasources {
		if !validIdentifier(datasource.ID) || !validIdentifier(datasource.ConfigRef) {
			return fmt.Errorf("%w: datasource id and config reference must be canonical identifiers", ErrInvalidSpec)
		}
		if _, found := datasources[datasource.ID]; found {
			return fmt.Errorf("%w: duplicate datasource %q", ErrInvalidSpec, datasource.ID)
		}
		datasources[datasource.ID] = struct{}{}
	}
	groups := make(map[string]struct{}, len(spec.Groups))
	capabilities := map[string]struct{}{}
	for _, group := range spec.Groups {
		if !validIdentifier(group.ID) || !validIdentifier(group.PrimaryDatasourceID) {
			return fmt.Errorf("%w: group identifiers must be canonical", ErrInvalidSpec)
		}
		if _, found := groups[group.ID]; found {
			return fmt.Errorf("%w: duplicate datasource group %q", ErrInvalidSpec, group.ID)
		}
		if _, found := datasources[group.PrimaryDatasourceID]; !found {
			return fmt.Errorf("%w: group %q primary datasource %q is not declared", ErrInvalidSpec, group.ID, group.PrimaryDatasourceID)
		}
		replicas := make(map[string]struct{}, len(group.ReplicaDatasourceIDs))
		for _, replicaID := range group.ReplicaDatasourceIDs {
			if !validIdentifier(replicaID) || replicaID == group.PrimaryDatasourceID {
				return fmt.Errorf("%w: group %q has invalid replica datasource", ErrInvalidSpec, group.ID)
			}
			if _, found := datasources[replicaID]; !found {
				return fmt.Errorf("%w: group %q replica datasource %q is not declared", ErrInvalidSpec, group.ID, replicaID)
			}
			if _, found := replicas[replicaID]; found {
				return fmt.Errorf("%w: group %q repeats replica datasource %q", ErrInvalidSpec, group.ID, replicaID)
			}
			replicas[replicaID] = struct{}{}
		}
		if len(group.Capabilities) == 0 {
			return fmt.Errorf("%w: datasource group %q has no capability bindings", ErrInvalidSpec, group.ID)
		}
		groups[group.ID] = struct{}{}
		for _, capability := range group.Capabilities {
			if !validIdentifier(capability) {
				return fmt.Errorf("%w: capability binding must be a canonical identifier", ErrInvalidSpec)
			}
			if _, found := capabilities[capability]; found {
				return fmt.Errorf("%w: capability %q is bound more than once", ErrInvalidSpec, capability)
			}
			capabilities[capability] = struct{}{}
		}
	}
	return nil
}

func capabilityBindings(groups []Group) map[string]string {
	bindings := make(map[string]string)
	for _, group := range groups {
		for _, capability := range group.Capabilities {
			bindings[capability] = group.PrimaryDatasourceID
		}
	}
	return bindings
}

func groupBindings(groups []Group) map[string]groupBinding {
	bindings := make(map[string]groupBinding, len(groups))
	for _, group := range groups {
		bindings[group.ID] = groupBinding{PrimaryDatasourceID: group.PrimaryDatasourceID, ReplicaDatasourceIDs: append([]string(nil), group.ReplicaDatasourceIDs...)}
	}
	return bindings
}

func validIdentifier(value string) bool {
	return value == strings.TrimSpace(value) && identifierPattern.MatchString(value)
}
