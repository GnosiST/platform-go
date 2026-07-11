import {
  CheckCircleOutlined,
  GlobalOutlined,
  LoadingOutlined,
  LoginOutlined,
  SafetyCertificateOutlined,
  UserOutlined,
} from "@ant-design/icons";
import { Button, Form, Input, Space, Tooltip, Typography } from "antd";
import { useEffect, useMemo, useRef, useState } from "react";
import {
  loginWithAuthProvider,
  type AdminCurrentSession,
  type AuthProvider,
  type BrandingConfig,
} from "../api/client";
import type { Dictionary, Language } from "../i18n";
import {
  beginOIDCLogin,
  clearPendingOIDCLogin,
  consumePendingOIDCLogin,
  OIDCCallbackError,
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
};

type CallbackPhase = "idle" | "processing" | "failed";

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
  const [loginError, setLoginError] = useState("");
  const [callbackPhase, setCallbackPhase] = useState<CallbackPhase>("idle");
  const callbackStartedRef = useRef(false);
  const errorHeadingRef = useRef<HTMLElement>(null);
  const selectedProvider = useMemo(
    () => providers.find((provider) => provider.id === selectedProviderID) ?? providers.find((provider) => provider.configured),
    [providers, selectedProviderID],
  );
  const productName = branding?.productName || "Platform Go";
  const shortName = branding?.shortName || productName;
  const targetLanguage = language === "zh" ? "en" : "zh";

  useEffect(() => {
    if (callbackStartedRef.current || callbackPhase !== "idle") return;
    const params = new URLSearchParams(search);
    if (!["code", "state", "error"].some((key) => params.has(key))) return;

    callbackStartedRef.current = true;
    setCallbackPhase("processing");
    setSubmitting(true);
    setLoginError("");
    void consumePendingOIDCLogin(search)
      .then((result) => {
        if (result) onLoginSuccess(result.principal);
      })
      .catch((nextError: unknown) => {
        setLoginError(callbackErrorMessage(dictionary, nextError));
        setCallbackPhase("failed");
      })
      .finally(() => setSubmitting(false));
  }, [callbackPhase, dictionary, onLoginSuccess, search]);

  useEffect(() => {
    if (callbackPhase === "failed") {
      errorHeadingRef.current?.focus({ preventScroll: true });
    }
  }, [callbackPhase]);

  const submit = async (values: LoginFormValues) => {
    if (submitting) return;
    if (!selectedProvider?.configured || selectedProvider.kind !== "demo") {
      setLoginError(dictionary.loginProviderUnavailable);
      return;
    }
    setSubmitting(true);
    setLoginError("");
    try {
      const result = await loginWithAuthProvider({
        provider: selectedProvider.id,
        username: values.username,
      });
      onLoginSuccess(result.principal);
    } catch (nextError) {
      setLoginError(nextError instanceof Error ? nextError.message : dictionary.loginFailed);
    } finally {
      setSubmitting(false);
    }
  };

  const startOIDC = async () => {
    if (submitting) return;
    if (!selectedProvider?.configured || selectedProvider.kind !== "oidc") {
      setLoginError(dictionary.loginProviderUnavailable);
      return;
    }
    setSubmitting(true);
    setLoginError("");
    try {
      await beginOIDCLogin(selectedProvider.id);
    } catch {
      clearPendingOIDCLogin();
      setLoginError(dictionary.loginOIDCStartFailed);
      setSubmitting(false);
    }
  };

  const recoverFromCallback = () => {
    clearPendingOIDCLogin();
    setCallbackPhase("idle");
    setLoginError("");
    setSubmitting(false);
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
        <Typography.Title level={2}>{dictionary.loginTitle}</Typography.Title>
        <Typography.Paragraph className="login-panel-subtitle">{dictionary.loginPanelSubtitle}</Typography.Paragraph>

        {error ? <AdminFeedback className="login-feedback" type="error" message={error} /> : null}
        {callbackPhase === "idle" ? (
          <>
            {loginError ? <AdminFeedback className="login-feedback" type="error" message={loginError} /> : null}
            <div className="login-provider-list" aria-label={dictionary.loginProvider}>
              {providers.map((provider) => {
                const providerTitle = provider.title[language] || provider.id;
                const providerStatus = provider.configured ? dictionary.configured : dictionary.notConfigured;
                return (
                  <button
                    aria-label={`${providerTitle}, ${providerStatus}`}
                    className={provider.id === selectedProvider?.id ? "login-provider active" : "login-provider"}
                    disabled={!provider.configured || loading || submitting}
                    key={provider.id}
                    type="button"
                    onClick={() => {
                      clearPendingOIDCLogin();
                      setLoginError("");
                      setSelectedProviderID(provider.id);
                    }}
                  >
                    <span>
                      <strong>{providerTitle}</strong>
                      <small>{providerStatus}</small>
                    </span>
                    {provider.id === selectedProvider?.id ? <CheckCircleOutlined aria-hidden /> : null}
                  </button>
                );
              })}
            </div>

            {selectedProvider && selectedProvider.kind === "demo" ? (
              <Form<LoginFormValues>
                layout="vertical"
                initialValues={{ username: "admin" }}
                requiredMark={false}
                onFinish={submit}
              >
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
              </Form>
            ) : null}

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
                <Typography.Paragraph>{loginError}</Typography.Paragraph>
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

function callbackErrorMessage(dictionary: Dictionary, error: unknown) {
  if (!(error instanceof OIDCCallbackError)) return dictionary.loginOIDCCallbackFailed;
  if (error.reason === "expired") return dictionary.loginOIDCTransactionExpired;
  if (error.reason === "state") return dictionary.loginOIDCTransactionInvalid;
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
