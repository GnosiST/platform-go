package capability

import (
	"strings"
	"testing"
)

func TestValidateAuthProviderDeclarationsAcceptsLocalizedProviderWithConfigKeys(t *testing.T) {
	manifests := []Manifest{
		{
			ID: "wechat-login",
			AuthProviders: []AuthProvider{
				{
					ID:          "wechat-miniapp",
					Kind:        "wechat",
					Title:       Text("微信登录", "WeChat Login"),
					Description: Text("微信 code 换取登录态。", "WeChat code exchange login."),
					Enabled:     true,
					ConfigKeys:  []string{"PLATFORM_WECHAT_MINIAPP_APP_ID", "PLATFORM_WECHAT_MINIAPP_SECRET"},
				},
			},
		},
	}

	if err := ValidateAuthProviderDeclarations(manifests); err != nil {
		t.Fatalf("ValidateAuthProviderDeclarations() error = %v", err)
	}
}

func TestValidateAuthProviderDeclarationsRejectsDuplicateProviderIDs(t *testing.T) {
	manifests := []Manifest{
		{
			ID: "session",
			AuthProviders: []AuthProvider{
				{ID: "demo", Kind: "demo", Title: Text("演示登录", "Demo Login"), Description: Text("演示登录。", "Demo login."), Enabled: true, Configured: true},
			},
		},
		{
			ID: "wechat-login",
			AuthProviders: []AuthProvider{
				{ID: " demo ", Kind: "wechat", Title: Text("微信登录", "WeChat Login"), Description: Text("微信登录。", "WeChat login."), Enabled: true},
			},
		},
	}

	err := ValidateAuthProviderDeclarations(manifests)

	if err == nil {
		t.Fatalf("ValidateAuthProviderDeclarations() error = nil, want duplicate provider id error")
	}
}

func TestValidateAuthProviderDeclarationsRejectsMissingKind(t *testing.T) {
	manifests := []Manifest{
		{
			ID: "session",
			AuthProviders: []AuthProvider{
				{ID: "demo", Title: Text("演示登录", "Demo Login"), Description: Text("演示登录。", "Demo login."), Enabled: true, Configured: true},
			},
		},
	}

	err := ValidateAuthProviderDeclarations(manifests)

	if err == nil {
		t.Fatalf("ValidateAuthProviderDeclarations() error = nil, want missing kind error")
	}
}

func TestValidateAuthProviderDeclarationsRejectsInvalidIDOrKind(t *testing.T) {
	for _, provider := range []AuthProvider{
		{ID: "WeChat", Kind: "wechat", Title: Text("微信登录", "WeChat Login"), Description: Text("微信登录。", "WeChat login.")},
		{ID: "wechat", Kind: "OAuth2", Title: Text("微信登录", "WeChat Login"), Description: Text("微信登录。", "WeChat login.")},
	} {
		t.Run(provider.ID+"/"+provider.Kind, func(t *testing.T) {
			err := ValidateAuthProviderDeclarations([]Manifest{{ID: "wechat-login", AuthProviders: []AuthProvider{provider}}})
			if err == nil {
				t.Fatalf("ValidateAuthProviderDeclarations() error = nil, want format error")
			}
			if !strings.Contains(err.Error(), "must use lowercase letters, numbers or hyphens") {
				t.Fatalf("ValidateAuthProviderDeclarations() error = %v, want format error", err)
			}
		})
	}
}

func TestValidateAuthProviderDeclarationsRejectsMissingLocalizedDescription(t *testing.T) {
	provider := AuthProvider{
		ID:          "wechat",
		Kind:        "wechat",
		Title:       Text("微信登录", "WeChat Login"),
		Description: Text("微信登录。", ""),
	}

	err := ValidateAuthProviderDeclarations([]Manifest{{ID: "wechat-login", AuthProviders: []AuthProvider{provider}}})

	if err == nil {
		t.Fatalf("ValidateAuthProviderDeclarations() error = nil, want description error")
	}
	if !strings.Contains(err.Error(), "description is required") {
		t.Fatalf("ValidateAuthProviderDeclarations() error = %v, want description error", err)
	}
}

func TestValidateAuthProviderDeclarationsRejectsInvalidConfigKeys(t *testing.T) {
	for _, keys := range [][]string{
		{""},
		{"platform_wechat_secret"},
		{"PLATFORM_WECHAT_SECRET", " PLATFORM_WECHAT_SECRET "},
		{"PLATFORM-WECHAT-SECRET"},
	} {
		t.Run(strings.Join(keys, ","), func(t *testing.T) {
			provider := AuthProvider{
				ID:          "wechat",
				Kind:        "wechat",
				Title:       Text("微信登录", "WeChat Login"),
				Description: Text("微信登录。", "WeChat login."),
				ConfigKeys:  keys,
			}

			err := ValidateAuthProviderDeclarations([]Manifest{{ID: "wechat-login", AuthProviders: []AuthProvider{provider}}})
			if err == nil {
				t.Fatalf("ValidateAuthProviderDeclarations() error = nil, want config key error")
			}
			if !strings.Contains(err.Error(), "config key") {
				t.Fatalf("ValidateAuthProviderDeclarations() error = %v, want config key error", err)
			}
		})
	}
}
