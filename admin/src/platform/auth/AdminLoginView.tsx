import {
  CheckCircleOutlined,
  GlobalOutlined,
  LockOutlined,
  LoadingOutlined,
  LoginOutlined,
  MailOutlined,
  PhoneOutlined,
  SafetyCertificateOutlined,
  UserOutlined,
} from "@ant-design/icons";
import { Button, Form, Input, Space, Tooltip, Typography } from "antd";
import { useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import {
  loginWithAuthProvider,
  startCredentialChallenge,
  startCredentialSMSOTP,
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
import { themeNames, type ThemeName } from "../theme";
import { AdminFeedback } from "../ui";

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
  onThemeChange: (theme: ThemeName) => void;
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
  onThemeChange,
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
  const loginHeadingRef = useRef<HTMLElement>(null);
  const errorHeadingRef = useRef<HTMLElement>(null);
  const [form] = Form.useForm<LoginFormValues>();
  const adminProviders = useMemo(() => filterAdminAuthProviders(providers), [providers]);
  const availableProviders = useMemo(
    () => adminProviders.filter((provider) => provider.enabled && provider.configured),
    [adminProviders],
  );
  const selectedProvider = useMemo(
    () => availableProviders.find((provider) => provider.id === selectedProviderID) ?? availableProviders[0],
    [availableProviders, selectedProviderID],
  );
  const providerOptionsClassName = useMemo(
    () => loginProviderOptionsClassName(availableProviders.length),
    [availableProviders.length],
  );
  const productName = branding?.productName || "Platform Go";
  const shortName = branding?.shortName || productName;
  const targetLanguage = language === "zh" ? "en" : "zh";
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
    if (!credentialSpec) {
      setCredentialChallenge(null);
      form.setFieldValue("challengeProof", "");
      return;
    }
    void refreshCredentialChallenge();
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
      const challenge = credentialChallengeRequest(credentialChallenge, values.challengeProof);
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
      setLoginError(nextError instanceof Error ? nextError.message : dictionary.loginFailed);
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
      setSendingSMSOTP(true);
      setLoginError("");
      const transaction = await startCredentialSMSOTP({
        provider: selectedProvider.id,
        identifier: { type: "phone", value: values.identifier },
      });
      setSMSOTPTransaction(transaction);
    } catch (nextError) {
      setLoginError(nextError instanceof Error ? nextError.message : dictionary.loginSMSStartFailed);
    } finally {
      setSendingSMSOTP(false);
    }
  };

  const refreshCredentialChallenge = async () => {
    setCredentialChallengeLoading(true);
    try {
      const nextChallenge = await startCredentialChallenge({ kind: "captcha", purpose: "login" });
      setCredentialChallenge(nextChallenge);
      form.setFieldValue("challengeProof", "");
    } catch (nextError) {
      setCredentialChallenge(null);
      setLoginError(nextError instanceof Error ? nextError.message : dictionary.loginChallengeStartFailed);
    } finally {
      setCredentialChallengeLoading(false);
    }
  };

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
      <section className="login-hero">
        <div className="login-brand-row">
          <div className="login-logo">{branding?.logoUrl ? <img alt="" src={branding.logoUrl} /> : <SafetyCertificateOutlined />}</div>
          <div>
            <Typography.Text className="login-brand-name">{shortName}</Typography.Text>
            <Typography.Text className="login-brand-subtitle">{dictionary.appSubtitle}</Typography.Text>
          </div>
        </div>
        <div className="login-hero-copy">
          <Typography.Text className="page-eyebrow">{dictionary.allSystems}</Typography.Text>
          <Typography.Title level={1}>{branding?.loginTitle || productName}</Typography.Title>
          <Typography.Paragraph>{branding?.loginSubtitle || dictionary.loginSubtitle}</Typography.Paragraph>
        </div>
        <div className="login-capability-strip" aria-label={dictionary.loginCapabilitySummary}>
          <div>
            <CheckCircleOutlined />
            <span>{dictionary.loginCapabilityRbac}</span>
          </div>
          <div>
            <CheckCircleOutlined />
            <span>{dictionary.loginCapabilityPlugins}</span>
          </div>
          <div>
            <CheckCircleOutlined />
            <span>{dictionary.loginCapabilityThemes}</span>
          </div>
        </div>
      </section>

      <section className="login-panel" aria-label={dictionary.loginTitle}>
        <Typography.Title className="login-heading" level={2} ref={loginHeadingRef} tabIndex={-1}>
          {dictionary.loginTitle}
        </Typography.Title>
        <Typography.Paragraph className="login-panel-subtitle">{dictionary.loginPanelSubtitle}</Typography.Paragraph>

        {error ? <AdminFeedback className="login-feedback" type="error" message={error} /> : null}
        {callbackPhase === "idle" ? (
          <>
            {loginError ? <AdminFeedback className="login-feedback" type="error" message={loginError} /> : null}
            {availableProviders.length ? (
              <div
                aria-label={dictionary.loginProvider}
                className={providerOptionsClassName}
                role="listbox"
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
                      {smsOTPTransaction.debugCode ? ` ${smsOTPTransaction.debugCode}` : ""}
                    </Typography.Text>
                  ) : null}
                  <CredentialChallengeField
                    challenge={credentialChallenge}
                    dictionary={dictionary}
                    loading={credentialChallengeLoading}
                    onRefresh={() => void refreshCredentialChallenge()}
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

        <div className="login-panel-toolbar">
          <Tooltip title={`${dictionary.switchLanguage}: ${targetLanguage === "zh" ? dictionary.cn : dictionary.en}`}>
            <Button
              aria-label={dictionary.switchLanguage}
              className="topbar-icon-button"
              icon={<GlobalOutlined />}
              onClick={() => onLanguageChange(targetLanguage)}
            />
          </Tooltip>
          <Space size={6}>
            {themeNames.map((themeName) => (
              <Tooltip title={themeLabel(dictionary, themeName)} key={themeName}>
                <button
                  aria-label={themeLabel(dictionary, themeName)}
                  className={`login-theme-swatch theme-${themeName}`}
                  type="button"
                  onClick={() => onThemeChange(themeName)}
                />
              </Tooltip>
            ))}
          </Space>
        </div>
      </section>
    </main>
  );
}

