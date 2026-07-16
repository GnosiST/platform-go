package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"platform-go/internal/platform/adminresource"
	"platform-go/internal/platform/errorcode"
	"platform-go/internal/platform/ratelimit"
)

const (
	appPhoneVerificationsResource = "app-phone-verifications"
	appPhoneBindingsResource      = "app-phone-bindings"
	appPhoneVerificationPurpose   = "bind"
	appPhoneVerificationTTL       = 10 * time.Minute
)

type appPhoneVerificationRequest struct {
	Phone   string `json:"phone"`
	Purpose string `json:"purpose"`
}

type appPhoneVerificationResponse struct {
	ID          string    `json:"id"`
	MaskedPhone string    `json:"maskedPhone"`
	Purpose     string    `json:"purpose"`
	ExpiresAt   time.Time `json:"expiresAt"`
	DebugCode   string    `json:"debugCode,omitempty"`
}

type appPhoneBindingRequest struct {
	Phone string `json:"phone"`
	Code  string `json:"code"`
}

type appPhoneBindingResponse struct {
	ID          string    `json:"id"`
	AppUsername string    `json:"appUsername"`
	MaskedPhone string    `json:"maskedPhone"`
	BoundAt     time.Time `json:"boundAt"`
}

func (s *Server) appPhoneCreateVerification(ctx *gin.Context) {
	appSession, ok := AppSessionFromContext(ctx)
	if !ok {
		writePlatformError(ctx, errorcode.CodeAuthUnauthorized)
		return
	}
	var input appPhoneVerificationRequest
	if err := ctx.ShouldBindJSON(&input); err != nil {
		writePlatformError(ctx, errorcode.CodeAppPhoneInvalidRequest)
		return
	}
	phone, ok := normalizeAppPhone(input.Phone)
	if !ok {
		writePlatformError(ctx, errorcode.CodeAppPhoneInvalidPhone)
		return
	}
	purpose := strings.TrimSpace(input.Purpose)
	if purpose == "" {
		purpose = appPhoneVerificationPurpose
	}
	if purpose != appPhoneVerificationPurpose {
		writePlatformError(ctx, errorcode.CodeAppPhonePurposeUnsupported)
		return
	}
	username := appUsername(appSession.Username)
	if !s.enforceRateLimit(ctx, ratelimit.OperationPhoneVerificationRequest, rateLimitClientIP(ctx), username, phone, purpose) {
		return
	}
	if s.phoneProtector == nil || s.phoneVerificationSender == nil {
		writePlatformErrorWithCause(ctx, s.internalErrorSink, errorcode.CodeAppPhoneVerificationUnavailable, errors.New("app phone verification dependency is not configured"))
		return
	}
	phoneDigest, err := s.phoneProtector.PhoneDigest(phone)
	if err != nil {
		writePlatformErrorWithCause(ctx, s.internalErrorSink, errorcode.CodeAppPhoneVerificationUnavailable, err)
		return
	}
	now := s.now().UTC()
	expiresAt := now.Add(appPhoneVerificationTTL)
	maskedPhone := maskAppPhone(phone)
	verificationCode, err := s.appPhoneCodeGenerator()
	if err != nil {
		writePlatformErrorWithCause(ctx, s.internalErrorSink, errorcode.CodeAppPhoneCodeGenerationFailed, err)
		return
	}
	codeDigest, err := s.phoneProtector.CodeDigest(phoneDigest, purpose, verificationCode)
	if err != nil {
		writePlatformErrorWithCause(ctx, s.internalErrorSink, errorcode.CodeAppPhoneVerificationUnavailable, err)
		return
	}
	if err := s.phoneVerificationSender.Send(ctx.Request.Context(), phone, purpose, verificationCode); err != nil {
		writePlatformErrorWithCause(ctx, s.internalErrorSink, errorcode.CodeAppPhoneVerificationDeliveryFailed, err)
		return
	}
	record, err := s.resources.CreateInternal(appPhoneVerificationsResource, adminresource.WriteInput{
		Code:        "phone-verification-" + phoneDigestTag(phoneDigest) + "-" + fmt.Sprintf("%d", now.UnixNano()),
		Name:        "Phone verification / " + username,
		Status:      "pending",
		Description: "App phone verification managed by platform auth.",
		Values: map[string]string{
			"appUsername": username,
			"maskedPhone": maskedPhone,
			"phoneHash":   phoneDigest,
			"purpose":     purpose,
			"codeHash":    codeDigest,
			"requestedAt": now.Format(time.RFC3339),
			"expiresAt":   expiresAt.Format(time.RFC3339),
		},
	})
	if err != nil {
		writePlatformErrorWithCause(ctx, s.internalErrorSink, errorcode.CodeAppPhoneVerificationCreateFailed, err)
		return
	}
	response := appPhoneVerificationResponse{
		ID:          record.ID,
		MaskedPhone: maskedPhone,
		Purpose:     purpose,
		ExpiresAt:   expiresAt,
	}
	if s.debugCodeEnabled {
		response.DebugCode = verificationCode
	}
	ctx.JSON(http.StatusCreated, Response[appPhoneVerificationResponse]{
		Data: response,
	})
}

