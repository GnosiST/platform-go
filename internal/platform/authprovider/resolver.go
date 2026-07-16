package authprovider

import (
	"strings"

	authoidc "github.com/GnosiST/platform-go/internal/platform/authprovider/oidc"
	authwechat "github.com/GnosiST/platform-go/internal/platform/authprovider/wechat"
	"github.com/GnosiST/platform-go/internal/platform/config"
	"github.com/GnosiST/platform-go/internal/platform/httpapi"
)

func AppIdentityResolverFromConfig(cfg config.Config) (httpapi.AppIdentityResolver, error) {
	if !wechatMiniAppConfigured(cfg) {
		return nil, nil
	}
	return authwechat.NewResolver(authwechat.Config{
		AppID:    cfg.WechatMiniAppID,
		Secret:   cfg.WechatMiniAppSecret,
		Endpoint: cfg.WechatMiniAppCode2SessionEndpoint,
	})
}

func AdminIdentityResolverFromConfig(cfg config.Config) (httpapi.AdminIdentityResolver, error) {
	if !cfg.AdminOIDCConfigured() {
		return nil, nil
	}
	return authoidc.NewResolver(authoidc.Config{
		IssuerURL:    cfg.AdminOIDCIssuerURL,
		ClientID:     cfg.AdminOIDCClientID,
		ClientSecret: cfg.AdminOIDCClientSecret,
		RedirectURL:  cfg.AdminOIDCRedirectURL,
		Scopes:       cfg.AdminOIDCScopes,
		StateKey:     authoidc.DeriveStateKey(cfg.JWTSecret),
	})
}

func wechatMiniAppConfigured(cfg config.Config) bool {
	return strings.TrimSpace(cfg.WechatMiniAppID) != "" && strings.TrimSpace(cfg.WechatMiniAppSecret) != ""
}
