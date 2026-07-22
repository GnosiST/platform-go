package notification

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestBuildDeliveryLedgerValuesRedactsSMSAndKeepsCorrelation(t *testing.T) {
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	values := BuildDeliveryLedgerValues(DeliveryLedgerInput{
		BaseValues: map[string]string{
			"tenantCode":        "platform",
			"channel":           ChannelSMS,
			"target":            "+8613800000000",
			"attempts":          "0",
			"templateParams":    `{"code":"123456"}`,
			"providerMessageId": "stale-message",
			"errorMessage":      "raw provider failed for +8613800000000 with 123456",
		},
		Channel: ChannelSMS,
		Target:  "+8613800000000",
		Receipt: DeliveryReceipt{
			Channel:        ChannelSMS,
			Provider:       SMSProviderTencent,
			MessageID:      "tencent-message-1",
			Status:         SMSDeliveryAccepted,
			RedactedTarget: "****0000",
		},
		DeliveryStatus: DeliveryStatusDelivered,
		AttemptedAt:    now,
		RequestID:      "req_0123456789abcdef0123456789abcdef",
		TraceID:        "4bf92f3577b34da6a3ce929d0e0e4736",
	})

	if values["target"] != "****0000" ||
		values["provider"] != SMSProviderTencent ||
		values["providerMessageId"] != "tencent-message-1" ||
		values["providerStatus"] != SMSDeliveryAccepted ||
		values["deliveryStatus"] != DeliveryStatusDelivered ||
		values["requestId"] != "req_0123456789abcdef0123456789abcdef" ||
		values["traceId"] != "4bf92f3577b34da6a3ce929d0e0e4736" ||
		values["attempts"] != "1" ||
		values["lastAttemptAt"] != now.Format(time.RFC3339) ||
		values["deliveredAt"] != now.Format(time.RFC3339) ||
		values["errorMessage"] != "" {
		t.Fatalf("ledger values = %+v, want redacted delivered SMS ledger with correlation", values)
	}
	assertDeliveryLedgerValuesDoNotLeak(t, values, "+8613800000000", "123456", "raw provider failed")
}

func TestBuildDeliveryLedgerValuesNormalizesFailedProviderError(t *testing.T) {
	now := time.Date(2026, 7, 19, 12, 5, 0, 0, time.UTC)
	values := BuildDeliveryLedgerValues(DeliveryLedgerInput{
		BaseValues: map[string]string{
			"channel":  ChannelSMS,
			"target":   "+8613800000000",
			"attempts": "2",
		},
		Channel:        ChannelSMS,
		Target:         "+8613800000000",
		Receipt:        DeliveryReceipt{Provider: SMSProviderAliyun, Status: "failed"},
		DeliveryStatus: DeliveryStatusFailed,
		ErrorMessage:   "aliyun raw error: code 123456 for +8613800000000",
		AttemptedAt:    now,
		RetryBackoff:   2 * time.Minute,
	})

	if values["deliveryStatus"] != DeliveryStatusFailed ||
		values["provider"] != SMSProviderAliyun ||
		values["providerStatus"] != "failed" ||
		values["target"] != "****0000" ||
		values["attempts"] != "3" ||
		values["providerMessageId"] != "" ||
		values["errorMessage"] != DeliveryLedgerErrorFailed ||
		values["retryBackoffSeconds"] != "120" ||
		values["nextRetryAt"] != now.Add(2*time.Minute).Format(time.RFC3339) ||
		values["deliveredAt"] != "" {
		t.Fatalf("ledger values = %+v, want safe failed SMS ledger", values)
	}
	assertDeliveryLedgerValuesDoNotLeak(t, values, "+8613800000000", "123456", "aliyun raw error")
}

func assertDeliveryLedgerValuesDoNotLeak(t *testing.T, values map[string]string, markers ...string) {
	t.Helper()
	checked := cloneDeliveryLedgerValues(values)
	delete(checked, "requestId")
	delete(checked, "traceId")
	encoded, err := json.Marshal(checked)
	if err != nil {
		t.Fatalf("marshal ledger values: %v", err)
	}
	body := string(encoded)
	for _, marker := range markers {
		if strings.Contains(body, marker) {
			t.Fatalf("ledger values leaked %q: %s", marker, body)
		}
	}
}
