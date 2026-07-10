package config

import (
	"errors"
	"fmt"
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
	FileStoragePublicURL              string
	FileStorageS3Endpoint             string
	FileStorageS3Region               string
	FileStorageS3Bucket               string
	FileStorageS3AccessKey            string
	FileStorageS3SecretKey            string
	FileStorageS3Prefix               string
	FileStorageS3PathStyle            bool
	WechatMiniAppID                   string
	WechatMiniAppSecret               string
	WechatMiniAppCode2SessionEndpoint string
	DisableDemoAuthProvider           bool
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

	defaultJWTSecret = "dev-platform-go-secret"
)

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
		FileStoragePublicURL:              env("PLATFORM_FILE_STORAGE_PUBLIC_URL", "/uploads"),
		FileStorageS3Endpoint:             env("PLATFORM_FILE_STORAGE_S3_ENDPOINT", ""),
		FileStorageS3Region:               env("PLATFORM_FILE_STORAGE_S3_REGION", ""),
		FileStorageS3Bucket:               env("PLATFORM_FILE_STORAGE_S3_BUCKET", ""),
		FileStorageS3AccessKey:            env("PLATFORM_FILE_STORAGE_S3_ACCESS_KEY", ""),
		FileStorageS3SecretKey:            env("PLATFORM_FILE_STORAGE_S3_SECRET_KEY", ""),
		FileStorageS3Prefix:               env("PLATFORM_FILE_STORAGE_S3_PREFIX", ""),
		FileStorageS3PathStyle:            boolEnv("PLATFORM_FILE_STORAGE_S3_FORCE_PATH_STYLE", false),
		WechatMiniAppID:                   env("PLATFORM_WECHAT_MINIAPP_APP_ID", ""),
		WechatMiniAppSecret:               env("PLATFORM_WECHAT_MINIAPP_SECRET", ""),
		WechatMiniAppCode2SessionEndpoint: env("PLATFORM_WECHAT_MINIAPP_CODE2SESSION_ENDPOINT", ""),
		DisableDemoAuthProvider:           boolEnv("PLATFORM_DISABLE_DEMO_AUTH_PROVIDER", false),
	}
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
	}
	if (strings.TrimSpace(c.WechatMiniAppID) == "") != (strings.TrimSpace(c.WechatMiniAppSecret) == "") {
		errs = append(errs, errors.New("wechat miniapp app id and secret must be configured together"))
	}

	if environment == RuntimeEnvironmentProduction {
		errs = append(errs, c.validateProductionRuntime()...)
	}

	return errors.Join(errs...)
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
	return errs
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
