package config

import (
	"encoding/base64"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestLoadUsesDefaults(t *testing.T) {
	t.Setenv("PLATFORM_RUNTIME_ENV", "")
	t.Setenv("PLATFORM_HTTP_ADDR", "")
	t.Setenv("PLATFORM_PUBLIC_BASE_URL", "")
	t.Setenv("PLATFORM_TRUSTED_PROXIES", "")
	t.Setenv("PLATFORM_EDGE_TRUSTED_PROXY", "")
	t.Setenv("PLATFORM_HTTP_MAX_BODY_BYTES", "")
	t.Setenv("PLATFORM_CAPABILITIES", "")
	t.Setenv("PLATFORM_ADMIN_RESOURCE_FILE", "")
	t.Setenv("PLATFORM_ADMIN_RESOURCE_DRIVER", "")
	t.Setenv("PLATFORM_ADMIN_RESOURCE_DSN", "")
	t.Setenv("PLATFORM_SESSION_FILE", "")
	t.Setenv("PLATFORM_SESSION_DRIVER", "")
	t.Setenv("PLATFORM_SESSION_DSN", "")
	t.Setenv("PLATFORM_LIFECYCLE_HISTORY_FILE", "")
	t.Setenv("PLATFORM_LIFECYCLE_HISTORY_DRIVER", "")
	t.Setenv("PLATFORM_LIFECYCLE_HISTORY_DSN", "")
	t.Setenv("PLATFORM_DATABASE_DRIVER", "")
	t.Setenv("PLATFORM_DATABASE_DSN", "")
	t.Setenv("PLATFORM_OPENAPI_FILE", "")
	t.Setenv("PLATFORM_JWT_SECRET", "")
	t.Setenv("PLATFORM_CACHE_DRIVER", "")
	t.Setenv("PLATFORM_CACHE_DEFAULT_TTL", "")
	t.Setenv("PLATFORM_REDIS_ADDR", "")
	t.Setenv("PLATFORM_REDIS_PASSWORD", "")
	t.Setenv("PLATFORM_REDIS_DB", "")
	t.Setenv("PLATFORM_RATE_LIMIT_HMAC_KEY", "")
	t.Setenv("PLATFORM_SENSITIVE_REVEAL_HMAC_KEY", "")
	t.Setenv("PLATFORM_DATA_KEY_PROVIDER", "")
	t.Setenv("PLATFORM_DATA_ENCRYPTION_ACTIVE_KEY_ID", "")
	t.Setenv("PLATFORM_DATA_ENCRYPTION_KEYRING_JSON", "")
	t.Setenv("PLATFORM_DATA_BLIND_INDEX_ACTIVE_KEY_ID", "")
	t.Setenv("PLATFORM_DATA_BLIND_INDEX_KEYRING_JSON", "")
	t.Setenv("PLATFORM_FILE_STORAGE_DRIVER", "")
	t.Setenv("PLATFORM_FILE_STORAGE_LOCAL_DIR", "")
	t.Setenv("PLATFORM_FILE_MAX_UPLOAD_BYTES", "")
	t.Setenv("PLATFORM_FILE_ALLOWED_MIME_TYPES", "")
	t.Setenv("PLATFORM_FILE_STORAGE_S3_ENDPOINT", "")
	t.Setenv("PLATFORM_FILE_STORAGE_S3_REGION", "")
	t.Setenv("PLATFORM_FILE_STORAGE_S3_BUCKET", "")
	t.Setenv("PLATFORM_FILE_STORAGE_S3_ACCESS_KEY", "")
	t.Setenv("PLATFORM_FILE_STORAGE_S3_SECRET_KEY", "")
	t.Setenv("PLATFORM_FILE_STORAGE_S3_PREFIX", "")
	t.Setenv("PLATFORM_FILE_STORAGE_S3_FORCE_PATH_STYLE", "")
	t.Setenv("PLATFORM_FILE_STORAGE_S3_SERVER_SIDE_ENCRYPTION", "")
	t.Setenv("PLATFORM_FILE_STORAGE_S3_KMS_KEY_ID", "")
	t.Setenv("PLATFORM_WECHAT_MINIAPP_APP_ID", "")
	t.Setenv("PLATFORM_WECHAT_MINIAPP_SECRET", "")
	t.Setenv("PLATFORM_WECHAT_MINIAPP_CODE2SESSION_ENDPOINT", "")
	t.Setenv("PLATFORM_ADMIN_OIDC_ISSUER_URL", "")
	t.Setenv("PLATFORM_ADMIN_OIDC_CLIENT_ID", "")
	t.Setenv("PLATFORM_ADMIN_OIDC_CLIENT_SECRET", "")
	t.Setenv("PLATFORM_ADMIN_OIDC_REDIRECT_URL", "")
	t.Setenv("PLATFORM_ADMIN_OIDC_SCOPES", "")
	t.Setenv("PLATFORM_ADMIN_STEP_UP_PHONE_RESOURCE", "")
	t.Setenv("PLATFORM_ADMIN_STEP_UP_PHONE_ACTOR_FIELD", "")
	t.Setenv("PLATFORM_ADMIN_STEP_UP_PHONE_FIELD", "")
	t.Setenv("PLATFORM_ADMIN_STEP_UP_PHONE_VERIFIED_AT_FIELD", "")
	t.Setenv("PLATFORM_ADMIN_STEP_UP_PHONE_VERIFIED_DIGEST_FIELD", "")

	cfg := Load()

	if cfg.RuntimeEnvironment != "development" {
		t.Fatalf("RuntimeEnvironment = %q, want development", cfg.RuntimeEnvironment)
	}
	if cfg.HTTPAddr != "127.0.0.1:9200" {
		t.Fatalf("HTTPAddr = %q", cfg.HTTPAddr)
	}
	if cfg.PublicBaseURL != "" {
		t.Fatalf("PublicBaseURL = %q, want empty by default", cfg.PublicBaseURL)
	}
	if len(cfg.TrustedProxies) != 0 {
		t.Fatalf("TrustedProxies = %#v, want none by default", cfg.TrustedProxies)
	}
	if cfg.EdgeTrustedProxy != "" {
		t.Fatalf("EdgeTrustedProxy = %q, want empty by default", cfg.EdgeTrustedProxy)
	}
	if cfg.HTTPMaxBodyBytes != 1<<20 {
		t.Fatalf("HTTPMaxBodyBytes = %d, want 1 MiB", cfg.HTTPMaxBodyBytes)
	}
	if len(cfg.Capabilities) == 0 {
		t.Fatalf("Capabilities is empty")
	}
	if cfg.AdminResourceFile != "" {
		t.Fatalf("AdminResourceFile = %q, want empty by default", cfg.AdminResourceFile)
	}
	if cfg.AdminResourceDriver != "" {
		t.Fatalf("AdminResourceDriver = %q, want empty by default", cfg.AdminResourceDriver)
	}
	if cfg.AdminResourceDSN != "" {
		t.Fatalf("AdminResourceDSN = %q, want empty by default", cfg.AdminResourceDSN)
	}
	if cfg.SessionFile != "" {
		t.Fatalf("SessionFile = %q, want empty by default", cfg.SessionFile)
	}
	if cfg.SessionDriver != "" {
		t.Fatalf("SessionDriver = %q, want empty by default", cfg.SessionDriver)
	}
	if cfg.SessionDSN != "" {
		t.Fatalf("SessionDSN = %q, want empty by default", cfg.SessionDSN)
	}
	if cfg.LifecycleHistoryFile != "" {
		t.Fatalf("LifecycleHistoryFile = %q, want empty by default", cfg.LifecycleHistoryFile)
	}
	if cfg.LifecycleHistoryDriver != "" {
		t.Fatalf("LifecycleHistoryDriver = %q, want empty by default", cfg.LifecycleHistoryDriver)
	}
	if cfg.LifecycleHistoryDSN != "" {
		t.Fatalf("LifecycleHistoryDSN = %q, want empty by default", cfg.LifecycleHistoryDSN)
	}
	if cfg.DatabaseDriver != "mysql" {
		t.Fatalf("DatabaseDriver = %q, want mysql by default", cfg.DatabaseDriver)
	}
	if cfg.DatabaseDSN != "" {
		t.Fatalf("DatabaseDSN = %q, want empty by default", cfg.DatabaseDSN)
	}
	if cfg.OpenAPIFile != "resources/generated/openapi.admin.json" {
		t.Fatalf("OpenAPIFile = %q, want generated admin openapi path", cfg.OpenAPIFile)
	}
	if cfg.JWTSecret != "dev-platform-go-secret" {
		t.Fatalf("JWTSecret = %q, want development default", cfg.JWTSecret)
	}
	if cfg.CacheDriver != "" {
		t.Fatalf("CacheDriver = %q, want empty by default", cfg.CacheDriver)
	}
	if cfg.CacheDefaultTTL.String() != "5m0s" {
		t.Fatalf("CacheDefaultTTL = %s, want 5m0s", cfg.CacheDefaultTTL)
	}
	if cfg.RedisAddr != "127.0.0.1:6379" {
		t.Fatalf("RedisAddr = %q", cfg.RedisAddr)
	}
	if cfg.RedisPassword != "" {
		t.Fatalf("RedisPassword = %q, want empty by default", cfg.RedisPassword)
	}
	if cfg.RedisDB != 0 {
		t.Fatalf("RedisDB = %d, want 0", cfg.RedisDB)
	}
	if cfg.RateLimitHMACKey != "" {
		t.Fatalf("RateLimitHMACKey = %q, want empty by default", cfg.RateLimitHMACKey)
	}
	if cfg.SensitiveRevealHMACKey != "" {
		t.Fatalf("SensitiveRevealHMACKey = %q, want empty by default", cfg.SensitiveRevealHMACKey)
	}
	if cfg.DataKeyProvider != "" || cfg.DataEncryptionActiveKeyID != "" || cfg.DataEncryptionKeyringJSON != "" || cfg.DataBlindIndexActiveKeyID != "" || cfg.DataBlindIndexKeyringJSON != "" {
		t.Fatal("data protection configuration must be empty by default")
	}
	if cfg.FileStorageDriver != "local" {
		t.Fatalf("FileStorageDriver = %q, want local", cfg.FileStorageDriver)
	}
	if cfg.FileStorageLocalDir != ".platform/uploads" {
		t.Fatalf("FileStorageLocalDir = %q", cfg.FileStorageLocalDir)
	}
	if cfg.FileMaxUploadBytes != 10<<20 {
		t.Fatalf("FileMaxUploadBytes = %d, want 10 MiB", cfg.FileMaxUploadBytes)
	}
	if !reflect.DeepEqual(cfg.FileAllowedMIMETypes, []string{"application/pdf", "image/jpeg", "image/png", "text/plain"}) {
		t.Fatalf("FileAllowedMIMETypes = %#v", cfg.FileAllowedMIMETypes)
	}
	if cfg.FileStorageS3ServerSideEncryption != "AES256" || cfg.FileStorageS3KMSKeyID != "" {
		t.Fatalf("S3 encryption defaults = %q/%q", cfg.FileStorageS3ServerSideEncryption, cfg.FileStorageS3KMSKeyID)
	}
	if cfg.WechatMiniAppID != "" || cfg.WechatMiniAppSecret != "" || cfg.WechatMiniAppCode2SessionEndpoint != "" {
		t.Fatalf("WeChat miniapp config = %q/%q/%q, want empty by default", cfg.WechatMiniAppID, cfg.WechatMiniAppSecret, cfg.WechatMiniAppCode2SessionEndpoint)
	}
	if cfg.AdminOIDCIssuerURL != "" || cfg.AdminOIDCClientID != "" || cfg.AdminOIDCClientSecret != "" || cfg.AdminOIDCRedirectURL != "" {
		t.Fatalf("Admin OIDC config = %+v, want empty by default", cfg)
	}
	if !reflect.DeepEqual(cfg.AdminOIDCScopes, []string{"openid", "profile", "email"}) {
		t.Fatalf("AdminOIDCScopes = %#v, want default OpenID scopes", cfg.AdminOIDCScopes)
	}
	if cfg.DisableDemoAuthProvider {
		t.Fatalf("DisableDemoAuthProvider = true, want false by default")
	}
	if cfg.AdminStepUpPhoneSourceConfigured() {
		t.Fatal("admin step-up phone source must be disabled by default")
	}
}

func TestLoadParsesRuntimeEnvironment(t *testing.T) {
	t.Setenv("PLATFORM_RUNTIME_ENV", "production")

	cfg := Load()
	if cfg.RuntimeEnvironment != "production" {
		t.Fatalf("RuntimeEnvironment = %q", cfg.RuntimeEnvironment)
	}
}

func TestLoadParsesDataProtectionConfiguration(t *testing.T) {
	t.Setenv("PLATFORM_DATA_KEY_PROVIDER", "env-aes256")
	t.Setenv("PLATFORM_DATA_ENCRYPTION_ACTIVE_KEY_ID", "enc-v2")
	t.Setenv("PLATFORM_DATA_ENCRYPTION_KEYRING_JSON", "{\"enc-v1\":\"first\",\"enc-v2\":\"second\"}")
	t.Setenv("PLATFORM_DATA_BLIND_INDEX_ACTIVE_KEY_ID", "idx-v2")
	t.Setenv("PLATFORM_DATA_BLIND_INDEX_KEYRING_JSON", "{\"idx-v1\":\"first\",\"idx-v2\":\"second\"}")

	cfg := Load()
	if cfg.DataKeyProvider != "env-aes256" || cfg.DataEncryptionActiveKeyID != "enc-v2" || cfg.DataBlindIndexActiveKeyID != "idx-v2" {
		t.Fatalf("data protection IDs = %q/%q/%q", cfg.DataKeyProvider, cfg.DataEncryptionActiveKeyID, cfg.DataBlindIndexActiveKeyID)
	}
	if !strings.Contains(cfg.DataEncryptionKeyringJSON, "enc-v1") || !strings.Contains(cfg.DataBlindIndexKeyringJSON, "idx-v1") {
		t.Fatal("Load() dropped data protection keyrings")
	}
}

func TestValidateRuntimeRejectsUnsafeDataProtectionConfiguration(t *testing.T) {
	secretMarker := "raw-keyring-secret-marker"
	tests := []struct {
		name   string
		mutate func(*Config)
		want   string
	}{
		{name: "missing provider", mutate: func(cfg *Config) { cfg.DataKeyProvider = "" }, want: "production runtime requires PLATFORM_DATA_KEY_PROVIDER=env-aes256"},
		{name: "local test provider", mutate: func(cfg *Config) { cfg.DataKeyProvider = "local-test" }, want: "not allowed"},
		{name: "unknown provider", mutate: func(cfg *Config) { cfg.DataKeyProvider = "kms" }, want: "unsupported data key provider"},
		{name: "missing active encryption key", mutate: func(cfg *Config) { cfg.DataEncryptionActiveKeyID = "enc-v2" }, want: "active encryption key is unavailable"},
		{name: "malformed encryption keyring", mutate: func(cfg *Config) { cfg.DataEncryptionKeyringJSON = "{\"enc-v1\":\"" + secretMarker + "\"" }, want: "keyring JSON is invalid"},
		{name: "reused key material", mutate: func(cfg *Config) { cfg.DataBlindIndexKeyringJSON = cfg.DataEncryptionKeyringJSON }, want: "key material must not be reused"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validProductionRuntimeConfig()
			tt.mutate(&cfg)
			err := cfg.ValidateRuntime()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("ValidateRuntime() error = %v, want %q", err, tt.want)
			}
			if strings.Contains(err.Error(), secretMarker) || strings.Contains(err.Error(), cfg.DataEncryptionKeyringJSON) {
				t.Fatalf("ValidateRuntime() exposed keyring material: %v", err)
			}
		})
	}
}

