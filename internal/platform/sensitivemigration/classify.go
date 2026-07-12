package sensitivemigration

import (
	"context"
	"errors"

	"platform-go/internal/platform/dataprotection"
)

type Classification string

const (
	ClassificationMissing           Classification = "missing"
	ClassificationPlaintext         Classification = "plaintext"
	ClassificationTargetEnvelope    Classification = "target-envelope"
	ClassificationForeignEnvelope   Classification = "foreign-envelope"
	ClassificationMalformedEnvelope Classification = "malformed-envelope"
)

func classifyValue(ctx context.Context, runtime dataprotection.Runtime, value string, policy dataprotection.FieldPolicy, fieldContext dataprotection.FieldContext) Classification {
	if value == "" {
		return ClassificationMissing
	}
	switch dataprotection.ClassifyEnvelopeShape(value) {
	case dataprotection.EnvelopeShapeNone:
		return ClassificationPlaintext
	case dataprotection.EnvelopeShapeForeign:
		return ClassificationForeignEnvelope
	case dataprotection.EnvelopeShapeMalformed:
		return ClassificationMalformedEnvelope
	}
	if !dataprotection.IsEnvelope(value) {
		return ClassificationMalformedEnvelope
	}
	if err := runtime.Validate(ctx, value, policy, fieldContext); err != nil {
		if errors.Is(err, dataprotection.ErrPolicyMismatch) || errors.Is(err, dataprotection.ErrKeyUnavailable) || errors.Is(err, dataprotection.ErrKeyMismatch) {
			return ClassificationForeignEnvelope
		}
		return ClassificationMalformedEnvelope
	}
	return ClassificationTargetEnvelope
}

func (c *Counts) add(classification Classification) {
	switch classification {
	case ClassificationMissing:
		c.Missing++
	case ClassificationPlaintext:
		c.Plaintext++
	case ClassificationTargetEnvelope:
		c.TargetEnvelope++
	case ClassificationForeignEnvelope:
		c.ForeignEnvelope++
	case ClassificationMalformedEnvelope:
		c.MalformedEnvelope++
	}
}
