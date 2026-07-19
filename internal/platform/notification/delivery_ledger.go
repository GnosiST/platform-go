package notification

import (
	"strconv"
	"strings"
	"time"

	"github.com/GnosiST/platform-go/internal/platform/kernel"
)

const (
	DeliveryLedgerErrorFailed                = "notification delivery failed"
	DeliveryLedgerErrorSenderUnavailable     = "notification delivery sender unavailable"
	DeliveryLedgerErrorMessageCenterTestSend = "message center test send failed"
)

type DeliveryLedgerInput struct {
	BaseValues     map[string]string
	Channel        string
	Target         string
	Receipt        DeliveryReceipt
	DeliveryStatus string
	ErrorMessage   string
	AttemptedAt    time.Time
	DeliveredAt    time.Time
	RequestID      string
	TraceID        string
}

func BuildDeliveryLedgerValues(input DeliveryLedgerInput) map[string]string {
	values := cloneDeliveryLedgerValues(input.BaseValues)
	deleteDeliveryLedgerSensitiveValues(values)

	channel := CanonicalChannel(firstNonBlank(input.Channel, input.Receipt.Channel, values["channel"]))
	if channel != "" {
		values["channel"] = channel
	}
	deliveryStatus := strings.TrimSpace(input.DeliveryStatus)
	if deliveryStatus == "" {
		deliveryStatus = DeliveryStatusDelivered
	}
	values["deliveryStatus"] = deliveryStatus
	values["attempts"] = strconv.Itoa(deliveryLedgerAttempts(values) + 1)

	if attemptedAt := deliveryLedgerTimestamp(input.AttemptedAt); attemptedAt != "" {
		values["lastAttemptAt"] = attemptedAt
	}
	if deliveryStatus == DeliveryStatusDelivered {
		deliveredAt := input.DeliveredAt
		if deliveredAt.IsZero() {
			deliveredAt = input.AttemptedAt
		}
		if formatted := deliveryLedgerTimestamp(deliveredAt); formatted != "" {
			values["deliveredAt"] = formatted
		}
		delete(values, "errorMessage")
	} else {
		delete(values, "deliveredAt")
		values["errorMessage"] = safeDeliveryLedgerErrorMessage(input.ErrorMessage)
	}

	target := strings.TrimSpace(input.Receipt.RedactedTarget)
	if target == "" {
		rawTarget := firstNonBlank(input.Target, values["target"])
		if rawTarget != "" {
			target = RedactMessageTarget(channel, rawTarget)
		}
	}
	if target != "" {
		values["target"] = target
	}

	provider := firstNonBlank(input.Receipt.Provider, values["provider"])
	if channel == ChannelSMS {
		provider = CanonicalSMSProvider(provider)
	} else {
		provider = CanonicalProvider(provider)
	}
	if provider != "" {
		values["provider"] = provider
	}

	messageID := strings.TrimSpace(input.Receipt.MessageID)
	if messageID != "" {
		values["providerMessageId"] = messageID
	} else {
		delete(values, "providerMessageId")
	}
	providerStatus := firstNonBlank(input.Receipt.Status, deliveryStatus)
	if providerStatus != "" {
		values["providerStatus"] = providerStatus
	}

	applyDeliveryLedgerCorrelation(values, input.RequestID, input.TraceID)
	return values
}

func safeDeliveryLedgerErrorMessage(message string) string {
	switch strings.TrimSpace(message) {
	case DeliveryLedgerErrorSenderUnavailable:
		return DeliveryLedgerErrorSenderUnavailable
	case DeliveryLedgerErrorMessageCenterTestSend:
		return DeliveryLedgerErrorMessageCenterTestSend
	default:
		return DeliveryLedgerErrorFailed
	}
}

func applyDeliveryLedgerCorrelation(values map[string]string, requestID string, traceID string) {
	correlation := kernel.Correlation{RequestID: strings.TrimSpace(requestID), TraceID: strings.TrimSpace(traceID)}
	if !kernel.ValidCorrelation(correlation) {
		correlation = kernel.Correlation{
			RequestID: strings.TrimSpace(values["requestId"]),
			TraceID:   strings.TrimSpace(values["traceId"]),
		}
	}
	if kernel.ValidCorrelation(correlation) {
		values["requestId"] = correlation.RequestID
		values["traceId"] = correlation.TraceID
		return
	}
	delete(values, "requestId")
	delete(values, "traceId")
}

func deleteDeliveryLedgerSensitiveValues(values map[string]string) {
	for _, key := range []string{"payload", "templateParams", "templateParam", "recipient", "phone", "otp", "smsOtp", "verificationCode"} {
		delete(values, key)
	}
}

func deliveryLedgerTimestamp(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func deliveryLedgerAttempts(values map[string]string) int {
	attempts, err := strconv.Atoi(strings.TrimSpace(values["attempts"]))
	if err != nil || attempts < 0 {
		return 0
	}
	return attempts
}

func cloneDeliveryLedgerValues(values map[string]string) map[string]string {
	cloned := make(map[string]string, len(values)+8)
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}
