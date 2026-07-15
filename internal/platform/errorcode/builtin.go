package errorcode

const (
	CodeAdminAuthStateRefreshFailed            Code = "ADMIN_AUTH_STATE_REFRESH_FAILED"
	CodeAdminDemoDataNotFound                  Code = "ADMIN_DEMO_DATA_NOT_FOUND"
	CodeAdminForbidden                         Code = "ADMIN_FORBIDDEN"
	CodeAdminMenuResolutionFailed              Code = "ADMIN_MENU_RESOLUTION_FAILED"
	CodeAdminPolicyReviewWatermarkInvalid      Code = "ADMIN_POLICY_REVIEW_WATERMARK_INVALID"
	CodeOpenAPINotConfigured                   Code = "OPENAPI_NOT_CONFIGURED"
	CodeAuthAuditFailed                        Code = "AUTH_AUDIT_FAILED"
	CodeAuthIdentityBindingFailed              Code = "AUTH_IDENTITY_BINDING_FAILED"
	CodeAuthIdentityInvalid                    Code = "AUTH_IDENTITY_INVALID"
	CodeAuthIdentityNotBound                   Code = "AUTH_IDENTITY_NOT_BOUND"
	CodeAuthIdentityTransactionInvalid         Code = "AUTH_IDENTITY_TRANSACTION_INVALID"
	CodeAuthIdentityTransactionRequired        Code = "AUTH_IDENTITY_TRANSACTION_REQUIRED"
	CodeAuthInvalidCredentials                 Code = "AUTH_INVALID_CREDENTIALS"
	CodeAuthInvalidRequest                     Code = "AUTH_INVALID_REQUEST"
	CodeAuthProviderNotConfigured              Code = "AUTH_PROVIDER_NOT_CONFIGURED"
	CodeAuthProviderNotFound                   Code = "AUTH_PROVIDER_NOT_FOUND"
	CodeAuthProviderResolverNotConfigured      Code = "AUTH_PROVIDER_RESOLVER_NOT_CONFIGURED"
	CodeAuthProviderResolveFailed              Code = "AUTH_PROVIDER_RESOLVE_FAILED"
	CodeAuthProviderStartFailed                Code = "AUTH_PROVIDER_START_FAILED"
	CodeAuthProviderStartInvalid               Code = "AUTH_PROVIDER_START_INVALID"
	CodeAuthProviderUnsupported                Code = "AUTH_PROVIDER_UNSUPPORTED"
	CodeAuthSessionCleanupFailed               Code = "AUTH_SESSION_CLEANUP_FAILED"
	CodeAuthSessionIssueFailed                 Code = "AUTH_SESSION_ISSUE_FAILED"
	CodeAuthSessionRenewFailed                 Code = "AUTH_SESSION_RENEW_FAILED"
	CodeAuthSessionRevokeFailed                Code = "AUTH_SESSION_REVOKE_FAILED"
	CodeAuthStateRefreshFailed                 Code = "AUTH_STATE_REFRESH_FAILED"
	CodeAuthTokenSignFailed                    Code = "AUTH_TOKEN_SIGN_FAILED"
	CodeAuthUnauthorized                       Code = "AUTH_UNAUTHORIZED"
	CodeAdminResourceError                     Code = "ADMIN_RESOURCE_ERROR"
	CodeAdminResourceNotFound                  Code = "ADMIN_RESOURCE_NOT_FOUND"
	CodeAdminResourceRecordNotFound            Code = "ADMIN_RESOURCE_RECORD_NOT_FOUND"
	CodeAdminResourceRevisionConflict          Code = "ADMIN_RESOURCE_REVISION_CONFLICT"
	CodeAdminResourceDomainOwnedMutation       Code = "ADMIN_RESOURCE_DOMAIN_OWNED_MUTATION"
	CodeAdminResourceLifecycleConflict         Code = "ADMIN_RESOURCE_LIFECYCLE_CONFLICT"
	CodeAdminResourceInvalidRecord             Code = "ADMIN_RESOURCE_INVALID_RECORD"
	CodeAdminSensitiveRevealConflict           Code = "ADMIN_SENSITIVE_REVEAL_CONFLICT"
	CodeAdminSensitiveRevealDeliveryFailed     Code = "ADMIN_SENSITIVE_REVEAL_DELIVERY_FAILED"
	CodeAdminSensitiveRevealExpired            Code = "ADMIN_SENSITIVE_REVEAL_EXPIRED"
	CodeAdminSensitiveRevealFailed             Code = "ADMIN_SENSITIVE_REVEAL_FAILED"
	CodeAdminSensitiveRevealInvalid            Code = "ADMIN_SENSITIVE_REVEAL_INVALID"
	CodeAdminSensitiveRevealNotFound           Code = "ADMIN_SENSITIVE_REVEAL_NOT_FOUND"
	CodeAdminSensitiveRevealProviderFailed     Code = "ADMIN_SENSITIVE_REVEAL_PROVIDER_FAILED"
	CodeAdminSensitiveRevealStateRefreshFailed Code = "ADMIN_SENSITIVE_REVEAL_STATE_REFRESH_FAILED"
	CodeAdminSensitiveRevealUnavailable        Code = "ADMIN_SENSITIVE_REVEAL_UNAVAILABLE"
	CodeAdminSensitiveRevealVerificationFailed Code = "ADMIN_SENSITIVE_REVEAL_VERIFICATION_FAILED"
	CodeAdminFileRestoreUnavailable            Code = "ADMIN_FILE_RESTORE_UNAVAILABLE"
	CodeAdminFileRestoreFailed                 Code = "ADMIN_FILE_RESTORE_FAILED"
	CodeAdminFileSaveFailed                    Code = "ADMIN_FILE_SAVE_FAILED"
	CodeAdminFileRollbackFailed                Code = "ADMIN_FILE_ROLLBACK_FAILED"
	CodeAdminFileObjectNotFound                Code = "ADMIN_FILE_OBJECT_NOT_FOUND"
	CodeAdminFileOpenFailed                    Code = "ADMIN_FILE_OPEN_FAILED"
	CodeAdminFileTooLarge                      Code = "ADMIN_FILE_TOO_LARGE"
	CodeAdminFileRequired                      Code = "ADMIN_FILE_REQUIRED"
	CodeAdminFileUploadOpenFailed              Code = "ADMIN_FILE_UPLOAD_OPEN_FAILED"
	CodeAdminFileReadFailed                    Code = "ADMIN_FILE_READ_FAILED"
	CodeAdminFileMIMEInvalid                   Code = "ADMIN_FILE_MIME_INVALID"
	CodeAdminFileMIMEMismatch                  Code = "ADMIN_FILE_MIME_MISMATCH"
	CodeAdminFileMIMENotAllowed                Code = "ADMIN_FILE_MIME_NOT_ALLOWED"
	CodeAppAuthAuditFailed                     Code = "APP_AUTH_AUDIT_FAILED"
	CodeAppAuthCodeRequired                    Code = "APP_AUTH_CODE_REQUIRED"
	CodeAppAuthIdentityBindingFailed           Code = "APP_AUTH_IDENTITY_BINDING_FAILED"
	CodeAppAuthIdentityInvalid                 Code = "APP_AUTH_IDENTITY_INVALID"
	CodeAppAuthInvalidRequest                  Code = "APP_AUTH_INVALID_REQUEST"
	CodeAppAuthProviderNotConfigured           Code = "APP_AUTH_PROVIDER_NOT_CONFIGURED"
	CodeAppAuthProviderNotFound                Code = "APP_AUTH_PROVIDER_NOT_FOUND"
	CodeAppAuthProviderResolverNotConfigured   Code = "APP_AUTH_PROVIDER_RESOLVER_NOT_CONFIGURED"
	CodeAppAuthProviderResolveFailed           Code = "APP_AUTH_PROVIDER_RESOLVE_FAILED"
	CodeAppAuthSessionCleanupFailed            Code = "APP_AUTH_SESSION_CLEANUP_FAILED"
	CodeAppAuthSessionIssueFailed              Code = "APP_AUTH_SESSION_ISSUE_FAILED"
	CodeAppAuthSessionRevokeFailed             Code = "APP_AUTH_SESSION_REVOKE_FAILED"
	CodeAppAuthTokenSignFailed                 Code = "APP_AUTH_TOKEN_SIGN_FAILED"
	CodeAppRouteHandlerNotConfigured           Code = "APP_ROUTE_HANDLER_NOT_CONFIGURED"
	CodeAppPhoneAlreadyBound                   Code = "APP_PHONE_ALREADY_BOUND"
	CodeAppPhoneAuditFailed                    Code = "APP_PHONE_AUDIT_FAILED"
	CodeAppPhoneBindingCreateFailed            Code = "APP_PHONE_BINDING_CREATE_FAILED"
	CodeAppPhoneCodeGenerationFailed           Code = "APP_PHONE_CODE_GENERATION_FAILED"
	CodeAppPhoneCodeRequired                   Code = "APP_PHONE_CODE_REQUIRED"
	CodeAppPhoneInvalidPhone                   Code = "APP_PHONE_INVALID_PHONE"
	CodeAppPhoneInvalidRequest                 Code = "APP_PHONE_INVALID_REQUEST"
	CodeAppPhonePurposeUnsupported             Code = "APP_PHONE_PURPOSE_UNSUPPORTED"
	CodeAppPhoneVerificationCreateFailed       Code = "APP_PHONE_VERIFICATION_CREATE_FAILED"
	CodeAppPhoneVerificationDeliveryFailed     Code = "APP_PHONE_VERIFICATION_DELIVERY_FAILED"
	CodeAppPhoneVerificationInvalid            Code = "APP_PHONE_VERIFICATION_INVALID"
	CodeAppPhoneVerificationUnavailable        Code = "APP_PHONE_VERIFICATION_UNAVAILABLE"
	CodeAppPhoneVerificationUpdateFailed       Code = "APP_PHONE_VERIFICATION_UPDATE_FAILED"
	CodeAppFileSaveFailed                      Code = "APP_FILE_SAVE_FAILED"
	CodeAppFileRollbackFailed                  Code = "APP_FILE_ROLLBACK_FAILED"
	CodeAppFileStateRefreshFailed              Code = "APP_FILE_STATE_REFRESH_FAILED"
	CodeAppFileNotFound                        Code = "APP_FILE_NOT_FOUND"
	CodeAppFileObjectNotFound                  Code = "APP_FILE_OBJECT_NOT_FOUND"
	CodeAppFileOpenFailed                      Code = "APP_FILE_OPEN_FAILED"
	CodeAppFileTooLarge                        Code = "APP_FILE_TOO_LARGE"
	CodeAppFileRequired                        Code = "APP_FILE_REQUIRED"
	CodeAppFileUploadOpenFailed                Code = "APP_FILE_UPLOAD_OPEN_FAILED"
	CodeAppFileReadFailed                      Code = "APP_FILE_READ_FAILED"
	CodeAppFileMIMEInvalid                     Code = "APP_FILE_MIME_INVALID"
	CodeAppFileMIMEMismatch                    Code = "APP_FILE_MIME_MISMATCH"
	CodeAppFileMIMENotAllowed                  Code = "APP_FILE_MIME_NOT_ALLOWED"
	CodeAppForbidden                           Code = "APP_FORBIDDEN"
	CodeRequestBodyInvalid                     Code = "REQUEST_BODY_INVALID"
	CodeRequestBodyTooLarge                    Code = "REQUEST_BODY_TOO_LARGE"
	CodeRateLimitUnavailable                   Code = "RATE_LIMIT_UNAVAILABLE"
	CodeRateLimited                            Code = "RATE_LIMITED"
	CodeFileUploadInvalid                      Code = "FILE_UPLOAD_INVALID"
	CodeServiceObjectUnavailable               Code = "SERVICE_OBJECT_UNAVAILABLE"
	CodeServiceObjectRequestInvalid            Code = "SERVICE_OBJECT_REQUEST_INVALID"
	CodeServiceObjectCostLimit                 Code = "SERVICE_OBJECT_COST_LIMIT"
	CodeServiceObjectIdempotencyConflict       Code = "SERVICE_OBJECT_IDEMPOTENCY_CONFLICT"
	CodeServiceObjectStateConflict             Code = "SERVICE_OBJECT_STATE_CONFLICT"
	CodeServiceObjectDomainValidation          Code = "SERVICE_OBJECT_DOMAIN_VALIDATION"
	CodeServiceObjectExecutionFailed           Code = "SERVICE_OBJECT_EXECUTION_FAILED"
	CodeInvalidExecutionContext                Code = "INVALID_EXECUTION_CONTEXT"
	CodeValidation                             Code = "VALIDATION_ERROR"
	CodeNotFound                               Code = "NOT_FOUND"
	CodeConflict                               Code = "CONFLICT"
	CodeForbidden                              Code = "FORBIDDEN"
	CodeInternal                               Code = "INTERNAL_ERROR"
)