func TestValidateRuntimeAllowsExplicitLocalTestDataKeysOnlyInDevelopmentAndTest(t *testing.T) {
	for _, environment := range []string{RuntimeEnvironmentDevelopment, RuntimeEnvironmentTest} {
		t.Run(environment, func(t *testing.T) {
			cfg := validDataProtectionConfig(environment, "local-test")
			if err := cfg.ValidateRuntime(); err != nil {
				t.Fatalf("ValidateRuntime() error = %v", err)
			}
		})
	}
	cfg := validDataProtectionConfig(RuntimeEnvironmentStaging, "local-test")
	if err := cfg.ValidateRuntime(); err == nil || !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("ValidateRuntime(staging local-test) error = %v", err)
	}
}

func TestLoadParsesTransportSecurityConfiguration(t *testing.T) {
	t.Setenv("PLATFORM_PUBLIC_BASE_URL", "https://platform.example.test")
	t.Setenv("PLATFORM_TRUSTED_PROXIES", "10.20.0.0/16,192.0.2.10")
	t.Setenv("PLATFORM_EDGE_TRUSTED_PROXY", "172.30.0.1")
	t.Setenv("PLATFORM_HTTP_MAX_BODY_BYTES", "2097152")

	cfg := Load()
	if cfg.PublicBaseURL != "https://platform.example.test" {
		t.Fatalf("PublicBaseURL = %q", cfg.PublicBaseURL)
	}
	if !reflect.DeepEqual(cfg.TrustedProxies, []string{"10.20.0.0/16", "192.0.2.10"}) {
		t.Fatalf("TrustedProxies = %#v", cfg.TrustedProxies)
	}
	if cfg.EdgeTrustedProxy != "172.30.0.1" {
		t.Fatalf("EdgeTrustedProxy = %q", cfg.EdgeTrustedProxy)
	}
	if cfg.HTTPMaxBodyBytes != 2<<20 {
		t.Fatalf("HTTPMaxBodyBytes = %d", cfg.HTTPMaxBodyBytes)
	}
}

