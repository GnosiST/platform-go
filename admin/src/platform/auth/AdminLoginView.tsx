import {
  CheckOutlined,
  ClockCircleOutlined,
  CloseOutlined,
  CustomerServiceOutlined,
  DownOutlined,
  GlobalOutlined,
  LockOutlined,
  LoadingOutlined,
  LoginOutlined,
  MailOutlined,
  PhoneOutlined,
  ReloadOutlined,
  SafetyOutlined,
  SafetyCertificateOutlined,
  UserOutlined,
} from "@ant-design/icons";
import { Button, Dropdown, Form, Input, Space, Tooltip, Typography, type MenuProps } from "antd";
import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type CSSProperties,
  type KeyboardEvent,
  type PointerEvent as ReactPointerEvent,
  type ReactNode,
} from "react";
import {
  AdminAPIError,
  loginWithAuthProvider,
  startCredentialChallenge,
  startCredentialSMSOTP,
  verifyCredentialChallenge,
  type AdminCurrentSession,
  type AuthProvider,
  type BrandingConfig,
  type CredentialChallengeStartResult,
  type CredentialSMSOTPStartResult,
} from "../api/client";
import type { Dictionary, Language } from "../i18n";
import {
  createSingleUseGuard,
  createSubmissionLock,
  filterAdminAuthProviders,
  hasOIDCCallbackParams,
  OIDCCallbackError,
  type OIDCCallbackFailure,
} from "./oidcPolicy";
import {
  beginOIDCLogin,
  clearPendingOIDCLogin,
  consumePendingOIDCLogin,
} from "../refine/authProvider";
import type { ThemeName } from "../theme";
import { AdminFeedback } from "../ui";
import {
  credentialChallengeSecondsRemaining,
  formatCredentialChallengeCountdown,
} from "./credentialChallengeTiming";
import {
  sliderHandleGrabOffsetFromTrackPointer,
  sliderTileXFromTrackPointer,
  sliderTrackContentRect,
} from "./sliderChallengeGeometry";

type AdminLoginViewProps = {
  language: Language;
  dictionary: Dictionary;
  branding: BrandingConfig | null;
  providers: AuthProvider[];
  loading: boolean;
  error: string;
  search: string;
  themeName: ThemeName;
  onLanguageChange: (language: Language) => void;
  onLoginSuccess: (session: AdminCurrentSession) => void;
};

type LoginFormValues = {
  username: string;
  identifier: string;
  password: string;
  smsCode: string;
  challengeProof: string;
};

type CallbackPhase = "idle" | "processing" | "failed";
type CredentialIdentifierType = "username" | "phone" | "email";
type CredentialProviderMode = "password" | "sms-otp";

type CredentialProviderSpec = {
  identifierType: CredentialIdentifierType;
  mode: CredentialProviderMode;
};

const MAX_VISIBLE_LOGIN_PROVIDERS = 3;

type SliderChallengeMaterial = {
  masterImage: string;
  tileImage: string;
  imageWidth: number;
  imageHeight: number;
  tileWidth: number;
  tileHeight: number;
  initialX: number;
  tileY: number;
};