func (s *Server) appPhoneCreateBinding(ctx *gin.Context) {
	appSession, ok := AppSessionFromContext(ctx)
	if !ok {
		writePlatformError(ctx, errorcode.CodeAuthUnauthorized)
		return
	}
	var input appPhoneBindingRequest
	if err := ctx.ShouldBindJSON(&input); err != nil {
		writePlatformError(ctx, errorcode.CodeAppPhoneInvalidRequest)
		return
	}
	phone, ok := normalizeAppPhone(input.Phone)
	if !ok {
		writePlatformError(ctx, errorcode.CodeAppPhoneInvalidPhone)
		return
	}
	code := strings.TrimSpace(input.Code)
	if code == "" {
		writePlatformError(ctx, errorcode.CodeAppPhoneCodeRequired)
		return
	}
	username := appUsername(appSession.Username)
	if !s.enforceRateLimit(ctx, ratelimit.OperationPhoneBindingVerification, rateLimitClientIP(ctx), username, phone) {
		return
	}

	if s.phoneProtector == nil {
		writePlatformErrorWithCause(ctx, s.internalErrorSink, errorcode.CodeAppPhoneVerificationUnavailable, errors.New("app phone verification protector is not configured"))
		return
	}
	phoneDigest, err := s.phoneProtector.PhoneDigest(phone)
	if err != nil {
		writePlatformErrorWithCause(ctx, s.internalErrorSink, errorcode.CodeAppPhoneVerificationUnavailable, err)
		return
	}
	codeDigest, err := s.phoneProtector.CodeDigest(phoneDigest, appPhoneVerificationPurpose, code)
	if err != nil {
		writePlatformErrorWithCause(ctx, s.internalErrorSink, errorcode.CodeAppPhoneVerificationUnavailable, err)
		return
	}
	maskedPhone := maskAppPhone(phone)
	bindingExists, err := s.appPhoneBindingExists(ctx.Request.Context(), phoneDigest)
	if err != nil {
		writePlatformErrorWithCause(ctx, s.internalErrorSink, errorcode.CodeAppPhoneVerificationUnavailable, err)
		return
	}
	if bindingExists {
		writePlatformError(ctx, errorcode.CodeAppPhoneAlreadyBound)
		return
	}
	now := s.now().UTC()
	verification, ok, err := s.validAppPhoneVerification(ctx.Request.Context(), username, phoneDigest, appPhoneVerificationPurpose, codeDigest, now)
	if err != nil {
		writePlatformErrorWithCause(ctx, s.internalErrorSink, errorcode.CodeAppPhoneVerificationUnavailable, err)
		return
	}
	if !ok {
		writePlatformError(ctx, errorcode.CodeAppPhoneVerificationInvalid)
		return
	}

	record, err := s.resources.CreateInternal(appPhoneBindingsResource, adminresource.WriteInput{
		Code:        "phone-binding-" + phoneDigestTag(phoneDigest),
		Name:        "Phone binding / " + username,
		Status:      "enabled",
		Description: "App phone binding managed by platform auth.",
		Values: map[string]string{
			"appUsername":    username,
			"maskedPhone":    maskedPhone,
			"phoneHash":      phoneDigest,
			"boundAt":        now.Format(time.RFC3339),
			"verificationId": verification.ID,
		},
	})
	if err != nil {
		writePlatformErrorWithCause(ctx, s.internalErrorSink, errorcode.CodeAppPhoneBindingCreateFailed, err)
		return
	}
	if err := s.markAppPhoneVerificationUsed(verification, now); err != nil {
		writePlatformErrorWithCause(ctx, s.internalErrorSink, errorcode.CodeAppPhoneVerificationUpdateFailed, err)
		return
	}
	actorID := appUserID(username)
	if err := s.recordAudit(ctx, "app.phone.bind", actorID, actorID, "success", "phone-bound"); err != nil {
		writePlatformErrorWithCause(ctx, s.internalErrorSink, errorcode.CodeAppPhoneAuditFailed, err)
		return
	}
	ctx.JSON(http.StatusCreated, Response[appPhoneBindingResponse]{
		Data: appPhoneBindingResponse{
			ID:          record.ID,
			AppUsername: username,
			MaskedPhone: maskedPhone,
			BoundAt:     now,
		},
	})
}