func TestLoadParsesCapabilities(t *testing.T) {
	t.Setenv("PLATFORM_CAPABILITIES", "tenant, identity, audit")

	cfg := Load()
	want := []string{"tenant", "identity", "audit"}
	if !reflect.DeepEqual(cfg.Capabilities, want) {
		t.Fatalf("Capabilities = %#v, want %#v", cfg.Capabilities, want)
	}
}

func TestLoadPreservesBlankCapabilityEntriesForValidation(t *testing.T) {
	t.Setenv("PLATFORM_CAPABILITIES", "tenant,,identity, ")

	cfg := Load()
	want := []string{"tenant", "", "identity", ""}
	if !reflect.DeepEqual(cfg.Capabilities, want) {
		t.Fatalf("Capabilities = %#v, want %#v", cfg.Capabilities, want)
	}
}

func TestLoadParsesAdminResourceFile(t *testing.T) {
	t.Setenv("PLATFORM_ADMIN_RESOURCE_FILE", "/tmp/platform-go-admin-resources.json")

	cfg := Load()
	if cfg.AdminResourceFile != "/tmp/platform-go-admin-resources.json" {
		t.Fatalf("AdminResourceFile = %q", cfg.AdminResourceFile)
	}
}

func TestLoadParsesAdminResourceSQLConfig(t *testing.T) {
	t.Setenv("PLATFORM_ADMIN_RESOURCE_DRIVER", "sqlite")
	t.Setenv("PLATFORM_ADMIN_RESOURCE_DSN", "file:platform.db")

	cfg := Load()
	if cfg.AdminResourceDriver != "sqlite" {
		t.Fatalf("AdminResourceDriver = %q", cfg.AdminResourceDriver)
	}
	if cfg.AdminResourceDSN != "file:platform.db" {
		t.Fatalf("AdminResourceDSN = %q", cfg.AdminResourceDSN)
	}
}

func TestLoadParsesSessionFile(t *testing.T) {
	t.Setenv("PLATFORM_SESSION_FILE", "/tmp/platform-go-sessions.json")

	cfg := Load()
	if cfg.SessionFile != "/tmp/platform-go-sessions.json" {
		t.Fatalf("SessionFile = %q", cfg.SessionFile)
	}
}

func TestLoadParsesSessionSQLConfig(t *testing.T) {
	t.Setenv("PLATFORM_SESSION_DRIVER", "sqlite")
	t.Setenv("PLATFORM_SESSION_DSN", "file:platform.db")

	cfg := Load()
	if cfg.SessionDriver != "sqlite" {
		t.Fatalf("SessionDriver = %q", cfg.SessionDriver)
	}
	if cfg.SessionDSN != "file:platform.db" {
		t.Fatalf("SessionDSN = %q", cfg.SessionDSN)
	}
}

func TestLoadParsesLifecycleHistoryFile(t *testing.T) {
	t.Setenv("PLATFORM_LIFECYCLE_HISTORY_FILE", "/tmp/platform-go-lifecycle.json")

	cfg := Load()
	if cfg.LifecycleHistoryFile != "/tmp/platform-go-lifecycle.json" {
		t.Fatalf("LifecycleHistoryFile = %q", cfg.LifecycleHistoryFile)
	}
}

func TestLoadParsesLifecycleHistorySQLConfig(t *testing.T) {
	t.Setenv("PLATFORM_LIFECYCLE_HISTORY_DRIVER", "sqlite")
	t.Setenv("PLATFORM_LIFECYCLE_HISTORY_DSN", "file:platform.db")

	cfg := Load()
	if cfg.LifecycleHistoryDriver != "sqlite" {
		t.Fatalf("LifecycleHistoryDriver = %q", cfg.LifecycleHistoryDriver)
	}
	if cfg.LifecycleHistoryDSN != "file:platform.db" {
		t.Fatalf("LifecycleHistoryDSN = %q", cfg.LifecycleHistoryDSN)
	}
}

func TestLoadParsesDatabaseConfig(t *testing.T) {
	t.Setenv("PLATFORM_DATABASE_DRIVER", "sqlite")
	t.Setenv("PLATFORM_DATABASE_DSN", "file:platform.db")

	cfg := Load()
	if cfg.DatabaseDriver != "sqlite" {
		t.Fatalf("DatabaseDriver = %q", cfg.DatabaseDriver)
	}
	if cfg.DatabaseDSN != "file:platform.db" {
		t.Fatalf("DatabaseDSN = %q", cfg.DatabaseDSN)
	}
}

func TestLoadParsesJWTSecret(t *testing.T) {
	t.Setenv("PLATFORM_JWT_SECRET", "test-secret")

	cfg := Load()
	if cfg.JWTSecret != "test-secret" {
		t.Fatalf("JWTSecret = %q", cfg.JWTSecret)
	}
}

func TestLoadParsesDisableDemoAuthProvider(t *testing.T) {
	t.Setenv("PLATFORM_DISABLE_DEMO_AUTH_PROVIDER", "true")

	cfg := Load()
	if !cfg.DisableDemoAuthProvider {
		t.Fatalf("DisableDemoAuthProvider = false, want true")
	}
}

func TestLoadParsesOpenAPIFile(t *testing.T) {
	t.Setenv("PLATFORM_OPENAPI_FILE", "/tmp/platform-openapi.json")

	cfg := Load()
	if cfg.OpenAPIFile != "/tmp/platform-openapi.json" {
		t.Fatalf("OpenAPIFile = %q", cfg.OpenAPIFile)
	}
}

func TestLoadParsesCacheConfig(t *testing.T) {
	t.Setenv("PLATFORM_CACHE_DRIVER", "redis")
	t.Setenv("PLATFORM_CACHE_DEFAULT_TTL", "90s")
	t.Setenv("PLATFORM_REDIS_ADDR", "127.0.0.1:6380")
	t.Setenv("PLATFORM_REDIS_PASSWORD", "secret")
	t.Setenv("PLATFORM_REDIS_DB", "2")
	t.Setenv("PLATFORM_RATE_LIMIT_HMAC_KEY", strings.Repeat("r", 32))
	t.Setenv("PLATFORM_SENSITIVE_REVEAL_HMAC_KEY", strings.Repeat("s", 32))

	cfg := Load()
	if cfg.CacheDriver != "redis" {
		t.Fatalf("CacheDriver = %q", cfg.CacheDriver)
	}
	if cfg.CacheDefaultTTL.String() != "1m30s" {
		t.Fatalf("CacheDefaultTTL = %s", cfg.CacheDefaultTTL)
	}
	if cfg.RedisAddr != "127.0.0.1:6380" {
		t.Fatalf("RedisAddr = %q", cfg.RedisAddr)
	}
	if cfg.RedisPassword != "secret" {
		t.Fatalf("RedisPassword = %q", cfg.RedisPassword)
	}
	if cfg.RedisDB != 2 {
		t.Fatalf("RedisDB = %d", cfg.RedisDB)
	}
	if cfg.RateLimitHMACKey != strings.Repeat("r", 32) {
		t.Fatalf("RateLimitHMACKey = %q", cfg.RateLimitHMACKey)
	}
	if cfg.SensitiveRevealHMACKey != strings.Repeat("s", 32) {
		t.Fatalf("SensitiveRevealHMACKey = %q", cfg.SensitiveRevealHMACKey)
	}
}

func TestValidateRuntimeRejectsShortSensitiveRevealHMACKey(t *testing.T) {
	cfg := validDevelopmentOIDCConfig("https://admin.example/login")
	cfg.SensitiveRevealHMACKey = "short"
	if err := cfg.ValidateRuntime(); err == nil || !strings.Contains(err.Error(), "PLATFORM_SENSITIVE_REVEAL_HMAC_KEY must contain at least 32 bytes") {
		t.Fatalf("ValidateRuntime() error = %v", err)
	}
}

func TestValidateRuntimeRejectsReusedSensitiveRevealHMACKey(t *testing.T) {
	tests := []struct {
		name  string
		apply func(*Config)
	}{
		{name: "jwt", apply: func(cfg *Config) { cfg.SensitiveRevealHMACKey = cfg.JWTSecret }},
		{name: "phone", apply: func(cfg *Config) {
			cfg.PhoneHMACKey = strings.Repeat("p", 32)
			cfg.SensitiveRevealHMACKey = cfg.PhoneHMACKey
		}},
		{name: "code", apply: func(cfg *Config) {
			cfg.PhoneCodeHMACKey = strings.Repeat("c", 32)
			cfg.SensitiveRevealHMACKey = cfg.PhoneCodeHMACKey
		}},
		{name: "rate limit", apply: func(cfg *Config) { cfg.SensitiveRevealHMACKey = cfg.RateLimitHMACKey }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validProductionRuntimeConfig()
			tt.apply(&cfg)
			if err := cfg.ValidateRuntime(); err == nil || !strings.Contains(err.Error(), "PLATFORM_SENSITIVE_REVEAL_HMAC_KEY must be distinct from JWT, phone, code, and rate-limit keys") {
				t.Fatalf("ValidateRuntime() error = %v", err)
			}
		})
	}
}

