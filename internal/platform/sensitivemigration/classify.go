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

func classifyValue(ctx context.Context, runtime dataprotection.Runtime, value string, policy dataprotection.FieldPolicy, fieldContext dataprotection.FieldContext) (Classification, error) {
	if ctx.Err() != nil {
		return "", ErrReadFailed
	}
	if value == "" {
		return ClassificationMissing, nil
	}
	switch dataprotection.ClassifyEnvelopeShape(value) {
	case dataprotection.EnvelopeShapeNone:
		return ClassificationPlaintext, nil
	case dataprotection.EnvelopeShapeForeign:
		return ClassificationForeignEnvelope, nil
	case dataprotection.EnvelopeShapeMalformed:
		return ClassificationMalformedEnvelope, nil
	}
	if !dataprotection.IsEnvelope(value) {
		return ClassificationMalformedEnvelope, nil
	}
	if err := runtime.Validate(ctx, value, policy, fieldContext); err != nil {
		if ctx.Err() != nil || errors.Is(err, dataprotection.ErrKeyUnavailable) {
			return "", ErrReadFailed
		}
		if errors.Is(err, dataprotection.ErrPolicyMismatch) || errors.Is(err, dataprotection.ErrKeyMismatch) {
			return ClassificationForeignEnvelope, nil
		}
		if errors.Is(err, dataprotection.ErrInvalidEnvelope) {
			return ClassificationMalformedEnvelope, nil
		}
		return "", ErrReadFailed
	}
	if ctx.Err() != nil {
		return "", ErrReadFailed
	}
	return ClassificationTargetEnvelope, nil
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
