package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"platform-go/internal/platform/adminresource"
	"platform-go/internal/platform/rbac"
)

type AdminMenuServingMode string

const (
	AdminMenuServingModeLegacy   AdminMenuServingMode = "legacy"
	AdminMenuServingModeDualRead AdminMenuServingMode = "dual-read"
	AdminMenuServingModeTarget   AdminMenuServingMode = "target"
)

type AdminMenuRoleRevision struct {
	RoleCode string
	Revision uint64
}

type AdminMenuRevision struct {
	GlobalRevision uint64
	RoleRevisions  []AdminMenuRoleRevision
}

type AdminMenuResolver interface {
	Revision(context.Context, rbac.Principal) (AdminMenuRevision, error)
	Resolve(context.Context, rbac.Principal, AdminMenuRevision) ([]adminresource.MenuItem, error)
}

type AdminMenuComparison struct {
	Equal          bool
	AddedCount     int
	RemovedCount   int
	GlobalRevision uint64
}

type AdminMenuComparisonSink interface {
	Record(context.Context, rbac.Principal, AdminMenuComparison)
}

type legacyAdminMenuComparisonSink interface {
	Record(context.Context, AdminMenuComparison)
}

type adminMenuServingCacheValue struct {
	Items      []adminresource.MenuItem `json:"items"`
	Comparison *AdminMenuComparison     `json:"comparison,omitempty"`
}

func (s *Server) resolveAdminMenus(ctx context.Context, principal rbac.Principal) ([]adminresource.MenuItem, error) {
	mode := s.adminMenuServingMode
	if mode == "" {
		mode = AdminMenuServingModeLegacy
	}
	revision := AdminMenuRevision{}
	if mode != AdminMenuServingModeLegacy {
		if s.adminMenuResolver == nil {
			return nil, fmt.Errorf("admin menu resolver is required for %s mode", mode)
		}
		var err error
		revision, err = s.adminMenuResolver.Revision(ctx, principal)
		if err != nil {
			return nil, err
		}
	}
	key := adminMenusCacheKey(mode, revision, principal)
	value, err := cachedJSONResult(ctx, s.cache, key, s.cacheTTL, func() (adminMenuServingCacheValue, error) {
		switch mode {
		case AdminMenuServingModeLegacy:
			return adminMenuServingCacheValue{Items: s.resources.MenuItemsForPrincipal(principal)}, nil
		case AdminMenuServingModeDualRead:
			legacy := s.resources.MenuItemsForPrincipal(principal)
			target, err := s.adminMenuResolver.Resolve(ctx, principal, revision)
			if err != nil {
				return adminMenuServingCacheValue{}, err
			}
			comparison := compareAdminMenus(legacy, target, revision.GlobalRevision)
			return adminMenuServingCacheValue{Items: legacy, Comparison: &comparison}, nil
		case AdminMenuServingModeTarget:
			target, err := s.adminMenuResolver.Resolve(ctx, principal, revision)
			if err != nil {
				return adminMenuServingCacheValue{}, err
			}
			return adminMenuServingCacheValue{Items: target}, nil
		default:
			return adminMenuServingCacheValue{}, fmt.Errorf("unsupported admin menu serving mode %q", mode)
		}
	})
	if err != nil {
		return nil, err
	}
	if value.Comparison != nil && s.adminMenuComparisonSink != nil {
		switch sink := s.adminMenuComparisonSink.(type) {
		case AdminMenuComparisonSink:
			sink.Record(ctx, principal, *value.Comparison)
		case legacyAdminMenuComparisonSink:
			sink.Record(ctx, *value.Comparison)
		}
	}
	return value.Items, nil
}

func adminMenusCacheKey(mode AdminMenuServingMode, revision AdminMenuRevision, principal rbac.Principal) string {
	username := strings.TrimSpace(principal.User.Username)
	if username == "" {
		username = "anonymous"
	}
	return adminMenusCacheKeyForUsername(mode, revision, username)
}

func adminMenusCacheKeyForUsername(mode AdminMenuServingMode, revision AdminMenuRevision, username string) string {
	roleRevisions := append([]AdminMenuRoleRevision(nil), revision.RoleRevisions...)
	sort.Slice(roleRevisions, func(i, j int) bool {
		if roleRevisions[i].RoleCode != roleRevisions[j].RoleCode {
			return roleRevisions[i].RoleCode < roleRevisions[j].RoleCode
		}
		return roleRevisions[i].Revision < roleRevisions[j].Revision
	})
	hash := sha256.New()
	for _, role := range roleRevisions {
		_, _ = fmt.Fprintf(hash, "%s=%d\n", strings.TrimSpace(role.RoleCode), role.Revision)
	}
	digest := hex.EncodeToString(hash.Sum(nil))
	return fmt.Sprintf("%s%s:g%d:r%s:%s", cacheKeyMenusPrefix, mode, revision.GlobalRevision, digest, strings.TrimSpace(username))
}

func compareAdminMenus(legacy, target []adminresource.MenuItem, globalRevision uint64) AdminMenuComparison {
	legacyCodes := make(map[string]struct{}, len(legacy))
	targetCodes := make(map[string]struct{}, len(target))
	for _, item := range legacy {
		legacyCodes[item.Name] = struct{}{}
	}
	for _, item := range target {
		targetCodes[item.Name] = struct{}{}
	}
	added := 0
	for code := range targetCodes {
		if _, exists := legacyCodes[code]; !exists {
			added++
		}
	}
	removed := 0
	for code := range legacyCodes {
		if _, exists := targetCodes[code]; !exists {
			removed++
		}
	}
	return AdminMenuComparison{Equal: added == 0 && removed == 0, AddedCount: added, RemovedCount: removed, GlobalRevision: globalRevision}
}
