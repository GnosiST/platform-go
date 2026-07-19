package notification

import (
	"context"
	"strings"
	"testing"
)

func TestMockLocalSMSSenderRecordsDevelopmentDeliveryWithRedactedReceipt(t *testing.T) {
	sender := NewMockLocalSMSSender()

	receipt, err := sender.SendSMS(context.Background(), SMSMessage{
		TenantCode:     "tenant-demo",
		Recipient:      "+8613800000000",
		TemplateID:     "login-template",
		TemplateParams: map[string]string{"code": "123456"},
		Purpose:        "login",
		TraceID:        "trace-1",
	})
	if err != nil {
		t.Fatalf("SendSMS() error = %v", err)
	}
	if receipt.Provider != SMSProviderMockLocal || receipt.MessageID == "" || receipt.Status != SMSDeliveryAccepted {
		t.Fatalf("receipt = %+v, want accepted mock-local receipt", receipt)
	}
	if receipt.RedactedTarget != "****0000" {
		t.Fatalf("RedactedTarget = %q, want masked last four digits", receipt.RedactedTarget)
	}
	sent := sender.Sent()
	if len(sent) != 1 || sent[0].TemplateParams["code"] != "123456" {
		t.Fatalf("sent messages = %+v", sent)
	}
	sent[0].TemplateParams["code"] = "mutated"
	if sender.Sent()[0].TemplateParams["code"] != "123456" {
		t.Fatal("Sent() exposed mutable internal template params")
	}
}

func TestMockLocalSMSSenderRejectsIncompleteMessage(t *testing.T) {
	sender := NewMockLocalSMSSender()
	if _, err := sender.SendSMS(context.Background(), SMSMessage{TemplateID: "login"}); err == nil {
		t.Fatal("SendSMS() error = nil, want recipient validation")
	}
	if _, err := sender.SendSMS(context.Background(), SMSMessage{Recipient: "+8613800000000"}); err == nil {
		t.Fatal("SendSMS() error = nil, want template validation")
	}
}

func TestDryRunSMSSenderReturnsVendorReceiptWithoutSending(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		cfg      SMSProviderConfig
	}{
		{
			name:     "aliyun",
			provider: SMSProviderAliyun,
			cfg: SMSProviderConfig{
				Provider:          SMSProviderAliyun,
				AliyunRegion:      "cn-hangzhou",
				AliyunAccessKeyID: "test-access-key",
				AliyunSecretKey:   "test-secret-key",
				SignName:          "Platform",
				DryRun:            true,
			},
		},
		{
			name:     "tencent",
			provider: SMSProviderTencent,
			cfg: SMSProviderConfig{
				Provider:         SMSProviderTencent,
				TencentRegion:    "ap-guangzhou",
				TencentSecretID:  "test-secret-id",
				TencentSecretKey: "test-secret-key",
				TencentSDKAppID:  "1400000000",
				SignName:         "Platform",
				DryRun:           true,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sender, err := NewVendorSMSSender(tt.cfg)
			if err != nil {
				t.Fatalf("NewVendorSMSSender() error = %v", err)
			}
			receipt, err := sender.SendSMS(context.Background(), SMSMessage{
				Recipient:  "+8613800000000",
				TemplateID: "login-template",
				Purpose:    "login",
				TraceID:    "trace-dry-run",
			})
			if err != nil {
				t.Fatalf("SendSMS() error = %v", err)
			}
			if receipt.Provider != tt.provider || receipt.Status != SMSDeliveryDryRunAccepted || receipt.MessageID == "" {
				t.Fatalf("receipt = %+v, want dry-run vendor receipt", receipt)
			}
			if !strings.HasPrefix(receipt.MessageID, tt.provider+"-dry-run-") {
				t.Fatalf("MessageID = %q, want provider dry-run prefix", receipt.MessageID)
			}
			if receipt.RedactedTarget != "****0000" {
				t.Fatalf("RedactedTarget = %q, want masked last four digits", receipt.RedactedTarget)
			}
		})
	}
}

func TestVendorSMSSenderRejectsMissingConfigurationWithoutSecretLeak(t *testing.T) {
	secret := "do-not-print-this-secret"
	tests := []struct {
		name string
		cfg  SMSProviderConfig
		want string
	}{
		{
			name: "aliyun access key",
			cfg: SMSProviderConfig{
				Provider:        SMSProviderAliyun,
				AliyunRegion:    "cn-hangzhou",
				AliyunSecretKey: secret,
				SignName:        "Platform",
				DryRun:          true,
			},
			want: EnvNotificationSMSAliyunAccessKeyID,
		},
		{
			name: "tencent sdk app id",
			cfg: SMSProviderConfig{
				Provider:         SMSProviderTencent,
				TencentRegion:    "ap-guangzhou",
				TencentSecretID:  "test-secret-id",
				TencentSecretKey: secret,
				SignName:         "Platform",
				DryRun:           true,
			},
			want: EnvNotificationSMSTencentSDKAppID,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewVendorSMSSender(tt.cfg)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("NewVendorSMSSender() error = %v, want %s", err, tt.want)
			}
			if strings.Contains(err.Error(), secret) {
				t.Fatalf("NewVendorSMSSender() leaked secret in error: %v", err)
			}
		})
	}
}

func TestVendorSMSSenderRejectsLiveSendUntilImplemented(t *testing.T) {
	_, err := NewVendorSMSSender(SMSProviderConfig{
		Provider:         SMSProviderTencent,
		TencentRegion:    "ap-guangzhou",
		TencentSecretID:  "test-secret-id",
		TencentSecretKey: "test-secret-key",
		TencentSDKAppID:  "1400000000",
		SignName:         "Platform",
		LiveSendEnabled:  true,
	})
	if err == nil || !strings.Contains(err.Error(), "live sending is not implemented") {
		t.Fatalf("NewVendorSMSSender() error = %v, want live-send implementation failure", err)
	}
}

func TestSMSProviderContract(t *testing.T) {
	for _, provider := range []string{SMSProviderAliyun, SMSProviderTencent, SMSProviderMockLocal} {
		if !IsSupportedSMSProvider(provider) {
			t.Fatalf("provider %q should be supported", provider)
		}
	}
	if IsSupportedSMSProvider("debug") {
		t.Fatal("debug should not be a notification SMS provider")
	}
	if CanonicalSMSProvider(" MOCK-LOCAL ") != SMSProviderMockLocal {
		t.Fatal("CanonicalSMSProvider() did not trim and lowercase")
	}
}
