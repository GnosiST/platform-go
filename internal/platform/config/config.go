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

	"platform-go/internal/platform/dataprotection"
)

type Config struct {
	RuntimeEnvironment                  string
	HTTPAddr                            string
	PublicBaseURL                       string
	TrustedProxies                      []string
	EdgeTrustedProxy                    string
	HTTPMaxBodyBytes                    int64
	Capabilities                        []string
	AdminResourceFile                   string
	AdminResourceDriver                 string
	AdminResourceDSN                    string
	OrganizationRBACMode                string
	AdminMenuServingMode                string
	AdminRoleMenuWriteEnabled           bool
	SessionFile                         string
	SessionDriver                       string
	SessionDSN                          string
	LifecycleHistoryFile                string
	LifecycleHistoryDriver              string
	LifecycleHistoryDSN                 string
	RetentionRunnerEnabled              bool
	RetentionRunnerInterval             time.Duration
	RetentionRunnerBatchSize            int
	RetentionRunnerMaxRetries           int
	DatabaseDriver                      string
	DatabaseDSN                         string
	OpenAPIFile                         string
	JWTSecret                           string
	CacheDriver                         string
	CacheDefaultTTL                     time.Duration
	RedisAddr                           string
	RedisPassword                       string
	RedisDB                             int
	MessageBusEnabled                   bool
	MessageBusAdapter                   string
	SearchEnabled                       bool
	SearchAdapter                       string
	RateLimitHMACKey                    string
	SensitiveRevealHMACKey              string
	DataKeyProvider                     string
	DataEncryptionActiveKeyID           string
	DataEncryptionKeyringJSON           string
	DataBlindIndexActiveKeyID           string
	DataBlindIndexKeyringJSON           string
	FileStorageDriver                   string
	FileStorageLocalDir                 string
	FileMaxUploadBytes                  int64
	FileAllowedMIMETypes                []string
	FileStorageS3Endpoint               string
	FileStorageS3Region                 string
	FileStorageS3Bucket                 string
	FileStorageS3AccessKey              string
	FileStorageS3SecretKey              string
	FileStorageS3Prefix                 string
	FileStorageS3PathStyle              bool
	FileStorageS3ServerSideEncryption   string
	FileStorageS3KMSKeyID               string
	WechatMiniAppID                     string
	WechatMiniAppSecret                 string
	WechatMiniAppCode2SessionEndpoint   string
	AdminOIDCIssuerURL                  string
	AdminOIDCClientID                   string
	AdminOIDCClientSecret               string
	AdminOIDCRedirectURL                string
	AdminOIDCScopes                     []string
	DisableDemoAuthProvider             bool
	PhoneHMACKey                        string
	PhoneCodeHMACKey                    string
	PhoneVerificationProvider           string
	AdminStepUpPhoneResource            string
	AdminStepUpPhoneActorField          string
	AdminStepUpPhoneField               string
	AdminStepUpPhoneVerifiedAtField     string
	AdminStepUpPhoneVerifiedDigestField string
	filePolicySource                    filePolicySource
	transportPolicySource               transportPolicySource
	retentionRunnerSource               retentionRunnerSource
	integrationSource                   integrationSource
	menuGovernanceSource                menuGovernanceSource
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

type retentionRunnerSource struct {
	enabled    envConfigState
	interval   envConfigState
	batchSize  envConfigState
	maxRetries envConfigState
}

type integrationSource struct {
	messageBusEnabled envConfigState
	searchEnabled     envConfigState
}

type menuGovernanceSource struct {
	roleMenuWriteEnabled envConfigState
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
	OrganizationRBACModeLegacy    = "legacy"
	OrganizationRBACModeTarget    = "target"
	AdminMenuServingModeLegacy    = "legacy"
	AdminMenuServingModeDualRead  = "dual-read"
	AdminMenuServingModeTarget    = "target"

	defaultJWTSecret   = "dev-platform-go-secret"
	maxFileUploadBytes = int64(100 << 20)
	maxHTTPBodyBytes   = int64(100 << 20)

	defaultRetentionRunnerInterval   = 24 * time.Hour
	defaultRetentionRunnerBatchSize  = 100
	defaultRetentionRunnerMaxRetries = 3
	maximumRetentionRunnerInterval   = 30 * 24 * time.Hour
	maximumRetentionRunnerBatchSize  = 1000
	maximumRetentionRunnerRetries    = 5
)

var defaultFileAllowedMIMETypes = []string{"application/pdf", "image/jpeg", "image/png", "text/plain"}

func Load() Config {
	fileMaxUploadBytes, fileMaxUploadBytesState := int64EnvWithState("PLATFORM_FILE_MAX_UPLOAD_BYTES", 10<<20)
	fileAllowedMIMETypes, fileAllowedMIMETypesState := csvEnvWithState("PLATFORM_FILE_ALLOWED_MIME_TYPES", defaultFileAllowedMIMETypes)
	fileS3Encryption, fileS3EncryptionState := envWithState("PLATFORM_FILE_STORAGE_S3_SERVER_SIDE_ENCRYPTION", "AES256")
	httpMaxBodyBytes, httpMaxBodyBytesState := int64EnvWithState("PLATFORM_HTTP_MAX_BODY_BYTES", 1<<20)
	retentionRunnerEnabled, retentionRunnerEnabledState := boolEnvWithState("PLATFORM_RETENTION_RUNNER_ENABLED", false)
	retentionRunnerInterval, retentionRunnerIntervalState := durationEnvWithState("PLATFORM_RETENTION_RUNNER_INTERVAL", defaultRetentionRunnerInterval)
	retentionRunnerBatchSize, retentionRunnerBatchSizeState := intEnvWithState("PLATFORM_RETENTION_RUNNER_BATCH_SIZE", defaultRetentionRunnerBatchSize)
	retentionRunnerMaxRetries, retentionRunnerMaxRetriesState := intEnvWithState("PLATFORM_RETENTION_RUNNER_MAX_RETRIES", defaultRetentionRunnerMaxRetries)
	messageBusEnabled, messageBusEnabledState := boolEnvWithState("PLATFORM_MESSAGE_BUS_ENABLED", false)
	searchEnabled, searchEnabledState := boolEnvWithState("PLATFORM_SEARCH_ENABLED", false)
	adminRoleMenuWriteEnabled, adminRoleMenuWriteEnabledState := boolEnvWithState("PLATFORM_ADMIN_ROLE_MENU_WRITE_ENABLED", false)
	return Config{
		RuntimeEnvironment:                  strings.ToLower(env("PLATFORM_RUNTIME_ENV", RuntimeEnvironmentDevelopment)),
		HTTPAddr:                            env("PLATFORM_HTTP_ADDR", "127.0.0.1:9200"),
		PublicBaseURL:                       env("PLATFORM_PUBLIC_BASE_URL", ""),
		TrustedProxies:                      csvEnv("PLATFORM_TRUSTED_PROXIES", nil),
		EdgeTrustedProxy:                    env("PLATFORM_EDGE_TRUSTED_PROXY", ""),
		HTTPMaxBodyBytes:                    httpMaxBodyBytes,
		Capabilities:                        csvEnv("PLATFORM_CAPABILITIES", defaultCapabilities),
		AdminResourceFile:                   env("PLATFORM_ADMIN_RESOURCE_FILE", ""),
		AdminResourceDriver:                 env("PLATFORM_ADMIN_RESOURCE_DRIVER", ""),
		AdminResourceDSN:                    env("PLATFORM_ADMIN_RESOURCE_DSN", ""),
		OrganizationRBACMode:                strings.ToLower(env("PLATFORM_ORGANIZATION_RBAC_MODE", OrganizationRBACModeLegacy)),
		AdminMenuServingMode:                strings.ToLower(env("PLATFORM_ADMIN_MENU_SERVING_MODE", AdminMenuServingModeLegacy)),
		AdminRoleMenuWriteEnabled:           adminRoleMenuWriteEnabled,
		SessionFile:                         env("PLATFORM_SESSION_FILE", ""),
		SessionDriver:                       env("PLATFORM_SESSION_DRIVER", ""),
		SessionDSN:                          env("PLATFORM_SESSION_DSN", ""),
		LifecycleHistoryFile:                env("PLATFORM_LIFECYCLE_HISTORY_FILE", ""),
		LifecycleHistoryDriver:              env("PLATFORM_LIFECYCLE_HISTORY_DRIVER", ""),
		LifecycleHistoryDSN:                 env("PLATFORM_LIFECYCLE_HISTORY_DSN", ""),
		RetentionRunnerEnabled:              retentionRunnerEnabled,
		RetentionRunnerInterval:             retentionRunnerInterval,
		RetentionRunnerBatchSize:            retentionRunnerBatchSize,
		RetentionRunnerMaxRetries:           retentionRunnerMaxRetries,
		DatabaseDriver:                      env("PLATFORM_DATABASE_DRIVER", "mysql"),
		DatabaseDSN:                         env("PLATFORM_DATABASE_DSN", ""),
		OpenAPIFile:                         env("PLATFORM_OPENAPI_FILE", "resources/generated/openapi.admin.json"),
		JWTSecret:                           env("PLATFORM_JWT_SECRET", defaultJWTSecret),
		CacheDriver:                         env("PLATFORM_CACHE_DRIVER", ""),
		CacheDefaultTTL:                     durationEnv("PLATFORM_CACHE_DEFAULT_TTL", 5*time.Minute),
		RedisAddr:                           env("PLATFORM_REDIS_ADDR", "127.0.0.1:6379"),
		RedisPassword:                       env("PLATFORM_REDIS_PASSWORD", ""),
		RedisDB:                             intEnv("PLATFORM_REDIS_DB", 0),
		MessageBusEnabled:                   messageBusEnabled,
		MessageBusAdapter:                   env("PLATFORM_MESSAGE_BUS_ADAPTER", ""),
		SearchEnabled:                       searchEnabled,
		SearchAdapter:                       env("PLATFORM_SEARCH_ADAPTER", ""),
		RateLimitHMACKey:                    env("PLATFORM_RATE_LIMIT_HMAC_KEY", ""),
		SensitiveRevealHMACKey:              env("PLATFORM_SENSITIVE_REVEAL_HMAC_KEY", ""),
		DataKeyProvider:                     env("PLATFORM_DATA_KEY_PROVIDER", ""),
		DataEncryptionActiveKeyID:           env("PLATFORM_DATA_ENCRYPTION_ACTIVE_KEY_ID", ""),
		DataEncryptionKeyringJSON:           env("PLATFORM_DATA_ENCRYPTION_KEYRING_JSON", ""),
		DataBlindIndexActiveKeyID:           env("PLATFORM_DATA_BLIND_INDEX_ACTIVE_KEY_ID", ""),
		DataBlindIndexKeyringJSON:           env("PLATFORM_DATA_BLIND_INDEX_KEYRING_JSON", ""),
		FileStorageDriver:                   env("PLATFORM_FILE_STORAGE_DRIVER", "local"),
		FileStorageLocalDir:                 env("PLATFORM_FILE_STORAGE_LOCAL_DIR", ".platform/uploads"),
		FileMaxUploadBytes:                  fileMaxUploadBytes,
		FileAllowedMIMETypes:                fileAllowedMIMETypes,
		FileStorageS3Endpoint:               env("PLATFORM_FILE_STORAGE_S3_ENDPOINT", ""),
		FileStorageS3Region:                 env("PLATFORM_FILE_STORAGE_S3_REGION", ""),
		FileStorageS3Bucket:                 env("PLATFORM_FILE_STORAGE_S3_BUCKET", ""),
		FileStorageS3AccessKey:              env("PLATFORM_FILE_STORAGE_S3_ACCESS_KEY", ""),
		FileStorageS3SecretKey:              env("PLATFORM_FILE_STORAGE_S3_SECRET_KEY", ""),
		FileStorageS3Prefix:                 env("PLATFORM_FILE_STORAGE_S3_PREFIX", ""),
		FileStorageS3PathStyle:              boolEnv("PLATFORM_FILE_STORAGE_S3_FORCE_PATH_STYLE", false),
		FileStorageS3ServerSideEncryption:   fileS3Encryption,
		FileStorageS3KMSKeyID:               env("PLATFORM_FILE_STORAGE_S3_KMS_KEY_ID", ""),
		WechatMiniAppID:                     env("PLATFORM_WECHAT_MINIAPP_APP_ID", ""),
		WechatMiniAppSecret:                 env("PLATFORM_WECHAT_MINIAPP_SECRET", ""),
		WechatMiniAppCode2SessionEndpoint:   env("PLATFORM_WECHAT_MINIAPP_CODE2SESSION_ENDPOINT", ""),
		AdminOIDCIssuerURL:                  env("PLATFORM_ADMIN_OIDC_ISSUER_URL", ""),
		AdminOIDCClientID:                   env("PLATFORM_ADMIN_OIDC_CLIENT_ID", ""),
		AdminOIDCClientSecret:               env("PLATFORM_ADMIN_OIDC_CLIENT_SECRET", ""),
		AdminOIDCRedirectURL:                env("PLATFORM_ADMIN_OIDC_REDIRECT_URL", ""),
		AdminOIDCScopes:                     csvEnv("PLATFORM_ADMIN_OIDC_SCOPES", []string{"openid", "profile", "email"}),
		DisableDemoAuthProvider:             boolEnv("PLATFORM_DISABLE_DEMO_AUTH_PROVIDER", false),
		PhoneHMACKey:                        env("PLATFORM_PHONE_HMAC_KEY", ""),
		PhoneCodeHMACKey:                    env("PLATFORM_PHONE_CODE_HMAC_KEY", ""),
		PhoneVerificationProvider:           env("PLATFORM_PHONE_VERIFICATION_PROVIDER", ""),
		AdminStepUpPhoneResource:            env("PLATFORM_ADMIN_STEP_UP_PHONE_RESOURCE", ""),
		AdminStepUpPhoneActorField:          env("PLATFORM_ADMIN_STEP_UP_PHONE_ACTOR_FIELD", ""),
		AdminStepUpPhoneField:               env("PLATFORM_ADMIN_STEP_UP_PHONE_FIELD", ""),
		AdminStepUpPhoneVerifiedAtField:     env("PLATFORM_ADMIN_STEP_UP_PHONE_VERIFIED_AT_FIELD", ""),
		AdminStepUpPhoneVerifiedDigestField: env("PLATFORM_ADMIN_STEP_UP_PHONE_VERIFIED_DIGEST_FIELD", ""),
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
		retentionRunnerSource: retentionRunnerSource{
			enabled: retentionRunnerEnabledState, interval: retentionRunnerIntervalState,
			batchSize: retentionRunnerBatchSizeState, maxRetries: retentionRunnerMaxRetriesState,
		},
		integrationSource: integrationSource{
			messageBusEnabled: messageBusEnabledState,
			searchEnabled:     searchEnabledState,
		},
		menuGovernanceSource: menuGovernanceSource{roleMenuWriteEnabled: adminRoleMenuWriteEnabledState},
	}
}

func (c Config) AdminOIDCConfigured() bool {
	return strings.TrimSpace(c.AdminOIDCIssuerURL) != "" &&
		strings.TrimSpace(c.AdminOIDCClientID) != "" &&
		strings.TrimSpace(c.AdminOIDCClientSecret) != "" &&
		strings.TrimSpace(c.AdminOIDCRedirectURL) != ""
}

func (c Config) AdminStepUpPhoneSourceConfigured() bool {
	return strings.TrimSpace(c.AdminStepUpPhoneResource) != "" &&
		strings.TrimSpace(c.AdminStepUpPhoneActorField) != "" &&
		strings.TrimSpace(c.AdminStepUpPhoneField) != "" &&
		strings.TrimSpace(c.AdminStepUpPhoneVerifiedAtField) != "" &&
		strings.TrimSpace(c.AdminStepUpPhoneVerifiedDigestField) != ""
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
	if key := strings.TrimSpace(c.SensitiveRevealHMACKey); key != "" {
		if len([]byte(key)) < 32 {
			errs = append(errs, errors.New("PLATFORM_SENSITIVE_REVEAL_HMAC_KEY must contain at least 32 bytes"))
		}
		if key == c.JWTSecret || key == c.PhoneHMACKey || key == c.PhoneCodeHMACKey || key == c.RateLimitHMACKey {
			errs = append(errs, errors.New("PLATFORM_SENSITIVE_REVEAL_HMAC_KEY must be distinct from JWT, phone, code, and rate-limit keys"))
		}
	}
	errs = append(errs, validateCapabilities(c.Capabilities)...)

	errs = append(errs, validateDriverPair("admin resource", c.AdminResourceDriver, c.AdminResourceDSN)...)
	organizationRBACMode := strings.TrimSpace(c.OrganizationRBACMode)
	if organizationRBACMode == "" {
		organizationRBACMode = OrganizationRBACModeLegacy
	}
	switch organizationRBACMode {
	case OrganizationRBACModeLegacy:
	case OrganizationRBACModeTarget:
		if !isGORMDriver(c.AdminResourceDriver) || strings.TrimSpace(c.AdminResourceDSN) == "" || strings.TrimSpace(c.AdminResourceFile) != "" {
			errs = append(errs, errors.New("PLATFORM_ORGANIZATION_RBAC_MODE=target requires persistent GORM Admin resource storage and no file repository"))
		}
	default:
		errs = append(errs, errors.New("PLATFORM_ORGANIZATION_RBAC_MODE must be legacy or target"))
	}
	rawMenuServingMode := c.AdminMenuServingMode
	menuServingMode := strings.ToLower(strings.TrimSpace(rawMenuServingMode))
	if menuServingMode == "" {
		menuServingMode = AdminMenuServingModeLegacy
	} else if rawMenuServingMode != menuServingMode {
		errs = append(errs, errors.New("PLATFORM_ADMIN_MENU_SERVING_MODE must be canonical trimmed lowercase"))
	}
	switch menuServingMode {
	case AdminMenuServingModeLegacy:
	case AdminMenuServingModeDualRead, AdminMenuServingModeTarget:
		if organizationRBACMode != OrganizationRBACModeTarget {
			errs = append(errs, errors.New("target or dual-read menu serving requires organization RBAC target mode"))
		}
	default:
		errs = append(errs, errors.New("PLATFORM_ADMIN_MENU_SERVING_MODE must be legacy, dual-read, or target"))
	}
	if c.menuGovernanceSource.roleMenuWriteEnabled == envConfigInvalid {
		errs = append(errs, errors.New("PLATFORM_ADMIN_ROLE_MENU_WRITE_ENABLED is invalid"))
	}
	if c.AdminRoleMenuWriteEnabled {
		if menuServingMode != AdminMenuServingModeTarget {
			errs = append(errs, errors.New("role menu writes require target menu serving"))
		}
	}
	errs = append(errs, validateDriverPair("session", c.SessionDriver, c.SessionDSN)...)
	errs = append(errs, validateDriverPair("lifecycle history", c.LifecycleHistoryDriver, c.LifecycleHistoryDSN)...)
	for key, state := range map[string]envConfigState{
		"PLATFORM_RETENTION_RUNNER_ENABLED":     c.retentionRunnerSource.enabled,
		"PLATFORM_RETENTION_RUNNER_INTERVAL":    c.retentionRunnerSource.interval,
		"PLATFORM_RETENTION_RUNNER_BATCH_SIZE":  c.retentionRunnerSource.batchSize,
		"PLATFORM_RETENTION_RUNNER_MAX_RETRIES": c.retentionRunnerSource.maxRetries,
	} {
		if state == envConfigInvalid {
			errs = append(errs, fmt.Errorf("%s is invalid", key))
		}
	}
	if c.RetentionRunnerEnabled {
		if !isGORMDriver(c.AdminResourceDriver) || strings.TrimSpace(c.AdminResourceDSN) == "" || strings.TrimSpace(c.AdminResourceFile) != "" {
			errs = append(errs, errors.New("PLATFORM_RETENTION_RUNNER_ENABLED requires persistent GORM Admin resource storage and no file repository"))
		}
		if c.RetentionRunnerInterval < time.Minute || c.RetentionRunnerInterval > maximumRetentionRunnerInterval {
			errs = append(errs, errors.New("PLATFORM_RETENTION_RUNNER_INTERVAL must be between 1m and 720h"))
		}
		if c.RetentionRunnerBatchSize < 1 || c.RetentionRunnerBatchSize > maximumRetentionRunnerBatchSize {
			errs = append(errs, errors.New("PLATFORM_RETENTION_RUNNER_BATCH_SIZE must be between 1 and 1000"))
		}
		if c.RetentionRunnerMaxRetries < 0 || c.RetentionRunnerMaxRetries > maximumRetentionRunnerRetries {
			errs = append(errs, errors.New("PLATFORM_RETENTION_RUNNER_MAX_RETRIES must be between 0 and 5"))
		}
	}

	if c.CacheDriver != "" && c.CacheDriver != "memory" && c.CacheDriver != "redis" {
		errs = append(errs, fmt.Errorf("unsupported cache driver %q", c.CacheDriver))
	}
	if c.CacheDriver == "redis" && strings.TrimSpace(c.RedisAddr) == "" {
		errs = append(errs, errors.New("redis address is required when cache driver is redis"))
	}
	errs = append(errs, validateOptionalIntegration("message bus", "PLATFORM_MESSAGE_BUS_ENABLED", "PLATFORM_MESSAGE_BUS_ADAPTER", c.MessageBusEnabled, c.MessageBusAdapter, c.integrationSource.messageBusEnabled)...)
	errs = append(errs, validateOptionalIntegration("search", "PLATFORM_SEARCH_ENABLED", "PLATFORM_SEARCH_ADAPTER", c.SearchEnabled, c.SearchAdapter, c.integrationSource.searchEnabled)...)

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
	errs = append(errs, c.validatePhoneVerification(environment)...)
	errs = append(errs, c.validateAdminStepUpPhoneSource()...)
	errs = append(errs, c.validateDataProtection(environment)...)

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
		for key, state := range map[string]envConfigState{
			"PLATFORM_MESSAGE_BUS_ENABLED": c.integrationSource.messageBusEnabled,
			"PLATFORM_SEARCH_ENABLED":      c.integrationSource.searchEnabled,
		} {
			if state == envConfigMissing || state == envConfigEmpty {
				errs = append(errs, fmt.Errorf("production runtime requires %s to be explicitly configured", key))
			}
		}
		errs = append(errs, c.validateProductionRuntime()...)
	}

	return errors.Join(errs...)
}

