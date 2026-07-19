package notification

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	aliyunsms "github.com/alibabacloud-go/dysmsapi-20170525/v5/client"
	tencentsms "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/sms/v20210111"
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

func TestVendorSMSSenderCreatesLiveSDKSenderWhenEnabled(t *testing.T) {
	sender, err := NewVendorSMSSender(SMSProviderConfig{
		Provider:         SMSProviderTencent,
		TencentRegion:    "ap-guangzhou",
		TencentSecretID:  "test-secret-id",
		TencentSecretKey: "test-secret-key",
		TencentSDKAppID:  "1400000000",
		SignName:         "Platform",
		LiveSendEnabled:  true,
	})
	if err != nil {
		t.Fatalf("NewVendorSMSSender() error = %v", err)
	}
	if sender.Kind() != SMSProviderTencent {
		t.Fatalf("sender.Kind() = %q, want tencent", sender.Kind())
	}
}

func TestAliyunSMSSenderLiveSendBuildsSDKRequestAndReceipt(t *testing.T) {
	fake := &fakeAliyunSMSClient{response: &aliyunsms.SendSmsResponse{Body: &aliyunsms.SendSmsResponseBody{
		Code:      testStringPtr("OK"),
		BizId:     testStringPtr("aliyun-biz-1"),
		RequestId: testStringPtr("aliyun-request-1"),
	}}}
	sender, err := newAliyunSMSSenderWithClient(SMSProviderConfig{
		Provider:          SMSProviderAliyun,
		AliyunRegion:      "cn-hangzhou",
		AliyunAccessKeyID: "test-access-key",
		AliyunSecretKey:   "test-secret-key",
		SignName:          "Platform",
		LiveSendEnabled:   true,
	}, fake)
	if err != nil {
		t.Fatalf("newAliyunSMSSenderWithClient() error = %v", err)
	}

	receipt, err := sender.SendSMS(context.Background(), SMSMessage{
		Recipient:  "+8613800000000",
		TemplateID: "SMS_123456",
		TemplateParams: map[string]string{
			SMSProviderTemplateParamOrderKey: "code",
			"code":                           "123456",
		},
		TraceID: "trace-aliyun",
	})
	if err != nil {
		t.Fatalf("SendSMS() error = %v", err)
	}
	if receipt.Provider != SMSProviderAliyun || receipt.MessageID != "aliyun-biz-1" || receipt.Status != SMSDeliveryAccepted || receipt.RedactedTarget != "****0000" {
		t.Fatalf("receipt = %+v, want accepted aliyun receipt", receipt)
	}
	if fake.request == nil ||
		stringPtrValue(fake.request.PhoneNumbers) != "+8613800000000" ||
		stringPtrValue(fake.request.SignName) != "Platform" ||
		stringPtrValue(fake.request.TemplateCode) != "SMS_123456" ||
		stringPtrValue(fake.request.OutId) != "trace-aliyun" {
		t.Fatalf("aliyun request = %+v, want hydrated SDK request", fake.request)
	}
	var params map[string]string
	if err := json.Unmarshal([]byte(stringPtrValue(fake.request.TemplateParam)), &params); err != nil {
		t.Fatalf("TemplateParam JSON error = %v", err)
	}
	if params["code"] != "123456" || params[SMSProviderTemplateParamOrderKey] != "" {
		t.Fatalf("TemplateParam = %+v, want template params without order metadata", params)
	}
}

