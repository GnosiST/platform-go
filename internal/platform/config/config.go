package config

import (
	"errors"
	"fmt"
	"mime"
	"net"
	"net/netip"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	RuntimeEnvironment                string
	HTTPAddr                          string
	PublicBaseURL                     string
	TrustedProxies                    []string
	EdgeTrustedProxy                  string
	HTTPMaxBodyBytes                  int64
	Capabilities                      []string
	AdminResourceFile                 string
	AdminResourceDriver               string
	AdminResourceDSN                  string
	SessionFile                       string
	SessionDriver                     string
	SessionDSN                        string
	LifecycleHistoryFile              string
	LifecycleHistoryDriver            string
	LifecycleHistoryDSN               string
	DatabaseDriver                    string
	DatabaseDSN                       string
	OpenAPIFile                       string
	JWTSecret                         string
	CacheDriver                       string
	CacheDefaultTTL                   time.Duration
	RedisAddr                         string
	RedisPassword                     string
	RedisDB                           int
	RateLimitHMACKey                  string
	FileStorageDriver                 string
	FileStorageLocalDir               string
	FileMaxUploadBytes                int64
	FileAllowedMIMETypes              []string
	FileStorageS3Endpoint             string
	FileStorageS3Region               string
	FileStorageS3Bucket               string
	FileStorageS3AccessKey            string
	FileStorageS3SecretKey            string
	FileStorageS3Prefix               string
	FileStorageS3PathStyle            bool
	FileStorageS3ServerSideEncryption string
	FileStorageS3KMSKeyID             string
	WechatMiniAppID                   string
	WechatMiniAppSecret               string
	WechatMiniAppCode2SessionEndpoint string
	AdminOIDCIssuerURL                string
	AdminOIDCClientID                 string
	AdminOIDCClientSecret             string
	AdminOIDCRedirectURL              string
	AdminOIDCScopes                   []string
	DisableDemoAuthProvider           bool
	PhoneHMACKey                      string
	PhoneCodeHMACKey                  string
	PhoneVerificationProvider         string
	filePolicySource                  filePolicySource
	transportPolicySource             transportPolicySource
}

type envConfigState uint8

const (
	envConfigUnknown envConfigState = iota
	envConfigMissing
	envConfigEmpty
	envConfigPresent
	envConfigInvalid
)

type filePolicySource struct {
	loadedFromEnvironment bool
	maxUploadBytes        envConfigState
	allowedMIMETypes      envConfigState
	s3Encryption          envConfigState
}

type transportPolicySource struct {
	loadedFromEnvironment bool
	maxBodyBytes          envConfigState
}

var defaultCapabilities = []string{
	"tenant",
	"identity",
	"session",
	"rbac",
	"menu",
	"api-resource",
	"audit",
	"wechat-login",
	"dictionary",
	"parameter",
	"file-storage",
	"admin-shell",
	"demo-data",
	"system-admin",
}

const (
	RuntimeEnvironmentDevelopment = "development"
	RuntimeEnvironmentTest        = "test"
	RuntimeEnvironmentStaging     = "staging"
	RuntimeEnvironmentProduction  = "production"

	defaultJWTSecret   = "dev-platform-go-secret"
	maxFileUploadBytes = int64(100 << 20)
	maxHTTPBodyBytes   = int64(100 << 20)
)

var defaultFileAllowedMIMETypes = []string{"application/pdf", "image/jpeg", "image/png", "text/plain"}

