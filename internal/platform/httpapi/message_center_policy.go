package httpapi

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/GnosiST/platform-go/internal/platform/adminresource"
	"github.com/GnosiST/platform-go/internal/platform/notification"
	"github.com/GnosiST/platform-go/internal/platform/ratelimit"
)

const (
	messageCenterChannels     = "notification-channels"
	messageCenterSendPolicies = "notification-send-policies"
)

type messageCenterDeliveryPolicyDecision struct {
	Allowed    bool
	Disabled   bool
	RetryAfter time.Duration
}

type messageCenterDeliveryPolicy struct {
	ChannelLimitPerMinute int
	SendLimitPerMinute    int
	DailyQuota            int
	Disabled              bool
}

type messageCenterDeliveryPolicyGate struct {
	server   *Server
	actor    string
	clientIP string
}

func (gate messageCenterDeliveryPolicyGate) AllowDelivery(ctx context.Context, input notification.DeliveryPolicyInput) (bool, error) {
	if gate.server == nil {
		return true, nil
	}
	decision, err := gate.server.messageCenterDeliveryPolicyDecision(ctx, input, gate.actor, gate.clientIP)
	if err != nil {
		return false, err
	}
	return decision.Allowed, nil
}

func (s *Server) messageCenterDeliveryPolicyDecision(ctx context.Context, input notification.DeliveryPolicyInput, actor string, clientIP string) (messageCenterDeliveryPolicyDecision, error) {
	if s == nil {
		return messageCenterDeliveryPolicyDecision{Allowed: true}, nil
	}
	policy, err := s.messageCenterDeliveryPolicy(ctx, input)
	if err != nil {
		return messageCenterDeliveryPolicyDecision{}, err
	}
	if policy.Disabled {
		return messageCenterDeliveryPolicyDecision{Disabled: true}, nil
	}
	for _, check := range []struct {
		scope  string
		limit  int
		window time.Duration
	}{
		{scope: "channel-minute", limit: policy.ChannelLimitPerMinute, window: time.Minute},
		{scope: "send-policy-minute", limit: policy.SendLimitPerMinute, window: time.Minute},
		{scope: "channel-daily", limit: policy.DailyQuota, window: 24 * time.Hour},
	} {
		if check.limit <= 0 {
			continue
		}
		if s.rateLimiter == nil || s.rateLimitKeyBuilder == nil {
			return messageCenterDeliveryPolicyDecision{}, errRateLimitUnavailable()
		}
		key, err := s.rateLimitKeyBuilder.Build(
			ratelimit.OperationMessageCenterDelivery,
			valueWithDefault(clientIP, "unknown-client"),
			valueWithDefault(actor, "unknown-actor"),
			check.scope,
			valueWithDefault(input.TenantCode, platformTenant),
			valueWithDefault(notification.CanonicalChannel(input.Channel), "unknown-channel"),
			valueWithDefault(canonicalMessageCenterProvider(input.Channel, input.Provider), "unknown-provider"),
			valueWithDefault(messageCenterFirstNonBlank(input.TemplateCode, input.TemplateID), "unknown-template"),
			valueWithDefault(input.Purpose, "notification"),
		)
		if err != nil {
			return messageCenterDeliveryPolicyDecision{}, err
		}
		decision, err := s.rateLimiter.Allow(ctx, key, check.limit, check.window)
		if err != nil {
			return messageCenterDeliveryPolicyDecision{}, err
		}
		if !decision.Allowed {
			return messageCenterDeliveryPolicyDecision{RetryAfter: decision.RetryAfter}, nil
		}
	}
	return messageCenterDeliveryPolicyDecision{Allowed: true}, nil
}

func (s *Server) messageCenterDeliveryPolicy(ctx context.Context, input notification.DeliveryPolicyInput) (messageCenterDeliveryPolicy, error) {
	policy := messageCenterDeliveryPolicy{}
	if s == nil || s.resources == nil {
		return policy, nil
	}
	channel := notification.CanonicalChannel(input.Channel)
	tenant := valueWithDefault(input.TenantCode, platformTenant)
	channels, err := s.resources.InternalRecordsContext(ctx, messageCenterChannels)
	if err != nil && !errors.Is(err, adminresource.ErrUnknownResource) {
		return messageCenterDeliveryPolicy{}, err
	}
	for _, record := range channels {
		if !messageCenterRecordMatchesTenant(record, tenant) || notification.CanonicalChannel(record.Values["channel"]) != channel {
			continue
		}
		if !messageCenterRecordEnabled(record) || !messageCenterBoolDefault(record.Values["enabled"], true) {
			policy.Disabled = true
			return policy, nil
		}
		policy.ChannelLimitPerMinute = minPositive(policy.ChannelLimitPerMinute, positiveInt(record.Values["rateLimitPerMinute"]))
		policy.DailyQuota = minPositive(policy.DailyQuota, positiveInt(record.Values["dailyQuota"]))
	}
	sendPolicies, err := s.resources.InternalRecordsContext(ctx, messageCenterSendPolicies)
	if err != nil && !errors.Is(err, adminresource.ErrUnknownResource) {
		return messageCenterDeliveryPolicy{}, err
	}
	for _, record := range sendPolicies {
		if !messageCenterRecordEnabled(record) || !messageCenterRecordMatchesTenant(record, tenant) || notification.CanonicalChannel(record.Values["channel"]) != channel {
			continue
		}
		policy.SendLimitPerMinute = minPositive(policy.SendLimitPerMinute, positiveInt(record.Values["rateLimitPerMinute"]))
	}
	return policy, nil
}

func messageCenterRecordEnabled(record adminresource.Record) bool {
	return record.DeletedAt == "" && strings.EqualFold(strings.TrimSpace(record.Status), "enabled")
}

func messageCenterRecordMatchesTenant(record adminresource.Record, tenant string) bool {
	recordTenant := strings.TrimSpace(record.Values["tenantCode"])
	return recordTenant == "" || recordTenant == platformTenant || recordTenant == strings.TrimSpace(tenant)
}

func messageCenterBoolDefault(value string, fallback bool) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "":
		return fallback
	case "1", "true", "yes", "enabled", "on":
		return true
	case "0", "false", "no", "disabled", "off":
		return false
	default:
		return fallback
	}
}

func positiveInt(value string) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed <= 0 {
		return 0
	}
	return parsed
}

func minPositive(current int, next int) int {
	if next <= 0 {
		return current
	}
	if current <= 0 || next < current {
		return next
	}
	return current
}

func messageCenterFirstNonBlank(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func canonicalMessageCenterProvider(channel string, provider string) string {
	if notification.CanonicalChannel(channel) == notification.ChannelSMS {
		return notification.CanonicalSMSProvider(provider)
	}
	return notification.CanonicalProvider(provider)
}

func errRateLimitUnavailable() error {
	return errors.New("message center delivery rate limiter is unavailable")
}