func TestValidateRuntimeRejectsUnsafeProductionRateLimitHMACKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{name: "missing", key: ""},
		{name: "short", key: "short-rate-limit-key"},
		{name: "same as phone", key: strings.Repeat("p", 32)},
		{name: "same as code", key: strings.Repeat("c", 32)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validProductionRuntimeConfig()
			cfg.Capabilities = append(cfg.Capabilities, "app-phone")
			cfg.PhoneHMACKey = strings.Repeat("p", 32)
			cfg.PhoneCodeHMACKey = strings.Repeat("c", 32)
			cfg.PhoneVerificationProvider = "sms-vendor"
			cfg.RateLimitHMACKey = tt.key

			err := cfg.ValidateRuntime()
			if err == nil {
				t.Fatalf("ValidateRuntime() error = nil, want rate limit key rejection")
			}
			if tt.name == "missing" || tt.name == "short" {
				if !strings.Contains(err.Error(), "PLATFORM_RATE_LIMIT_HMAC_KEY to be at least 32 bytes") {
					t.Fatalf("ValidateRuntime() error = %v, want key length rejection", err)
				}
			} else if !strings.Contains(err.Error(), "distinct from phone and code HMAC keys") {
				t.Fatalf("ValidateRuntime() error = %v, want key separation rejection", err)
			}
		})
	}
}

func TestLoadParsesFileStorageConfig(t *testing.T) {
	t.Setenv("PLATFORM_FILE_STORAGE_DRIVER", "s3")
	t.Setenv("PLATFORM_FILE_STORAGE_LOCAL_DIR", "/tmp/platform-files")
	t.Setenv("PLATFORM_FILE_MAX_UPLOAD_BYTES", "2097152")
	t.Setenv("PLATFORM_FILE_ALLOWED_MIME_TYPES", "image/png,application/pdf")
	t.Setenv("PLATFORM_FILE_STORAGE_S3_ENDPOINT", "https://s3.example.test")
	t.Setenv("PLATFORM_FILE_STORAGE_S3_REGION", "ap-southeast-1")
	t.Setenv("PLATFORM_FILE_STORAGE_S3_BUCKET", "platform")
	t.Setenv("PLATFORM_FILE_STORAGE_S3_ACCESS_KEY", "access")
	t.Setenv("PLATFORM_FILE_STORAGE_S3_SECRET_KEY", "secret")
	t.Setenv("PLATFORM_FILE_STORAGE_S3_PREFIX", "tenant/platform")
	t.Setenv("PLATFORM_FILE_STORAGE_S3_FORCE_PATH_STYLE", "true")
	t.Setenv("PLATFORM_FILE_STORAGE_S3_SERVER_SIDE_ENCRYPTION", "aws:kms")
	t.Setenv("PLATFORM_FILE_STORAGE_S3_KMS_KEY_ID", "kms-key")

	cfg := Load()
	if cfg.FileStorageDriver != "s3" {
		t.Fatalf("FileStorageDriver = %q", cfg.FileStorageDriver)
	}
	if cfg.FileStorageLocalDir != "/tmp/platform-files" {
		t.Fatalf("FileStorageLocalDir = %q", cfg.FileStorageLocalDir)
	}
	if cfg.FileMaxUploadBytes != 2097152 || !reflect.DeepEqual(cfg.FileAllowedMIMETypes, []string{"image/png", "application/pdf"}) {
		t.Fatalf("upload policy mismatch: %+v", cfg)
	}
	if cfg.FileStorageS3Endpoint != "https://s3.example.test" || cfg.FileStorageS3Region != "ap-southeast-1" || cfg.FileStorageS3Bucket != "platform" {
		t.Fatalf("S3 config mismatch: %+v", cfg)
	}
	if cfg.FileStorageS3AccessKey != "access" || cfg.FileStorageS3SecretKey != "secret" || cfg.FileStorageS3Prefix != "tenant/platform" {
		t.Fatalf("S3 credential/prefix config mismatch: %+v", cfg)
	}
	if !cfg.FileStorageS3PathStyle {
		t.Fatalf("FileStorageS3PathStyle = false, want true")
	}
	if cfg.FileStorageS3ServerSideEncryption != "aws:kms" || cfg.FileStorageS3KMSKeyID != "kms-key" {
		t.Fatalf("S3 encryption config mismatch: %+v", cfg)
	}
}

func TestLoadPreservesInvalidFileUploadLimitForRuntimeValidation(t *testing.T) {
	t.Setenv("PLATFORM_FILE_MAX_UPLOAD_BYTES", "not-a-number")

	cfg := Load()
	if cfg.FileMaxUploadBytes >= 0 {
		t.Fatalf("FileMaxUploadBytes = %d, want invalid sentinel", cfg.FileMaxUploadBytes)
	}
	if err := cfg.ValidateRuntime(); err == nil || !strings.Contains(err.Error(), "file upload limit") {
		t.Fatalf("ValidateRuntime() error = %v, want invalid file upload limit", err)
	}
}

func TestLoadProductionRequiresExplicitFileSecurityPolicy(t *testing.T) {
	for _, test := range []struct {
		name  string
		empty bool
	}{
		{name: "missing"},
		{name: "explicit empty", empty: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			setValidProductionLoadEnvironment(t)
			for _, key := range []string{
				"PLATFORM_FILE_MAX_UPLOAD_BYTES",
				"PLATFORM_FILE_ALLOWED_MIME_TYPES",
				"PLATFORM_FILE_STORAGE_S3_SERVER_SIDE_ENCRYPTION",
			} {
				if test.empty {
					t.Setenv(key, "")
					continue
				}
				if err := os.Unsetenv(key); err != nil {
					t.Fatalf("Unsetenv(%s): %v", key, err)
				}
			}

			cfg := Load()
			err := cfg.ValidateRuntime()
			if err == nil {
				t.Fatal("ValidateRuntime() error = nil, want explicit production file policy errors")
			}
			for _, key := range []string{
				"PLATFORM_FILE_MAX_UPLOAD_BYTES",
				"PLATFORM_FILE_ALLOWED_MIME_TYPES",
				"PLATFORM_FILE_STORAGE_S3_SERVER_SIDE_ENCRYPTION",
			} {
				if !strings.Contains(err.Error(), key+" to be explicitly configured") {
					t.Fatalf("ValidateRuntime() error = %q, missing explicit %s error", err, key)
				}
			}
		})
	}
}

func TestLoadProductionAcceptsExplicitFileSecurityPolicy(t *testing.T) {
	setValidProductionLoadEnvironment(t)

	cfg := Load()
	if err := cfg.ValidateRuntime(); err != nil {
		t.Fatalf("ValidateRuntime() error = %v", err)
	}
}

func TestLoadProductionDistinguishesInvalidFileUploadLimitFromMissing(t *testing.T) {
	setValidProductionLoadEnvironment(t)
	t.Setenv("PLATFORM_FILE_MAX_UPLOAD_BYTES", "not-a-number")

	cfg := Load()
	err := cfg.ValidateRuntime()
	if err == nil || !strings.Contains(err.Error(), "file upload limit") {
		t.Fatalf("ValidateRuntime() error = %v, want invalid upload limit", err)
	}
	if strings.Contains(err.Error(), "PLATFORM_FILE_MAX_UPLOAD_BYTES to be explicitly configured") {
		t.Fatalf("ValidateRuntime() error = %v, invalid value must not be reported as missing", err)
	}
}

func TestValidateRuntimeRejectsProductionS3WithoutPrivateEncryptionPolicy(t *testing.T) {
	cfg := validProductionRuntimeConfig()
	cfg.FileStorageS3ServerSideEncryption = ""

	err := cfg.ValidateRuntime()
	if err == nil || !strings.Contains(err.Error(), "production s3 file storage requires server-side encryption") {
		t.Fatalf("ValidateRuntime() error = %v, want S3 encryption policy error", err)
	}
}

func TestValidateRuntimeRejectsKMSWithoutKeyAndNonHTTPSEndpoint(t *testing.T) {
	cfg := validProductionRuntimeConfig()
	cfg.FileStorageS3ServerSideEncryption = "aws:kms"
	cfg.FileStorageS3KMSKeyID = ""
	cfg.FileStorageS3Endpoint = "http://s3.example.test"

	err := cfg.ValidateRuntime()
	if err == nil || !strings.Contains(err.Error(), "KMS key ID") || !strings.Contains(err.Error(), "must use https") {
		t.Fatalf("ValidateRuntime() error = %v, want KMS key and HTTPS errors", err)
	}
}