func Load() Config {
	fileMaxUploadBytes, fileMaxUploadBytesState := int64EnvWithState("PLATFORM_FILE_MAX_UPLOAD_BYTES", 10<<20)
	fileAllowedMIMETypes, fileAllowedMIMETypesState := csvEnvWithState("PLATFORM_FILE_ALLOWED_MIME_TYPES", defaultFileAllowedMIMETypes)
	fileS3Encryption, fileS3EncryptionState := envWithState("PLATFORM_FILE_STORAGE_S3_SERVER_SIDE_ENCRYPTION", "AES256")
	httpMaxBodyBytes, httpMaxBodyBytesState := int64EnvWithState("PLATFORM_HTTP_MAX_BODY_BYTES", 1<<20)
	return Config{
		RuntimeEnvironment:                strings.ToLower(env("PLATFORM_RUNTIME_ENV", RuntimeEnvironmentDevelopment)),
		HTTPAddr:                          env("PLATFORM_HTTP_ADDR", "127.0.0.1:9200"),
		PublicBaseURL:                     env("PLATFORM_PUBLIC_BASE_URL", ""),
		TrustedProxies:                    csvEnv("PLATFORM_TRUSTED_PROXIES", nil),
		EdgeTrustedProxy:                  env("PLATFORM_EDGE_TRUSTED_PROXY", ""),
		HTTPMaxBodyBytes:                  httpMaxBodyBytes,
		Capabilities:                      csvEnv("PLATFORM_CAPABILITIES", defaultCapabilities),
		AdminResourceFile:                 env("PLATFORM_ADMIN_RESOURCE_FILE", ""),
		AdminResourceDriver:               env("PLATFORM_ADMIN_RESOURCE_DRIVER", ""),
		AdminResourceDSN:                  env("PLATFORM_ADMIN_RESOURCE_DSN", ""),
		SessionFile:                       env("PLATFORM_SESSION_FILE", ""),
		SessionDriver:                     env("PLATFORM_SESSION_DRIVER", ""),
		SessionDSN:                        env("PLATFORM_SESSION_DSN", ""),
		LifecycleHistoryFile:              env("PLATFORM_LIFECYCLE_HISTORY_FILE", ""),
		LifecycleHistoryDriver:            env("PLATFORM_LIFECYCLE_HISTORY_DRIVER", ""),
		LifecycleHistoryDSN:               env("PLATFORM_LIFECYCLE_HISTORY_DSN", ""),
		DatabaseDriver:                    env("PLATFORM_DATABASE_DRIVER", "mysql"),
		DatabaseDSN:                       env("PLATFORM_DATABASE_DSN", ""),
		OpenAPIFile:                       env("PLATFORM_OPENAPI_FILE", "resources/generated/openapi.admin.json"),
		JWTSecret:                         env("PLATFORM_JWT_SECRET", defaultJWTSecret),
		CacheDriver:                       env("PLATFORM_CACHE_DRIVER", ""),
		CacheDefaultTTL:                   durationEnv("PLATFORM_CACHE_DEFAULT_TTL", 5*time.Minute),
		RedisAddr:                         env("PLATFORM_REDIS_ADDR", "127.0.0.1:6379"),
		RedisPassword:                     env("PLATFORM_REDIS_PASSWORD", ""),
		RedisDB:                           intEnv("PLATFORM_REDIS_DB", 0),
		RateLimitHMACKey:                  env("PLATFORM_RATE_LIMIT_HMAC_KEY", ""),
		FileStorageDriver:                 env("PLATFORM_FILE_STORAGE_DRIVER", "local"),
		FileStorageLocalDir:               env("PLATFORM_FILE_STORAGE_LOCAL_DIR", ".platform/uploads"),
		FileMaxUploadBytes:                fileMaxUploadBytes,
		FileAllowedMIMETypes:              fileAllowedMIMETypes,
		FileStorageS3Endpoint:             env("PLATFORM_FILE_STORAGE_S3_ENDPOINT", ""),
		FileStorageS3Region:               env("PLATFORM_FILE_STORAGE_S3_REGION", ""),
		FileStorageS3Bucket:               env("PLATFORM_FILE_STORAGE_S3_BUCKET", ""),
		FileStorageS3AccessKey:            env("PLATFORM_FILE_STORAGE_S3_ACCESS_KEY", ""),
		FileStorageS3SecretKey:            env("PLATFORM_FILE_STORAGE_S3_SECRET_KEY", ""),
		FileStorageS3Prefix:               env("PLATFORM_FILE_STORAGE_S3_PREFIX", ""),
		FileStorageS3PathStyle:            boolEnv("PLATFORM_FILE_STORAGE_S3_FORCE_PATH_STYLE", false),
		FileStorageS3ServerSideEncryption: fileS3Encryption,
		FileStorageS3KMSKeyID:             env("PLATFORM_FILE_STORAGE_S3_KMS_KEY_ID", ""),
		WechatMiniAppID:                   env("PLATFORM_WECHAT_MINIAPP_APP_ID", ""),
		WechatMiniAppSecret:               env("PLATFORM_WECHAT_MINIAPP_SECRET", ""),
		WechatMiniAppCode2SessionEndpoint: env("PLATFORM_WECHAT_MINIAPP_CODE2SESSION_ENDPOINT", ""),
		AdminOIDCIssuerURL:                env("PLATFORM_ADMIN_OIDC_ISSUER_URL", ""),
		AdminOIDCClientID:                 env("PLATFORM_ADMIN_OIDC_CLIENT_ID", ""),
		AdminOIDCClientSecret:             env("PLATFORM_ADMIN_OIDC_CLIENT_SECRET", ""),
		AdminOIDCRedirectURL:              env("PLATFORM_ADMIN_OIDC_REDIRECT_URL", ""),
		AdminOIDCScopes:                   csvEnv("PLATFORM_ADMIN_OIDC_SCOPES", []string{"openid", "profile", "email"}),
		DisableDemoAuthProvider:           boolEnv("PLATFORM_DISABLE_DEMO_AUTH_PROVIDER", false),
		PhoneHMACKey:                      env("PLATFORM_PHONE_HMAC_KEY", ""),
		PhoneCodeHMACKey:                  env("PLATFORM_PHONE_CODE_HMAC_KEY", ""),
		PhoneVerificationProvider:         env("PLATFORM_PHONE_VERIFICATION_PROVIDER", ""),
		filePolicySource: filePolicySource{
			loadedFromEnvironment: true,
			maxUploadBytes:        fileMaxUploadBytesState,
			allowedMIMETypes:      fileAllowedMIMETypesState,
			s3Encryption:          fileS3EncryptionState,
		},
		transportPolicySource: transportPolicySource{
			loadedFromEnvironment: true,
			maxBodyBytes:          httpMaxBodyBytesState,
		},
	}
}

