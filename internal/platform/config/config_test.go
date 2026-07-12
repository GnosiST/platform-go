package config

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestLoadUsesDefaults(t *testing.T) {
	t.Setenv("PLATFORM_RUNTIME_ENV", "")
	t.Setenv("PLATFORM_HTTP_ADDR", "")
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

	cfg := Load()

	if cfg.RuntimeEnvironment != "development" {
		t.Fatalf("RuntimeEnvironment = %q, want development", cfg.RuntimeEnvironment)
	}
	if cfg.HTTPAddr != "127.0.0.1:9200" {
		t.Fatalf("HTTPAddr = %q", cfg.HTTPAddr)
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
}

func TestLoadParsesRuntimeEnvironment(t *testing.T) {
	t.Setenv("PLATFORM_RUNTIME_ENV", "production")

	cfg := Load()
	if cfg.RuntimeEnvironment != "production" {
		t.Fatalf("RuntimeEnvironment = %q", cfg.RuntimeEnvironment)
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

func TestValidateRuntimeAcceptsDevelopmentDefaults(t *testing.T) {
	cfg := Load()

	if err := cfg.ValidateRuntime(); err != nil {
		t.Fatalf("ValidateRuntime() error = %v", err)
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
	return Config{
		RuntimeEnvironment:                "production",
		HTTPAddr:                          "0.0.0.0:9200",
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
	values := map[string]string{
		"PLATFORM_RUNTIME_ENV":                            "production",
		"PLATFORM_HTTP_ADDR":                              "0.0.0.0:9200",
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