func TestValidateRuntimeAllowsLoopbackHTTPObjectStorageOnlyInDevelopmentOrTest(t *testing.T) {
	cfg := Config{
		RuntimeEnvironment:                RuntimeEnvironmentDevelopment,
		HTTPAddr:                          "127.0.0.1:9200",
		Capabilities:                      []string{"tenant"},
		JWTSecret:                         "development-secret",
		CacheDefaultTTL:                   1,
		FileStorageDriver:                 "s3",
		FileStorageS3Endpoint:             "http://127.0.0.1:9000",
		FileStorageS3Region:               "us-east-1",
		FileStorageS3Bucket:               "platform",
		FileStorageS3ServerSideEncryption: "AES256",
	}

	if err := cfg.ValidateRuntime(); err != nil {
		t.Fatalf("ValidateRuntime() error = %v", err)
	}
	cfg.RuntimeEnvironment = RuntimeEnvironmentStaging
	if err := cfg.ValidateRuntime(); err == nil || !strings.Contains(err.Error(), "must use https") {
		t.Fatalf("staging ValidateRuntime() error = %v, want HTTPS error", err)
	}
}

func TestValidateRuntimeRejectsEmptyOrUnboundedProductionUploadPolicy(t *testing.T) {
	cfg := validProductionRuntimeConfig()
	cfg.FileMaxUploadBytes = 0
	cfg.FileAllowedMIMETypes = nil

	err := cfg.ValidateRuntime()
	if err == nil || !strings.Contains(err.Error(), "positive bounded file upload limit") || !strings.Contains(err.Error(), "non-empty file MIME allowlist") {
		t.Fatalf("ValidateRuntime() error = %v, want production upload policy errors", err)
	}

	cfg = validProductionRuntimeConfig()
	cfg.FileMaxUploadBytes = 1 << 31
	err = cfg.ValidateRuntime()
	if err == nil || !strings.Contains(err.Error(), "positive bounded file upload limit") {
		t.Fatalf("ValidateRuntime() error = %v, want bounded upload limit error", err)
	}
}

func TestLoadParsesWechatMiniAppConfig(t *testing.T) {
	t.Setenv("PLATFORM_WECHAT_MINIAPP_APP_ID", "wx-app")
	t.Setenv("PLATFORM_WECHAT_MINIAPP_SECRET", "wx-secret")
	t.Setenv("PLATFORM_WECHAT_MINIAPP_CODE2SESSION_ENDPOINT", "https://wechat.example.test/sns/jscode2session")

	cfg := Load()
	if cfg.WechatMiniAppID != "wx-app" {
		t.Fatalf("WechatMiniAppID = %q", cfg.WechatMiniAppID)
	}
	if cfg.WechatMiniAppSecret != "wx-secret" {
		t.Fatalf("WechatMiniAppSecret = %q", cfg.WechatMiniAppSecret)
	}
	if cfg.WechatMiniAppCode2SessionEndpoint != "https://wechat.example.test/sns/jscode2session" {
		t.Fatalf("WechatMiniAppCode2SessionEndpoint = %q", cfg.WechatMiniAppCode2SessionEndpoint)
	}
}

func TestLoadParsesAdminOIDCConfiguration(t *testing.T) {
	t.Setenv("PLATFORM_ADMIN_OIDC_ISSUER_URL", "https://id.example/realms/platform")
	t.Setenv("PLATFORM_ADMIN_OIDC_CLIENT_ID", "platform-admin")
	t.Setenv("PLATFORM_ADMIN_OIDC_CLIENT_SECRET", "client-secret")
	t.Setenv("PLATFORM_ADMIN_OIDC_REDIRECT_URL", "https://admin.example/login")
	t.Setenv("PLATFORM_ADMIN_OIDC_SCOPES", "openid,profile,email")

	cfg := Load()
	if !cfg.AdminOIDCConfigured() || len(cfg.AdminOIDCScopes) != 3 {
		t.Fatalf("AdminOIDCConfigured() = %t, scopes = %d", cfg.AdminOIDCConfigured(), len(cfg.AdminOIDCScopes))
	}
}

func TestLoadParsesPhoneVerificationConfiguration(t *testing.T) {
	t.Setenv("PLATFORM_PHONE_HMAC_KEY", "phone-key")
	t.Setenv("PLATFORM_PHONE_CODE_HMAC_KEY", "code-key")
	t.Setenv("PLATFORM_PHONE_VERIFICATION_PROVIDER", "SMS-VENDOR")

	cfg := Load()
	if cfg.PhoneHMACKey != "phone-key" || cfg.PhoneCodeHMACKey != "code-key" || cfg.PhoneVerificationProvider != "SMS-VENDOR" {
		t.Fatalf("phone verification config = %+v", cfg)
	}
}

func TestLoadParsesAdminStepUpPhoneSource(t *testing.T) {
	t.Setenv("PLATFORM_ADMIN_STEP_UP_PHONE_RESOURCE", "staff-profiles")
	t.Setenv("PLATFORM_ADMIN_STEP_UP_PHONE_ACTOR_FIELD", "accountCode")
	t.Setenv("PLATFORM_ADMIN_STEP_UP_PHONE_FIELD", "mobile")
	t.Setenv("PLATFORM_ADMIN_STEP_UP_PHONE_VERIFIED_AT_FIELD", "mobileVerifiedAt")
	t.Setenv("PLATFORM_ADMIN_STEP_UP_PHONE_VERIFIED_DIGEST_FIELD", "mobileVerifiedDigest")

	cfg := Load()
	if !cfg.AdminStepUpPhoneSourceConfigured() {
		t.Fatalf("admin step-up phone source = %+v, want configured", cfg)
	}
	if cfg.AdminStepUpPhoneResource != "staff-profiles" || cfg.AdminStepUpPhoneActorField != "accountCode" || cfg.AdminStepUpPhoneField != "mobile" || cfg.AdminStepUpPhoneVerifiedAtField != "mobileVerifiedAt" || cfg.AdminStepUpPhoneVerifiedDigestField != "mobileVerifiedDigest" {
		t.Fatalf("admin step-up phone source = %+v", cfg)
	}
}

func TestValidateRuntimeRejectsPartialOrUntrimmedAdminStepUpPhoneSource(t *testing.T) {
	cfg := Load()
	cfg.AdminStepUpPhoneResource = "staff-profiles"
	if err := cfg.ValidateRuntime(); err == nil || !strings.Contains(err.Error(), "must be configured together") {
		t.Fatalf("ValidateRuntime() partial source error = %v", err)
	}

	cfg.AdminStepUpPhoneActorField = "accountCode"
	cfg.AdminStepUpPhoneField = " mobile "
	cfg.AdminStepUpPhoneVerifiedAtField = "mobileVerifiedAt"
	cfg.AdminStepUpPhoneVerifiedDigestField = "mobileVerifiedDigest"
	if err := cfg.ValidateRuntime(); err == nil || !strings.Contains(err.Error(), "PLATFORM_ADMIN_STEP_UP_PHONE_FIELD must be trimmed") {
		t.Fatalf("ValidateRuntime() untrimmed source error = %v", err)
	}
}

func TestValidateRuntimeAcceptsDevelopmentDefaults(t *testing.T) {
	cfg := Load()

	if err := cfg.ValidateRuntime(); err != nil {
		t.Fatalf("ValidateRuntime() error = %v", err)
	}
}

func TestValidateRuntimeRejectsProductionNonHTTPSPublicBaseURL(t *testing.T) {
	for _, publicBaseURL := range []string{"", "http://platform.example.test", "https://platform.example.test/", "https://platform.example.test/path", "https://user@platform.example.test", "https://:443"} {
		t.Run(publicBaseURL, func(t *testing.T) {
			cfg := validProductionRuntimeConfig()
			cfg.PublicBaseURL = publicBaseURL

			err := cfg.ValidateRuntime()
			if err == nil || !strings.Contains(err.Error(), "production runtime requires PLATFORM_PUBLIC_BASE_URL to be an absolute HTTPS origin") {
				t.Fatalf("ValidateRuntime() error = %v, want public HTTPS origin error", err)
			}
		})
	}
}

func TestValidateRuntimeRejectsInvalidOrEmptyProductionTrustedProxyPolicy(t *testing.T) {
	tests := []struct {
		name    string
		proxies []string
	}{
		{name: "empty", proxies: nil},
		{name: "empty item", proxies: []string{"10.0.0.0/8", ""}},
		{name: "hostname", proxies: []string{"edge.internal"}},
		{name: "invalid cidr", proxies: []string{"10.0.0.0/99"}},
		{name: "trust all", proxies: []string{"0.0.0.0/0"}},
		{name: "cumulative ipv4 trust all", proxies: []string{"0.0.0.0/1", "128.0.0.0/1"}},
		{name: "cumulative ipv6 trust all", proxies: []string{"::/1", "8000::/1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validProductionRuntimeConfig()
			cfg.TrustedProxies = tt.proxies

			err := cfg.ValidateRuntime()
			if err == nil || !strings.Contains(err.Error(), "PLATFORM_TRUSTED_PROXIES") {
				t.Fatalf("ValidateRuntime() error = %v, want trusted proxy policy error", err)
			}
		})
	}
}