func (c Config) AdminOIDCConfigured() bool {
	return strings.TrimSpace(c.AdminOIDCIssuerURL) != "" &&
		strings.TrimSpace(c.AdminOIDCClientID) != "" &&
		strings.TrimSpace(c.AdminOIDCClientSecret) != "" &&
		strings.TrimSpace(c.AdminOIDCRedirectURL) != ""
}

func (c Config) ValidateRuntime() error {
	var errs []error
	environment := strings.ToLower(strings.TrimSpace(c.RuntimeEnvironment))
	if environment == "" {
		environment = RuntimeEnvironmentDevelopment
	}
	switch environment {
	case RuntimeEnvironmentDevelopment, RuntimeEnvironmentTest, RuntimeEnvironmentStaging, RuntimeEnvironmentProduction:
	default:
		errs = append(errs, fmt.Errorf("unsupported runtime environment %q", c.RuntimeEnvironment))
	}

	if strings.TrimSpace(c.HTTPAddr) == "" {
		errs = append(errs, errors.New("http address is required"))
	}
	if c.HTTPMaxBodyBytes < 0 || c.HTTPMaxBodyBytes > maxHTTPBodyBytes || (environment == RuntimeEnvironmentProduction && c.HTTPMaxBodyBytes == 0) {
		errs = append(errs, errors.New("PLATFORM_HTTP_MAX_BODY_BYTES must be between 1 and 104857600 bytes"))
	}
	errs = append(errs, validateTrustedProxies(c.TrustedProxies, environment == RuntimeEnvironmentProduction)...)
	if strings.TrimSpace(c.JWTSecret) == "" {
		errs = append(errs, errors.New("jwt secret is required"))
	}
	if c.CacheDefaultTTL <= 0 {
		errs = append(errs, errors.New("cache default ttl must be positive"))
	}
	errs = append(errs, validateCapabilities(c.Capabilities)...)

	errs = append(errs, validateDriverPair("admin resource", c.AdminResourceDriver, c.AdminResourceDSN)...)
	errs = append(errs, validateDriverPair("session", c.SessionDriver, c.SessionDSN)...)
	errs = append(errs, validateDriverPair("lifecycle history", c.LifecycleHistoryDriver, c.LifecycleHistoryDSN)...)

	if c.CacheDriver != "" && c.CacheDriver != "memory" && c.CacheDriver != "redis" {
		errs = append(errs, fmt.Errorf("unsupported cache driver %q", c.CacheDriver))
	}
	if c.CacheDriver == "redis" && strings.TrimSpace(c.RedisAddr) == "" {
		errs = append(errs, errors.New("redis address is required when cache driver is redis"))
	}

	if c.FileStorageDriver != "" && c.FileStorageDriver != "local" && c.FileStorageDriver != "s3" {
		errs = append(errs, fmt.Errorf("unsupported file storage driver %q", c.FileStorageDriver))
	}
	if c.FileStorageDriver == "local" && strings.TrimSpace(c.FileStorageLocalDir) == "" {
		errs = append(errs, errors.New("file storage local dir is required when file storage driver is local"))
	}
	if c.FileStorageDriver == "s3" {
		if strings.TrimSpace(c.FileStorageS3Region) == "" {
			errs = append(errs, errors.New("file storage s3 region is required when file storage driver is s3"))
		}
		if strings.TrimSpace(c.FileStorageS3Bucket) == "" {
			errs = append(errs, errors.New("file storage s3 bucket is required when file storage driver is s3"))
		}
		if (strings.TrimSpace(c.FileStorageS3AccessKey) == "") != (strings.TrimSpace(c.FileStorageS3SecretKey) == "") {
			errs = append(errs, errors.New("file storage s3 access key and secret key must be configured together"))
		}
		switch strings.TrimSpace(c.FileStorageS3ServerSideEncryption) {
		case "AES256":
			if strings.TrimSpace(c.FileStorageS3KMSKeyID) != "" {
				errs = append(errs, errors.New("file storage s3 KMS key ID requires aws:kms server-side encryption"))
			}
		case "aws:kms":
			if strings.TrimSpace(c.FileStorageS3KMSKeyID) == "" {
				errs = append(errs, errors.New("file storage s3 KMS key ID is required for aws:kms server-side encryption"))
			}
		default:
			errs = append(errs, errors.New("file storage s3 server-side encryption must be AES256 or aws:kms"))
		}
		errs = append(errs, validateObjectStorageEndpoint(environment, c.FileStorageS3Endpoint)...)
	}
	errs = append(errs, validateSecureEndpoint("wechat miniapp code2session endpoint", environment, c.WechatMiniAppCode2SessionEndpoint)...)
	errs = append(errs, validateFileUploadPolicy(c.FileMaxUploadBytes, c.FileAllowedMIMETypes, environment == RuntimeEnvironmentProduction)...)
	if (strings.TrimSpace(c.WechatMiniAppID) == "") != (strings.TrimSpace(c.WechatMiniAppSecret) == "") {
		errs = append(errs, errors.New("wechat miniapp app id and secret must be configured together"))
	}
	errs = append(errs, c.validateAdminOIDC(environment)...)
	errs = append(errs, c.validateAppPhone(environment)...)

	if environment == RuntimeEnvironmentProduction {
		errs = append(errs, validateProductionPublicBaseURL(c.PublicBaseURL)...)
		if c.transportPolicySource.loadedFromEnvironment && (c.transportPolicySource.maxBodyBytes == envConfigMissing || c.transportPolicySource.maxBodyBytes == envConfigEmpty) {
			errs = append(errs, errors.New("production runtime requires PLATFORM_HTTP_MAX_BODY_BYTES to be explicitly configured"))
		}
		if c.filePolicySource.loadedFromEnvironment {
			if c.filePolicySource.maxUploadBytes == envConfigMissing || c.filePolicySource.maxUploadBytes == envConfigEmpty {
				errs = append(errs, errors.New("production runtime requires PLATFORM_FILE_MAX_UPLOAD_BYTES to be explicitly configured"))
			}
			if c.filePolicySource.allowedMIMETypes == envConfigMissing || c.filePolicySource.allowedMIMETypes == envConfigEmpty {
				errs = append(errs, errors.New("production runtime requires PLATFORM_FILE_ALLOWED_MIME_TYPES to be explicitly configured"))
			}
			if c.FileStorageDriver == "s3" && (c.filePolicySource.s3Encryption == envConfigMissing || c.filePolicySource.s3Encryption == envConfigEmpty) {
				errs = append(errs, errors.New("production runtime requires PLATFORM_FILE_STORAGE_S3_SERVER_SIDE_ENCRYPTION to be explicitly configured"))
			}
		}
		errs = append(errs, c.validateProductionRuntime()...)
	}

	return errors.Join(errs...)
}