export function AdminLoginView({
  language,
  dictionary,
  branding,
  providers,
  loading,
  error,
  search,
  themeName,
  onLanguageChange,
  onLoginSuccess,
}: AdminLoginViewProps) {
  const [selectedProviderID, setSelectedProviderID] = useState("demo");
  const [submitting, setSubmitting] = useState(false);
  const [sendingSMSOTP, setSendingSMSOTP] = useState(false);
  const [smsOTPTransaction, setSMSOTPTransaction] = useState<CredentialSMSOTPStartResult | null>(null);
  const [credentialChallenge, setCredentialChallenge] = useState<CredentialChallengeStartResult | null>(null);
  const [credentialChallengeLoading, setCredentialChallengeLoading] = useState(false);
  const [loginError, setLoginError] = useState("");
  const [callbackPhase, setCallbackPhase] = useState<CallbackPhase>("idle");
  const [callbackFailure, setCallbackFailure] = useState<OIDCCallbackFailure | "generic" | null>(null);
  const callbackGuardRef = useRef(createSingleUseGuard());
  const submissionLockRef = useRef(createSubmissionLock());
  const clientFingerprintRef = useRef(createLoginClientFingerprint());
  const credentialChallengeRefreshInFlightRef = useRef(false);
  const credentialChallengeRefreshSequenceRef = useRef(0);
  const loginHeadingRef = useRef<HTMLElement>(null);
  const errorHeadingRef = useRef<HTMLElement>(null);
  const [form] = Form.useForm<LoginFormValues>();
  const adminProviders = useMemo(() => filterAdminAuthProviders(providers), [providers]);
  const availableProviders = useMemo(
    () => adminProviders.filter((provider) => provider.enabled && provider.configured).slice(0, MAX_VISIBLE_LOGIN_PROVIDERS),
    [adminProviders],
  );
  const selectedProvider = useMemo(
    () => availableProviders.find((provider) => provider.id === selectedProviderID) ?? availableProviders[0],
    [availableProviders, selectedProviderID],
  );
  const productName = branding?.productName || "Platform Go";
  const currentLanguageLabel = language === "zh" ? dictionary.cn : dictionary.en;
  const languageMenuItems: MenuProps["items"] = [
    { key: "zh", label: dictionary.cn },
    { key: "en", label: dictionary.en },
  ];
  const credentialSpec = credentialProviderSpec(selectedProvider);

  useEffect(() => {
    if (!availableProviders.length) return;
    if (availableProviders.some((provider) => provider.id === selectedProviderID)) return;
    setSelectedProviderID(availableProviders[0].id);
    setSMSOTPTransaction(null);
    setCredentialChallenge(null);
    form.resetFields();
  }, [availableProviders, form, selectedProviderID]);

  useEffect(() => {
    credentialChallengeRefreshSequenceRef.current += 1;
    credentialChallengeRefreshInFlightRef.current = false;
    if (!credentialSpec) {
      setCredentialChallengeLoading(false);
      setCredentialChallenge(null);
      form.setFieldValue("challengeProof", "");
      return;
    }
    void refreshCredentialChallenge();
    return () => {
      credentialChallengeRefreshSequenceRef.current += 1;
      credentialChallengeRefreshInFlightRef.current = false;
    };
  }, [credentialSpec?.identifierType, credentialSpec?.mode, selectedProvider?.id]);

  useEffect(() => {
    if (callbackPhase !== "idle" || !hasOIDCCallbackParams(search)) return;
    if (!callbackGuardRef.current.acquire()) return;

    setCallbackPhase("processing");
    setSubmitting(true);
    setLoginError("");
    setCallbackFailure(null);
    void consumePendingOIDCLogin(search)
      .then((result) => {
        if (result) onLoginSuccess(result.principal);
      })
      .catch((nextError: unknown) => {
        setCallbackFailure(callbackFailureReason(nextError));
        setCallbackPhase("failed");
      })
      .finally(() => setSubmitting(false));
  }, [callbackPhase, onLoginSuccess, search]);

  useEffect(() => {
    if (callbackPhase === "failed") {
      errorHeadingRef.current?.focus({ preventScroll: true });
    }
  }, [callbackPhase]);

  const submit = async (values: LoginFormValues) => {
    if (!submissionLockRef.current.acquire()) return;
    if (!selectedProvider?.configured) {
      submissionLockRef.current.release();
      setLoginError(dictionary.loginProviderUnavailable);
      return;
    }
    setSubmitting(true);
    setLoginError("");
    try {
      let result;
      const challenge = credentialChallengeRequest(credentialChallenge, values.challengeProof, clientFingerprintRef.current);
      if (credentialSpec && credentialChallenge && credentialChallengeExpired(credentialChallenge)) {
        throw new Error(dictionary.loginChallengeExpired);
      }
      if (credentialSpec && !challenge) throw new Error(dictionary.loginChallengeRequired);
      if (selectedProvider.kind === "demo") {
        result = await loginWithAuthProvider({
          provider: selectedProvider.id,
          username: values.username,
        });
      } else if (credentialSpec?.mode === "password") {
        result = await loginWithAuthProvider({
          provider: selectedProvider.id,
          identifier: { type: credentialSpec.identifierType, value: values.identifier },
          secret: { type: "password", value: values.password },
          challenge,
        });
      } else if (credentialSpec?.mode === "sms-otp") {
        if (!smsOTPTransaction?.transactionId) throw new Error(dictionary.loginSMSCodeRequired);
        result = await loginWithAuthProvider({
          provider: selectedProvider.id,
          identifier: { type: "phone", value: values.identifier },
          secret: { type: "sms-otp", transactionId: smsOTPTransaction.transactionId, code: values.smsCode },
          challenge,
        });
      } else {
        throw new Error(dictionary.loginProviderUnavailable);
      }
      onLoginSuccess(result.principal);
    } catch (nextError) {
      setLoginError(loginAPIErrorMessage(nextError, dictionary, dictionary.loginFailed));
      if (credentialSpec) void refreshCredentialChallenge();
    } finally {
      submissionLockRef.current.release();
      setSubmitting(false);
    }
  };

  const sendSMSOTP = async () => {
    if (!selectedProvider?.configured || credentialSpec?.mode !== "sms-otp") {
      setLoginError(dictionary.loginProviderUnavailable);
      return;
    }
    try {
      const values = await form.validateFields(["identifier"]);
      const challenge = credentialChallengeRequest(credentialChallenge, form.getFieldValue("challengeProof"), clientFingerprintRef.current);
      if (credentialChallenge && credentialChallengeExpired(credentialChallenge)) {
        throw new Error(dictionary.loginChallengeExpired);
      }
      if (!challenge) throw new Error(dictionary.loginChallengeRequired);
      setSendingSMSOTP(true);
      setLoginError("");
      const transaction = await startCredentialSMSOTP({
        provider: selectedProvider.id,
        identifier: { type: "phone", value: values.identifier },
        challenge,
      });
      setSMSOTPTransaction(transaction);
      void refreshCredentialChallenge();
    } catch (nextError) {
      setLoginError(loginAPIErrorMessage(nextError, dictionary, dictionary.loginSMSStartFailed));
      if (credentialSpec?.mode === "sms-otp") void refreshCredentialChallenge();
    } finally {
      setSendingSMSOTP(false);
    }
  };

  const refreshCredentialChallenge = async () => {
    if (credentialChallengeRefreshInFlightRef.current) return;
    credentialChallengeRefreshInFlightRef.current = true;
    const refreshSequence = credentialChallengeRefreshSequenceRef.current + 1;
    credentialChallengeRefreshSequenceRef.current = refreshSequence;
    setCredentialChallengeLoading(true);
    const fallback = credentialChallenge ? dictionary.loginChallengeRefreshFailed : dictionary.loginChallengeStartFailed;
    try {
      const nextChallenge = await startLoginCredentialChallenge(clientFingerprintRef.current);
      if (refreshSequence !== credentialChallengeRefreshSequenceRef.current) return;
      setCredentialChallenge(nextChallenge);
      form.setFieldValue("challengeProof", "");
    } catch (nextError) {
      if (refreshSequence !== credentialChallengeRefreshSequenceRef.current) return;
      setCredentialChallenge(null);
      setLoginError(loginAPIErrorMessage(nextError, dictionary, fallback));
    } finally {
      if (refreshSequence === credentialChallengeRefreshSequenceRef.current) {
        credentialChallengeRefreshInFlightRef.current = false;
        setCredentialChallengeLoading(false);
      }
    }
  };

  const verifySliderChallengeProof = useCallback(async (challengeID: string, proof: string) => {
    const result = await verifyCredentialChallenge({
      id: challengeID,
      kind: "slider",
      proof,
      clientFingerprint: clientFingerprintRef.current,
    });
    return result.valid;
  }, []);

  const startOIDC = async () => {
    if (!submissionLockRef.current.acquire()) return;
    if (!selectedProvider?.configured || selectedProvider.kind !== "oidc") {
      submissionLockRef.current.release();
      setLoginError(dictionary.loginProviderUnavailable);
      return;
    }
    setSubmitting(true);
    setLoginError("");
    try {
      await beginOIDCLogin(selectedProvider);
    } catch {
      clearPendingOIDCLogin();
      submissionLockRef.current.release();
      setLoginError(dictionary.loginOIDCStartFailed);
      setSubmitting(false);
    }
  };

  const recoverFromCallback = () => {
    clearPendingOIDCLogin();
    setCallbackPhase("idle");
    setCallbackFailure(null);
    setLoginError("");
    setSubmitting(false);
    loginHeadingRef.current?.focus({ preventScroll: true });
  };

  const selectProvider = (providerID: string) => {
    if (providerID === selectedProviderID || submitting) return;
    const provider = availableProviders.find((item) => item.id === providerID);
    clearPendingOIDCLogin();
    setLoginError("");
    setSMSOTPTransaction(null);
    setCredentialChallenge(null);
    form.resetFields();
    if (provider?.kind === "demo") {
      form.setFieldValue("username", "admin");
    }
    setSelectedProviderID(providerID);
  };

  return (
    <main className="login-page" data-theme={themeName}>
      <aside className="login-aside">
        <div className="login-aside-brand">
          <div className="login-logo">{branding?.logoUrl ? <img alt="" src={branding.logoUrl} /> : <SafetyCertificateOutlined />}</div>
          <div>
            <Typography.Text className="login-brand-name">{productName}</Typography.Text>
            <Typography.Text className="login-brand-subtitle">{dictionary.loginAsideSubtitle}</Typography.Text>
          </div>
        </div>

        <div className="login-aside-divider" />

        <div className="login-assurance-list" aria-label={dictionary.loginCapabilitySummary}>
          <div className="login-assurance-item">
            <SafetyOutlined aria-hidden />
            <div>
              <Typography.Text strong>{dictionary.loginTrustSecurity}</Typography.Text>
              <Typography.Text type="secondary">{dictionary.loginTrustSecurityDescription}</Typography.Text>
            </div>
          </div>
          <div className="login-assurance-item">
            <ClockCircleOutlined aria-hidden />
            <div>
              <Typography.Text strong>{dictionary.loginTrustReliability}</Typography.Text>
              <Typography.Text type="secondary">{dictionary.loginTrustReliabilityDescription}</Typography.Text>
            </div>
          </div>
          <div className="login-assurance-item">
            <CustomerServiceOutlined aria-hidden />
            <div>
              <Typography.Text strong>{dictionary.loginTrustSupport}</Typography.Text>
              <Typography.Text type="secondary">{dictionary.loginTrustSupportDescription}</Typography.Text>
            </div>
          </div>
        </div>

        <div className="login-aside-footer">
          <Dropdown
            menu={{
              items: languageMenuItems,
              selectable: true,
              selectedKeys: [language],
              onClick: ({ key }) => onLanguageChange(key as Language),
            }}
            overlayClassName="login-language-menu"
            placement="topLeft"
            trigger={["click"]}
          >
            <Button
              aria-label={`${dictionary.language}: ${currentLanguageLabel}`}
              className="login-language-trigger"
              icon={<GlobalOutlined aria-hidden />}
              type="text"
            >
              <span className="login-language-label">{currentLanguageLabel}</span>
              <span aria-hidden className="login-language-label-compact">{dictionary.languageCompact}</span>
              <DownOutlined aria-hidden className="login-language-chevron" />
            </Button>
          </Dropdown>
          <div className="login-runtime-status">
            <SafetyCertificateOutlined aria-hidden />
            <span>{dictionary.loginSystemStatus}</span>
            <span className="login-runtime-status-dot" />
            <strong>{dictionary.loginStatusNormal}</strong>
          </div>
        </div>
      </aside>

      <section className="login-panel" aria-label={dictionary.loginTitle}>
        <Typography.Title className="login-heading" level={2} ref={loginHeadingRef} tabIndex={-1}>
          {dictionary.loginWorkspaceTitle}
        </Typography.Title>
        <Typography.Paragraph className="login-panel-subtitle">{dictionary.loginWorkspaceSubtitle}</Typography.Paragraph>
        {callbackPhase === "idle" ? (
          <>
            {loginError ? <AdminFeedback className="login-feedback" type="error" message={loginError} /> : null}
          {availableProviders.length ? (
            <div
              className="login-provider-options"
              role="listbox"
              style={{ gridTemplateColumns: `repeat(${availableProviders.length}, minmax(0, 1fr))` }}
            >
              {availableProviders.map((provider) => {
                const providerSpec = credentialProviderSpec(provider);
                const selected = selectedProvider?.id === provider.id;
                return (
                  <button
                    aria-selected={selected}
                    className={`login-provider-option${selected ? " selected" : ""}`}
                    disabled={submitting}
                    key={provider.id}
                    role="option"
                    type="button"
                    onClick={() => selectProvider(provider.id)}
                  >
                    <span aria-hidden className="login-provider-option-icon">{loginProviderIcon(provider, providerSpec)}</span>
                    <span className="login-provider-option-copy">
                      <span className="login-provider-option-title">{provider.title[language] || provider.id}</span>
                      <span className="login-provider-option-description">{loginProviderDescription(dictionary, provider, providerSpec)}</span>
                    </span>
                  </button>
                );
              })}
            </div>
          ) : (
            <AdminFeedback
              className="login-feedback"
              type={loading ? "info" : "error"}
              message={loading ? dictionary.loginProvidersLoading : dictionary.loginNoAvailableProviders}
            />
          )}
          {error ? <AdminFeedback className="login-feedback" type="error" message={error} /> : null}

            <Form<LoginFormValues>
              className="login-form-shell"
              form={form}
              layout="vertical"
              initialValues={{ username: "admin" }}
              requiredMark={false}
              onFinish={submit}
            >
              {selectedProvider && selectedProvider.kind === "demo" ? (
                <>
                  <Form.Item
                    label={dictionary.loginUsername}
                    name="username"
                    rules={[{ required: true, message: dictionary.loginUsernameRequired }]}
                  >
                    <Input prefix={<UserOutlined />} autoComplete="username" placeholder={dictionary.loginUsernamePlaceholder} />
                  </Form.Item>
                  <Button
                    block
                    className="login-submit"
                    htmlType="submit"
                    icon={<LoginOutlined />}
                    loading={submitting}
                    type="primary"
                    disabled={!selectedProvider.configured || loading || submitting}
                  >
                    {dictionary.login}
                  </Button>
                </>
              ) : null}

              {selectedProvider && credentialSpec?.mode === "password" ? (
                <>
                  <Form.Item
                    label={credentialIdentifierLabel(dictionary, credentialSpec.identifierType)}
                    name="identifier"
                    rules={[{ required: true, message: credentialIdentifierRequired(dictionary, credentialSpec.identifierType) }]}
                  >
                    <Input prefix={credentialIdentifierIcon(credentialSpec.identifierType)} autoComplete={credentialIdentifierAutocomplete(credentialSpec.identifierType)} placeholder={credentialIdentifierPlaceholder(dictionary, credentialSpec.identifierType)} />
                  </Form.Item>
                  <Form.Item
                    label={dictionary.loginPassword}
                    name="password"
                    rules={[{ required: true, message: dictionary.loginPasswordRequired }]}
                  >
                    <Input.Password prefix={<LockOutlined />} autoComplete="current-password" placeholder={dictionary.loginPasswordCredentialPlaceholder} />
                  </Form.Item>
                  <CredentialChallengeField
                    challenge={credentialChallenge}
                    dictionary={dictionary}
                    loading={credentialChallengeLoading}
                    onRefresh={() => void refreshCredentialChallenge()}
                    onVerifyProof={verifySliderChallengeProof}
                  />
                  <Button
                    block
                    className="login-submit"
                    htmlType="submit"
                    icon={<LoginOutlined />}
                    loading={submitting}
                    type="primary"
                    disabled={!selectedProvider.configured || loading || submitting || !credentialChallenge}
                  >
                    {dictionary.login}
                  </Button>
                </>
              ) : null}

              {selectedProvider && credentialSpec?.mode === "sms-otp" ? (
                <>
                  <Form.Item
                    label={dictionary.loginPhone}
                    name="identifier"
                    rules={[{ required: true, message: dictionary.loginPhoneRequired }]}
                  >
                    <Input prefix={<PhoneOutlined />} autoComplete="tel" placeholder={dictionary.loginPhonePlaceholder} />
                  </Form.Item>
                  <Form.Item label={dictionary.loginSMSCode} required>
                    <Space.Compact className="login-sms-code-row">
                      <Form.Item
                        noStyle
                        name="smsCode"
                        rules={[{ required: true, message: dictionary.loginSMSCodeRequired }]}
                      >
                        <Input inputMode="numeric" autoComplete="one-time-code" placeholder={dictionary.loginSMSCodePlaceholder} />
                      </Form.Item>
                      <Button loading={sendingSMSOTP} disabled={loading || submitting || sendingSMSOTP} onClick={() => void sendSMSOTP()}>
                        {sendingSMSOTP ? dictionary.loginSMSSending : dictionary.loginSMSSendCode}
                      </Button>
                    </Space.Compact>
                  </Form.Item>
                  {smsOTPTransaction ? (
                    <Typography.Text className="login-sms-status" type="secondary">
                      {dictionary.loginSMSSentTo.replace("{destination}", smsOTPTransaction.maskedIdentifier)}
                    </Typography.Text>
                  ) : null}
                  <CredentialChallengeField
                    challenge={credentialChallenge}
                    dictionary={dictionary}
                    loading={credentialChallengeLoading}
                    onRefresh={() => void refreshCredentialChallenge()}
                    onVerifyProof={verifySliderChallengeProof}
                  />
                  <Button
                    block
                    className="login-submit"
                    htmlType="submit"
                    icon={<LoginOutlined />}
                    loading={submitting}
                    type="primary"
                    disabled={!selectedProvider.configured || loading || submitting || !smsOTPTransaction || !credentialChallenge}
                  >
                    {dictionary.login}
                  </Button>
                </>
              ) : null}
            </Form>

            {selectedProvider && selectedProvider.kind === "oidc" ? (
              <Button
                block
                aria-label={dictionary.loginOIDCContinue.replace("{provider}", selectedProvider.title[language] || selectedProvider.id)}
                className="login-oidc-action"
                icon={<LoginOutlined />}
                loading={submitting}
                type="primary"
                disabled={!selectedProvider.configured || loading || submitting}
                onClick={() => void startOIDC()}
              >
                {submitting
                  ? dictionary.loginOIDCStarting
                  : dictionary.loginOIDCContinue.replace("{provider}", selectedProvider.title[language] || selectedProvider.id)}
              </Button>
            ) : null}
          </>
        ) : (
          <div className="login-callback-status" aria-busy={callbackPhase === "processing"} aria-live="polite">
            {callbackPhase === "processing" ? (
              <>
                <LoadingOutlined aria-hidden className="login-callback-icon" />
                <Typography.Title level={3}>{dictionary.loginOIDCCallbackProgress}</Typography.Title>
              </>
            ) : (
              <>
                <Typography.Title className="login-error-heading" level={3} ref={errorHeadingRef} tabIndex={-1}>
                  {dictionary.loginFailed}
                </Typography.Title>
                <Typography.Paragraph>{callbackErrorMessage(dictionary, callbackFailure)}</Typography.Paragraph>
                <Button block className="login-recovery-action" onClick={recoverFromCallback}>
                  {dictionary.loginOIDCRecovery}
                </Button>
              </>
            )}
          </div>
        )}
      </section>
    </main>
  );
}

