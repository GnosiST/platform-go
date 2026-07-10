package capability

import (
	"fmt"
	"sort"
	"strings"
)

type AppRouteContract struct {
	CapabilityID ID
	Method       string
	Path         string
	Auth         AppRouteAuth
	Permission   string
	Description  LocalizedText
}

func ValidateAppSurface(manifests []Manifest) error {
	routes := map[string]ID{}
	for _, manifest := range manifests {
		for _, route := range manifest.App.Routes {
			normalized, err := validateAppRoute(manifest.ID, route)
			if err != nil {
				return err
			}
			if owner, exists := routes[normalized]; exists {
				return fmt.Errorf("capability %q app route %q already registered by capability %q", manifest.ID, normalized, owner)
			}
			routes[normalized] = manifest.ID
		}
	}
	return nil
}

func AppRouteContracts(manifests []Manifest) ([]AppRouteContract, error) {
	if err := ValidateAppSurface(manifests); err != nil {
		return nil, err
	}

	contracts := make([]AppRouteContract, 0)
	for _, manifest := range manifests {
		for _, route := range manifest.App.Routes {
			contracts = append(contracts, AppRouteContract{
				CapabilityID: manifest.ID,
				Method:       strings.ToUpper(strings.TrimSpace(route.Method)),
				Path:         strings.TrimSpace(route.Path),
				Auth:         route.Auth,
				Permission:   strings.TrimSpace(route.Permission),
				Description:  route.Description,
			})
		}
	}

	sort.SliceStable(contracts, func(i, j int) bool {
		if contracts[i].Path != contracts[j].Path {
			return contracts[i].Path < contracts[j].Path
		}
		if contracts[i].Method != contracts[j].Method {
			return contracts[i].Method < contracts[j].Method
		}
		return contracts[i].CapabilityID < contracts[j].CapabilityID
	})
	return contracts, nil
}

func validateAppRoute(owner ID, route AppRoute) (string, error) {
	method := strings.ToUpper(strings.TrimSpace(route.Method))
	path := strings.TrimSpace(route.Path)
	switch {
	case method == "":
		return "", fmt.Errorf("capability %q app route method is required", owner)
	case !validHTTPMethod(method):
		return "", fmt.Errorf("capability %q app route method %q is unsupported", owner, route.Method)
	case path == "":
		return "", fmt.Errorf("capability %q app route path is required", owner)
	case strings.ContainsAny(path, "?#"):
		return "", fmt.Errorf("capability %q app route path must not include query or fragment", owner)
	case !strings.HasPrefix(path, "/api/app/"):
		return "", fmt.Errorf("capability %q app route path must start with /api/app/", owner)
	case route.Auth == "":
		return "", fmt.Errorf("capability %q app route %q auth mode is required", owner, path)
	case route.Auth != AppRouteAuthPublic && route.Auth != AppRouteAuthSession:
		return "", fmt.Errorf("capability %q app route %q auth mode %q is unsupported", owner, path, route.Auth)
	case route.Auth == AppRouteAuthPublic && strings.TrimSpace(route.Permission) != "":
		return "", fmt.Errorf("capability %q app route %q public route cannot declare a permission", owner, path)
	case strings.TrimSpace(route.Permission) != "" && !strings.HasPrefix(strings.TrimSpace(route.Permission), "app:"):
		return "", fmt.Errorf("capability %q app route %q permission must start with app:", owner, path)
	case strings.TrimSpace(route.Permission) != "" && !validAppPermission(strings.TrimSpace(route.Permission)):
		return "", fmt.Errorf("capability %q app route %q permission must match app:<domain>:<action>", owner, path)
	case !hasLocalizedText(route.Description):
		return "", fmt.Errorf("capability %q app route %q description is required", owner, path)
	}
	return method + " " + path, nil
}

func validHTTPMethod(method string) bool {
	switch method {
	case "GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS":
		return true
	default:
		return false
	}
}

func validAppPermission(permission string) bool {
	segments := strings.Split(strings.TrimSpace(permission), ":")
	return len(segments) == 3 &&
		segments[0] == "app" &&
		validAdminPermissionSegment(segments[1]) &&
		validAdminPermissionSegment(segments[2])
}