func (c Config) validateAppPhone(environment string) []error {
	if !hasCapability(c.Capabilities, "app-phone") {
		return nil
	}
	var errs []error
	prefix := "app-phone"
	if environment == RuntimeEnvironmentProduction {
		prefix = "production app-phone"
	}
	if len([]byte(c.PhoneHMACKey)) < 32 {
		errs = append(errs, fmt.Errorf("%s requires PLATFORM_PHONE_HMAC_KEY to be at least 32 bytes", prefix))
	}
	if len([]byte(c.PhoneCodeHMACKey)) < 32 {
		errs = append(errs, fmt.Errorf("%s requires PLATFORM_PHONE_CODE_HMAC_KEY to be at least 32 bytes", prefix))
	}
	if c.PhoneHMACKey == c.PhoneCodeHMACKey {
		errs = append(errs, fmt.Errorf("%s requires distinct phone and code HMAC keys", prefix))
	}
	rawProvider := c.PhoneVerificationProvider
	provider := strings.ToLower(strings.TrimSpace(rawProvider))
	if rawProvider != provider {
		errs = append(errs, fmt.Errorf("%s requires PLATFORM_PHONE_VERIFICATION_PROVIDER to be canonical trimmed lowercase", prefix))
	}
	if provider == "" {
		errs = append(errs, fmt.Errorf("%s requires PLATFORM_PHONE_VERIFICATION_PROVIDER", prefix))
	}
	if provider == "unknown" {
		errs = append(errs, errors.New("PLATFORM_PHONE_VERIFICATION_PROVIDER must identify a configured provider"))
	}
	if provider == "debug" && environment != RuntimeEnvironmentDevelopment && environment != RuntimeEnvironmentTest {
		errs = append(errs, errors.New("app-phone debug provider is allowed only in development or test"))
	}
	return errs
}

