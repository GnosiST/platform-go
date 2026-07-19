package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/GnosiST/platform-go/internal/platform/adminresource"
	"github.com/GnosiST/platform-go/internal/platform/errorcode"
	"github.com/GnosiST/platform-go/internal/platform/notification"
	"github.com/GnosiST/platform-go/internal/platform/ratelimit"
)

const (
	messageCenterTestPurpose       = "message_center_test"
	messageCenterNotifications     = "notifications"
	messageCenterDeliveries        = "notification-deliveries"
	messageCenterNotificationEvent = "message_center.test_send.notification"
	messageCenterDeliveryEvent     = "message_center.test_send.delivery"
)

type adminMessageCenterTestSendRequest struct {
	Channel        string            `json:"channel"`
	TenantCode     string            `json:"tenantCode"`
	Recipient      string            `json:"recipient"`
	TemplateID     string            `json:"templateId"`
	TemplateParams map[string]string `json:"templateParams"`
	Title          string            `json:"title"`
	Body           string            `json:"body"`
}

type adminMessageCenterTestSendResponse struct {
	Notification adminresource.Record         `json:"notification"`
	Delivery     adminresource.Record         `json:"delivery"`
	Receipt      notification.DeliveryReceipt `json:"receipt"`
}

type adminMessageCenterDeliveriesRunRequest struct {
	Limit int `json:"limit,omitempty"`
}

func (s *Server) adminMessageCenterTestSend(ctx *gin.Context) {
	if !s.authorize(ctx, "admin:message-center:update") {
		return
	}
	var input adminMessageCenterTestSendRequest
	decoder := json.NewDecoder(ctx.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writePlatformError(ctx, errorcode.CodeRequestBodyInvalid)
		return
	}
	prepared, err := s.prepareMessageCenterTestSend(input)
	if err != nil {
		writeAdminResourceError(ctx, s.internalErrorSink, err)
		return
	}
	if prepared.message.Channel == notification.ChannelSMS && smsSenderNil(s.notificationSMSSender) {
		writePlatformError(ctx, errorcode.CodeAdminMessageCenterUnavailable)
		return
	}
	decision, err := s.messageCenterDeliveryPolicyDecision(ctx.Request.Context(), notification.DeliveryPolicyInput{
		TenantCode: prepared.message.TenantCode,
		Channel:    prepared.message.Channel,
		Provider:   s.messageCenterRuntimeProvider(prepared.message.Channel),
		TemplateID: prepared.message.TemplateID,
		Purpose:    prepared.message.Purpose,
	}, s.auditActorID(ctx), rateLimitClientIP(ctx))
	if err != nil {
		writeRateLimitDependencyError(ctx, s.internalErrorSink, err)
		return
	}
	if decision.Disabled {
		writePlatformError(ctx, errorcode.CodeAdminMessageCenterUnavailable)
		return
	}
	if !decision.Allowed {
		writeMessageCenterRateLimited(ctx, decision.RetryAfter)
		return
	}
	notificationMutation, err := s.resources.CreateInternalWithAudit(messageCenterNotifications, prepared.notificationInput, s.mutationAuditEvent(ctx, messageCenterNotificationEvent, messageCenterNotifications, "test-send"))
	if err != nil {
		writeAdminResourceError(ctx, s.internalErrorSink, err)
		return
	}
	correlation := correlationFromGinContext(ctx)
	prepared.message.TraceID = correlation.TraceID
	receipt, sendErr := s.sendMessageCenterTestMessage(ctx, prepared.message)
	deliveryStatus := "delivered"
	deliveredAt := prepared.now
	errorMessage := ""
	if sendErr != nil {
		deliveryStatus = "failed"
		deliveredAt = time.Time{}
		errorMessage = "message center test send failed"
		receipt = notification.DeliveryReceipt{
			Channel:        prepared.message.Channel,
			Provider:       notification.DefaultProviderForChannel(prepared.message.Channel),
			Status:         "failed",
			RedactedTarget: notification.RedactMessageTarget(prepared.message.Channel, prepared.message.Recipient),
		}
		if prepared.message.Channel == notification.ChannelSMS && !smsSenderNil(s.notificationSMSSender) {
			receipt.Provider = s.notificationSMSSender.Kind()
		}
	}
	deliveryInput := prepared.deliveryInput(notificationMutation.Record.Code, notification.DeliveryLedgerInput{
		Receipt:        receipt,
		DeliveryStatus: deliveryStatus,
		ErrorMessage:   errorMessage,
		AttemptedAt:    prepared.now,
		DeliveredAt:    deliveredAt,
		RequestID:      correlation.RequestID,
		TraceID:        correlation.TraceID,
	})
	deliveryMutation, err := s.resources.CreateInternalWithAudit(messageCenterDeliveries, deliveryInput, s.mutationAuditEvent(ctx, messageCenterDeliveryEvent, messageCenterDeliveries, "test-send"))
	if err != nil {
		writeAdminResourceError(ctx, s.internalErrorSink, err)
		return
	}
	s.invalidateCachesForResource(ctx.Request.Context(), messageCenterNotifications)
	s.invalidateCachesForResource(ctx.Request.Context(), messageCenterDeliveries)
	response := adminMessageCenterTestSendResponse{
		Notification: notificationMutation.Record,
		Delivery:     deliveryMutation.Record,
		Receipt:      receipt,
	}
	if sendErr != nil {
		writePlatformErrorWithCause(ctx, s.internalErrorSink, errorcode.CodeAdminMessageCenterUnavailable, sendErr)
		return
	}
	ctx.JSON(http.StatusCreated, Response[adminMessageCenterTestSendResponse]{Data: response})
}