function loginProviderIcon(provider: AuthProvider, spec: CredentialProviderSpec | null): ReactNode {
  if (provider.kind === "demo") return <UserOutlined />;
  if (provider.kind === "oidc") return <SafetyCertificateOutlined />;
  if (spec?.mode === "sms-otp") return <PhoneOutlined />;
  if (spec?.identifierType === "phone") return <PhoneOutlined />;
  if (spec?.identifierType === "email") return <MailOutlined />;
  if (spec?.identifierType === "username") return <LockOutlined />;
  return <SafetyCertificateOutlined />;
}

function loginProviderDescription(dictionary: Dictionary, provider: AuthProvider, spec: CredentialProviderSpec | null) {
  if (provider.kind === "demo") return dictionary.loginProviderDemoDescription;
  if (provider.kind === "oidc") return dictionary.loginProviderOIDCDescription;
  if (spec?.mode === "sms-otp") return dictionary.loginProviderSMSOTPDescription;
  if (spec?.identifierType === "phone") return dictionary.loginProviderPhonePasswordDescription;
  if (spec?.identifierType === "email") return dictionary.loginProviderEmailPasswordDescription;
  if (spec?.identifierType === "username") return dictionary.loginProviderUsernamePasswordDescription;
  return dictionary.loginProviderFallbackDescription;
}