func (c Config) validateProductionRuntime() []error {
	var errs []error
	if strings.TrimSpace(c.JWTSecret) == defaultJWTSecret {
		errs = append(errs, errors.New("production runtime requires PLATFORM_JWT_SECRET to be changed from the development default"))
	}
	if len(strings.TrimSpace(c.JWTSecret)) < 32 {
		errs = append(errs, errors.New("production runtime requires PLATFORM_JWT_SECRET to be at least 32 characters"))
	}
	if !isGORMDriver(c.AdminResourceDriver) {
		errs = append(errs, errors.New("production runtime requires PLATFORM_ADMIN_RESOURCE_DRIVER to be mysql, postgres, or sqlite"))
	}
	if !isGORMDriver(c.SessionDriver) {
		errs = append(errs, errors.New("production runtime requires PLATFORM_SESSION_DRIVER to be mysql, postgres, or sqlite"))
	}
	if !isGORMDriver(c.LifecycleHistoryDriver) {
		errs = append(errs, errors.New("production runtime requires PLATFORM_LIFECYCLE_HISTORY_DRIVER to be mysql, postgres, or sqlite"))
	}
	if c.CacheDriver != "redis" {
		errs = append(errs, errors.New("production runtime requires PLATFORM_CACHE_DRIVER=redis"))
	}
	if err := validateEdgeTrustedProxy(c.EdgeTrustedProxy); err != nil {
		errs = append(errs, err)
	}
	if len([]byte(c.RateLimitHMACKey)) < 32 {
		errs = append(errs, errors.New("production runtime requires PLATFORM_RATE_LIMIT_HMAC_KEY to be at least 32 bytes"))
	}
	if c.RateLimitHMACKey != "" && (c.RateLimitHMACKey == c.PhoneHMACKey || c.RateLimitHMACKey == c.PhoneCodeHMACKey) {
		errs = append(errs, errors.New("production runtime requires PLATFORM_RATE_LIMIT_HMAC_KEY to be distinct from phone and code HMAC keys"))
	}
	if hasCapability(c.Capabilities, "demo-data") {
		errs = append(errs, errors.New("production runtime must not enable demo-data capability"))
	}
	if !c.DisableDemoAuthProvider {
		errs = append(errs, errors.New("production runtime requires PLATFORM_DISABLE_DEMO_AUTH_PROVIDER=true"))
	}
	if c.FileStorageDriver == "s3" && strings.TrimSpace(c.FileStorageS3ServerSideEncryption) == "" {
		errs = append(errs, errors.New("production s3 file storage requires server-side encryption"))
	}
	if c.DisableDemoAuthProvider && (!hasCapability(c.Capabilities, "admin-oidc") || !c.AdminOIDCConfigured()) {
		errs = append(errs, errors.New("production runtime requires a configured admin auth provider"))
	}
	return errs
}

