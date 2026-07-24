package outbox

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"
)

const EnvelopeVersion = "1.0"

var (
	ErrInvalidEnvelope = errors.New("invalid outbox event envelope")

	eventIdentifierPattern = regexp.MustCompile(`^[a-z][a-z0-9.-]{0,127}$`)
	tenantCodePattern      = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)
	traceParentPattern     = regexp.MustCompile(`^00-[0-9a-f]{32}-[0-9a-f]{16}-[0-9a-f]{2}$`)
	sensitivePayloadKeys   = map[string]struct{}{
		"password": {}, "secret": {}, "token": {}, "access_token": {}, "refresh_token": {},
		"otp": {}, "smsotp": {}, "verificationcode": {}, "phone": {}, "email": {}, "recipient": {},
	}
)

// Envelope is the only payload shape persisted and published by the outbox.
type Envelope struct {
	EventID     string          `json:"eventId"`
	EventType   string          `json:"eventType"`
	Version     string          `json:"eventVersion"`
	OccurredAt  time.Time       `json:"occurredAt"`
	Producer    string          `json:"producer"`
	TenantCode  string          `json:"tenantContext"`
	TraceParent string          `json:"traceContext"`
	TraceState  string          `json:"traceState,omitempty"`
	Payload     json.RawMessage `json:"payload"`
}

func NewEventID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return "evt-" + hex.EncodeToString(value), nil
}

func ValidateEnvelope(envelope Envelope) (Envelope, error) {
	envelope.EventID = strings.TrimSpace(envelope.EventID)
	envelope.EventType = strings.TrimSpace(envelope.EventType)
	envelope.Version = strings.TrimSpace(envelope.Version)
	envelope.Producer = strings.TrimSpace(envelope.Producer)
	envelope.TenantCode = strings.TrimSpace(envelope.TenantCode)
	envelope.TraceParent = strings.TrimSpace(envelope.TraceParent)
	envelope.TraceState = strings.TrimSpace(envelope.TraceState)
	if !eventIdentifierPattern.MatchString(envelope.EventID) || !eventIdentifierPattern.MatchString(envelope.EventType) ||
		envelope.Version != EnvelopeVersion || !eventIdentifierPattern.MatchString(envelope.Producer) || !tenantCodePattern.MatchString(envelope.TenantCode) ||
		!traceParentPattern.MatchString(envelope.TraceParent) || len(envelope.TraceState) > 512 || envelope.OccurredAt.IsZero() || len(envelope.Payload) == 0 || len(envelope.Payload) > 1<<20 || !json.Valid(envelope.Payload) {
		return Envelope{}, ErrInvalidEnvelope
	}
	var payload any
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil || containsSensitivePayloadKey(payload) {
		return Envelope{}, ErrInvalidEnvelope
	}
	envelope.OccurredAt = envelope.OccurredAt.UTC()
	envelope.Payload = append(json.RawMessage(nil), envelope.Payload...)
	return envelope, nil
}

func containsSensitivePayloadKey(value any) bool {
	switch current := value.(type) {
	case map[string]any:
		for key, nested := range current {
			if _, found := sensitivePayloadKeys[strings.ToLower(strings.TrimSpace(key))]; found || containsSensitivePayloadKey(nested) {
				return true
			}
		}
	case []any:
		for _, nested := range current {
			if containsSensitivePayloadKey(nested) {
				return true
			}
		}
	}
	return false
}
