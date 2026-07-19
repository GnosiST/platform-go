package notification

import (
	"context"
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