func validateEdgeTrustedProxy(raw string) error {
	value := strings.TrimSpace(raw)
	if value == "" || value != raw {
		return errors.New("production runtime requires PLATFORM_EDGE_TRUSTED_PROXY to be one canonical IP address")
	}
	address, err := netip.ParseAddr(value)
	if err != nil || address.String() != value || address.IsUnspecified() || address.IsLoopback() || address.IsMulticast() {
		return errors.New("production runtime requires PLATFORM_EDGE_TRUSTED_PROXY to be one canonical IP address")
	}
	return nil
}

func validateFileUploadPolicy(maxBytes int64, allowedMIMETypes []string, production bool) []error {
	var errs []error
	if maxBytes < 0 || maxBytes > maxFileUploadBytes || (production && maxBytes <= 0) {
		label := "file upload limit must be between 1 and 104857600 bytes when configured"
		if production {
			label = "production runtime requires a positive bounded file upload limit"
		}
		errs = append(errs, errors.New(label))
	}
	if production && len(allowedMIMETypes) == 0 {
		errs = append(errs, errors.New("production runtime requires a non-empty file MIME allowlist"))
	}
	seen := map[string]struct{}{}
	for _, raw := range allowedMIMETypes {
		value := strings.TrimSpace(raw)
		mediaType, params, err := mime.ParseMediaType(value)
		if err != nil || mediaType == "" || len(params) != 0 || value != strings.ToLower(mediaType) {
			errs = append(errs, fmt.Errorf("file MIME allowlist entry %q must be a canonical media type without parameters", raw))
			continue
		}
		if _, exists := seen[mediaType]; exists {
			errs = append(errs, fmt.Errorf("file MIME allowlist contains duplicate %q", mediaType))
		}
		seen[mediaType] = struct{}{}
	}
	return errs
}

func validateObjectStorageEndpoint(environment string, raw string) []error {
	return validateSecureEndpoint("file storage s3 endpoint", environment, raw)
}

func validateSecureEndpoint(label string, environment string, raw string) []error {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil
	}
	endpoint, err := url.Parse(value)
	if err != nil || endpoint.Hostname() == "" {
		return []error{fmt.Errorf("%s must be an absolute URL", label)}
	}
	if endpoint.Scheme == "https" {
		return nil
	}
	if endpoint.Scheme == "http" && (environment == RuntimeEnvironmentDevelopment || environment == RuntimeEnvironmentTest) && isLoopbackHost(endpoint.Hostname()) {
		return nil
	}
	return []error{fmt.Errorf("%s must use https outside loopback development and test", label)}
}

