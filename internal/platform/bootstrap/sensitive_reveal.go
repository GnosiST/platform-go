package bootstrap

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"strings"
	"time"

	"platform-go/internal/platform/capability"
	"platform-go/internal/platform/config"
	"platform-go/internal/platform/sensitivereveal"
	"platform-go/internal/platform/storage"
)

func SensitiveRevealRuntimeFromConfig(cfg config.Config, manifests []capability.Manifest, phoneRuntime PhoneVerificationRuntime) (*sensitivereveal.Runtime, error) {
	policies := sensitiveRevealPolicies(manifests)
	if len(policies) == 0 {
		return nil, nil
	}
	if strings.EqualFold(strings.TrimSpace(cfg.RuntimeEnvironment), config.RuntimeEnvironmentProduction) && sensitiveRevealUsesFactor(manifests, capability.AdminRevealFactorSMSOTP) {
		if !cfg.AdminStepUpPhoneSourceConfigured() {
			return nil, errors.New("production SMS reveal factor requires a configured admin step-up phone source")
		}
		if phoneRuntime.Protector == nil || phoneVerificationSenderNil(phoneRuntime.Sender) {
			return nil, errors.New("production SMS reveal factor requires a registered phone verification sender")
		}
	}
	key := []byte(strings.TrimSpace(cfg.SensitiveRevealHMACKey))
	if len(key) == 0 {
		if strings.EqualFold(strings.TrimSpace(cfg.RuntimeEnvironment), config.RuntimeEnvironmentProduction) {
			return nil, errors.New("PLATFORM_SENSITIVE_REVEAL_HMAC_KEY is required when production reveal policies are enabled")
		}
		key = deriveSensitiveRevealKey(cfg.JWTSecret)
	}
	var store sensitivereveal.Store = sensitivereveal.NewMemoryStore()
	if isGORMAdminResourceDriver(cfg.AdminResourceDriver) {
		if strings.TrimSpace(cfg.AdminResourceDSN) == "" {
			return nil, errors.New("admin resource dsn is required for sensitive reveal persistence")
		}
		db, err := storage.OpenGORM(storage.Config{Driver: cfg.AdminResourceDriver, DSN: cfg.AdminResourceDSN})
		if err != nil {
			return nil, err
		}
		store, err = sensitivereveal.NewGORMStore(context.Background(), db)
		if err != nil {
			sqlDB, dbErr := db.DB()
			if dbErr == nil {
				_ = sqlDB.Close()
			}
			return nil, err
		}
	}
	return sensitivereveal.NewRuntime(sensitivereveal.RuntimeOptions{Store: store, Policies: policies, HashKey: key})
}

func sensitiveRevealUsesFactor(manifests []capability.Manifest, target string) bool {
	for _, manifest := range manifests {
		for _, policy := range manifest.Admin.RevealPolicies {
			for _, factor := range policy.Factors {
				if strings.TrimSpace(factor) == target {
					return true
				}
			}
		}
	}
	return false
}

func sensitiveRevealPolicies(manifests []capability.Manifest) []sensitivereveal.Policy {
	policies := make([]sensitivereveal.Policy, 0)
	for _, manifest := range manifests {
		for _, policy := range manifest.Admin.RevealPolicies {
			factors := make([]sensitivereveal.FactorRule, 0, len(policy.Factors))
			for _, factor := range policy.Factors {
				rule := sensitivereveal.FactorRule{MaxAttempts: 3}
				switch strings.TrimSpace(factor) {
				case capability.AdminRevealFactorOIDCReauthentication:
					rule.Factor = sensitivereveal.FactorOIDCReauthentication
				case capability.AdminRevealFactorSMSOTP:
					rule.Factor = sensitivereveal.FactorSMSOTP
					rule.MaxAttempts = 5
				default:
					continue
				}
				factors = append(factors, rule)
			}
			purposes := make([]string, 0, len(policy.Purposes))
			for _, purpose := range policy.Purposes {
				purposes = append(purposes, purpose.Code)
			}
			policies = append(policies, sensitivereveal.Policy{
				ID: policy.ID, Mode: sensitivereveal.PolicyMode(policy.Mode), Factors: factors, PurposeCodes: purposes,
				ChallengeTTL: time.Duration(policy.ChallengeTTLSeconds) * time.Second,
				GrantTTL:     time.Duration(policy.GrantTTLSeconds) * time.Second,
			})
		}
	}
	return policies
}

func deriveSensitiveRevealKey(jwtSecret string) []byte {
	mac := hmac.New(sha256.New, []byte(strings.TrimSpace(jwtSecret)))
	_, _ = mac.Write([]byte("platform-sensitive-reveal-hmac-v1"))
	return mac.Sum(nil)
}
