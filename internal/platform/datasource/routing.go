package datasource

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"
)

type Operation string

const (
	OperationRead  Operation = "read"
	OperationWrite Operation = "write"
)

var ErrInvalidRoute = errors.New("invalid datasource route")

type RoutingPolicy struct {
	StickyWindow      time.Duration
	MaximumReplicaLag time.Duration
}

type RouteRequest struct {
	TenantCode  string
	GroupID     string
	Operation   Operation
	Transaction bool
}

type ReplicaHealth struct {
	Healthy bool
	Lag     time.Duration
}

type ReplicaHealthSource interface {
	Health(context.Context, string) (ReplicaHealth, error)
}

type Router struct {
	runtime *Runtime
	policy  RoutingPolicy
	health  ReplicaHealthSource
	now     func() time.Time

	mu     sync.Mutex
	sticky map[string]time.Time
	next   map[string]uint64
}

func NewRouter(runtime *Runtime, policy RoutingPolicy, health ReplicaHealthSource, now func() time.Time) (*Router, error) {
	if runtime == nil || health == nil || now == nil || policy.StickyWindow <= 0 || policy.MaximumReplicaLag < 0 {
		return nil, ErrInvalidRoute
	}
	return &Router{runtime: runtime, policy: policy, health: health, now: now, sticky: map[string]time.Time{}, next: map[string]uint64{}}, nil
}

func (r *Router) Route(ctx context.Context, request RouteRequest) (*DB, error) {
	if r == nil || ctx == nil || !validIdentifier(request.TenantCode) || !validIdentifier(request.GroupID) || (request.Operation != OperationRead && request.Operation != OperationWrite) {
		return nil, ErrInvalidRoute
	}
	if request.Operation == OperationWrite || request.Transaction || r.isSticky(request.TenantCode, request.GroupID) {
		return r.runtime.DBForGroup(request.GroupID)
	}
	replicas, err := r.runtime.ReplicaDatasourceIDsForGroup(request.GroupID)
	if err != nil {
		return nil, err
	}
	for _, replicaID := range r.rotate(replicas, request.GroupID) {
		health, err := r.health.Health(ctx, replicaID)
		if err != nil || !health.Healthy || health.Lag > r.policy.MaximumReplicaLag {
			continue
		}
		db := r.runtime.databases[replicaID]
		if db != nil {
			return &DB{DB: db, datasourceID: replicaID}, nil
		}
	}
	return r.runtime.DBForGroup(request.GroupID)
}

// RecordWrite is called only after the selected primary transaction commits.
func (r *Router) RecordWrite(tenantCode, groupID string) {
	if r == nil || !validIdentifier(tenantCode) || !validIdentifier(groupID) {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sticky[routeKey(tenantCode, groupID)] = r.now().UTC().Add(r.policy.StickyWindow)
}

func (r *Router) isSticky(tenantCode, groupID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := routeKey(tenantCode, groupID)
	expiresAt, found := r.sticky[key]
	if !found {
		return false
	}
	if !expiresAt.After(r.now().UTC()) {
		delete(r.sticky, key)
		return false
	}
	return true
}

func (r *Router) rotate(values []string, groupID string) []string {
	if len(values) < 2 {
		return append([]string(nil), values...)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	start := int(r.next[groupID] % uint64(len(values)))
	r.next[groupID]++
	rotated := make([]string, 0, len(values))
	rotated = append(rotated, values[start:]...)
	rotated = append(rotated, values[:start]...)
	return rotated
}

func routeKey(tenantCode, groupID string) string {
	return strings.Join([]string{tenantCode, groupID}, "\x00")
}