function CredentialChallengeField({
  challenge,
  dictionary,
  loading,
  onRefresh,
  onVerifyProof,
}: {
  challenge: CredentialChallengeStartResult | null;
  dictionary: Dictionary;
  loading: boolean;
  onRefresh: () => void;
  onVerifyProof: (challengeID: string, proof: string) => Promise<boolean>;
}) {
  const form = Form.useFormInstance<LoginFormValues>();
  const sliderMaterial = sliderChallengeMaterial(challenge);
  const secondsRemaining = useCredentialChallengeCountdown(challenge?.expiresAt);
  const expired = challenge ? secondsRemaining === 0 : false;
  const setChallengeProof = useCallback(
    (proof: string) => {
      form.setFieldValue("challengeProof", proof);
    },
    [form],
  );

  return (
    <Form.Item label={dictionary.loginChallenge} required>
      <div className="login-challenge-card">
        <div className="login-challenge-meta">
          {expired ? (
            <Typography.Text type="danger">{dictionary.loginChallengeExpired}</Typography.Text>
          ) : challenge && secondsRemaining !== null ? (
            <span className={`login-challenge-countdown${secondsRemaining <= 10 ? " warning" : ""}`}>
              <ClockCircleOutlined aria-hidden />
              <span>
                {dictionary.loginChallengeExpiresIn.replace(
                  "{time}",
                  formatCredentialChallengeCountdown(secondsRemaining),
                )}
              </span>
            </span>
          ) : challenge ? (
            <Typography.Text type="secondary">
              {dictionary.loginChallengeExpiresAt.replace("{time}", formatChallengeTime(challenge.expiresAt))}
            </Typography.Text>
          ) : <span />}
          <Tooltip title={dictionary.loginChallengeRefresh}>
            <Button
              aria-label={dictionary.loginChallengeRefresh}
              className="login-challenge-refresh"
              icon={<ReloadOutlined />}
              loading={loading}
              onClick={onRefresh}
            >
              {dictionary.loginChallengeRefresh}
            </Button>
          </Tooltip>
        </div>
        <CredentialChallengePayload
          challenge={challenge}
          dictionary={dictionary}
          expired={expired}
          loading={loading}
          onProofChange={setChallengeProof}
          onRefresh={onRefresh}
          onVerifyProof={onVerifyProof}
        />
        {sliderMaterial ? (
          <Form.Item
            hidden
            name="challengeProof"
            rules={[{ required: true, message: dictionary.loginChallengeSliderRequired }]}
          >
            <Input />
          </Form.Item>
        ) : challenge ? (
          <div className="login-challenge-row">
            <Form.Item noStyle name="challengeProof" rules={[{ required: true, message: dictionary.loginChallengeRequired }]}>
              <Input
                prefix={<SafetyCertificateOutlined />}
                autoComplete="off"
                disabled={!challenge || loading}
                placeholder={dictionary.loginChallengePlaceholder}
              />
            </Form.Item>
          </div>
        ) : null}
      </div>
    </Form.Item>
  );
}