var builtinDefinitions = []Definition{
	public(CodeAdminAuthStateRefreshFailed, "platform.http", []Plane{PlaneAdmin}, []Audience{AudienceOperator}, CategoryDependency, 503, "authorization state is unavailable"),
	public(CodeAdminDemoDataNotFound, "platform.http", []Plane{PlaneAdmin}, []Audience{AudienceOperator}, CategoryNotFound, 404, "demo data set not found"),
	public(CodeAdminForbidden, "platform.http", []Plane{PlaneAdmin}, []Audience{AudienceOperator}, CategoryAuthorization, 403, "permission denied"),
	public(CodeAdminMenuResolutionFailed, "platform.http", []Plane{PlaneAdmin}, []Audience{AudienceOperator}, CategoryDependency, 503, "admin menu navigation is unavailable"),
	public(CodeAdminPolicyReviewWatermarkInvalid, "platform.http", []Plane{PlaneAdmin}, []Audience{AudienceOperator}, CategoryValidation, 400, "watermark must be true or false"),
	public(CodeOpenAPINotConfigured, "platform.http", []Plane{PlaneAdmin}, []Audience{AudienceOperator}, CategoryNotFound, 404, "openapi document is not configured"),
	public(CodeAuthAuditFailed, "platform.auth", []Plane{PlaneAdmin}, []Audience{AudienceOperator}, CategoryInternal, 500, "auth audit failed"),
	public(CodeAuthIdentityBindingFailed, "platform.auth", []Plane{PlaneAdmin}, []Audience{AudienceOperator}, CategoryInternal, 500, "admin identity binding failed"),
	public(CodeAuthIdentityInvalid, "platform.auth", []Plane{PlaneAdmin}, []Audience{AudienceOperator}, CategoryValidation, 400, "invalid admin identity"),
	public(CodeAuthIdentityNotBound, "platform.auth", []Plane{PlaneAdmin}, []Audience{AudienceOperator}, CategoryAuthorization, 401, "admin identity is not bound"),
	public(CodeAuthIdentityTransactionInvalid, "platform.auth", []Plane{PlaneAdmin}, []Audience{AudienceOperator}, CategoryAuthorization, 401, "invalid admin identity transaction"),
	public(CodeAuthIdentityTransactionRequired, "platform.auth", []Plane{PlaneAdmin}, []Audience{AudienceOperator}, CategoryValidation, 400, "auth identity transaction is required"),
	public(CodeAuthInvalidCredentials, "platform.auth", []Plane{PlaneAdmin}, []Audience{AudienceOperator}, CategoryAuthorization, 401, "invalid credentials"),
	public(CodeAuthInvalidRequest, "platform.auth", []Plane{PlaneAdmin}, []Audience{AudienceOperator}, CategoryValidation, 400, "invalid auth login request"),
	public(CodeAuthProviderNotConfigured, "platform.auth", []Plane{PlaneAdmin}, []Audience{AudienceOperator}, CategoryValidation, 400, "auth provider is not configured"),
	public(CodeAuthProviderNotFound, "platform.auth", []Plane{PlaneAdmin}, []Audience{AudienceOperator}, CategoryValidation, 400, "auth provider not found"),
	nonRetryableDependency(CodeAuthProviderResolverNotConfigured, "platform.auth", []Plane{PlaneAdmin}, []Audience{AudienceOperator}, 501, "auth provider resolver is not configured"),
	public(CodeAuthProviderResolveFailed, "platform.auth", []Plane{PlaneAdmin}, []Audience{AudienceOperator}, CategoryDependency, 502, "auth provider resolve failed"),
	public(CodeAuthProviderStartFailed, "platform.auth", []Plane{PlaneAdmin}, []Audience{AudienceOperator}, CategoryDependency, 502, "auth provider start failed"),
	public(CodeAuthProviderStartInvalid, "platform.auth", []Plane{PlaneAdmin}, []Audience{AudienceOperator}, CategoryValidation, 400, "invalid auth provider start request"),
	public(CodeAuthProviderUnsupported, "platform.auth", []Plane{PlaneAdmin}, []Audience{AudienceOperator}, CategoryValidation, 400, "auth provider is not supported"),
	public(CodeAuthSessionCleanupFailed, "platform.auth", []Plane{PlaneAdmin}, []Audience{AudienceOperator}, CategoryInternal, 500, "session cleanup failed"),
	public(CodeAuthSessionIssueFailed, "platform.auth", []Plane{PlaneAdmin}, []Audience{AudienceOperator}, CategoryInternal, 500, "session issue failed"),
	public(CodeAuthSessionRenewFailed, "platform.auth", []Plane{PlaneAdmin}, []Audience{AudienceOperator}, CategoryInternal, 500, "session renewal failed"),
	public(CodeAuthSessionRevokeFailed, "platform.auth", []Plane{PlaneAdmin}, []Audience{AudienceOperator}, CategoryInternal, 500, "session revoke failed"),
	public(CodeAuthStateRefreshFailed, "platform.auth", []Plane{PlaneAdmin}, []Audience{AudienceOperator}, CategoryDependency, 503, "auth state is unavailable"),
	public(CodeAuthTokenSignFailed, "platform.auth", []Plane{PlaneAdmin}, []Audience{AudienceOperator}, CategoryInternal, 500, "auth token sign failed"),
	public(CodeAuthUnauthorized, "platform.auth", []Plane{PlaneAdmin, PlaneApp}, []Audience{AudienceOperator, AudiencePublic}, CategoryAuthorization, 401, "unauthorized"),
	public(CodeAdminResourceError, "platform.adminresource", []Plane{PlaneAdmin}, []Audience{AudienceOperator}, CategoryInternal, 500, "admin resource operation failed"),
	public(CodeAdminResourceNotFound, "platform.adminresource", []Plane{PlaneAdmin}, []Audience{AudienceOperator}, CategoryNotFound, 404, "admin resource not found"),
	public(CodeAdminResourceRecordNotFound, "platform.adminresource", []Plane{PlaneAdmin}, []Audience{AudienceOperator}, CategoryNotFound, 404, "admin resource record not found"),
	public(CodeAdminResourceRevisionConflict, "platform.adminresource", []Plane{PlaneAdmin}, []Audience{AudienceOperator}, CategoryConflict, 409, "admin resource revision conflict"),
	public(CodeAdminResourceDomainOwnedMutation, "platform.adminresource", []Plane{PlaneAdmin}, []Audience{AudienceOperator}, CategoryConflict, 409, "resource mutation requires a governed domain command"),
	public(CodeAdminResourceLifecycleConflict, "platform.adminresource", []Plane{PlaneAdmin}, []Audience{AudienceOperator}, CategoryConflict, 409, "admin resource lifecycle conflict"),
	public(CodeAdminResourceInvalidRecord, "platform.adminresource", []Plane{PlaneAdmin}, []Audience{AudienceOperator}, CategoryValidation, 400, "invalid admin resource record"),
	public(CodeAdminSensitiveRevealConflict, "platform.sensitive", []Plane{PlaneAdmin}, []Audience{AudienceOperator}, CategoryConflict, 409, "sensitive reveal request conflicts with its current state"),
	public(CodeAdminSensitiveRevealDeliveryFailed, "platform.sensitive", []Plane{PlaneAdmin}, []Audience{AudienceOperator}, CategoryDependency, 502, "sensitive reveal verification delivery failed"),
	public(CodeAdminSensitiveRevealExpired, "platform.sensitive", []Plane{PlaneAdmin}, []Audience{AudienceOperator}, CategoryConflict, 410, "sensitive reveal request has expired"),
	public(CodeAdminSensitiveRevealFailed, "platform.sensitive", []Plane{PlaneAdmin}, []Audience{AudienceOperator}, CategoryInternal, 500, "sensitive field reveal failed"),
	public(CodeAdminSensitiveRevealInvalid, "platform.sensitive", []Plane{PlaneAdmin}, []Audience{AudienceOperator}, CategoryValidation, 400, "invalid sensitive reveal request"),
	public(CodeAdminSensitiveRevealNotFound, "platform.sensitive", []Plane{PlaneAdmin}, []Audience{AudienceOperator}, CategoryNotFound, 404, "sensitive reveal request was not found"),
	public(CodeAdminSensitiveRevealProviderFailed, "platform.sensitive", []Plane{PlaneAdmin}, []Audience{AudienceOperator}, CategoryDependency, 502, "sensitive reveal identity provider failed"),
	public(CodeAdminSensitiveRevealStateRefreshFailed, "platform.sensitive", []Plane{PlaneAdmin}, []Audience{AudienceOperator}, CategoryDependency, 503, "sensitive reveal authorization state is unavailable"),
	public(CodeAdminSensitiveRevealUnavailable, "platform.sensitive", []Plane{PlaneAdmin}, []Audience{AudienceOperator}, CategoryDependency, 503, "sensitive field reveal is unavailable"),
	public(CodeAdminSensitiveRevealVerificationFailed, "platform.sensitive", []Plane{PlaneAdmin}, []Audience{AudienceOperator}, CategoryValidation, 422, "sensitive reveal verification failed"),
	public(CodeAdminFileRestoreUnavailable, "platform.file", []Plane{PlaneAdmin}, []Audience{AudienceOperator}, CategoryConflict, 409, "file restore is unavailable"),
	public(CodeAdminFileRestoreFailed, "platform.file", []Plane{PlaneAdmin}, []Audience{AudienceOperator}, CategoryInternal, 500, "file restore failed"),
	public(CodeAdminFileSaveFailed, "platform.file", []Plane{PlaneAdmin}, []Audience{AudienceOperator}, CategoryInternal, 500, "file save failed"),
	public(CodeAdminFileRollbackFailed, "platform.file", []Plane{PlaneAdmin}, []Audience{AudienceOperator}, CategoryInternal, 500, "file upload rollback failed"),
	public(CodeAdminFileObjectNotFound, "platform.file", []Plane{PlaneAdmin}, []Audience{AudienceOperator}, CategoryNotFound, 404, "file object not found"),
	public(CodeAdminFileOpenFailed, "platform.file", []Plane{PlaneAdmin}, []Audience{AudienceOperator}, CategoryInternal, 500, "file open failed"),
	public(CodeAdminFileTooLarge, "platform.file", []Plane{PlaneAdmin}, []Audience{AudienceOperator}, CategoryValidation, 413, "file exceeds the configured upload limit"),
	public(CodeAdminFileRequired, "platform.file", []Plane{PlaneAdmin}, []Audience{AudienceOperator}, CategoryValidation, 400, "file is required"),
	public(CodeAdminFileUploadOpenFailed, "platform.file", []Plane{PlaneAdmin}, []Audience{AudienceOperator}, CategoryValidation, 400, "open uploaded file failed"),
	public(CodeAdminFileReadFailed, "platform.file", []Plane{PlaneAdmin}, []Audience{AudienceOperator}, CategoryValidation, 400, "read uploaded file failed"),
	public(CodeAdminFileMIMEInvalid, "platform.file", []Plane{PlaneAdmin}, []Audience{AudienceOperator}, CategoryValidation, 415, "uploaded file MIME type is invalid"),
	public(CodeAdminFileMIMEMismatch, "platform.file", []Plane{PlaneAdmin}, []Audience{AudienceOperator}, CategoryValidation, 415, "declared and detected file MIME types do not match"),
	public(CodeAdminFileMIMENotAllowed, "platform.file", []Plane{PlaneAdmin}, []Audience{AudienceOperator}, CategoryValidation, 415, "uploaded file MIME type is not allowed"),
	public(CodeAppAuthAuditFailed, "platform.appauth", []Plane{PlaneApp}, []Audience{AudiencePublic}, CategoryInternal, 500, "app auth audit failed"),
	public(CodeAppAuthCodeRequired, "platform.appauth", []Plane{PlaneApp}, []Audience{AudiencePublic}, CategoryValidation, 400, "app auth code is required"),
	public(CodeAppAuthIdentityBindingFailed, "platform.appauth", []Plane{PlaneApp}, []Audience{AudiencePublic}, CategoryInternal, 500, "app identity binding failed"),
	public(CodeAppAuthIdentityInvalid, "platform.appauth", []Plane{PlaneApp}, []Audience{AudiencePublic}, CategoryAuthorization, 401, "invalid app identity"),
	public(CodeAppAuthInvalidRequest, "platform.appauth", []Plane{PlaneApp}, []Audience{AudiencePublic}, CategoryValidation, 400, "invalid app auth login request"),
	public(CodeAppAuthProviderNotConfigured, "platform.appauth", []Plane{PlaneApp}, []Audience{AudiencePublic}, CategoryValidation, 400, "app auth provider is not configured"),
	public(CodeAppAuthProviderNotFound, "platform.appauth", []Plane{PlaneApp}, []Audience{AudiencePublic}, CategoryValidation, 400, "app auth provider not found"),
	nonRetryableDependency(CodeAppAuthProviderResolverNotConfigured, "platform.appauth", []Plane{PlaneApp}, []Audience{AudiencePublic}, 501, "app auth provider resolver is not configured"),
	public(CodeAppAuthProviderResolveFailed, "platform.appauth", []Plane{PlaneApp}, []Audience{AudiencePublic}, CategoryDependency, 502, "app auth provider resolve failed"),
	public(CodeAppAuthSessionCleanupFailed, "platform.appauth", []Plane{PlaneApp}, []Audience{AudiencePublic}, CategoryInternal, 500, "app session cleanup failed"),
	public(CodeAppAuthSessionIssueFailed, "platform.appauth", []Plane{PlaneApp}, []Audience{AudiencePublic}, CategoryInternal, 500, "app session issue failed"),
	public(CodeAppAuthSessionRevokeFailed, "platform.appauth", []Plane{PlaneApp}, []Audience{AudiencePublic}, CategoryInternal, 500, "app session revoke failed"),
	public(CodeAppAuthTokenSignFailed, "platform.appauth", []Plane{PlaneApp}, []Audience{AudiencePublic}, CategoryInternal, 500, "app auth token sign failed"),
	nonRetryableDependency(CodeAppRouteHandlerNotConfigured, "platform.http", []Plane{PlaneApp}, []Audience{AudiencePublic}, 501, "app route handler is not configured"),
	public(CodeAppPhoneAlreadyBound, "platform.appphone", []Plane{PlaneApp}, []Audience{AudiencePublic}, CategoryConflict, 409, "app phone is already bound"),
	public(CodeAppPhoneAuditFailed, "platform.appphone", []Plane{PlaneApp}, []Audience{AudiencePublic}, CategoryInternal, 500, "app phone audit failed"),
	public(CodeAppPhoneBindingCreateFailed, "platform.appphone", []Plane{PlaneApp}, []Audience{AudiencePublic}, CategoryInternal, 500, "app phone binding create failed"),
	public(CodeAppPhoneCodeGenerationFailed, "platform.appphone", []Plane{PlaneApp}, []Audience{AudiencePublic}, CategoryInternal, 500, "app phone code generation failed"),
	public(CodeAppPhoneCodeRequired, "platform.appphone", []Plane{PlaneApp}, []Audience{AudiencePublic}, CategoryValidation, 400, "app phone verification code is required"),
	public(CodeAppPhoneInvalidPhone, "platform.appphone", []Plane{PlaneApp}, []Audience{AudiencePublic}, CategoryValidation, 400, "invalid phone number"),
	public(CodeAppPhoneInvalidRequest, "platform.appphone", []Plane{PlaneApp}, []Audience{AudiencePublic}, CategoryValidation, 400, "invalid app phone request"),
	public(CodeAppPhonePurposeUnsupported, "platform.appphone", []Plane{PlaneApp}, []Audience{AudiencePublic}, CategoryValidation, 400, "app phone purpose is unsupported"),
	public(CodeAppPhoneVerificationCreateFailed, "platform.appphone", []Plane{PlaneApp}, []Audience{AudiencePublic}, CategoryInternal, 500, "app phone verification create failed"),
	public(CodeAppPhoneVerificationDeliveryFailed, "platform.appphone", []Plane{PlaneApp}, []Audience{AudiencePublic}, CategoryDependency, 502, "app phone verification delivery failed"),
	public(CodeAppPhoneVerificationInvalid, "platform.appphone", []Plane{PlaneApp}, []Audience{AudiencePublic}, CategoryValidation, 400, "app phone verification is invalid"),
	public(CodeAppPhoneVerificationUnavailable, "platform.appphone", []Plane{PlaneApp}, []Audience{AudiencePublic}, CategoryDependency, 503, "app phone verification is unavailable"),
	public(CodeAppPhoneVerificationUpdateFailed, "platform.appphone", []Plane{PlaneApp}, []Audience{AudiencePublic}, CategoryInternal, 500, "app phone verification update failed"),
	public(CodeAppFileSaveFailed, "platform.file", []Plane{PlaneApp}, []Audience{AudiencePublic}, CategoryInternal, 500, "file save failed"),
	public(CodeAppFileRollbackFailed, "platform.file", []Plane{PlaneApp}, []Audience{AudiencePublic}, CategoryInternal, 500, "file upload rollback failed"),
	public(CodeAppFileStateRefreshFailed, "platform.file", []Plane{PlaneApp}, []Audience{AudiencePublic}, CategoryDependency, 503, "file authorization state is unavailable"),
	public(CodeAppFileNotFound, "platform.file", []Plane{PlaneApp}, []Audience{AudiencePublic}, CategoryNotFound, 404, "file not found"),
	public(CodeAppFileObjectNotFound, "platform.file", []Plane{PlaneApp}, []Audience{AudiencePublic}, CategoryNotFound, 404, "file object not found"),
	public(CodeAppFileOpenFailed, "platform.file", []Plane{PlaneApp}, []Audience{AudiencePublic}, CategoryInternal, 500, "file open failed"),
	public(CodeAppFileTooLarge, "platform.file", []Plane{PlaneApp}, []Audience{AudiencePublic}, CategoryValidation, 413, "file exceeds the configured upload limit"),
	public(CodeAppFileRequired, "platform.file", []Plane{PlaneApp}, []Audience{AudiencePublic}, CategoryValidation, 400, "file is required"),
	public(CodeAppFileUploadOpenFailed, "platform.file", []Plane{PlaneApp}, []Audience{AudiencePublic}, CategoryValidation, 400, "open uploaded file failed"),
	public(CodeAppFileReadFailed, "platform.file", []Plane{PlaneApp}, []Audience{AudiencePublic}, CategoryValidation, 400, "read uploaded file failed"),
	public(CodeAppFileMIMEInvalid, "platform.file", []Plane{PlaneApp}, []Audience{AudiencePublic}, CategoryValidation, 415, "uploaded file MIME type is invalid"),
	public(CodeAppFileMIMEMismatch, "platform.file", []Plane{PlaneApp}, []Audience{AudiencePublic}, CategoryValidation, 415, "declared and detected file MIME types do not match"),
	public(CodeAppFileMIMENotAllowed, "platform.file", []Plane{PlaneApp}, []Audience{AudiencePublic}, CategoryValidation, 415, "uploaded file MIME type is not allowed"),
	public(CodeAppForbidden, "platform.http", []Plane{PlaneApp}, []Audience{AudiencePublic}, CategoryAuthorization, 403, "permission denied"),
	public(CodeRequestBodyInvalid, "platform.http", []Plane{PlaneAdmin, PlaneApp}, []Audience{AudienceOperator, AudiencePublic}, CategoryValidation, 400, "request body is invalid"),
	public(CodeRequestBodyTooLarge, "platform.http", []Plane{PlaneAdmin, PlaneApp}, []Audience{AudienceOperator, AudiencePublic}, CategoryValidation, 413, "request body exceeds configured limit"),
	public(CodeRateLimitUnavailable, "platform.http", []Plane{PlaneAdmin, PlaneApp}, []Audience{AudienceOperator, AudiencePublic}, CategoryDependency, 503, "rate limit service is unavailable"),
	rateLimited(CodeRateLimited, "platform.http", []Plane{PlaneAdmin, PlaneApp}, []Audience{AudienceOperator, AudiencePublic}, 429, "request rate limit exceeded"),
	public(CodeFileUploadInvalid, "platform.file", []Plane{PlaneAdmin, PlaneApp}, []Audience{AudienceOperator, AudiencePublic}, CategoryValidation, 400, "invalid file upload"),
	public(CodeServiceObjectUnavailable, "platform.serviceobject", []Plane{PlaneAdmin, PlaneService}, []Audience{AudienceOperator, AudienceInternal}, CategoryNotFound, 404, "service object is unavailable"),
	public(CodeServiceObjectRequestInvalid, "platform.serviceobject", []Plane{PlaneAdmin, PlaneService}, []Audience{AudienceOperator, AudienceInternal}, CategoryValidation, 400, "service object request is invalid"),
	public(CodeServiceObjectCostLimit, "platform.serviceobject", []Plane{PlaneAdmin, PlaneService}, []Audience{AudienceOperator, AudienceInternal}, CategoryRateCost, 422, "service object cost limit exceeded"),
	public(CodeServiceObjectIdempotencyConflict, "platform.serviceobject", []Plane{PlaneAdmin, PlaneService}, []Audience{AudienceOperator, AudienceInternal}, CategoryConflict, 409, "service object idempotency conflict"),
	public(CodeServiceObjectStateConflict, "platform.serviceobject", []Plane{PlaneAdmin, PlaneService}, []Audience{AudienceOperator, AudienceInternal}, CategoryConflict, 409, "service object state conflict"),
	public(CodeServiceObjectDomainValidation, "platform.serviceobject", []Plane{PlaneAdmin, PlaneService}, []Audience{AudienceOperator, AudienceInternal}, CategoryValidation, 422, "service object domain validation failed"),
	public(CodeServiceObjectExecutionFailed, "platform.serviceobject", []Plane{PlaneAdmin, PlaneService}, []Audience{AudienceOperator, AudienceInternal}, CategoryInternal, 500, "service object execution failed"),
	public(CodeInvalidExecutionContext, "platform.kernel", []Plane{PlaneAdmin, PlaneApp, PlaneService, PlaneData, PlaneControl, PlaneExternal}, []Audience{AudienceInternal}, CategoryValidation, 400, "invalid execution context"),
	public(CodeValidation, "platform.kernel", []Plane{PlaneAdmin, PlaneApp, PlaneService, PlaneData, PlaneControl, PlaneExternal}, []Audience{AudienceInternal}, CategoryValidation, 400, "validation failed"),
	public(CodeNotFound, "platform.kernel", []Plane{PlaneAdmin, PlaneApp, PlaneService, PlaneData, PlaneControl, PlaneExternal}, []Audience{AudienceInternal}, CategoryNotFound, 404, "resource not found"),
	public(CodeConflict, "platform.kernel", []Plane{PlaneAdmin, PlaneApp, PlaneService, PlaneData, PlaneControl, PlaneExternal}, []Audience{AudienceInternal}, CategoryConflict, 409, "resource conflict"),
	public(CodeForbidden, "platform.kernel", []Plane{PlaneAdmin, PlaneApp, PlaneService, PlaneData, PlaneControl, PlaneExternal}, []Audience{AudienceInternal}, CategoryAuthorization, 403, "permission denied"),
	internalError(CodeInternal, "platform.kernel", []Plane{PlaneAdmin, PlaneApp, PlaneService, PlaneData, PlaneControl, PlaneExternal}, []Audience{AudienceInternal}, 500, "internal server error"),
}