func validateOptionalIntegration(label string, enabledKey string, adapterKey string, enabled bool, adapter string, enabledState envConfigState) []error {
	var errs []error
	if enabledState == envConfigInvalid {
		errs = append(errs, fmt.Errorf("%s is invalid", enabledKey))
	}
	canonical := strings.ToLower(strings.TrimSpace(adapter))
	if adapter != canonical {
		errs = append(errs, fmt.Errorf("%s must be canonical trimmed lowercase", adapterKey))
	}
	if enabled && canonical == "" {
		errs = append(errs, fmt.Errorf("%s requires %s", enabledKey, adapterKey))
	}
	if !enabled && canonical != "" {
		errs = append(errs, fmt.Errorf("%s requires %s=true", adapterKey, enabledKey))
	}
	return errs
}

func (c Config) validateAdminStepUpPhoneSource() []error {
	values := []struct {
		name  string
		value string
	}{
		{name: "PLATFORM_ADMIN_STEP_UP_PHONE_RESOURCE", value: c.AdminStepUpPhoneResource},
		{name: "PLATFORM_ADMIN_STEP_UP_PHONE_ACTOR_FIELD", value: c.AdminStepUpPhoneActorField},
		{name: "PLATFORM_ADMIN_STEP_UP_PHONE_FIELD", value: c.AdminStepUpPhoneField},
		{name: "PLATFORM_ADMIN_STEP_UP_PHONE_VERIFIED_AT_FIELD", value: c.AdminStepUpPhoneVerifiedAtField},
		{name: "PLATFORM_ADMIN_STEP_UP_PHONE_VERIFIED_DIGEST_FIELD", value: c.AdminStepUpPhoneVerifiedDigestField},
	}
	configured := 0
	var errs []error
	for _, item := range values {
		trimmed := strings.TrimSpace(item.value)
		if trimmed != "" {
			configured++
		}
		if item.value != trimmed {
			errs = append(errs, fmt.Errorf("%s must be trimmed", item.name))
		}
	}
	if configured != 0 && configured != len(values) {
		errs = append(errs, errors.New("admin step-up phone source resource and fields must be configured together"))
	}
	return errs
}