function CredentialChallengePayload({
  challenge,
  dictionary,
  expired,
  loading,
  onProofChange,
  onRefresh,
  onVerifyProof,
}: {
  challenge: CredentialChallengeStartResult | null;
  dictionary: Dictionary;
  expired: boolean;
  loading: boolean;
  onProofChange: (proof: string) => void;
  onRefresh: () => void;
  onVerifyProof: (challengeID: string, proof: string) => Promise<boolean>;
}) {
  if (!challenge) {
    return (
      <div className="login-challenge-prompt">
        <Typography.Text strong>{dictionary.loginChallengeLoading}</Typography.Text>
      </div>
    );
  }
  const sliderMaterial = sliderChallengeMaterial(challenge);
  if (sliderMaterial) {
    return (
      <SliderChallengePayload
        challenge={challenge}
        dictionary={dictionary}
        expired={expired}
        loading={loading}
        material={sliderMaterial}
        onProofChange={onProofChange}
        onRefresh={onRefresh}
        onVerifyProof={(proof) => onVerifyProof(challenge.id, proof)}
      />
    );
  }
  if (challenge.kind === "captcha") {
    return <CaptchaChallengePayload challenge={challenge} dictionary={dictionary} />;
  }
  return <ProviderSpecificChallengePayload challenge={challenge} dictionary={dictionary} />;
}

