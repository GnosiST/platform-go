package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"platform-go/internal/platform/adminresource"
	"platform-go/internal/platform/cache"
	"platform-go/internal/platform/capability"
	"platform-go/internal/platform/rbac"
)

func TestAdminMenusTargetUsesRevisionAwareCache(t *testing.T) {
	cacheStore := cache.NewMemoryStore(cache.MemoryStoreOptions{})
	resolver := &adminMenuResolverStub{
		revision: AdminMenuRevision{GlobalRevision: 7, RoleRevisions: []AdminMenuRoleRevision{{RoleCode: "operator", Revision: 2}}},
		items:    []adminresource.MenuItem{{Name: "target", Route: "/target"}},
	}
	server := newTestServer(ServerOptions{
		Capabilities: []capability.Manifest{{ID: "tenant"}}, Cache: cacheStore,
		AdminMenuServingMode: AdminMenuServingModeTarget, AdminMenuResolver: resolver,
	})

	for range 2 {
		payload := requestAdminMenus(t, server, "ops")
		if len(payload.Data.Items) != 1 || payload.Data.Items[0].Name != "target" {
			t.Fatalf("target menus = %+v", payload.Data.Items)
		}
	}
	if resolver.revisionCalls != 2 || resolver.resolveCalls != 1 {
		t.Fatalf("resolver calls = revision:%d resolve:%d, want 2/1", resolver.revisionCalls, resolver.resolveCalls)
	}
	principal := rbac.Principal{User: rbac.User{Username: "ops"}}
	if _, ok, err := cacheStore.Get(context.Background(), adminMenusCacheKey(AdminMenuServingModeTarget, resolver.revision, principal)); err != nil || !ok {
		t.Fatalf("target cache ok = %t error = %v", ok, err)
	}

	resolver.revision.GlobalRevision++
	requestAdminMenus(t, server, "ops")
	if resolver.resolveCalls != 2 {
		t.Fatalf("resolve calls after revision change = %d, want 2", resolver.resolveCalls)
	}
}

func TestAdminMenusDualReadReturnsLegacyAndRecordsValueFreeComparison(t *testing.T) {
	resolver := &adminMenuResolverStub{
		revision: AdminMenuRevision{GlobalRevision: 11, RoleRevisions: []AdminMenuRoleRevision{{RoleCode: "operator", Revision: 3}}},
	}
	sink := &adminMenuComparisonSinkStub{}
	server := newTestServer(ServerOptions{
		Capabilities: []capability.Manifest{{ID: "tenant"}}, AdminMenuServingMode: AdminMenuServingModeDualRead,
		AdminMenuResolver: resolver, AdminMenuComparisonSink: sink,
	})
	principal := server.resources.CurrentPrincipal("ops")
	legacy := server.resources.MenuItemsForPrincipal(principal)
	if len(legacy) < 2 {
		t.Fatalf("legacy fixture has %d menus, want at least 2", len(legacy))
	}
	resolver.items = append([]adminresource.MenuItem(nil), legacy[1:]...)
	resolver.items = append(resolver.items, adminresource.MenuItem{Name: "target-only", Route: "/target-only"})

	payload := requestAdminMenus(t, server, "ops")
	responseNames := make([]string, 0, len(payload.Data.Items))
	legacyNames := make([]string, 0, len(legacy))
	for _, item := range payload.Data.Items {
		responseNames = append(responseNames, item.Name)
	}
	for _, item := range legacy {
		legacyNames = append(legacyNames, item.Name)
	}
	if !reflect.DeepEqual(responseNames, legacyNames) {
		t.Fatalf("dual-read menu names = %+v, want legacy %+v", responseNames, legacyNames)
	}
	want := AdminMenuComparison{Equal: false, AddedCount: 1, RemovedCount: 1, GlobalRevision: 11}
	if len(sink.events) != 1 || sink.events[0] != want {
		t.Fatalf("comparison events = %+v, want %+v", sink.events, want)
	}
}

