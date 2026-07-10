package authprovider

import (
	"strings"

	authwechat "platform-go/internal/platform/authprovider/wechat"
	"platform-go/internal/platform/config"
	"platform-go/internal/platform/httpapi"
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

func wechatMiniAppConfigured(cfg config.Config) bool {
	return strings.TrimSpace(cfg.WechatMiniAppID) != "" && strings.TrimSpace(cfg.WechatMiniAppSecret) != ""
}