func (s *Server) appPhoneBindingExists(ctx context.Context, phoneHash string) (bool, error) {
	records, err := s.resources.InternalRecordsContext(ctx, appPhoneBindingsResource)
	if err != nil {
		return false, err
	}
	for _, record := range records {
		if record.Status == "disabled" {
			continue
		}
		if record.Values["phoneHash"] == phoneHash {
			return true, nil
		}
	}
	return false, nil
}

func (s *Server) validAppPhoneVerification(ctx context.Context, username string, phoneHash string, purpose string, codeDigest string, now time.Time) (adminresource.Record, bool, error) {
	records, err := s.resources.InternalRecordsContext(ctx, appPhoneVerificationsResource)
	if err != nil {
		return adminresource.Record{}, false, err
	}
	for index := len(records) - 1; index >= 0; index-- {
		record := records[index]
		if record.Status != "pending" || record.Values["appUsername"] != username || record.Values["phoneHash"] != phoneHash || record.Values["purpose"] != purpose {
			continue
		}
		expiresAt, err := time.Parse(time.RFC3339, record.Values["expiresAt"])
		if err != nil || !expiresAt.After(now) {
			continue
		}
		if subtle.ConstantTimeCompare([]byte(record.Values["codeHash"]), []byte(codeDigest)) != 1 {
			continue
		}
		return record, true, nil
	}
	return adminresource.Record{}, false, nil
}

func (s *Server) markAppPhoneVerificationUsed(record adminresource.Record, now time.Time) error {
	values := cloneStringMap(record.Values)
	values["verifiedAt"] = now.UTC().Format(time.RFC3339)
	_, err := s.resources.UpdateInternal(appPhoneVerificationsResource, record.ID, adminresource.WriteInput{
		Code:        record.Code,
		Name:        record.Name,
		Status:      "verified",
		Description: record.Description,
		Values:      values,
	})
	return err
}

func normalizeAppPhone(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}
	var builder strings.Builder
	for index, r := range raw {
		switch {
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '+' && index == 0:
			builder.WriteRune(r)
		case r == ' ' || r == '-' || r == '(' || r == ')':
			continue
		default:
			return "", false
		}
	}
	phone := builder.String()
	digits := strings.TrimPrefix(phone, "+")
	if len(digits) < 6 || len(digits) > 18 {
		return "", false
	}
	return phone, true
}

func maskAppPhone(phone string) string {
	value := []rune(strings.TrimSpace(phone))
	if len(value) == 0 {
		return ""
	}
	if len(value) <= 7 {
		return string(value[:1]) + "***" + string(value[len(value)-1:])
	}
	return string(value[:3]) + "****" + string(value[len(value)-4:])
}

func phoneDigestTag(digest string) string {
	digest = strings.TrimSpace(digest)
	if len(digest) <= 12 {
		return digest
	}
	return digest[len(digest)-12:]
}

func newAppPhoneDebugCode() (string, error) {
	value, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", value.Int64()), nil
}