func (c Config) validateAdminOIDC(environment string) []error {
	values := []string{c.AdminOIDCIssuerURL, c.AdminOIDCClientID, c.AdminOIDCClientSecret, c.AdminOIDCRedirectURL}
	configured := 0
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			configured++
		}
	}
	if configured == 0 {
		return nil
	}
	if configured != len(values) {
		return []error{errors.New("admin oidc issuer, client id, client secret, and redirect url must be configured together")}
	}

	var errs []error
	errs = append(errs, validateSecureEndpoint("admin oidc issuer url", environment, c.AdminOIDCIssuerURL)...)
	if !containsString(c.AdminOIDCScopes, "openid") {
		errs = append(errs, errors.New("admin oidc scopes must include openid"))
	}
	redirectURL, err := url.Parse(strings.TrimSpace(c.AdminOIDCRedirectURL))
	if err != nil || redirectURL.Hostname() == "" {
		return append(errs, errors.New("admin oidc redirect url must be an absolute URL"))
	}
	if redirectURL.Scheme == "https" {
		return errs
	}
	if redirectURL.Scheme == "http" && (environment == RuntimeEnvironmentDevelopment || environment == RuntimeEnvironmentTest) && isLoopbackHost(redirectURL.Hostname()) {
		return errs
	}
	if environment == RuntimeEnvironmentProduction {
		return append(errs, errors.New("production admin oidc redirect url must use https"))
	}
	return append(errs, errors.New("admin oidc redirect url must use https except for loopback development or test redirects"))
}

func validateProductionPublicBaseURL(raw string) []error {
	value := strings.TrimSpace(raw)
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Opaque != "" || parsed.Path != "" {
		return []error{errors.New("production runtime requires PLATFORM_PUBLIC_BASE_URL to be an absolute HTTPS origin")}
	}
	return nil
}

func validateTrustedProxies(values []string, production bool) []error {
	if production && len(values) == 0 {
		return []error{errors.New("production runtime requires a non-empty PLATFORM_TRUSTED_PROXIES policy")}
	}
	var errs []error
	seen := map[netip.Prefix]struct{}{}
	coverage := map[int]*prefixCoverageNode{
		32:  {},
		128: {},
	}
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		var prefix netip.Prefix
		if parsed, err := netip.ParsePrefix(value); err == nil {
			prefix = parsed.Masked()
		} else if address, addressErr := netip.ParseAddr(value); addressErr == nil {
			prefix = netip.PrefixFrom(address, address.BitLen())
		} else {
			errs = append(errs, fmt.Errorf("PLATFORM_TRUSTED_PROXIES entry %q must be an IP address or CIDR", raw))
			continue
		}
		if prefix.Bits() == 0 {
			errs = append(errs, errors.New("PLATFORM_TRUSTED_PROXIES must not trust all addresses"))
			continue
		}
		if _, exists := seen[prefix]; exists {
			errs = append(errs, fmt.Errorf("PLATFORM_TRUSTED_PROXIES contains duplicate %q", prefix.String()))
		}
		seen[prefix] = struct{}{}
		coverage[prefix.Addr().BitLen()].insert(prefix)
	}
	if coverage[32].covered {
		errs = append(errs, errors.New("PLATFORM_TRUSTED_PROXIES must not cumulatively trust all IPv4 addresses"))
	}
	if coverage[128].covered {
		errs = append(errs, errors.New("PLATFORM_TRUSTED_PROXIES must not cumulatively trust all IPv6 addresses"))
	}
	return errs
}

type prefixCoverageNode struct {
	covered bool
	child   [2]*prefixCoverageNode
}

func (n *prefixCoverageNode) insert(prefix netip.Prefix) {
	address := prefix.Addr()
	var raw []byte
	if address.Is4() {
		value := address.As4()
		raw = value[:]
	} else {
		value := address.As16()
		raw = value[:]
	}
	n.insertBits(raw, prefix.Bits(), 0)
}