function SliderChallengePayload({
  challenge,
  dictionary,
  expired,
  loading,
  material,
  onProofChange,
  onRefresh,
  onVerifyProof,
}: {
  challenge: CredentialChallengeStartResult;
  dictionary: Dictionary;
  expired: boolean;
  loading: boolean;
  material: SliderChallengeMaterial;
  onProofChange: (proof: string) => void;
  onRefresh: () => void;
  onVerifyProof: (proof: string) => Promise<boolean>;
}) {
  const trackRef = useRef<HTMLDivElement>(null);
  const activePointerRef = useRef<number | null>(null);
  const pointerGrabOffsetRef = useRef(0);
  const verificationSequenceRef = useRef(0);
  const recoveryTimerRef = useRef<number | null>(null);
  const maxX = Math.max(0, material.imageWidth - material.tileWidth);
  const initialX = clampNumber(material.initialX, 0, maxX);
  const [tileX, setTileX] = useState(() => initialX);
  const [proofReady, setProofReady] = useState(false);
  const [proofRejected, setProofRejected] = useState(false);
  const [verifying, setVerifying] = useState(false);
  const [verificationError, setVerificationError] = useState("");
  const [dragging, setDragging] = useState(false);
  const interactionLocked = loading || expired || verifying || proofReady || proofRejected;
  const boundedTileX = clampNumber(tileX, 0, maxX);
  const handlePercent = ((boundedTileX + material.tileWidth / 2) / material.imageWidth) * 100;
  const stageStyle: CSSProperties = {
    aspectRatio: `${material.imageWidth} / ${material.imageHeight}`,
  };
  const tileStyle: CSSProperties = {
    height: `${(material.tileHeight / material.imageHeight) * 100}%`,
    left: `${(boundedTileX / material.imageWidth) * 100}%`,
    top: `${(material.tileY / material.imageHeight) * 100}%`,
    width: `${(material.tileWidth / material.imageWidth) * 100}%`,
  };
  const trackFillStyle: CSSProperties = {
    width: `${handlePercent}%`,
  };
  const trackStyle: CSSProperties = {};
  const handleStyle: CSSProperties = {
    left: `${handlePercent}%`,
  };

  const resetSliderProof = useCallback((rejected = false) => {
    if (recoveryTimerRef.current !== null) {
      window.clearTimeout(recoveryTimerRef.current);
      recoveryTimerRef.current = null;
    }
    verificationSequenceRef.current += 1;
    activePointerRef.current = null;
    pointerGrabOffsetRef.current = 0;
    setDragging(false);
    setVerifying(false);
    setProofReady(false);
    setProofRejected(rejected);
    setVerificationError("");
    setTileX(initialX);
    onProofChange("");
  }, [initialX, onProofChange]);

  useEffect(() => {
    resetSliderProof(false);
  }, [challenge.id, resetSliderProof]);

  useEffect(() => {
    if (expired) resetSliderProof(false);
  }, [expired, resetSliderProof]);

  useEffect(() => {
    if (!loading || recoveryTimerRef.current === null) return;
    window.clearTimeout(recoveryTimerRef.current);
    recoveryTimerRef.current = null;
    setProofRejected(false);
  }, [loading]);

  useEffect(() => () => {
    verificationSequenceRef.current += 1;
    if (recoveryTimerRef.current !== null) window.clearTimeout(recoveryTimerRef.current);
  }, []);

  const rejectSliderProof = useCallback(() => {
    resetSliderProof(true);
    recoveryTimerRef.current = window.setTimeout(() => {
      recoveryTimerRef.current = null;
      setProofRejected(false);
      onRefresh();
    }, 700);
  }, [onRefresh, resetSliderProof]);

  const updateDraftFromPointer = (event: ReactPointerEvent<HTMLDivElement>) => {
    const nextX = sliderTileXFromTrackPointer(
      event.clientX,
      sliderTrackContentRect(trackRef.current),
      material,
      maxX,
      pointerGrabOffsetRef.current,
    );
    setTileX(nextX);
    setProofReady(false);
    setProofRejected(false);
    setVerificationError("");
    onProofChange("");
    return nextX;
  };

  const commitProof = async (nextX: number) => {
    const proofX = Math.round(clampNumber(nextX, 0, maxX));
    const proofY = Math.round(material.tileY);
    const proof = formatSliderChallengeProof(proofX, proofY);
    const verificationSequence = verificationSequenceRef.current + 1;
    verificationSequenceRef.current = verificationSequence;
    setTileX(proofX);
    setProofReady(false);
    setProofRejected(false);
    setVerifying(true);
    onProofChange("");
    try {
      const valid = await onVerifyProof(proof);
      if (verificationSequence !== verificationSequenceRef.current) return;
      if (credentialChallengeSecondsRemaining(challenge.expiresAt) === 0) {
        resetSliderProof(false);
        onRefresh();
        return;
      }
      if (!valid) {
        rejectSliderProof();
        return;
      }
      setVerifying(false);
      setProofReady(true);
      onProofChange(proof);
    } catch (nextError) {
      if (verificationSequence !== verificationSequenceRef.current) return;
      if (sliderChallengeVerificationRejected(nextError)) {
        rejectSliderProof();
        return;
      }
      resetSliderProof(false);
      setVerificationError(sliderChallengeVerificationErrorMessage(nextError, dictionary));
    }
  };

  const handlePointerDown = (event: ReactPointerEvent<HTMLDivElement>) => {
    if (event.button !== 0 || interactionLocked) return;
    event.preventDefault();
    const track = sliderTrackContentRect(trackRef.current);
    const startedOnHandle = event.target instanceof Element && event.target.closest(".login-slider-handle") !== null;
    pointerGrabOffsetRef.current = startedOnHandle
      ? sliderHandleGrabOffsetFromTrackPointer(event.clientX, track, material, boundedTileX)
      : 0;
    activePointerRef.current = event.pointerId;
    event.currentTarget.setPointerCapture(event.pointerId);
    setDragging(true);
    updateDraftFromPointer(event);
  };

  const handlePointerMove = (event: ReactPointerEvent<HTMLDivElement>) => {
    if (activePointerRef.current !== event.pointerId) return;
    event.preventDefault();
    updateDraftFromPointer(event);
  };

  const handlePointerUp = (event: ReactPointerEvent<HTMLDivElement>) => {
    if (activePointerRef.current !== event.pointerId) return;
    event.preventDefault();
    const nextX = updateDraftFromPointer(event);
    activePointerRef.current = null;
    pointerGrabOffsetRef.current = 0;
    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId);
    }
    setDragging(false);
    void commitProof(nextX);
  };

  const handlePointerCancel = (event: ReactPointerEvent<HTMLDivElement>) => {
    if (activePointerRef.current !== event.pointerId) return;
    activePointerRef.current = null;
    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId);
    }
    resetSliderProof(false);
  };

  const handleTrackKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (interactionLocked) return;
    const smallStep = event.shiftKey ? 10 : 1;
    const largeStep = event.shiftKey ? 24 : 12;
    let nextX: number | null = null;
    if (event.key === "ArrowLeft") nextX = boundedTileX - smallStep;
    if (event.key === "ArrowRight") nextX = boundedTileX + smallStep;
    if (event.key === "PageDown") nextX = boundedTileX - largeStep;
    if (event.key === "PageUp") nextX = boundedTileX + largeStep;
    if (event.key === "Home") nextX = 0;
    if (event.key === "End") nextX = maxX;
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      void commitProof(boundedTileX);
      return;
    }
    if (nextX === null) return;
    event.preventDefault();
    setTileX(clampNumber(nextX, 0, maxX));
    setProofReady(false);
    setProofRejected(false);
    setVerificationError("");
    onProofChange("");
  };

  const trackLabel = expired
    ? dictionary.loginChallengeSliderExpired
    : verifying
      ? dictionary.loginChallengeSliderChecking
      : proofReady
        ? dictionary.loginChallengeSliderComplete
        : proofRejected
          ? dictionary.loginChallengeSliderRejected
          : verificationError || dictionary.loginChallengeSliderInstruction;
  const sliderStateClass = `${proofReady ? " ready" : ""}${proofRejected ? " rejected" : ""}${expired ? " expired" : ""}${verificationError ? " unavailable" : ""}`;

  return (
    <div className={`login-challenge-prompt login-slider-challenge${sliderStateClass}`}>
      <div className="login-challenge-payload">
        <Typography.Text strong>{dictionary.loginChallengePrompt}</Typography.Text>
        <div className="login-slider-stage" style={stageStyle}>
          <img alt={dictionary.loginChallengeSliderImageAlt} className="login-slider-background" src={material.masterImage} />
          <img alt={dictionary.loginChallengeSliderTileAlt} className="login-slider-tile" src={material.tileImage} style={tileStyle} />
        </div>
        <div
          aria-label={dictionary.loginChallengeSliderTrackLabel}
          aria-busy={verifying}
          aria-disabled={interactionLocked}
          aria-valuemax={Math.round(maxX)}
          aria-valuemin={0}
          aria-valuenow={Math.round(boundedTileX)}
          aria-valuetext={trackLabel}
          className={`login-slider-track${dragging ? " dragging" : ""}${loading ? " disabled" : ""}${verifying ? " verifying" : ""}${sliderStateClass}`}
          ref={trackRef}
          role="slider"
          style={trackStyle}
          tabIndex={0}
          onKeyDown={handleTrackKeyDown}
          onPointerCancel={handlePointerCancel}
          onPointerDown={handlePointerDown}
          onPointerMove={handlePointerMove}
          onPointerUp={handlePointerUp}
        >
          <span aria-hidden className="login-slider-track-fill" style={trackFillStyle} />
          <span aria-hidden className="login-slider-handle" style={handleStyle}>
            {verifying ? (
              <LoadingOutlined spin />
            ) : proofReady ? (
              <CheckOutlined />
            ) : proofRejected ? (
              <CloseOutlined />
            ) : expired ? (
              <ClockCircleOutlined />
            ) : (
              <LoginOutlined />
            )}
          </span>
          <span aria-live="polite" className="login-slider-track-label">
            {trackLabel}
          </span>
        </div>
      </div>
    </div>
  );
}