var registry = mustRegistry(builtinDefinitions)

func public(code Code, owner string, planes []Plane, audiences []Audience, category Category, status int, message string) Definition {
	retry := RetryNever
	redaction := RedactionPublicSafe
	switch category {
	case CategoryDependency:
		retry = RetryBackoff
		redaction = RedactionGenericOnly
	case CategoryInternal:
		redaction = RedactionCorrelationOnly
	}
	return Definition{
		Code: code, Owner: owner, Planes: planes, Audiences: audiences, Category: category,
		HTTPStatus: status, RetryPolicy: retry, RedactionClass: redaction,
		PublicMessage: message, IntroducedIn: "0.1.0",
	}
}

func rateLimited(code Code, owner string, planes []Plane, audiences []Audience, status int, message string) Definition {
	definition := public(code, owner, planes, audiences, CategoryRateCost, status, message)
	definition.RetryPolicy = RetryAfterDelay
	return definition
}

func nonRetryableDependency(code Code, owner string, planes []Plane, audiences []Audience, status int, message string) Definition {
	definition := public(code, owner, planes, audiences, CategoryDependency, status, message)
	definition.RetryPolicy = RetryNever
	return definition
}

func internalError(code Code, owner string, planes []Plane, audiences []Audience, status int, message string) Definition {
	return public(code, owner, planes, audiences, CategoryInternal, status, message)
}

func mustRegistry(definitions []Definition) map[Code]Definition {
	if err := validateDefinitions(definitions); err != nil {
		panic(err)
	}
	result := make(map[Code]Definition, len(definitions))
	for _, definition := range definitions {
		result[definition.Code] = cloneDefinition(definition)
	}
	return result
}