func (n *prefixCoverageNode) insertBits(address []byte, bits int, depth int) {
	if n.covered {
		return
	}
	if depth == bits {
		n.covered = true
		n.child = [2]*prefixCoverageNode{}
		return
	}
	bit := int((address[depth/8] >> (7 - depth%8)) & 1)
	if n.child[bit] == nil {
		n.child[bit] = &prefixCoverageNode{}
	}
	n.child[bit].insertBits(address, bits, depth+1)
	if n.child[0] != nil && n.child[0].covered && n.child[1] != nil && n.child[1].covered {
		n.covered = true
		n.child = [2]*prefixCoverageNode{}
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == target {
			return true
		}
	}
	return false
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(strings.TrimSpace(host), "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func hasCapability(capabilities []string, target string) bool {
	for _, capability := range capabilities {
		if strings.TrimSpace(capability) == target {
			return true
		}
	}
	return false
}

func validateCapabilities(capabilities []string) []error {
	var errs []error
	if len(capabilities) == 0 {
		return []error{errors.New("PLATFORM_CAPABILITIES must not be empty")}
	}
	seen := map[string]struct{}{}
	for _, capability := range capabilities {
		id := strings.TrimSpace(capability)
		if id == "" {
			errs = append(errs, errors.New("PLATFORM_CAPABILITIES contains an empty capability id"))
			continue
		}
		if !validCapabilityID(id) {
			errs = append(errs, fmt.Errorf("PLATFORM_CAPABILITIES capability %q must use lowercase letters, numbers, and hyphens", capability))
			continue
		}
		if _, exists := seen[id]; exists {
			errs = append(errs, fmt.Errorf("PLATFORM_CAPABILITIES contains duplicate capability %q", id))
			continue
		}
		seen[id] = struct{}{}
	}
	return errs
}

func validCapabilityID(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if char >= 'a' && char <= 'z' {
			continue
		}
		if char >= '0' && char <= '9' {
			continue
		}
		if char == '-' {
			continue
		}
		return false
	}
	return true
}

func validateDriverPair(label string, driver string, dsn string) []error {
	var errs []error
	driver = strings.TrimSpace(driver)
	dsn = strings.TrimSpace(dsn)
	if driver != "" && dsn == "" {
		errs = append(errs, fmt.Errorf("%s dsn is required when %s driver is set", label, label))
	}
	if driver == "" && dsn != "" {
		errs = append(errs, fmt.Errorf("%s driver is required when %s dsn is set", label, label))
	}
	if driver != "" && !isGORMDriver(driver) {
		errs = append(errs, fmt.Errorf("unsupported %s driver %q", label, driver))
	}
	return errs
}

func isGORMDriver(driver string) bool {
	switch strings.TrimSpace(driver) {
	case "mysql", "postgres", "sqlite":
		return true
	default:
		return false
	}
}

func env(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func csvEnv(key string, fallback []string) []string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return append([]string(nil), fallback...)
	}
	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		items = append(items, item)
	}
	return items
}

func durationEnv(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func intEnv(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envWithState(key string, fallback string) (string, envConfigState) {
	raw, exists := os.LookupEnv(key)
	if !exists {
		return fallback, envConfigMissing
	}
	value := strings.TrimSpace(raw)
	if value == "" {
		return fallback, envConfigEmpty
	}
	return value, envConfigPresent
}

func csvEnvWithState(key string, fallback []string) ([]string, envConfigState) {
	value, state := envWithState(key, "")
	if state != envConfigPresent {
		return append([]string(nil), fallback...), state
	}
	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		items = append(items, strings.TrimSpace(part))
	}
	return items, envConfigPresent
}

func int64EnvWithState(key string, fallback int64) (int64, envConfigState) {
	value, state := envWithState(key, "")
	if state != envConfigPresent {
		return fallback, state
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return -1, envConfigInvalid
	}
	return parsed, envConfigPresent
}

func boolEnv(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}