func TestValidateRuntimeAllowsMultipleNarrowTrustedProxyCIDRs(t *testing.T) {
	cfg := validProductionRuntimeConfig()
	cfg.TrustedProxies = []string{"10.0.0.0/8", "192.168.0.0/16", "2001:db8::/32"}

	if err := cfg.ValidateRuntime(); err != nil {
		t.Fatalf("ValidateRuntime() error = %v", err)
	}
}

func TestValidateRuntimeRejectsInvalidProductionEdgeTrustedProxy(t *testing.T) {
	for _, value := range []string{
		"", "edge.internal", "0.0.0.0", "::", "127.0.0.1", "::1", "224.0.0.1", "ff02::1",
		"0.0.0.0/0", "128.0.0.0/1", "172.30.0.1/24", "172.30.0.1/32",
		"8000::/1", "2001:db8::/64", "2001:db8::1/128", "10.0.0.0/99",
		"2001:DB8::1", " 172.30.0.1 ",
	} {
		t.Run(value, func(t *testing.T) {
			cfg := validProductionRuntimeConfig()
			cfg.EdgeTrustedProxy = value

			err := cfg.ValidateRuntime()
			if err == nil || !strings.Contains(err.Error(), "PLATFORM_EDGE_TRUSTED_PROXY") {
				t.Fatalf("ValidateRuntime() error = %v, want edge trusted proxy rejection", err)
			}
		})
	}
}

func TestValidateRuntimeAllowsCanonicalProductionEdgeTrustedProxy(t *testing.T) {
	for _, value := range []string{"172.30.0.1", "2001:db8::1"} {
		t.Run(value, func(t *testing.T) {
			cfg := validProductionRuntimeConfig()
			cfg.EdgeTrustedProxy = value
			if err := cfg.ValidateRuntime(); err != nil {
				t.Fatalf("ValidateRuntime() error = %v", err)
			}
		})
	}
}

func TestValidateRuntimeRejectsInvalidHTTPMaxBodyBytes(t *testing.T) {
	for _, maxBytes := range []int64{0, -1, 101 << 20} {
		t.Run(strconv.FormatInt(maxBytes, 10), func(t *testing.T) {
			cfg := validProductionRuntimeConfig()
			cfg.HTTPMaxBodyBytes = maxBytes

			err := cfg.ValidateRuntime()
			if err == nil || !strings.Contains(err.Error(), "PLATFORM_HTTP_MAX_BODY_BYTES") {
				t.Fatalf("ValidateRuntime() error = %v, want HTTP body limit error", err)
			}
		})
	}
}

func TestStagingRejectsNonLoopbackHTTPProviderAndStorageEndpoints(t *testing.T) {
	cfg := validProductionRuntimeConfig()
	cfg.RuntimeEnvironment = RuntimeEnvironmentStaging
	cfg.AdminOIDCIssuerURL = "http://id.example.test/realms/platform"
	cfg.WechatMiniAppID = "wx-app"
	cfg.WechatMiniAppSecret = "wx-secret"
	cfg.WechatMiniAppCode2SessionEndpoint = "http://wechat.example.test/sns/jscode2session"
	cfg.FileStorageS3Endpoint = "http://s3.example.test"

	err := cfg.ValidateRuntime()
	if err == nil {
		t.Fatal("ValidateRuntime() error = nil, want HTTPS endpoint errors")
	}
	for _, want := range []string{"admin oidc issuer url must use https", "wechat miniapp code2session endpoint must use https", "file storage s3 endpoint must use https"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("ValidateRuntime() error = %q, missing %q", err, want)
		}
	}
}

func TestValidateRuntimeRejectsDriverWithoutDSN(t *testing.T) {
	cfg := Config{RuntimeEnvironment: "development", AdminResourceDriver: "sqlite"}

	err := cfg.ValidateRuntime()
	if err == nil {
		t.Fatalf("ValidateRuntime() error = nil, want admin resource dsn error")
	}
	if got := err.Error(); !strings.Contains(got, "admin resource dsn is required when admin resource driver is set") {
		t.Fatalf("ValidateRuntime() error = %q", got)
	}
}

func TestValidateRuntimeRejectsUnsupportedRuntimeEnvironment(t *testing.T) {
	cfg := Config{RuntimeEnvironment: "demo", JWTSecret: "development-secret", CacheDefaultTTL: 1}

	err := cfg.ValidateRuntime()
	if err == nil {
		t.Fatalf("ValidateRuntime() error = nil, want unsupported runtime environment")
	}
	if got := err.Error(); !strings.Contains(got, "unsupported runtime environment") {
		t.Fatalf("ValidateRuntime() error = %q", got)
	}
}

