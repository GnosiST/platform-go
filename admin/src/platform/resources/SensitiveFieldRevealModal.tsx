import { CheckCircleOutlined, CopyOutlined, EyeOutlined, SafetyCertificateOutlined } from "@ant-design/icons";
import { Button, Input, Modal, Radio, Select, Space, Tag, Typography } from "antd";
import { useEffect, useMemo, useRef, useState } from "react";
import {
  AdminAPIError,
  completeAdminSensitiveRevealSMS,
  createAdminSensitiveRevealChallenge,
  getAdminSensitiveRevealPolicy,
  revealAdminSensitiveField,
  startAdminSensitiveRevealSMS,
  type AdminResourceField,
  type AdminSensitiveRevealChallenge,
  type AdminSensitiveRevealFactor,
  type AdminSensitiveRevealFactorComplete,
  type AdminSensitiveRevealPolicy,
} from "../api/client";
import type { Dictionary, Language } from "../i18n";
import type { SensitiveRevealOIDCBeginInput, SensitiveRevealOIDCResume } from "../security/sensitiveRevealOIDC";
import { AdminFeedback } from "../ui";

type RevealPhase = "loading" | "choosing" | "redirecting" | "sms" | "verifying" | "revealed" | "expired" | "error";

type SensitiveFieldRevealModalProps = {
  open: boolean;
  resource: string;
  recordId?: string;
  field?: AdminResourceField;
  language: Language;
  dictionary: Dictionary;
  oidcResume?: SensitiveRevealOIDCResume<AdminSensitiveRevealFactorComplete> | null;
  onOIDCResumeConsumed: () => void;
  onStartOIDC: (input: SensitiveRevealOIDCBeginInput) => Promise<void>;
  onClose: () => void;
};

const OIDC_FACTOR: AdminSensitiveRevealFactor["type"] = "oidc-reauth-v1";
const SMS_FACTOR: AdminSensitiveRevealFactor["type"] = "admin-sms-otp-v1";