function CaptchaChallengePayload({
  challenge,
  dictionary,
}: {
  challenge: CredentialChallengeStartResult;
  dictionary: Dictionary;
}) {
  const imageSource =
    challengeStringParameter(challenge, "imageUrl") ||
    challengeStringParameter(challenge, "imageData") ||
    challengeStringParameter(challenge, "image");
  const displayText = challengeStringParameter(challenge, "displayText") || challengeStringParameter(challenge, "display");
  return (
    <div className="login-challenge-prompt">
      <div className="login-challenge-payload">
        <Typography.Text strong>{challenge.prompt || dictionary.loginChallengePrompt}</Typography.Text>
        {imageSource ? <img alt={dictionary.loginChallengeImageAlt} className="login-challenge-image" src={imageSource} /> : null}
        {!imageSource && displayText ? <Typography.Text type="secondary">{displayText}</Typography.Text> : null}
      </div>
    </div>
  );
}

function ProviderSpecificChallengePayload({
  challenge,
  dictionary,
}: {
  challenge: CredentialChallengeStartResult;
  dictionary: Dictionary;
}) {
  const provider = challengeStringParameter(challenge, "provider");
  const component = challengeStringParameter(challenge, "component");
  const displayText = challengeStringParameter(challenge, "displayText") || challengeStringParameter(challenge, "display");
  return (
    <div className="login-challenge-prompt">
      <div className="login-challenge-payload">
        <Typography.Text strong>{challenge.prompt || dictionary.loginChallengeProviderPrompt.replace("{kind}", challenge.kind)}</Typography.Text>
        {provider || component ? (
          <Typography.Text type="secondary">
            {dictionary.loginChallengeProviderPayload
              .replace("{provider}", provider || challenge.kind)
              .replace("{component}", component || challenge.kind)}
          </Typography.Text>
        ) : null}
        {displayText ? <Typography.Text type="secondary">{displayText}</Typography.Text> : null}
      </div>
    </div>
  );
}

function useCredentialChallengeCountdown(expiresAt: string | undefined) {
  const [now, setNow] = useState(() => Date.now());

  useEffect(() => {
    const deadline = Date.parse(expiresAt ?? "");
    if (!Number.isFinite(deadline)) {
      setNow(Date.now());
      return;
    }
    let timer: number | null = null;
    const tick = () => {
      const current = Date.now();
      setNow(current);
      const remaining = deadline - current;
      if (remaining <= 0) return;
      const nextBoundary = remaining % 1000 || 1000;
      timer = window.setTimeout(tick, Math.max(50, nextBoundary));
    };
    tick();
    return () => {
      if (timer !== null) window.clearTimeout(timer);
    };
  }, [expiresAt]);

  return credentialChallengeSecondsRemaining(expiresAt, now);
}

async function startLoginCredentialChallenge(clientFingerprint: string) {
  let sliderError: unknown;
  try {
    const sliderChallenge = await startCredentialChallenge({ kind: "slider", purpose: "login", clientFingerprint });
    if (!challengeNeedsCaptchaFallback(sliderChallenge)) {
      return sliderChallenge;
    }
  } catch (nextError) {
    sliderError = nextError;
  }
  try {
    return await startCredentialChallenge({ kind: "captcha", purpose: "login", clientFingerprint });
  } catch (captchaError) {
    throw captchaError instanceof Error ? captchaError : sliderError ?? captchaError;
  }
}

function credentialChallengeRequest(challenge: CredentialChallengeStartResult | null, proof: string | undefined, clientFingerprint: string) {
  const normalizedProof = String(proof ?? "").trim();
  if (!challenge || normalizedProof === "") return undefined;
  return {
    id: challenge.id,
    kind: challenge.kind,
    proof: normalizedProof,
    clientFingerprint,
  };
}

function credentialChallengeExpired(challenge: CredentialChallengeStartResult) {
  return credentialChallengeSecondsRemaining(challenge.expiresAt) === 0;
}