func TestValidateRuntimeRejectsInvalidCapabilityList(t *testing.T) {
	tests := []struct {
		name         string
		capabilities []string
		want         string
	}{
		{
			name:         "nil list",
			capabilities: nil,
			want:         "PLATFORM_CAPABILITIES must not be empty",
		},
		{
			name:         "empty list",
			capabilities: []string{},
			want:         "PLATFORM_CAPABILITIES must not be empty",
		},
		{
			name:         "blank id",
			capabilities: []string{"tenant", " "},
			want:         "PLATFORM_CAPABILITIES contains an empty capability id",
		},
		{
			name:         "invalid id",
			capabilities: []string{"tenant", "WeChat"},
			want:         `PLATFORM_CAPABILITIES capability "WeChat" must use lowercase letters, numbers, and hyphens`,
		},
		{
			name:         "duplicate id after trim",
			capabilities: []string{"tenant", " tenant "},
			want:         `PLATFORM_CAPABILITIES contains duplicate capability "tenant"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{
				RuntimeEnvironment:  "development",
				HTTPAddr:            "127.0.0.1:9200",
				JWTSecret:           "development-secret",
				CacheDefaultTTL:     1,
				FileStorageDriver:   "local",
				FileStorageLocalDir: ".platform/uploads",
				Capabilities:        tt.capabilities,
			}

			err := cfg.ValidateRuntime()
			if err == nil {
				t.Fatalf("ValidateRuntime() error = nil, want %q", tt.want)
			}
			if got := err.Error(); !strings.Contains(got, tt.want) {
				t.Fatalf("ValidateRuntime() error = %q, want containing %q", got, tt.want)
			}
		})
	}
}

func TestValidateRuntimeRejectsProductionWithoutPersistentRuntime(t *testing.T) {
	cfg := Config{
		RuntimeEnvironment:  "production",
		JWTSecret:           "dev-platform-go-secret",
		CacheDriver:         "memory",
		CacheDefaultTTL:     1,
		FileStorageDriver:   "local",
		FileStorageLocalDir: ".platform/uploads",
	}

	err := cfg.ValidateRuntime()
	if err == nil {
		t.Fatalf("ValidateRuntime() error = nil, want production baseline errors")
	}
	got := err.Error()
	for _, want := range []string{
		"production runtime requires PLATFORM_JWT_SECRET to be changed from the development default",
		"production runtime requires PLATFORM_ADMIN_RESOURCE_DRIVER",
		"production runtime requires PLATFORM_SESSION_DRIVER",
		"production runtime requires PLATFORM_LIFECYCLE_HISTORY_DRIVER",
		"production runtime requires PLATFORM_CACHE_DRIVER=redis",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("ValidateRuntime() error = %q, missing %q", got, want)
		}
	}
}

func TestValidateRuntimeRejectsProductionShortJWTSecret(t *testing.T) {
	cfg := validProductionRuntimeConfig()
	cfg.JWTSecret = "short-production-secret"

	err := cfg.ValidateRuntime()
	if err == nil {
		t.Fatalf("ValidateRuntime() error = nil, want short jwt secret error")
	}
	if got := err.Error(); !strings.Contains(got, "production runtime requires PLATFORM_JWT_SECRET to be at least 32 characters") {
		t.Fatalf("ValidateRuntime() error = %q", got)
	}
}

func TestValidateRuntimeRejectsProductionNonGORMDrivers(t *testing.T) {
	cfg := validProductionRuntimeConfig()
	cfg.AdminResourceDriver = "file"
	cfg.SessionDriver = "memory"
	cfg.LifecycleHistoryDriver = "json"

	err := cfg.ValidateRuntime()
	if err == nil {
		t.Fatalf("ValidateRuntime() error = nil, want production driver errors")
	}
	got := err.Error()
	for _, want := range []string{
		`unsupported admin resource driver "file"`,
		`unsupported session driver "memory"`,
		`unsupported lifecycle history driver "json"`,
		"production runtime requires PLATFORM_ADMIN_RESOURCE_DRIVER to be mysql, postgres, or sqlite",
		"production runtime requires PLATFORM_SESSION_DRIVER to be mysql, postgres, or sqlite",
		"production runtime requires PLATFORM_LIFECYCLE_HISTORY_DRIVER to be mysql, postgres, or sqlite",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("ValidateRuntime() error = %q, missing %q", got, want)
		}
	}
}

func TestValidateRuntimeRejectsProductionRedisWithoutAddress(t *testing.T) {
	cfg := validProductionRuntimeConfig()
	cfg.RedisAddr = ""

	err := cfg.ValidateRuntime()
	if err == nil {
		t.Fatalf("ValidateRuntime() error = nil, want redis address error")
	}
	if got := err.Error(); !strings.Contains(got, "redis address is required when cache driver is redis") {
		t.Fatalf("ValidateRuntime() error = %q", got)
	}
}

func TestValidateRuntimeRejectsProductionDemoDataCapability(t *testing.T) {
	cfg := validProductionRuntimeConfig()
	cfg.Capabilities = append(cfg.Capabilities, "demo-data")

	err := cfg.ValidateRuntime()
	if err == nil {
		t.Fatalf("ValidateRuntime() error = nil, want demo-data capability rejection")
	}
	if got := err.Error(); !strings.Contains(got, "production runtime must not enable demo-data capability") {
		t.Fatalf("ValidateRuntime() error = %q", got)
	}
}

func TestValidateRuntimeRejectsProductionDemoAuthProvider(t *testing.T) {
	cfg := validProductionRuntimeConfig()
	cfg.DisableDemoAuthProvider = false

	err := cfg.ValidateRuntime()
	if err == nil {
		t.Fatalf("ValidateRuntime() error = nil, want demo auth provider rejection")
	}
	if got := err.Error(); !strings.Contains(got, "production runtime requires PLATFORM_DISABLE_DEMO_AUTH_PROVIDER=true") {
		t.Fatalf("ValidateRuntime() error = %q", got)
	}
}

func TestValidateRuntimeRejectsProductionAppPhoneWithoutProviderAndDistinctKeys(t *testing.T) {
	cfg := validProductionRuntimeConfig()
	cfg.Capabilities = append(cfg.Capabilities, "app-phone")
	cfg.PhoneHMACKey = "short"
	cfg.PhoneCodeHMACKey = "short"

	err := cfg.ValidateRuntime()
	if err == nil {
		t.Fatal("ValidateRuntime() error = nil, want app-phone protection errors")
	}
	for _, want := range []string{
		"production app-phone requires PLATFORM_PHONE_HMAC_KEY to be at least 32 bytes",
		"production app-phone requires PLATFORM_PHONE_CODE_HMAC_KEY to be at least 32 bytes",
		"production app-phone requires distinct phone and code HMAC keys",
		"production app-phone requires PLATFORM_PHONE_VERIFICATION_PROVIDER",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("ValidateRuntime() error = %q, missing %q", err, want)
		}
	}
}

func TestValidateRuntimeRejectsDebugPhoneProviderOutsideDevelopmentAndTest(t *testing.T) {
	for _, environment := range []string{RuntimeEnvironmentStaging, RuntimeEnvironmentProduction} {
		t.Run(environment, func(t *testing.T) {
			cfg := validProductionRuntimeConfig()
			cfg.RuntimeEnvironment = environment
			cfg.Capabilities = append(cfg.Capabilities, "app-phone")
			cfg.PhoneHMACKey = strings.Repeat("p", 32)
			cfg.PhoneCodeHMACKey = strings.Repeat("c", 32)
			cfg.PhoneVerificationProvider = "debug"

			err := cfg.ValidateRuntime()
			if err == nil || !strings.Contains(err.Error(), "app-phone debug provider is allowed only in development or test") {
				t.Fatalf("ValidateRuntime() error = %v, want debug provider environment error", err)
			}
		})
	}
}

func TestValidateRuntimeRejectsNonCanonicalPhoneProvider(t *testing.T) {
	cfg := validProductionRuntimeConfig()
	cfg.Capabilities = append(cfg.Capabilities, "app-phone")
	cfg.PhoneHMACKey = strings.Repeat("p", 32)
	cfg.PhoneCodeHMACKey = strings.Repeat("c", 32)
	cfg.PhoneVerificationProvider = " SMS-VENDOR "

	err := cfg.ValidateRuntime()
	if err == nil || !strings.Contains(err.Error(), "PLATFORM_PHONE_VERIFICATION_PROVIDER to be canonical trimmed lowercase") {
		t.Fatalf("ValidateRuntime() error = %v, want canonical provider error", err)
	}
}

func TestValidateRuntimeRejectsUnknownPhoneProvider(t *testing.T) {
	cfg := validProductionRuntimeConfig()
	cfg.Capabilities = append(cfg.Capabilities, "app-phone")
	cfg.PhoneHMACKey = strings.Repeat("p", 32)
	cfg.PhoneCodeHMACKey = strings.Repeat("c", 32)
	cfg.PhoneVerificationProvider = "unknown"

	err := cfg.ValidateRuntime()
	if err == nil || !strings.Contains(err.Error(), "PLATFORM_PHONE_VERIFICATION_PROVIDER must identify a configured provider") {
		t.Fatalf("ValidateRuntime() error = %v, want unknown provider error", err)
	}
}

func TestValidateRuntimeAcceptsProductionAppPhoneProtectionConfig(t *testing.T) {
	cfg := validProductionRuntimeConfig()
	cfg.Capabilities = append(cfg.Capabilities, "app-phone")
	cfg.PhoneHMACKey = strings.Repeat("p", 32)
	cfg.PhoneCodeHMACKey = strings.Repeat("c", 32)
	cfg.PhoneVerificationProvider = "sms-vendor"

	if err := cfg.ValidateRuntime(); err != nil {
		t.Fatalf("ValidateRuntime() error = %v", err)
	}
}

func TestValidateRuntimeRequiresPhoneProtectionForAdminStepUpWithoutAppPhone(t *testing.T) {
	cfg := Load()
	cfg.AdminStepUpPhoneResource = "staff-profiles"
	cfg.AdminStepUpPhoneActorField = "accountCode"
	cfg.AdminStepUpPhoneField = "mobile"
	cfg.AdminStepUpPhoneVerifiedAtField = "mobileVerifiedAt"
	cfg.AdminStepUpPhoneVerifiedDigestField = "mobileVerifiedDigest"

	err := cfg.ValidateRuntime()
	if err == nil {
		t.Fatal("ValidateRuntime() error = nil, want Admin step-up phone protection errors")
	}
	for _, want := range []string{
		"admin step-up phone requires PLATFORM_PHONE_HMAC_KEY to be at least 32 bytes",
		"admin step-up phone requires PLATFORM_PHONE_CODE_HMAC_KEY to be at least 32 bytes",
		"admin step-up phone requires distinct phone and code HMAC keys",
		"admin step-up phone requires PLATFORM_PHONE_VERIFICATION_PROVIDER",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("ValidateRuntime() error = %q, missing %q", err, want)
		}
	}
}

func TestValidateRuntimeRejectsPartialAdminOIDCCredentials(t *testing.T) {
	tests := []struct {
		name  string
		clear func(*Config)
	}{
		{name: "issuer", clear: func(cfg *Config) { cfg.AdminOIDCIssuerURL = "" }},
		{name: "client id", clear: func(cfg *Config) { cfg.AdminOIDCClientID = "" }},
		{name: "client secret", clear: func(cfg *Config) { cfg.AdminOIDCClientSecret = "" }},
		{name: "redirect", clear: func(cfg *Config) { cfg.AdminOIDCRedirectURL = "" }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validDevelopmentOIDCConfig("https://admin.example/login")
			tt.clear(&cfg)

			err := cfg.ValidateRuntime()
			if err == nil || !strings.Contains(err.Error(), "admin oidc issuer, client id, client secret, and redirect url must be configured together") {
				t.Fatalf("ValidateRuntime() error = %v, want partial OIDC configuration error", err)
			}
		})
	}
}

func TestValidateRuntimeRejectsAdminOIDCScopesWithoutOpenID(t *testing.T) {
	cfg := validDevelopmentOIDCConfig("https://admin.example/login")
	cfg.AdminOIDCScopes = []string{"profile", "email"}

	err := cfg.ValidateRuntime()
	if err == nil || !strings.Contains(err.Error(), "admin oidc scopes must include openid") {
		t.Fatalf("ValidateRuntime() error = %v, want missing openid error", err)
	}
}

func TestValidateRuntimeRejectsProductionAdminOIDCNonHTTPSRedirect(t *testing.T) {
	cfg := validProductionRuntimeConfig()
	cfg.AdminOIDCRedirectURL = "http://admin.example/login"

	err := cfg.ValidateRuntime()
	if err == nil || !strings.Contains(err.Error(), "production admin oidc redirect url must use https") {
		t.Fatalf("ValidateRuntime() error = %v, want HTTPS redirect error", err)
	}
}

func TestValidateRuntimeAcceptsDevelopmentAndTestLoopbackAdminOIDCRedirects(t *testing.T) {
	for _, tt := range []struct {
		environment string
		redirectURL string
	}{
		{environment: RuntimeEnvironmentDevelopment, redirectURL: "http://localhost:5173/login"},
		{environment: RuntimeEnvironmentTest, redirectURL: "http://127.0.0.1:5173/login"},
	} {
		t.Run(tt.environment, func(t *testing.T) {
			cfg := validDevelopmentOIDCConfig(tt.redirectURL)
			cfg.RuntimeEnvironment = tt.environment
			if err := cfg.ValidateRuntime(); err != nil {
				t.Fatalf("ValidateRuntime() error = %v", err)
			}
		})
	}
}

func TestValidateRuntimeRejectsProductionWithoutConfiguredAdminProvider(t *testing.T) {
	cfg := validProductionRuntimeConfig()
	cfg.AdminOIDCClientSecret = ""

	err := cfg.ValidateRuntime()
	if err == nil || !strings.Contains(err.Error(), "production runtime requires a configured admin auth provider") {
		t.Fatalf("ValidateRuntime() error = %v, want configured admin provider error", err)
	}
}

func TestValidateRuntimeAcceptsProductionBaseline(t *testing.T) {
	cfg := validProductionRuntimeConfig()

	if err := cfg.ValidateRuntime(); err != nil {
		t.Fatalf("ValidateRuntime() error = %v", err)
	}
}

func validProductionRuntimeConfig() Config {
	cfg := Config{
		RuntimeEnvironment:                "production",
		HTTPAddr:                          "0.0.0.0:9200",
		PublicBaseURL:                     "https://platform.example.test",
		TrustedProxies:                    []string{"10.20.0.0/16"},
		EdgeTrustedProxy:                  "172.30.0.1",
		HTTPMaxBodyBytes:                  1 << 20,
		AdminResourceDriver:               "postgres",
		AdminResourceDSN:                  "postgres://platform:secret@localhost:5432/platform",
		SessionDriver:                     "postgres",
		SessionDSN:                        "postgres://platform:secret@localhost:5432/platform",
		LifecycleHistoryDriver:            "postgres",
		LifecycleHistoryDSN:               "postgres://platform:secret@localhost:5432/platform",
		JWTSecret:                         "0123456789abcdef0123456789abcdef",
		CacheDriver:                       "redis",
		CacheDefaultTTL:                   1,
		RedisAddr:                         "127.0.0.1:6379",
		RateLimitHMACKey:                  strings.Repeat("r", 32),
		FileStorageDriver:                 "s3",
		FileMaxUploadBytes:                10 << 20,
		FileAllowedMIMETypes:              []string{"application/pdf", "image/jpeg", "image/png", "text/plain"},
		FileStorageS3Region:               "us-east-1",
		FileStorageS3Bucket:               "platform",
		FileStorageS3AccessKey:            "access",
		FileStorageS3SecretKey:            "secret",
		FileStorageS3ServerSideEncryption: "AES256",
		DisableDemoAuthProvider:           true,
		AdminOIDCIssuerURL:                "https://id.example/realms/platform",
		AdminOIDCClientID:                 "platform-admin",
		AdminOIDCClientSecret:             "client-secret",
		AdminOIDCRedirectURL:              "https://admin.example/login",
		AdminOIDCScopes:                   []string{"openid", "profile", "email"},
		Capabilities: []string{
			"dictionary",
			"tenant",
			"identity",
			"session",
			"rbac",
			"menu",
			"admin-shell",
			"admin-oidc",
		},
	}
	dataProtection := validDataProtectionConfig(RuntimeEnvironmentProduction, "env-aes256")
	cfg.DataKeyProvider = dataProtection.DataKeyProvider
	cfg.DataEncryptionActiveKeyID = dataProtection.DataEncryptionActiveKeyID
	cfg.DataEncryptionKeyringJSON = dataProtection.DataEncryptionKeyringJSON
	cfg.DataBlindIndexActiveKeyID = dataProtection.DataBlindIndexActiveKeyID
	cfg.DataBlindIndexKeyringJSON = dataProtection.DataBlindIndexKeyringJSON
	return cfg
}

func validDataProtectionConfig(environment string, provider string) Config {
	encodedEncryption := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("e", 32)))
	encodedIndex := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("i", 32)))
	return Config{
		RuntimeEnvironment:        environment,
		HTTPAddr:                  "127.0.0.1:9200",
		JWTSecret:                 "development-secret",
		CacheDefaultTTL:           1,
		FileStorageDriver:         "local",
		FileStorageLocalDir:       ".platform/uploads",
		DataKeyProvider:           provider,
		DataEncryptionActiveKeyID: "enc-v1",
		DataEncryptionKeyringJSON: "{\"enc-v1\":\"" + encodedEncryption + "\"}",
		DataBlindIndexActiveKeyID: "idx-v1",
		DataBlindIndexKeyringJSON: "{\"idx-v1\":\"" + encodedIndex + "\"}",
		Capabilities:              []string{"tenant"},
	}
}

func validDevelopmentOIDCConfig(redirectURL string) Config {
	return Config{
		RuntimeEnvironment:    RuntimeEnvironmentDevelopment,
		HTTPAddr:              "127.0.0.1:9200",
		Capabilities:          []string{"tenant"},
		JWTSecret:             "development-secret",
		CacheDefaultTTL:       1,
		FileStorageDriver:     "local",
		FileStorageLocalDir:   ".platform/uploads",
		AdminOIDCIssuerURL:    "https://id.example/realms/platform",
		AdminOIDCClientID:     "platform-admin",
		AdminOIDCClientSecret: "client-secret",
		AdminOIDCRedirectURL:  redirectURL,
		AdminOIDCScopes:       []string{"openid", "profile", "email"},
	}
}

func setValidProductionLoadEnvironment(t *testing.T) {
	t.Helper()
	encodedEncryption := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("e", 32)))
	encodedIndex := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("i", 32)))
	values := map[string]string{
		"PLATFORM_RUNTIME_ENV":                            "production",
		"PLATFORM_HTTP_ADDR":                              "0.0.0.0:9200",
		"PLATFORM_PUBLIC_BASE_URL":                        "https://platform.example.test",
		"PLATFORM_TRUSTED_PROXIES":                        "10.20.0.0/16",
		"PLATFORM_EDGE_TRUSTED_PROXY":                     "172.30.0.1",
		"PLATFORM_HTTP_MAX_BODY_BYTES":                    "1048576",
		"PLATFORM_CAPABILITIES":                           "tenant,identity,session,rbac,menu,audit,dictionary,parameter,file-storage,admin-shell,admin-oidc",
		"PLATFORM_JWT_SECRET":                             "0123456789abcdef0123456789abcdef",
		"PLATFORM_ADMIN_RESOURCE_DRIVER":                  "postgres",
		"PLATFORM_ADMIN_RESOURCE_DSN":                     "postgres://platform:secret@localhost:5432/platform",
		"PLATFORM_SESSION_DRIVER":                         "postgres",
		"PLATFORM_SESSION_DSN":                            "postgres://platform:secret@localhost:5432/platform",
		"PLATFORM_LIFECYCLE_HISTORY_DRIVER":               "postgres",
		"PLATFORM_LIFECYCLE_HISTORY_DSN":                  "postgres://platform:secret@localhost:5432/platform",
		"PLATFORM_CACHE_DRIVER":                           "redis",
		"PLATFORM_REDIS_ADDR":                             "127.0.0.1:6379",
		"PLATFORM_RATE_LIMIT_HMAC_KEY":                    "rate-limit-production-key-value-0001",
		"PLATFORM_DATA_KEY_PROVIDER":                      "env-aes256",
		"PLATFORM_DATA_ENCRYPTION_ACTIVE_KEY_ID":          "enc-v1",
		"PLATFORM_DATA_ENCRYPTION_KEYRING_JSON":           "{\"enc-v1\":\"" + encodedEncryption + "\"}",
		"PLATFORM_DATA_BLIND_INDEX_ACTIVE_KEY_ID":         "idx-v1",
		"PLATFORM_DATA_BLIND_INDEX_KEYRING_JSON":          "{\"idx-v1\":\"" + encodedIndex + "\"}",
		"PLATFORM_DISABLE_DEMO_AUTH_PROVIDER":             "true",
		"PLATFORM_FILE_STORAGE_DRIVER":                    "s3",
		"PLATFORM_FILE_MAX_UPLOAD_BYTES":                  "10485760",
		"PLATFORM_FILE_ALLOWED_MIME_TYPES":                "application/pdf,image/jpeg,image/png,text/plain",
		"PLATFORM_FILE_STORAGE_S3_REGION":                 "us-east-1",
		"PLATFORM_FILE_STORAGE_S3_BUCKET":                 "platform",
		"PLATFORM_FILE_STORAGE_S3_SERVER_SIDE_ENCRYPTION": "AES256",
		"PLATFORM_ADMIN_OIDC_ISSUER_URL":                  "https://id.example/realms/platform",
		"PLATFORM_ADMIN_OIDC_CLIENT_ID":                   "platform-admin",
		"PLATFORM_ADMIN_OIDC_CLIENT_SECRET":               "client-secret",
		"PLATFORM_ADMIN_OIDC_REDIRECT_URL":                "https://admin.example/login",
		"PLATFORM_ADMIN_OIDC_SCOPES":                      "openid,profile,email",
	}
	for key, value := range values {
		t.Setenv(key, value)
	}
}