export function SensitiveFieldRevealModal({
  open,
  resource,
  recordId,
  field,
  language,
  dictionary,
  oidcResume,
  onOIDCResumeConsumed,
  onStartOIDC,
  onClose,
}: SensitiveFieldRevealModalProps) {
  const [phase, setPhase] = useState<RevealPhase>("loading");
  const [policy, setPolicy] = useState<AdminSensitiveRevealPolicy | null>(null);
  const [purpose, setPurpose] = useState("");
  const [selectedFactor, setSelectedFactor] = useState<AdminSensitiveRevealFactor["type"] | "">("");
  const [challenge, setChallenge] = useState<AdminSensitiveRevealChallenge | null>(null);
  const [completedFactors, setCompletedFactors] = useState<AdminSensitiveRevealFactor["type"][]>([]);
  const [activeFactor, setActiveFactor] = useState<AdminSensitiveRevealFactor["type"] | "">("");
  const [transactionToken, setTransactionToken] = useState("");
  const [maskedDestination, setMaskedDestination] = useState("");
  const [verificationCode, setVerificationCode] = useState("");
  const [revealedValue, setRevealedValue] = useState("");
  const [revealedUntil, setRevealedUntil] = useState("");
  const [copyAllowed, setCopyAllowed] = useState(false);
  const [copied, setCopied] = useState(false);
  const [errorMessage, setErrorMessage] = useState("");
  const headingRef = useRef<HTMLHeadingElement>(null);
  const resumeHandledRef = useRef("");
  const operationGenerationRef = useRef(0);

  const availableFactors = useMemo(() => policy?.factors.filter((candidate) => candidate.available) ?? [], [policy]);
  const requiredFactors = useMemo(() => policy?.factors.map((candidate) => candidate.type) ?? [], [policy]);

  useEffect(() => {
    if (!open || !recordId || !field) return;
    let cancelled = false;
    const generation = ++operationGenerationRef.current;
    clearSensitiveState();
    setPhase("loading");
    void getAdminSensitiveRevealPolicy(resource, recordId, field.key)
      .then((nextPolicy) => {
        if (cancelled || operationGenerationRef.current !== generation) return;
        setPolicy(nextPolicy);
        setPurpose(nextPolicy.purposes[0]?.code ?? "");
        setSelectedFactor(nextPolicy.factors.find((candidate) => candidate.available)?.type ?? "");
        setPhase("choosing");
      })
      .catch((error: unknown) => {
        if (cancelled || operationGenerationRef.current !== generation) return;
        setErrorMessage(sensitiveRevealErrorMessage(error, dictionary.sensitiveRevealLoadFailed, dictionary));
        setPhase(error instanceof AdminAPIError && error.statusCode === 410 ? "expired" : "error");
      });
    return () => {
      cancelled = true;
      if (operationGenerationRef.current === generation) {
        operationGenerationRef.current += 1;
      }
    };
  }, [dictionary, field, open, recordId, resource]);

  useEffect(() => {
    if (!open) return;
    const handleVisibilityChange = () => {
      if (document.visibilityState === "hidden") closeAndClear();
    };
    document.addEventListener("visibilitychange", handleVisibilityChange);
    return () => document.removeEventListener("visibilitychange", handleVisibilityChange);
  }, [open]);

  useEffect(() => {
    if (phase !== "revealed" || !revealedUntil) return;
    const remaining = Date.parse(revealedUntil) - Date.now();
    if (!Number.isFinite(remaining) || remaining <= 0) {
      setRevealedValue("");
      setPhase("expired");
      return;
    }
    const timer = window.setTimeout(() => {
      setRevealedValue("");
      setPhase("expired");
    }, remaining);
    return () => window.clearTimeout(timer);
  }, [phase, revealedUntil]);

  useEffect(() => {
    if (!open || !policy || !oidcResume || !recordId || !field) return;
    const resumeKey = [oidcResume.context.challengeId, oidcResume.context.field, oidcResume.completion?.grantToken ?? oidcResume.error ?? "pending"].join(":");
    if (resumeHandledRef.current === resumeKey) return;
    resumeHandledRef.current = resumeKey;
    onOIDCResumeConsumed();
    const generation = ++operationGenerationRef.current;
    if (
      oidcResume.context.resource !== resource ||
      oidcResume.context.recordId !== recordId ||
      oidcResume.context.field !== field.key
    ) {
      setErrorMessage(dictionary.sensitiveRevealVerificationFailed);
      setPhase("error");
      return;
    }
    setPurpose(oidcResume.context.purpose);
    const nextCompleted = uniqueFactors([...oidcResume.context.completedFactors, OIDC_FACTOR]);
    setCompletedFactors(nextCompleted);
    const resumedChallenge: AdminSensitiveRevealChallenge = {
      challengeId: oidcResume.context.challengeId,
      challengeToken: oidcResume.context.challengeToken,
      policyId: policy.policyId,
      mode: policy.mode,
      factors: requiredFactors,
      expiresAt: oidcResume.context.challengeExpiresAt,
    };
    setChallenge(resumedChallenge);
    if (oidcResume.error || !oidcResume.completion) {
      setErrorMessage(oidcResume.error === "expired" ? dictionary.sensitiveRevealExpired : dictionary.sensitiveRevealVerificationFailed);
      setPhase(oidcResume.error === "expired" ? "expired" : "error");
      return;
    }
    if (oidcResume.completion.grantToken) {
      void finishReveal(resumedChallenge, oidcResume.context.purpose, oidcResume.completion, generation);
      return;
    }
    void startNextFactor(resumedChallenge, nextCompleted, "", generation).catch((error: unknown) => {
      failReveal(error, dictionary.sensitiveRevealVerificationFailed, generation);
    });
  }, [dictionary, field, oidcResume, onOIDCResumeConsumed, open, policy, recordId, requiredFactors, resource]);

  const closeAndClear = () => {
    operationGenerationRef.current += 1;
    clearSensitiveState();
    setPolicy(null);
    setPurpose("");
    setSelectedFactor("");
    setPhase("loading");
    resumeHandledRef.current = "";
    onClose();
  };

  const clearSensitiveState = () => {
    setChallenge(null);
    setCompletedFactors([]);
    setActiveFactor("");
    setTransactionToken("");
    setMaskedDestination("");
    setVerificationCode("");
    setRevealedValue("");
    setRevealedUntil("");
    setCopyAllowed(false);
    setCopied(false);
    setErrorMessage("");
  };

  const beginVerification = async () => {
    if (!policy || !recordId || !field || !purpose) return;
    const generation = ++operationGenerationRef.current;
    setPhase("verifying");
    setErrorMessage("");
    try {
      const nextChallenge = await createAdminSensitiveRevealChallenge(resource, recordId, field.key, purpose);
      if (operationGenerationRef.current !== generation) return;
      setChallenge(nextChallenge);
      setCompletedFactors([]);
      await startNextFactor(nextChallenge, [], policy.mode === "anyOf" ? selectedFactor : "", generation);
    } catch (error) {
      failReveal(error, dictionary.sensitiveRevealStartFailed, generation);
    }
  };

  const startNextFactor = async (
    activeChallenge: AdminSensitiveRevealChallenge,
    completed: AdminSensitiveRevealFactor["type"][],
    preferred: AdminSensitiveRevealFactor["type"] | "",
    generation: number,
  ) => {
    if (!policy || !recordId || !field || operationGenerationRef.current !== generation) return;
    const factor = policy.mode === "anyOf" ? preferred : activeChallenge.factors.find((candidate) => !completed.includes(candidate));
    const descriptor = policy.factors.find((candidate) => candidate.type === factor && candidate.available);
    if (!factor || !descriptor) {
      setErrorMessage(dictionary.sensitiveRevealUnavailable);
      setPhase("error");
      return;
    }
    setActiveFactor(factor);
    setPhase("verifying");
    if (factor === SMS_FACTOR) {
      const started = await startAdminSensitiveRevealSMS(resource, recordId, field.key, activeChallenge.challengeId, {
        challengeToken: activeChallenge.challengeToken,
        purpose,
      });
      if (operationGenerationRef.current !== generation) return;
      setTransactionToken(started.transactionToken);
      setMaskedDestination(started.maskedPhone || descriptor.maskedDestination || "");
      setVerificationCode(started.debugCode ?? "");
      setPhase("sms");
      return;
    }
    const provider = descriptor.providers?.[0];
    if (!provider) {
      setErrorMessage(dictionary.sensitiveRevealUnavailable);
      setPhase("error");
      return;
    }
    setPhase("redirecting");
    await onStartOIDC({
      resource,
      recordId,
      field: field.key,
      purpose,
      challengeId: activeChallenge.challengeId,
      challengeToken: activeChallenge.challengeToken,
      challengeExpiresAt: activeChallenge.expiresAt,
      provider: provider.id,
      returnPath: window.location.pathname,
      completedFactors: completed,
    });
  };

  const completeSMS = async () => {
    if (!challenge || !recordId || !field || !verificationCode.trim() || !transactionToken) return;
    const generation = ++operationGenerationRef.current;
    setPhase("verifying");
    try {
      const completion = await completeAdminSensitiveRevealSMS(resource, recordId, field.key, challenge.challengeId, {
        challengeToken: challenge.challengeToken,
        purpose,
        transactionToken,
        code: verificationCode.trim(),
      });
      if (operationGenerationRef.current !== generation) return;
      const nextCompleted = uniqueFactors([...completedFactors, SMS_FACTOR]);
      setCompletedFactors(nextCompleted);
      if (completion.grantToken) {
        await finishReveal(challenge, purpose, completion, generation);
        return;
      }
      await startNextFactor(challenge, nextCompleted, "", generation);
    } catch (error) {
      failReveal(error, dictionary.sensitiveRevealVerificationFailed, generation);
    }
  };

  const finishReveal = async (
    activeChallenge: AdminSensitiveRevealChallenge,
    activePurpose: string,
    completion: AdminSensitiveRevealFactorComplete,
    generation: number,
  ) => {
    if (operationGenerationRef.current !== generation) return;
    if (!recordId || !field || !completion.grantToken) {
      setErrorMessage(dictionary.sensitiveRevealVerificationFailed);
      setPhase("error");
      return;
    }
    setPhase("verifying");
    try {
      const result = await revealAdminSensitiveField(resource, recordId, field.key, {
        purpose: activePurpose,
        grantToken: completion.grantToken,
      });
      if (operationGenerationRef.current !== generation) return;
      setChallenge(activeChallenge);
      setRevealedValue(result.value);
      setCopyAllowed(Boolean(result.copyAllowed && policy?.copyAllowed && field.reveal?.copyAllowed));
      setRevealedUntil(completion.grantExpiresAt ?? new Date(Date.now() + (policy?.grantTtlSeconds ?? 30) * 1000).toISOString());
      setPhase("revealed");
      requestAnimationFrame(() => headingRef.current?.focus({ preventScroll: true }));
    } catch (error) {
      failReveal(error, dictionary.sensitiveRevealVerificationFailed, generation);
    }
  };

  const failReveal = (error: unknown, fallback: string, generation = operationGenerationRef.current) => {
    if (operationGenerationRef.current !== generation) return;
    setRevealedValue("");
    setErrorMessage(sensitiveRevealErrorMessage(error, fallback, dictionary));
    setPhase(error instanceof AdminAPIError && error.statusCode === 410 ? "expired" : "error");
    requestAnimationFrame(() => headingRef.current?.focus({ preventScroll: true }));
  };

  const retry = () => {
    operationGenerationRef.current += 1;
    clearSensitiveState();
    setPurpose(policy?.purposes[0]?.code ?? "");
    setSelectedFactor(policy?.factors.find((candidate) => candidate.available)?.type ?? "");
    setPhase(policy ? "choosing" : "loading");
  };

  const copyValue = async () => {
    if (!copyAllowed || !revealedValue) return;
    const generation = operationGenerationRef.current;
    try {
      await navigator.clipboard.writeText(revealedValue);
      if (operationGenerationRef.current !== generation) return;
      setCopied(true);
    } catch {
      if (operationGenerationRef.current !== generation) return;
      setErrorMessage(dictionary.sensitiveRevealVerificationFailed);
    }
  };

  const title = (
    <Typography.Title className="sensitive-reveal-heading" level={3} ref={headingRef} tabIndex={-1}>
      {dictionary.sensitiveRevealTitle}
    </Typography.Title>
  );

  return (
    <Modal
      className="sensitive-reveal-modal"
      destroyOnHidden
      maskClosable={false}
      open={open}
      title={title}
      width={520}
      afterOpenChange={(nextOpen) => {
        if (nextOpen) headingRef.current?.focus({ preventScroll: true });
      }}
      footer={revealFooter({
        phase,
        canContinue: Boolean(purpose && (policy?.mode === "allOf" ? requiredFactors.every((factor) => availableFactors.some((candidate) => candidate.type === factor)) : selectedFactor)),
        canVerify: Boolean(verificationCode.trim()),
        copyAllowed,
        copied,
        dictionary,
        onClose: closeAndClear,
        onContinue: () => void beginVerification(),
        onCopy: () => void copyValue(),
        onRetry: retry,
        onVerify: () => void completeSMS(),
      })}
      onCancel={closeAndClear}
    >
      <div className="sensitive-reveal-content" aria-live="polite">
        {phase === "loading" ? <AdminFeedback type="info" message={dictionary.sensitiveRevealDescription} /> : null}
        {phase === "choosing" && policy ? (
          <>
            <AdminFeedback
              type="warning"
              message={dictionary.sensitiveRevealDescription}
              description={policy.mode === "allOf" ? dictionary.sensitiveRevealAllOfHint : dictionary.sensitiveRevealAnyOfHint}
            />
            <label className="sensitive-reveal-control">
              <span>{dictionary.sensitiveRevealPurpose}</span>
              <Select
                aria-label={dictionary.sensitiveRevealPurpose}
                options={policy.purposes.map((candidate) => ({ value: candidate.code, label: candidate.label[language] || candidate.code }))}
                placeholder={dictionary.sensitiveRevealPurposePlaceholder}
                value={purpose || undefined}
                onChange={setPurpose}
              />
            </label>
            {policy.mode === "anyOf" ? (
              <fieldset className="sensitive-reveal-factor-fieldset">
                <legend>{dictionary.sensitiveRevealFactor}</legend>
                <Radio.Group value={selectedFactor} onChange={(event) => setSelectedFactor(event.target.value)}>
                  <Space direction="vertical">
                    {policy.factors.map((candidate) => (
                      <Radio disabled={!candidate.available} key={candidate.type} value={candidate.type}>
                        {factorLabel(candidate.type, dictionary)}
                      </Radio>
                    ))}
                  </Space>
                </Radio.Group>
              </fieldset>
            ) : (
              <FactorProgress factors={requiredFactors} completed={completedFactors} dictionary={dictionary} />
            )}
            {availableFactors.length === 0 ? <AdminFeedback type="error" message={dictionary.sensitiveRevealUnavailable} /> : null}
          </>
        ) : null}
        {phase === "sms" ? (
          <>
            <AdminFeedback
              type="info"
              message={dictionary.sensitiveRevealSMSDestination.replace("{destination}", maskedDestination || "-")}
            />
            <label className="sensitive-reveal-control">
              <span>{dictionary.sensitiveRevealCode}</span>
              <Input
                autoComplete="one-time-code"
                inputMode="numeric"
                placeholder={dictionary.sensitiveRevealCodePlaceholder}
                value={verificationCode}
                onChange={(event) => setVerificationCode(event.target.value)}
                onPressEnter={() => void completeSMS()}
              />
            </label>
            {policy?.mode === "allOf" ? <FactorProgress factors={requiredFactors} completed={completedFactors} dictionary={dictionary} active={SMS_FACTOR} /> : null}
          </>
        ) : null}
        {phase === "redirecting" ? <AdminFeedback type="info" message={dictionary.sensitiveRevealOIDCRedirect} /> : null}
        {phase === "verifying" ? <AdminFeedback type="info" message={dictionary.sensitiveRevealVerify} /> : null}
        {phase === "revealed" ? (
          <>
            <AdminFeedback type="success" message={dictionary.sensitiveRevealProtectedValue} description={dictionary.sensitiveRevealPrivacyNotice} />
            <div className="sensitive-reveal-value" data-testid="sensitive-reveal-value">
              <EyeOutlined aria-hidden />
              <span>{revealedValue}</span>
            </div>
          </>
        ) : null}
        {phase === "expired" ? <AdminFeedback type="warning" message={dictionary.sensitiveRevealExpired} /> : null}
        {phase === "error" ? <AdminFeedback type="error" message={errorMessage || dictionary.sensitiveRevealVerificationFailed} /> : null}
      </div>
    </Modal>
  );
}

