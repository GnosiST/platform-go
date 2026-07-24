package sharding

import (
	"errors"
	"fmt"

	"github.com/GnosiST/platform-go/internal/platform/kernel"
)

var ErrRouteNotFound = errors.New("shard route not found")

type Route struct {
	TenantCode        string `json:"tenantCode"`
	CapabilityID      string `json:"capabilityId"`
	ShardID           string `json:"shardId"`
	DatasourceGroupID string `json:"datasourceGroupId"`
	Version           uint64 `json:"version"`
}

type Router struct {
	routes map[string]Route
}

func NewRouter(routes []Route) (*Router, error) {
	router := &Router{routes: make(map[string]Route, len(routes))}
	for _, route := range routes {
		if !validIdentifier(route.TenantCode) || !validIdentifier(route.CapabilityID) || !validIdentifier(route.ShardID) || !validIdentifier(route.DatasourceGroupID) || route.Version == 0 {
			return nil, ErrInvalidMigration
		}
		key := routeKey(route.TenantCode, route.CapabilityID)
		if _, found := router.routes[key]; found {
			return nil, ErrInvalidMigration
		}
		router.routes[key] = route
	}
	return router, nil
}

func (r *Router) Resolve(execution kernel.ExecutionContext, capabilityID string) (Route, error) {
	if r == nil || execution.ValidatePermissioned() != nil || execution.TenantScope.PlatformWide || !validIdentifier(execution.TenantScope.TenantCode) || !validIdentifier(capabilityID) {
		return Route{}, ErrRouteNotFound
	}
	route, found := r.routes[routeKey(execution.TenantScope.TenantCode, capabilityID)]
	if !found {
		return Route{}, fmt.Errorf("%w: tenant and capability have no shard route", ErrRouteNotFound)
	}
	return route, nil
}

func routeKey(tenantCode, capabilityID string) string {
	return tenantCode + "\x00" + capabilityID
}
