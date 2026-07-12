package httpapi

import (
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
		writeUnauthorized(ctx)
		return
	}
	var input appPhoneVerificationRequest
	if err := ctx.ShouldBindJSON(&input); err != nil {
		writeAuthError(ctx, http.StatusBadRequest, "APP_PHONE_INVALID_REQUEST", "invalid app phone verification request")
		return
	}
	phone, ok := normalizeAppPhone(input.Phone)
	if !ok {
		writeAuthError(ctx, http.StatusBadRequest, "APP_PHONE_INVALID_PHONE", "invalid phone number")
		return
	}
	purpose := strings.TrimSpace(input.Purpose)
	if purpose == "" {
		purpose = appPhoneVerificationPurpose
	}
	if purpose != appPhoneVerificationPurpose {
		writeAuthError(ctx, http.StatusBadRequest, "APP_PHONE_PURPOSE_UNSUPPORTED", "app phone purpose is unsupported")
		return
	}
	username := appUsername(appSession.Username)
	if !s.enforceRateLimit(ctx, ratelimit.OperationPhoneVerificationRequest, rateLimitClientIP(ctx), username, phone, purpose) {
		return
	}
	if s.phoneProtector == nil || s.phoneVerificationSender == nil {
		writeAuthError(ctx, http.StatusServiceUnavailable, "APP_PHONE_VERIFICATION_UNAVAILABLE", "app phone verification is unavailable")
		return
	}
	phoneDigest, err := s.phoneProtector.PhoneDigest(phone)
	if err != nil {
		writeAuthError(ctx, http.StatusServiceUnavailable, "APP_PHONE_VERIFICATION_UNAVAILABLE", "app phone verification is unavailable")
		return
	}
	now := s.now().UTC()
	expiresAt := now.Add(appPhoneVerificationTTL)
	maskedPhone := maskAppPhone(phone)
	verificationCode, err := newAppPhoneDebugCode()
	if err != nil {
		writeAuthError(ctx, http.StatusInternalServerError, "APP_PHONE_CODE_GENERATION_FAILED", "app phone code generation failed")
		return
	}
	codeDigest, err := s.phoneProtector.CodeDigest(phoneDigest, purpose, verificationCode)
	if err != nil {
		writeAuthError(ctx, http.StatusServiceUnavailable, "APP_PHONE_VERIFICATION_UNAVAILABLE", "app phone verification is unavailable")
		return
	}
	if err := s.phoneVerificationSender.Send(ctx.Request.Context(), phone, purpose, verificationCode); err != nil {
		writeAuthError(ctx, http.StatusBadGateway, "APP_PHONE_VERIFICATION_DELIVERY_FAILED", "app phone verification delivery failed")
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
		writeAuthError(ctx, http.StatusInternalServerError, "APP_PHONE_VERIFICATION_CREATE_FAILED", "app phone verification create failed")
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
		writeUnauthorized(ctx)
		return
	}
	var input appPhoneBindingRequest
	if err := ctx.ShouldBindJSON(&input); err != nil {
		writeAuthError(ctx, http.StatusBadRequest, "APP_PHONE_INVALID_REQUEST", "invalid app phone binding request")
		return
	}
	phone, ok := normalizeAppPhone(input.Phone)
	if !ok {
		writeAuthError(ctx, http.StatusBadRequest, "APP_PHONE_INVALID_PHONE", "invalid phone number")
		return
	}
	code := strings.TrimSpace(input.Code)
	if code == "" {
		writeAuthError(ctx, http.StatusBadRequest, "APP_PHONE_CODE_REQUIRED", "app phone verification code is required")
		return
	}
	username := appUsername(appSession.Username)
	if !s.enforceRateLimit(ctx, ratelimit.OperationPhoneBindingVerification, rateLimitClientIP(ctx), username, phone) {
		return
	}

	if s.phoneProtector == nil {
		writeAuthError(ctx, http.StatusServiceUnavailable, "APP_PHONE_VERIFICATION_UNAVAILABLE", "app phone verification is unavailable")
		return
	}
	phoneDigest, err := s.phoneProtector.PhoneDigest(phone)
	if err != nil {
		writeAuthError(ctx, http.StatusServiceUnavailable, "APP_PHONE_VERIFICATION_UNAVAILABLE", "app phone verification is unavailable")
		return
	}
	codeDigest, err := s.phoneProtector.CodeDigest(phoneDigest, appPhoneVerificationPurpose, code)
	if err != nil {
		writeAuthError(ctx, http.StatusServiceUnavailable, "APP_PHONE_VERIFICATION_UNAVAILABLE", "app phone verification is unavailable")
		return
	}
	maskedPhone := maskAppPhone(phone)
	if s.appPhoneBindingExists(phoneDigest) {
		writeAuthError(ctx, http.StatusConflict, "APP_PHONE_ALREADY_BOUND", "app phone is already bound")
		return
	}
	now := s.now().UTC()
	verification, ok := s.validAppPhoneVerification(username, phoneDigest, appPhoneVerificationPurpose, codeDigest, now)
	if !ok {
		writeAuthError(ctx, http.StatusBadRequest, "APP_PHONE_VERIFICATION_INVALID", "app phone verification is invalid")
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
		writeAuthError(ctx, http.StatusInternalServerError, "APP_PHONE_BINDING_CREATE_FAILED", "app phone binding create failed")
		return
	}
	if err := s.markAppPhoneVerificationUsed(verification, now); err != nil {
		writeAuthError(ctx, http.StatusInternalServerError, "APP_PHONE_VERIFICATION_UPDATE_FAILED", "app phone verification update failed")
		return
	}
	actorID := appUserID(username)
	if err := s.recordAudit("app.phone.bind", actorID, actorID, "success", "phone-bound"); err != nil {
		writeAuthError(ctx, http.StatusInternalServerError, "APP_PHONE_AUDIT_FAILED", "app phone audit failed")
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

func (s *Server) appPhoneBindingExists(phoneHash string) bool {
	records, err := s.resources.List(appPhoneBindingsResource)
	if errors.Is(err, adminresource.ErrUnknownResource) {
		return false
	}
	if err != nil {
		return false
	}
	for _, record := range records {
		if record.Status == "disabled" {
			continue
		}
		if record.Values["phoneHash"] == phoneHash {
			return true
		}
	}
	return false
}

func (s *Server) validAppPhoneVerification(username string, phoneHash string, purpose string, codeDigest string, now time.Time) (adminresource.Record, bool) {
	records, err := s.resources.List(appPhoneVerificationsResource)
	if err != nil {
		return adminresource.Record{}, false
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
		return record, true
	}
	return adminresource.Record{}, false
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
