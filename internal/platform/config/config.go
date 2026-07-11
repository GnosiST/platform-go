package config

import (
	"errors"
	"fmt"
	"mime"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	RuntimeEnvironment                string
	HTTPAddr                          string
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
)

var defaultFileAllowedMIMETypes = []string{"application/pdf", "image/jpeg", "image/png", "text/plain"}

func Load() Config {
	return Config{
		RuntimeEnvironment:                strings.ToLower(env("PLATFORM_RUNTIME_ENV", RuntimeEnvironmentDevelopment)),
		HTTPAddr:                          env("PLATFORM_HTTP_ADDR", "127.0.0.1:9200"),
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
		FileStorageDriver:                 env("PLATFORM_FILE_STORAGE_DRIVER", "local"),
		FileStorageLocalDir:               env("PLATFORM_FILE_STORAGE_LOCAL_DIR", ".platform/uploads"),
		FileMaxUploadBytes:                int64Env("PLATFORM_FILE_MAX_UPLOAD_BYTES", 10<<20),
		FileAllowedMIMETypes:              csvEnv("PLATFORM_FILE_ALLOWED_MIME_TYPES", defaultFileAllowedMIMETypes),
		FileStorageS3Endpoint:             env("PLATFORM_FILE_STORAGE_S3_ENDPOINT", ""),
		FileStorageS3Region:               env("PLATFORM_FILE_STORAGE_S3_REGION", ""),
		FileStorageS3Bucket:               env("PLATFORM_FILE_STORAGE_S3_BUCKET", ""),
		FileStorageS3AccessKey:            env("PLATFORM_FILE_STORAGE_S3_ACCESS_KEY", ""),
		FileStorageS3SecretKey:            env("PLATFORM_FILE_STORAGE_S3_SECRET_KEY", ""),
		FileStorageS3Prefix:               env("PLATFORM_FILE_STORAGE_S3_PREFIX", ""),
		FileStorageS3PathStyle:            boolEnv("PLATFORM_FILE_STORAGE_S3_FORCE_PATH_STYLE", false),
		FileStorageS3ServerSideEncryption: env("PLATFORM_FILE_STORAGE_S3_SERVER_SIDE_ENCRYPTION", "AES256"),
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
	errs = append(errs, validateFileUploadPolicy(c.FileMaxUploadBytes, c.FileAllowedMIMETypes, environment == RuntimeEnvironmentProduction)...)
	if (strings.TrimSpace(c.WechatMiniAppID) == "") != (strings.TrimSpace(c.WechatMiniAppSecret) == "") {
		errs = append(errs, errors.New("wechat miniapp app id and secret must be configured together"))
	}
	errs = append(errs, c.validateAdminOIDC(environment)...)
	errs = append(errs, c.validateAppPhone(environment)...)

	if environment == RuntimeEnvironmentProduction {
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
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil
	}
	endpoint, err := url.Parse(value)
	if err != nil || endpoint.Hostname() == "" {
		return []error{errors.New("file storage s3 endpoint must be an absolute URL")}
	}
	if endpoint.Scheme == "https" {
		return nil
	}
	if endpoint.Scheme == "http" && (environment == RuntimeEnvironmentDevelopment || environment == RuntimeEnvironmentTest) && isLoopbackHost(endpoint.Hostname()) {
		return nil
	}
	return []error{errors.New("file storage s3 endpoint must use https outside loopback development and test")}
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

func int64Env(key string, fallback int64) int64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return -1
	}
	return parsed
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