function FactorProgress({
  factors,
  completed,
  active,
  dictionary,
}: {
  factors: AdminSensitiveRevealFactor["type"][];
  completed: AdminSensitiveRevealFactor["type"][];
  active?: AdminSensitiveRevealFactor["type"];
  dictionary: Dictionary;
}) {
  return (
    <section className="sensitive-reveal-progress" aria-label={dictionary.sensitiveRevealRequiredFactors}>
      <Typography.Text strong>{dictionary.sensitiveRevealRequiredFactors}</Typography.Text>
      <Space wrap>
        {factors.map((factor) => {
          const done = completed.includes(factor);
          return (
            <Tag color={done ? "success" : active === factor ? "processing" : "default"} icon={done ? <CheckCircleOutlined /> : <SafetyCertificateOutlined />} key={factor}>
              {factorLabel(factor, dictionary)}{done ? ` · ${dictionary.sensitiveRevealFactorCompleted}` : ""}
            </Tag>
          );
        })}
      </Space>
    </section>
  );
}

function revealFooter({
  phase,
  canContinue,
  canVerify,
  copyAllowed,
  copied,
  dictionary,
  onClose,
  onContinue,
  onCopy,
  onRetry,
  onVerify,
}: {
  phase: RevealPhase;
  canContinue: boolean;
  canVerify: boolean;
  copyAllowed: boolean;
  copied: boolean;
  dictionary: Dictionary;
  onClose: () => void;
  onContinue: () => void;
  onCopy: () => void;
  onRetry: () => void;
  onVerify: () => void;
}) {
  if (phase === "choosing") {
    return [
      <Button key="cancel" onClick={onClose}>{dictionary.cancel}</Button>,
      <Button disabled={!canContinue} icon={<SafetyCertificateOutlined />} key="continue" type="primary" onClick={onContinue}>
        {dictionary.sensitiveRevealContinue}
      </Button>,
    ];
  }
  if (phase === "sms") {
    return [
      <Button key="cancel" onClick={onClose}>{dictionary.cancel}</Button>,
      <Button disabled={!canVerify} icon={<EyeOutlined />} key="verify" type="primary" onClick={onVerify}>
        {dictionary.sensitiveRevealVerify}
      </Button>,
    ];
  }
  if (phase === "revealed") {
    return [
      copyAllowed ? (
        <Button icon={copied ? <CheckCircleOutlined /> : <CopyOutlined />} key="copy" onClick={onCopy}>
          {copied ? dictionary.sensitiveRevealCopied : dictionary.sensitiveRevealCopy}
        </Button>
      ) : null,
      <Button key="close" type="primary" onClick={onClose}>{dictionary.sensitiveRevealClose}</Button>,
    ].filter(Boolean);
  }
  if (phase === "expired" || phase === "error") {
    return [
      <Button key="close" onClick={onClose}>{dictionary.sensitiveRevealClose}</Button>,
      <Button key="retry" type="primary" onClick={onRetry}>{dictionary.sensitiveRevealRetry}</Button>,
    ];
  }
  return [<Button key="close" onClick={onClose}>{dictionary.sensitiveRevealClose}</Button>];
}

function factorLabel(factor: AdminSensitiveRevealFactor["type"], dictionary: Dictionary) {
  return factor === OIDC_FACTOR ? dictionary.sensitiveRevealFactorOIDC : dictionary.sensitiveRevealFactorSMS;
}

function uniqueFactors(factors: AdminSensitiveRevealFactor["type"][]) {
  return Array.from(new Set(factors));
}

function sensitiveRevealErrorMessage(error: unknown, fallback: string, dictionary: Dictionary) {
  if (error instanceof AdminAPIError && error.statusCode === 410) return dictionary.sensitiveRevealExpired;
  return fallback;
}
