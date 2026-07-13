package masking

import (
	"context"
	"errors"
	"testing"
)

func TestRuntimeMasksVersionedStrategies(t *testing.T) {
	runtime := NewRuntime()
	tests := []struct {
		name   string
		policy Policy
		value  string
		want   string
	}{
		{name: "custom partial", policy: Policy{Strategy: StrategyPartialV1, PreservePrefix: 2, PreserveSuffix: 2, MaskLength: 6}, value: "custom-reference", want: "cu******ce"},
		{name: "phone", policy: Policy{Strategy: StrategyPhoneV1}, value: "13800138000", want: "138****8000"},
		{name: "email", policy: Policy{Strategy: StrategyEmailV1}, value: "name@example.com", want: "na***@example.com"},
		{name: "identity", policy: Policy{Strategy: StrategyIdentityCNV1}, value: "170101199001011204", want: "17******04"},
		{name: "address", policy: Policy{Strategy: StrategyAddressCNV1}, value: "北京市朝阳区建国路88号", want: "北京市朝阳区******"},
		{name: "unicode replacement", policy: Policy{Strategy: StrategyPartialV1, PreservePrefix: 1, PreserveSuffix: 1, MaskLength: 3, Replacement: "·"}, value: "甲乙丙丁", want: "甲···丁"},
		{name: "short value still concealed", policy: Policy{Strategy: StrategyPhoneV1}, value: "12", want: "1****"},
		{name: "single rune concealed", policy: Policy{Strategy: StrategyPhoneV1}, value: "1", want: "****"},
		{name: "empty stays empty", policy: Policy{Strategy: StrategyPhoneV1}, value: "", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := runtime.Mask(context.Background(), tt.policy, tt.value)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("Mask() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRuntimeRejectsInvalidPoliciesAndCanceledContext(t *testing.T) {
	runtime := NewRuntime()
	tests := []Policy{
		{Strategy: "unknown-v1"},
		{Strategy: StrategyPartialV1, PreservePrefix: 1, PreserveSuffix: 1},
		{Strategy: StrategyPartialV1, PreservePrefix: -1, MaskLength: 2},
		{Strategy: StrategyPartialV1, MaskLength: 65},
		{Strategy: StrategyPhoneV1, Replacement: "**"},
		{Strategy: StrategyPhoneV1, Replacement: " * "},
		{Strategy: StrategyPhoneV1, Replacement: " "},
		{Strategy: StrategyPhoneV1, Replacement: "\n"},
		{Strategy: StrategyPhoneV1, Replacement: "\u200b"},
		{Strategy: StrategyPhoneV1, Replacement: "\u0301"},
		{Strategy: StrategyEmailV1, PreservePrefix: 1},
	}
	for _, policy := range tests {
		if err := runtime.Validate(policy); !errors.Is(err, ErrInvalidPolicy) {
			t.Fatalf("Validate(%+v) error = %v, want ErrInvalidPolicy", policy, err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := runtime.Mask(ctx, Policy{Strategy: StrategyPhoneV1}, "13800138000"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Mask(canceled) error = %v, want context.Canceled", err)
	}
}