func TestAdminMenusResolverFailureReturnsServiceUnavailableWithoutLegacyFallback(t *testing.T) {
	for _, testCase := range []struct {
		name     string
		mode     AdminMenuServingMode
		resolver *adminMenuResolverStub
	}{
		{name: "target revision", mode: AdminMenuServingModeTarget, resolver: &adminMenuResolverStub{revisionErr: errors.New("revision unavailable")}},
		{name: "target resolve", mode: AdminMenuServingModeTarget, resolver: &adminMenuResolverStub{revision: AdminMenuRevision{GlobalRevision: 1}, resolveErr: errors.New("resolve unavailable")}},
		{name: "dual read resolve", mode: AdminMenuServingModeDualRead, resolver: &adminMenuResolverStub{revision: AdminMenuRevision{GlobalRevision: 1}, resolveErr: errors.New("resolve unavailable")}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			server := newTestServer(ServerOptions{
				Capabilities: []capability.Manifest{{ID: "tenant"}}, AdminMenuServingMode: testCase.mode,
				AdminMenuResolver: testCase.resolver,
			})
			request := httptest.NewRequest(http.MethodGet, "/api/admin/menus", nil)
			request.Header.Set("X-Platform-User", "ops")
			recorder := httptest.NewRecorder()
			server.Router().ServeHTTP(recorder, request)
			wantResolveCalls := 1
			if testCase.resolver.revisionErr != nil {
				wantResolveCalls = 0
			}
			if recorder.Code != http.StatusServiceUnavailable || testCase.resolver.resolveCalls != wantResolveCalls {
				t.Fatalf("status = %d body = %s resolver = %+v", recorder.Code, recorder.Body.String(), testCase.resolver)
			}
		})
	}
}

func TestAdminMenusCacheKeyIsModeRevisionRoleAndPrincipalSpecific(t *testing.T) {
	principal := rbac.Principal{User: rbac.User{Username: "ops"}}
	left := AdminMenuRevision{GlobalRevision: 4, RoleRevisions: []AdminMenuRoleRevision{{RoleCode: "b", Revision: 2}, {RoleCode: "a", Revision: 1}}}
	right := AdminMenuRevision{GlobalRevision: 4, RoleRevisions: []AdminMenuRoleRevision{{RoleCode: "a", Revision: 1}, {RoleCode: "b", Revision: 2}}}
	if adminMenusCacheKey(AdminMenuServingModeTarget, left, principal) != adminMenusCacheKey(AdminMenuServingModeTarget, right, principal) {
		t.Fatal("role revision order changed cache key")
	}
	if adminMenusCacheKey(AdminMenuServingModeTarget, left, principal) == adminMenusCacheKey(AdminMenuServingModeDualRead, left, principal) ||
		adminMenusCacheKey(AdminMenuServingModeTarget, left, principal) == adminMenusCacheKey(AdminMenuServingModeTarget, AdminMenuRevision{GlobalRevision: 5, RoleRevisions: left.RoleRevisions}, principal) {
		t.Fatal("mode or global revision did not change cache key")
	}
}

type adminMenuResolverStub struct {
	revision      AdminMenuRevision
	items         []adminresource.MenuItem
	revisionErr   error
	resolveErr    error
	revisionCalls int
	resolveCalls  int
}

func (r *adminMenuResolverStub) Revision(context.Context, rbac.Principal) (AdminMenuRevision, error) {
	r.revisionCalls++
	return r.revision, r.revisionErr
}

func (r *adminMenuResolverStub) Resolve(_ context.Context, _ rbac.Principal, revision AdminMenuRevision) ([]adminresource.MenuItem, error) {
	r.resolveCalls++
	if revision.GlobalRevision != r.revision.GlobalRevision {
		return nil, errors.New("unexpected revision")
	}
	return append([]adminresource.MenuItem(nil), r.items...), r.resolveErr
}

type adminMenuComparisonSinkStub struct {
	events []AdminMenuComparison
}

func (s *adminMenuComparisonSinkStub) Record(_ context.Context, comparison AdminMenuComparison) {
	s.events = append(s.events, comparison)
}

func requestAdminMenus(t *testing.T, server *Server, username string) adminMenuListTestPayload {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/api/admin/menus", nil)
	request.Header.Set("X-Platform-User", username)
	recorder := httptest.NewRecorder()
	server.Router().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET admin menus status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	var payload adminMenuListTestPayload
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode admin menus: %v body = %s", err, recorder.Body.String())
	}
	return payload
}