function loginProviderOptionsClassName(count: number) {
  if (count <= 1) return "login-provider-options login-provider-count-1";
  if (count === 2) return "login-provider-options login-provider-count-2";
  if (count === 3) return "login-provider-options login-provider-count-3";
  return "login-provider-options login-provider-count-many";
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
}: {
  challenge: CredentialChallengeStartResult | null;
  dictionary: Dictionary;
  loading: boolean;
  onRefresh: () => void;
}) {
  return (
    <Form.Item label={dictionary.loginChallenge} required>
      <div className="login-challenge-card">
        <div className="login-challenge-prompt">
          <Typography.Text strong>{challengePrompt(challenge, dictionary)}</Typography.Text>
          {challenge?.debugProof ? (
            <Typography.Text code>{dictionary.loginChallengeDebugProof.replace("{proof}", challenge.debugProof)}</Typography.Text>
          ) : null}
        </div>
        <Space.Compact className="login-challenge-row">
          <Form.Item noStyle name="challengeProof" rules={[{ required: true, message: dictionary.loginChallengeRequired }]}>
            <Input
              prefix={<SafetyCertificateOutlined />}
              autoComplete="off"
              disabled={!challenge || loading}
              placeholder={dictionary.loginChallengePlaceholder}
            />
          </Form.Item>
          <Button loading={loading} onClick={onRefresh}>
            {dictionary.loginChallengeRefresh}
          </Button>
        </Space.Compact>
      </div>
    </Form.Item>
  );
}

function credentialChallengeRequest(challenge: CredentialChallengeStartResult | null, proof: string | undefined) {
  const normalizedProof = String(proof ?? "").trim();
  if (!challenge || normalizedProof === "") return undefined;
  return {
    id: challenge.id,
    kind: challenge.kind,
    proof: normalizedProof,
  };
}

function challengePrompt(challenge: CredentialChallengeStartResult | null, dictionary: Dictionary) {
  if (!challenge) return dictionary.loginChallengeLoading;
  const text = challenge.parameters?.text;
  if (text) return dictionary.loginChallengeText.replace("{text}", text);
  return challenge.prompt || dictionary.loginChallengePlaceholder;
}

function callbackFailureReason(error: unknown): OIDCCallbackFailure | "generic" {
  return error instanceof OIDCCallbackError ? error.reason : "generic";
}

function callbackErrorMessage(dictionary: Dictionary, failure: OIDCCallbackFailure | "generic" | null) {
  if (failure === "expired") return dictionary.loginOIDCTransactionExpired;
  if (failure === "state") return dictionary.loginOIDCTransactionInvalid;
  return dictionary.loginOIDCCallbackFailed;
}

function themeLabel(dictionary: Dictionary, themeName: ThemeName) {
  const labels = {
    tech: dictionary.themeTech,
    white: dictionary.themeWhite,
    black: dictionary.themeBlack,
    warm: dictionary.themeWarm,
  };
  return labels[themeName];
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