function formatChallengeTime(value: string) {
  const timestamp = Date.parse(value);
  if (!Number.isFinite(timestamp)) return value;
  return new Date(timestamp).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
}

function loginAPIErrorMessage(error: unknown, dictionary: Dictionary, fallback: string) {
  if (!(error instanceof AdminAPIError)) {
    return error instanceof Error ? error.message : fallback;
  }
  let message = error.message || fallback;
  if (error.code === "AUTH_INVALID_CREDENTIALS") {
    message = dictionary.loginInvalidCredentials;
  } else if (error.code === "RATE_LIMITED") {
    message = dictionary.loginRateLimited;
  } else if (error.code === "RATE_LIMIT_UNAVAILABLE") {
    message = dictionary.loginRateLimitUnavailable;
  }
  const correlation = error.requestId || error.traceId;
  return correlation ? `${message} ${dictionary.loginErrorCorrelation.replace("{id}", correlation)}` : message;
}

function sliderChallengeVerificationRejected(error: unknown) {
  return error instanceof AdminAPIError && error.code === "AUTH_INVALID_CREDENTIALS";
}

function sliderChallengeVerificationErrorMessage(error: unknown, dictionary: Dictionary) {
  if (error instanceof AdminAPIError && error.code === "RATE_LIMITED") return dictionary.loginRateLimited;
  if (error instanceof AdminAPIError && error.code === "RATE_LIMIT_UNAVAILABLE") return dictionary.loginRateLimitUnavailable;
  return dictionary.loginChallengeSliderUnavailable;
}

function createLoginClientFingerprint() {
  if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
    return `login:${crypto.randomUUID()}`;
  }
  const randomPart = Math.random().toString(36).slice(2);
  return `login:${Date.now().toString(36)}:${randomPart}`;
}

function challengeStringParameter(challenge: CredentialChallengeStartResult, key: string) {
  const value = challenge.parameters?.[key];
  return typeof value === "string" && value.trim() !== "" ? value : "";
}

function challengeNumberParameter(challenge: CredentialChallengeStartResult, key: string) {
  const value = challenge.parameters?.[key];
  const numberValue = typeof value === "number" ? value : Number(String(value ?? "").trim());
  return Number.isFinite(numberValue) ? numberValue : 0;
}

function challengeNeedsCaptchaFallback(challenge: CredentialChallengeStartResult) {
  return challenge.kind === "slider" && !sliderChallengeMaterial(challenge);
}

function sliderChallengeMaterial(challenge: CredentialChallengeStartResult | null): SliderChallengeMaterial | null {
  if (!challenge || challenge.kind !== "slider") return null;
  const masterImage = challengeStringParameter(challenge, "masterImage");
  const tileImage = challengeStringParameter(challenge, "tileImage");
  const imageWidth = Math.round(challengeNumberParameter(challenge, "imageWidth"));
  const imageHeight = Math.round(challengeNumberParameter(challenge, "imageHeight"));
  const tileWidth = Math.round(challengeNumberParameter(challenge, "tileWidth"));
  const tileHeight = Math.round(challengeNumberParameter(challenge, "tileHeight"));
  const initialX = challengeNumberParameter(challenge, "initialX") || challengeNumberParameter(challenge, "tileX");
  if (!masterImage || !tileImage || imageWidth <= 0 || imageHeight <= 0 || tileWidth <= 0 || tileHeight <= 0) {
    return null;
  }
  if (tileWidth > imageWidth || tileHeight > imageHeight) {
    return null;
  }
  return {
    masterImage,
    tileImage,
    imageWidth,
    imageHeight,
    tileWidth,
    tileHeight,
    initialX: clampNumber(initialX, 0, imageWidth - tileWidth),
    tileY: clampNumber(challengeNumberParameter(challenge, "tileY"), 0, imageHeight - tileHeight),
  };
}

function formatSliderChallengeProof(proofX: number, proofY: number) {
  return `x=${proofX}&y=${proofY}`;
}

function clampNumber(value: number, min: number, max: number) {
  if (!Number.isFinite(value)) return min;
  return Math.min(Math.max(value, min), max);
}

function callbackFailureReason(error: unknown): OIDCCallbackFailure | "generic" {
  return error instanceof OIDCCallbackError ? error.reason : "generic";
}

function callbackErrorMessage(dictionary: Dictionary, failure: OIDCCallbackFailure | "generic" | null) {
  if (failure === "expired") return dictionary.loginOIDCTransactionExpired;
  if (failure === "state") return dictionary.loginOIDCTransactionInvalid;
  return dictionary.loginOIDCCallbackFailed;
}

function credentialProviderSpec(provider?: AuthProvider): CredentialProviderSpec | null {
  if (!provider) return null;
  if (provider.kind === "credential-password") {
    if (provider.id === "username-password") return { identifierType: "username", mode: "password" };
    if (provider.id === "phone-password") return { identifierType: "phone", mode: "password" };
    if (provider.id === "email-password") return { identifierType: "email", mode: "password" };
  }
  if (provider.kind === "credential-sms-otp" && provider.id === "phone-sms-otp") {
    return { identifierType: "phone", mode: "sms-otp" };
  }
  return null;
}

function credentialIdentifierLabel(dictionary: Dictionary, type: CredentialIdentifierType) {
  if (type === "phone") return dictionary.loginPhone;
  if (type === "email") return dictionary.loginEmail;
  return dictionary.loginUsername;
}

function credentialIdentifierRequired(dictionary: Dictionary, type: CredentialIdentifierType) {
  if (type === "phone") return dictionary.loginPhoneRequired;
  if (type === "email") return dictionary.loginEmailRequired;
  return dictionary.loginUsernameRequired;
}

function credentialIdentifierPlaceholder(dictionary: Dictionary, type: CredentialIdentifierType) {
  if (type === "phone") return dictionary.loginPhonePlaceholder;
  if (type === "email") return dictionary.loginEmailPlaceholder;
  return dictionary.loginUsernamePlaceholder;
}

function credentialIdentifierAutocomplete(type: CredentialIdentifierType) {
  if (type === "phone") return "tel";
  if (type === "email") return "email";
  return "username";
}

function credentialIdentifierIcon(type: CredentialIdentifierType) {
  if (type === "phone") return <PhoneOutlined />;
  if (type === "email") return <MailOutlined />;
  return <UserOutlined />;
}