func TestTencentSMSSenderLiveSendBuildsSDKRequestAndReceipt(t *testing.T) {
	fake := &fakeTencentSMSClient{response: &tencentsms.SendSmsResponse{Response: &tencentsms.SendSmsResponseParams{
		RequestId: testStringPtr("tencent-request-1"),
		SendStatusSet: []*tencentsms.SendStatus{{
			Code:     testStringPtr("Ok"),
			SerialNo: testStringPtr("tencent-serial-1"),
		}},
	}}}
	sender, err := newTencentSMSSenderWithClient(SMSProviderConfig{
		Provider:         SMSProviderTencent,
		TencentRegion:    "ap-guangzhou",
		TencentSecretID:  "test-secret-id",
		TencentSecretKey: "test-secret-key",
		TencentSDKAppID:  "1400000000",
		SignName:         "Platform",
		LiveSendEnabled:  true,
	}, fake)
	if err != nil {
		t.Fatalf("newTencentSMSSenderWithClient() error = %v", err)
	}

	receipt, err := sender.SendSMS(context.Background(), SMSMessage{
		Recipient:  "+8613800000000",
		TemplateID: "123456",
		TemplateParams: map[string]string{
			SMSProviderTemplateParamOrderKey: "code,name",
			"code":                           "123456",
			"name":                           "Kai",
		},
		TraceID: "trace-tencent",
	})
	if err != nil {
		t.Fatalf("SendSMS() error = %v", err)
	}
	if receipt.Provider != SMSProviderTencent || receipt.MessageID != "tencent-serial-1" || receipt.Status != SMSDeliveryAccepted || receipt.RedactedTarget != "****0000" {
		t.Fatalf("receipt = %+v, want accepted tencent receipt", receipt)
	}
	if fake.request == nil ||
		len(fake.request.PhoneNumberSet) != 1 ||
		stringPtrValue(fake.request.PhoneNumberSet[0]) != "+8613800000000" ||
		stringPtrValue(fake.request.SmsSdkAppId) != "1400000000" ||
		stringPtrValue(fake.request.SignName) != "Platform" ||
		stringPtrValue(fake.request.TemplateId) != "123456" ||
		stringPtrValue(fake.request.SessionContext) != "trace-tencent" {
		t.Fatalf("tencent request = %+v, want hydrated SDK request", fake.request)
	}
	if len(fake.request.TemplateParamSet) != 2 ||
		stringPtrValue(fake.request.TemplateParamSet[0]) != "123456" ||
		stringPtrValue(fake.request.TemplateParamSet[1]) != "Kai" {
		t.Fatalf("TemplateParamSet = %+v, want declared order", fake.request.TemplateParamSet)
	}
}

func TestVendorSMSSenderNormalizesProviderFailureWithoutLeakingRawError(t *testing.T) {
	secret := "do-not-print-this-secret"
	fake := &fakeTencentSMSClient{err: errors.New("provider failed for +8613800000000 with " + secret)}
	sender, err := newTencentSMSSenderWithClient(SMSProviderConfig{
		Provider:         SMSProviderTencent,
		TencentRegion:    "ap-guangzhou",
		TencentSecretID:  "test-secret-id",
		TencentSecretKey: secret,
		TencentSDKAppID:  "1400000000",
		SignName:         "Platform",
		LiveSendEnabled:  true,
	}, fake)
	if err != nil {
		t.Fatalf("newTencentSMSSenderWithClient() error = %v", err)
	}

	receipt, err := sender.SendSMS(context.Background(), SMSMessage{Recipient: "+8613800000000", TemplateID: "123456"})
	if err == nil || !strings.Contains(err.Error(), "request failed") {
		t.Fatalf("SendSMS() error = %v, want normalized provider failure", err)
	}
	if strings.Contains(err.Error(), "+8613800000000") || strings.Contains(err.Error(), secret) {
		t.Fatalf("SendSMS() leaked raw provider error: %v", err)
	}
	if receipt.Provider != SMSProviderTencent || receipt.RedactedTarget != "****0000" || receipt.Status != "failed" {
		t.Fatalf("receipt = %+v, want failed redacted receipt", receipt)
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

type fakeAliyunSMSClient struct {
	request  *aliyunsms.SendSmsRequest
	response *aliyunsms.SendSmsResponse
	err      error
}

func (c *fakeAliyunSMSClient) SendSms(request *aliyunsms.SendSmsRequest) (*aliyunsms.SendSmsResponse, error) {
	c.request = request
	return c.response, c.err
}

type fakeTencentSMSClient struct {
	request  *tencentsms.SendSmsRequest
	response *tencentsms.SendSmsResponse
	err      error
}

func (c *fakeTencentSMSClient) SendSms(request *tencentsms.SendSmsRequest) (*tencentsms.SendSmsResponse, error) {
	c.request = request
	return c.response, c.err
}

func testStringPtr(value string) *string {
	return &value
}