func (s *Server) sendMessageCenterTestMessage(ctx *gin.Context, message notification.Message) (notification.DeliveryReceipt, error) {
	if message.Channel == notification.ChannelSMS {
		receipt, err := s.notificationSMSSender.SendSMS(ctx.Request.Context(), notification.SMSMessageFromMessage(message))
		return notification.DeliveryReceiptFromSMS(receipt), err
	}
	sender, err := notification.NewDryRunMessageSender(message.Channel, notification.DefaultProviderForChannel(message.Channel))
	if err != nil {
		return notification.DeliveryReceipt{}, err
	}
	return sender.SendMessage(ctx.Request.Context(), message)
}

func (s *Server) adminMessageCenterDeliveriesRun(ctx *gin.Context) {
	principal, ok := s.authorizeAdminBearerSession(ctx, "admin:message-center:update")
	if !ok {
		return
	}
	if !s.enforceAdminRateLimit(ctx, ratelimit.OperationMessageCenterDelivery, principal.User.Username, rateLimitClientIP(ctx)) {
		return
	}
	var input adminMessageCenterDeliveriesRunRequest
	if ctx.Request.Body != nil && ctx.Request.ContentLength != 0 {
		decoder := json.NewDecoder(ctx.Request.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil {
			writePlatformError(ctx, errorcode.CodeRequestBodyInvalid)
			return
		}
		if input.Limit < 0 || input.Limit > 100 {
			writePlatformError(ctx, errorcode.CodeRequestBodyInvalid)
			return
		}
	}
	options := notification.DeliveryWorkerOptions{Now: s.now}
	if input.Limit > 0 {
		options.MaxBatch = input.Limit
	}
	if !smsSenderNil(s.notificationSMSSender) {
		provider := notification.CanonicalSMSProvider(s.notificationSMSSender.Kind())
		options.DefaultSMSProvider = provider
		options.SMSSenders = map[string]notification.SMSSender{provider: s.notificationSMSSender}
	}
	options.PolicyGate = messageCenterDeliveryPolicyGate{
		server:   s,
		actor:    principal.User.Username,
		clientIP: rateLimitClientIP(ctx),
	}
	result, err := notification.NewDeliveryWorker(s.resources, options).RunOnce(ctx.Request.Context())
	if err != nil {
		writeAdminResourceError(ctx, s.internalErrorSink, err)
		return
	}
	if result.Attempted > 0 {
		s.invalidateCachesForResource(ctx.Request.Context(), messageCenterDeliveries)
	}
	ctx.JSON(http.StatusOK, Response[notification.DeliveryWorkerResult]{Data: result})
}

func (s *Server) messageCenterRuntimeProvider(channel string) string {
	channel = notification.CanonicalChannel(channel)
	if channel == notification.ChannelSMS && !smsSenderNil(s.notificationSMSSender) {
		return s.notificationSMSSender.Kind()
	}
	return notification.DefaultProviderForChannel(channel)
}

func writeMessageCenterRateLimited(ctx *gin.Context, retryAfter time.Duration) {
	retryAfterSeconds := int64((retryAfter + time.Second - 1) / time.Second)
	if retryAfterSeconds < 1 {
		retryAfterSeconds = 1
	}
	ctx.Header("Retry-After", strconv.FormatInt(retryAfterSeconds, 10))
	writePlatformError(ctx, errorcode.CodeRateLimited)
}

type preparedMessageCenterTestSend struct {
	now               time.Time
	message           notification.Message
	notificationInput adminresource.WriteInput
	deliveryCode      string
	deliveryName      string
	deliveryBase      map[string]string
}

func (prepared preparedMessageCenterTestSend) deliveryInput(notificationCode string, ledger notification.DeliveryLedgerInput) adminresource.WriteInput {
	values := cloneStringMap(prepared.deliveryBase)
	values["notificationCode"] = notificationCode
	ledger.BaseValues = values
	ledger.Channel = prepared.message.Channel
	ledger.Target = prepared.message.Recipient
	if ledger.AttemptedAt.IsZero() {
		ledger.AttemptedAt = prepared.now
	}
	values = notification.BuildDeliveryLedgerValues(ledger)
	return adminresource.WriteInput{
		Code:        prepared.deliveryCode,
		Name:        prepared.deliveryName,
		Status:      "enabled",
		Description: "Message center test-send delivery ledger.",
		Values:      values,
	}
}

func (s *Server) prepareMessageCenterTestSend(input adminMessageCenterTestSendRequest) (preparedMessageCenterTestSend, error) {
	channel := strings.ToLower(strings.TrimSpace(input.Channel))
	if channel == "" {
		channel = notification.ChannelSMS
	}
	if !notification.IsSupportedChannel(channel) {
		return preparedMessageCenterTestSend{}, adminresource.ErrInvalidRecord
	}
	recipient := strings.TrimSpace(input.Recipient)
	templateID := strings.TrimSpace(input.TemplateID)
	if recipient == "" || templateID == "" {
		return preparedMessageCenterTestSend{}, adminresource.ErrInvalidRecord
	}
	title := strings.TrimSpace(input.Title)
	if title == "" {
		title = "Message center SMS test"
	}
	body := strings.TrimSpace(input.Body)
	if body == "" {
		body = "Message center SMS test send."
	}
	tenantCode := strings.TrimSpace(input.TenantCode)
	if tenantCode == "" {
		tenantCode = platformTenant
	}
	now := s.now().UTC()
	notificationCode, err := messageCenterGeneratedCode("notice-test")
	if err != nil {
		return preparedMessageCenterTestSend{}, err
	}
	deliveryCode, err := messageCenterGeneratedCode("delivery-test")
	if err != nil {
		return preparedMessageCenterTestSend{}, err
	}
	payload, err := messageCenterNotificationPayload(channel, notification.RedactMessageTarget(channel, recipient), templateID, input.TemplateParams)
	if err != nil {
		return preparedMessageCenterTestSend{}, err
	}
	values := map[string]string{
		"tenantCode": tenantCode,
		"category":   messageCenterTestPurpose,
		"priority":   "normal",
		"readStatus": "unread",
		"payload":    payload,
		"sentAt":     now.Format(time.RFC3339),
	}
	return preparedMessageCenterTestSend{
		now: now,
		message: notification.Message{
			TenantCode:     tenantCode,
			Channel:        channel,
			Recipient:      recipient,
			TemplateID:     templateID,
			TemplateParams: cloneStringMap(input.TemplateParams),
			Title:          title,
			Body:           body,
			Purpose:        messageCenterTestPurpose,
		},
		notificationInput: adminresource.WriteInput{
			Code:        notificationCode,
			Name:        title,
			Status:      "sent",
			Description: body,
			Values:      values,
		},
		deliveryCode: deliveryCode,
		deliveryName: "Delivery for " + notificationCode,
		deliveryBase: map[string]string{
			"tenantCode": tenantCode,
			"channel":    channel,
		},
	}, nil
}

func messageCenterNotificationPayload(channel string, redactedTarget string, templateID string, templateParams map[string]string) (string, error) {
	return messageCenterNotificationPayloadForPurpose(channel, redactedTarget, templateID, templateParams, messageCenterTestPurpose)
}

func messageCenterNotificationPayloadForPurpose(channel string, redactedTarget string, templateID string, templateParams map[string]string, purpose string) (string, error) {
	paramKeys := make([]string, 0, len(templateParams))
	for key := range templateParams {
		key = strings.TrimSpace(key)
		if key != "" {
			paramKeys = append(paramKeys, key)
		}
	}
	sort.Strings(paramKeys)
	payload := struct {
		Channel           string   `json:"channel"`
		RedactedTarget    string   `json:"redactedTarget"`
		TemplateID        string   `json:"templateId"`
		TemplateParamKeys []string `json:"templateParamKeys,omitempty"`
		Purpose           string   `json:"purpose"`
	}{
		Channel:           channel,
		RedactedTarget:    redactedTarget,
		TemplateID:        templateID,
		TemplateParamKeys: paramKeys,
		Purpose:           purpose,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func messageCenterGeneratedCode(prefix string) (string, error) {
	var suffix [8]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return "", err
	}
	return prefix + "-" + hex.EncodeToString(suffix[:]), nil
}

func smsSenderNil(sender notification.SMSSender) bool {
	if sender == nil {
		return true
	}
	value := reflect.ValueOf(sender)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}
