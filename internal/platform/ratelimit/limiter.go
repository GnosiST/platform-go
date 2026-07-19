package ratelimit

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

type Limiter interface {
	Allow(context.Context, string, int, time.Duration) (Decision, error)
}

type Decision struct {
	Allowed    bool
	RetryAfter time.Duration
}

type Operation string

const (
	OperationAdminLogin                Operation = "admin-login"
	OperationAppLogin                  Operation = "app-login"
	OperationCredentialChallenge       Operation = "credential-challenge"
	OperationAdminOIDCStart            Operation = "admin-oidc-start"
	OperationPhoneVerificationRequest  Operation = "phone-verification-request"
	OperationPhoneBindingVerification  Operation = "phone-binding-verification"
	OperationAdminUpload               Operation = "admin-upload"
	OperationAppUpload                 Operation = "app-upload"
	OperationAdminServiceObjectQuery   Operation = "admin-service-object-query"
	OperationAdminServiceObjectCommand Operation = "admin-service-object-command"
	OperationAdminProfilePassword      Operation = "admin-profile-password"
	OperationMessageCenterDelivery     Operation = "message-center-delivery"
	OperationSensitiveRevealChallenge  Operation = "sensitive-reveal-challenge"
	OperationSensitiveRevealFactor     Operation = "sensitive-reveal-factor"
	OperationSensitiveRevealConsume    Operation = "sensitive-reveal-consume"
)

type Policy struct {
	Limit  int
	Window time.Duration
}

var policies = map[Operation]Policy{
	OperationAdminLogin:                {Limit: 10, Window: 5 * time.Minute},
	OperationAppLogin:                  {Limit: 10, Window: 5 * time.Minute},
	OperationCredentialChallenge:       {Limit: 20, Window: 5 * time.Minute},
	OperationAdminOIDCStart:            {Limit: 20, Window: 5 * time.Minute},
	OperationPhoneVerificationRequest:  {Limit: 5, Window: 10 * time.Minute},
	OperationPhoneBindingVerification:  {Limit: 10, Window: 10 * time.Minute},
	OperationAdminUpload:               {Limit: 30, Window: time.Minute},
	OperationAppUpload:                 {Limit: 30, Window: time.Minute},
	OperationAdminServiceObjectQuery:   {Limit: 120, Window: time.Minute},
	OperationAdminServiceObjectCommand: {Limit: 60, Window: time.Minute},
	OperationAdminProfilePassword:      {Limit: 10, Window: time.Minute},
	OperationMessageCenterDelivery:     {Limit: 10, Window: time.Minute},
	OperationSensitiveRevealChallenge:  {Limit: 10, Window: 10 * time.Minute},
	OperationSensitiveRevealFactor:     {Limit: 15, Window: 10 * time.Minute},
	OperationSensitiveRevealConsume:    {Limit: 10, Window: 5 * time.Minute},
}

func PolicyFor(operation Operation) (Policy, bool) {
	policy, ok := policies[operation]
	return policy, ok
}

type KeyBuilder struct {
	key []byte
}

func NewKeyBuilder(key []byte) (*KeyBuilder, error) {
	if len(key) == 0 {
		return nil, errors.New("rate limit HMAC key is required")
	}
	return &KeyBuilder{key: append([]byte(nil), key...)}, nil
}

func (b *KeyBuilder) Build(operation Operation, dimensions ...string) (string, error) {
	if b == nil || len(b.key) == 0 {
		return "", errors.New("rate limit HMAC key is required")
	}
	if _, ok := PolicyFor(operation); !ok {
		return "", errors.New("rate limit operation is invalid")
	}
	if len(dimensions) == 0 {
		return "", errors.New("rate limit dimensions are required")
	}
	mac := hmac.New(sha256.New, b.key)
	_, _ = mac.Write([]byte("platform-rate-limit-v1\x00"))
	_, _ = mac.Write([]byte(operation))
	for _, dimension := range dimensions {
		normalized := strings.TrimSpace(dimension)
		if normalized == "" || !utf8.ValidString(normalized) || strings.IndexFunc(normalized, unicode.IsControl) >= 0 {
			return "", errors.New("rate limit dimension is invalid")
		}
		_, _ = fmt.Fprintf(mac, "\x00%d:", len(normalized))
		_, _ = mac.Write([]byte(normalized))
	}
	return "platform:ratelimit:v1:" + string(operation) + ":" + hex.EncodeToString(mac.Sum(nil)), nil
}

func validateAllowInput(key string, limit int, window time.Duration) error {
	if strings.TrimSpace(key) == "" {
		return errors.New("rate limit key is required")
	}
	if limit <= 0 {
		return errors.New("rate limit must be positive")
	}
	if window <= 0 {
		return errors.New("rate limit window must be positive")
	}
	return nil
}
