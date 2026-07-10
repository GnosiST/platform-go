package wechat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"platform-go/internal/platform/httpapi"
)

const defaultCode2SessionEndpoint = "https://api.weixin.qq.com/sns/jscode2session"

var ErrMiniAppCredentialsRequired = errors.New("wechat miniapp credentials are required")
var ErrProviderResolveFailed = errors.New("wechat provider resolve failed")

type Config struct {
	AppID      string
	Secret     string
	Endpoint   string
	HTTPClient *http.Client
}

type Resolver struct {
	appID      string
	secret     string
	endpoint   string
	httpClient *http.Client
}

func NewResolver(config Config) (*Resolver, error) {
	appID := strings.TrimSpace(config.AppID)
	secret := strings.TrimSpace(config.Secret)
	if appID == "" || secret == "" {
		return nil, ErrMiniAppCredentialsRequired
	}
	endpoint := strings.TrimSpace(config.Endpoint)
	if endpoint == "" {
		endpoint = defaultCode2SessionEndpoint
	}
	client := config.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	return &Resolver{appID: appID, secret: secret, endpoint: endpoint, httpClient: client}, nil
}

func (r *Resolver) ResolveAppIdentity(ctx context.Context, input httpapi.AppIdentityResolveInput) (httpapi.AppIdentity, error) {
	if input.Provider.Kind != "wechat" {
		return httpapi.AppIdentity{}, httpapi.ErrAppIdentityInvalid
	}
	code := strings.TrimSpace(input.Code)
	if code == "" {
		return httpapi.AppIdentity{}, httpapi.ErrAppIdentityInvalid
	}
	session, err := r.exchangeCode(ctx, code)
	if err != nil {
		return httpapi.AppIdentity{}, err
	}
	if strings.TrimSpace(session.OpenID) == "" {
		return httpapi.AppIdentity{}, httpapi.ErrAppIdentityInvalid
	}
	return httpapi.AppIdentity{
		ProviderSubject:      strings.TrimSpace(session.OpenID),
		ProviderUnionSubject: strings.TrimSpace(session.UnionID),
	}, nil
}

func (r *Resolver) exchangeCode(ctx context.Context, code string) (codeSession, error) {
	endpoint, err := url.Parse(r.endpoint)
	if err != nil {
		return codeSession{}, fmt.Errorf("%w: invalid endpoint", ErrProviderResolveFailed)
	}
	query := endpoint.Query()
	query.Set("appid", r.appID)
	query.Set("secret", r.secret)
	query.Set("js_code", code)
	query.Set("grant_type", "authorization_code")
	endpoint.RawQuery = query.Encode()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return codeSession{}, fmt.Errorf("%w: invalid request", ErrProviderResolveFailed)
	}
	response, err := r.httpClient.Do(request)
	if err != nil {
		return codeSession{}, fmt.Errorf("%w: request failed", ErrProviderResolveFailed)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return codeSession{}, fmt.Errorf("%w: status %d", ErrProviderResolveFailed, response.StatusCode)
	}
	var payload codeSessionPayload
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return codeSession{}, fmt.Errorf("%w: invalid response", ErrProviderResolveFailed)
	}
	if payload.ErrorCode != 0 {
		return codeSession{}, fmt.Errorf("%w: provider code %d", ErrProviderResolveFailed, payload.ErrorCode)
	}
	return codeSession{OpenID: payload.OpenID, UnionID: payload.UnionID}, nil
}

type codeSession struct {
	OpenID  string
	UnionID string
}

type codeSessionPayload struct {
	OpenID       string `json:"openid"`
	UnionID      string `json:"unionid"`
	ErrorCode    int    `json:"errcode"`
	ErrorMessage string `json:"errmsg"`
}