func (c Config) validateDataProtection(environment string) []error {
	rawProvider := c.DataKeyProvider
	provider := strings.ToLower(strings.TrimSpace(rawProvider))
	configuredKeys := strings.TrimSpace(c.DataEncryptionActiveKeyID) != "" || strings.TrimSpace(c.DataEncryptionKeyringJSON) != "" ||
		strings.TrimSpace(c.DataBlindIndexActiveKeyID) != "" || strings.TrimSpace(c.DataBlindIndexKeyringJSON) != ""
	if provider == "" {
		if environment == RuntimeEnvironmentProduction {
			return []error{errors.New("production runtime requires PLATFORM_DATA_KEY_PROVIDER=env-aes256")}
		}
		if configuredKeys {
			return []error{errors.New("data protection keys require PLATFORM_DATA_KEY_PROVIDER")}
		}
		return nil
	}
	var errs []error
	if rawProvider != provider {
		errs = append(errs, errors.New("PLATFORM_DATA_KEY_PROVIDER must be canonical trimmed lowercase"))
	}
	switch provider {
	case dataprotection.ProviderEnvAES256:
	case dataprotection.ProviderLocalTest:
		if environment != RuntimeEnvironmentDevelopment && environment != RuntimeEnvironmentTest {
			errs = append(errs, fmt.Errorf("data key provider local-test is not allowed in %s", environment))
		}
	default:
		errs = append(errs, fmt.Errorf("unsupported data key provider %q", provider))
		return errs
	}
	if environment == RuntimeEnvironmentProduction && provider != dataprotection.ProviderEnvAES256 {
		errs = append(errs, errors.New("production runtime requires PLATFORM_DATA_KEY_PROVIDER=env-aes256"))
	}
	encryptionKeys, err := dataprotection.ParseEncodedKeyring(c.DataEncryptionKeyringJSON)
	if err != nil {
		errs = append(errs, fmt.Errorf("data encryption keyring: %w", err))
		return errs
	}
	blindIndexKeys, err := dataprotection.ParseEncodedKeyring(c.DataBlindIndexKeyringJSON)
	if err != nil {
		errs = append(errs, fmt.Errorf("data blind-index keyring: %w", err))
		return errs
	}
	_, err = dataprotection.NewStaticKeyProvider(dataprotection.StaticKeyProviderConfig{
		Kind:                  provider,
		ActiveEncryptionKeyID: c.DataEncryptionActiveKeyID, EncryptionKeys: encryptionKeys,
		ActiveBlindIndexKeyID: c.DataBlindIndexActiveKeyID, BlindIndexKeys: blindIndexKeys,
	})
	if err != nil {
		errs = append(errs, fmt.Errorf("data protection provider: %w", err))
	}
	return errs
}

