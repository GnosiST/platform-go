package bootstrap

import (
	"context"
	"strings"
	"testing"

	"github.com/GnosiST/platform-go/internal/platform/capability"
	"github.com/GnosiST/platform-go/internal/platform/config"
	"github.com/GnosiST/platform-go/internal/platform/httpapi"
	"github.com/GnosiST/platform-go/internal/platform/sensitivereveal"
)

func TestSensitiveRevealRuntimeUsesManifestPolicies(t *testing.T) {
	cfg := config.Config{RuntimeEnvironment: config.RuntimeEnvironmentDevelopment, JWTSecret: "development-jwt-secret"}
	runtime, err := SensitiveRevealRuntimeFromConfig(cfg, sensitiveRevealManifestsForTest(), PhoneVerificationRuntime{})
	if err != nil {
		t.Fatalf("SensitiveRevealRuntimeFromConfig() error = %v", err)
	}
	result, err := runtime.BeginChallenge(context.Background(), sensitivereveal.BeginChallengeRequest{
		PolicyID: "admin-sensitive-any-v1",
		Scope: sensitivereveal.Scope{
			Actor: "admin", SessionDigest: "session-digest", Tenant: "platform", Resource: "contacts", Record: "contact-1",
			Field: "phone", Purpose: "support-case", Permission: "admin:contact:reveal",
		},
	})
	if err != nil || result.Mode != sensitivereveal.PolicyAnyOf || len(result.Factors) != 2 {
		t.Fatalf("BeginChallenge() = %+v, %v", result, err)
	}
}

func TestSensitiveRevealRuntimeIsAbsentWithoutPolicies(t *testing.T) {
	runtime, err := SensitiveRevealRuntimeFromConfig(config.Config{}, nil, PhoneVerificationRuntime{})
	if err != nil || runtime != nil {
		t.Fatalf("SensitiveRevealRuntimeFromConfig() = %v, %v", runtime, err)
	}
}

func TestSensitiveRevealRuntimeRequiresDedicatedProductionKey(t *testing.T) {
	cfg := config.Config{RuntimeEnvironment: config.RuntimeEnvironmentProduction, JWTSecret: strings.Repeat("j", 32)}
	if _, err := SensitiveRevealRuntimeFromConfig(cfg, oidcOnlySensitiveRevealManifestsForTest(), PhoneVerificationRuntime{}); err == nil || !strings.Contains(err.Error(), "PLATFORM_SENSITIVE_REVEAL_HMAC_KEY is required") {
		t.Fatalf("SensitiveRevealRuntimeFromConfig() error = %v", err)
	}
}

func TestSensitiveRevealRuntimeRequiresProductionSMSDependencies(t *testing.T) {
	cfg := config.Config{
		RuntimeEnvironment:                  config.RuntimeEnvironmentProduction,
		SensitiveRevealHMACKey:              strings.Repeat("r", 32),
		AdminStepUpPhoneResource:            "users",
		AdminStepUpPhoneActorField:          "username",
		AdminStepUpPhoneField:               "phone",
		AdminStepUpPhoneVerifiedAtField:     "phoneVerifiedAt",
		AdminStepUpPhoneVerifiedDigestField: "verifiedPhoneDigest",
	}
	if _, err := SensitiveRevealRuntimeFromConfig(cfg, sensitiveRevealManifestsForTest(), PhoneVerificationRuntime{}); err == nil || !strings.Contains(err.Error(), "registered phone verification sender") {
		t.Fatalf("SensitiveRevealRuntimeFromConfig() missing sender error = %v", err)
	}
	cfg.AdminStepUpPhoneResource = ""
	if _, err := SensitiveRevealRuntimeFromConfig(cfg, sensitiveRevealManifestsForTest(), PhoneVerificationRuntime{}); err == nil || !strings.Contains(err.Error(), "configured admin step-up phone source") {
		t.Fatalf("SensitiveRevealRuntimeFromConfig() missing source error = %v", err)
	}
	cfg.AdminStepUpPhoneResource = "users"
	phoneRuntime := PhoneVerificationRuntime{
		Protector: httpapi.NewHMACPhoneProtector([]byte(strings.Repeat("p", 32)), []byte(strings.Repeat("c", 32))),
		Sender:    phoneVerificationSenderStub{kind: "sms-vendor"},
	}
	if runtime, err := SensitiveRevealRuntimeFromConfig(cfg, sensitiveRevealManifestsForTest(), phoneRuntime); err != nil || runtime == nil {
		t.Fatalf("SensitiveRevealRuntimeFromConfig() valid production SMS runtime = %v, %v", runtime, err)
	}
}

func sensitiveRevealManifestsForTest() []capability.Manifest {
	return []capability.Manifest{{ID: "contacts", Admin: capability.AdminSurface{RevealPolicies: []capability.AdminRevealPolicy{{
		ID: "admin-sensitive-any-v1", Mode: capability.AdminRevealModeAnyOf,
		Factors:             []string{capability.AdminRevealFactorOIDCReauthentication, capability.AdminRevealFactorSMSOTP},
		Purposes:            []capability.AdminRevealPurpose{{Code: "support-case", Label: capability.Text("客户支持", "Support Case")}},
		ChallengeTTLSeconds: 300, GrantTTLSeconds: 90,
	}}}}}
}

func oidcOnlySensitiveRevealManifestsForTest() []capability.Manifest {
	manifests := sensitiveRevealManifestsForTest()
	manifests[0].Admin.RevealPolicies[0].Factors = []string{capability.AdminRevealFactorOIDCReauthentication}
	return manifests
}