func (c Config) validatePhoneVerification(environment string) []error {
	appPhoneEnabled := hasCapability(c.Capabilities, "app-phone")
	adminStepUpEnabled := c.AdminStepUpPhoneSourceConfigured()
	if !appPhoneEnabled && !adminStepUpEnabled {
		return nil
	}
	var errs []error
	prefix := "phone verification"
	if appPhoneEnabled && !adminStepUpEnabled {
		prefix = "app-phone"
	} else if adminStepUpEnabled && !appPhoneEnabled {
		prefix = "admin step-up phone"
	}
	if environment == RuntimeEnvironmentProduction {
		prefix = "production " + prefix
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
		errs = append(errs, fmt.Errorf("%s debug provider is allowed only in development or test", prefix))
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
	organizationRBACMode := strings.TrimSpace(c.OrganizationRBACMode)
	if organizationRBACMode == "" {
		organizationRBACMode = OrganizationRBACModeLegacy
	}
	if organizationRBACMode != OrganizationRBACModeTarget {
		errs = append(errs, errors.New("production runtime requires PLATFORM_ORGANIZATION_RBAC_MODE=target"))
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

func durationEnvWithState(key string, fallback time.Duration) (time.Duration, envConfigState) {
	value, state := envWithState(key, "")
	if state != envConfigPresent {
		return fallback, state
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback, envConfigInvalid
	}
	return parsed, envConfigPresent
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

func intEnvWithState(key string, fallback int) (int, envConfigState) {
	value, state := envWithState(key, "")
	if state != envConfigPresent {
		return fallback, state
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback, envConfigInvalid
	}
	return parsed, envConfigPresent
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

func boolEnvWithState(key string, fallback bool) (bool, envConfigState) {
	value, state := envWithState(key, "")
	if state != envConfigPresent {
		return fallback, state
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback, envConfigInvalid
	}
	return parsed, envConfigPresent
}
