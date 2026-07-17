import { Alert, App, Input, Select, Space, Spin, Tag, Typography, type FormInstance } from "antd";
import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { getAdminResourceSchema, type AdminResourceField, type AdminResourceRecord, type AdminResourceSchema } from "../api/client";
import {
  changeUserOrganization,
  getOrganizationRoleGroupChangeConflicts,
  getOrganizationRoleGroupChangeImpact,
  getOrganizationRolePool,
  getUserOrganizationChangeImpact,
  prepareOrganizationRoleGroupChange,
  prepareUserOrganizationChange,
  replaceOrganizationRoleGroups,
  type OrganizationChangeConflict,
  type OrganizationRolePoolItem,
  type OrganizationRoleRemediation,
} from "../api/organizationRBAC";
import type { Dictionary, Language } from "../i18n";
import { platformPopupContainer } from "../ui";
import {
  ResourceExperienceCancelledError,
  type ResourceExperienceController,
  type ResourceExperienceKey,
  type ResourceExperienceSubmitContext,
  type ResourceFormValues,
} from "./resourceExperience";
import {
  dispatchOrganizationUserWrite,
  organizationTreeFieldOption,
  organizationUserRuntimeCapabilities,
  resolveOrganizationUserRuntimeMode,
  type OrganizationUserRuntimeMode,
} from "./organizationUserRuntime";

type UseOrganizationUserExperienceInput = {
  experienceKey?: ResourceExperienceKey;
  resourceKey: string;
  schema: AdminResourceSchema;
  form: FormInstance<ResourceFormValues>;
  formValues: ResourceFormValues;
  editingRecord: AdminResourceRecord | null;
  modalOpen: boolean;
  language: Language;
  dictionary: Dictionary;
  loadRecords: (resource: string) => Promise<AdminResourceRecord[]>;
};

export function useOrganizationUserExperience({
  experienceKey,
  resourceKey,
  schema,
  form,
  formValues,
  editingRecord,
  modalOpen,
  language,
  dictionary,
  loadRecords,
}: UseOrganizationUserExperienceInput): ResourceExperienceController {
  const { modal } = App.useApp();
  const active = experienceKey === "organization-user" && (resourceKey === "org-units" || resourceKey === "users");
  const [runtimeMode, setRuntimeMode] = useState<OrganizationUserRuntimeMode | "loading">("loading");
  const [orgUnits, setOrgUnits] = useState<AdminResourceRecord[]>([]);
  const [roleGroups, setRoleGroups] = useState<AdminResourceRecord[]>([]);
  const [rolePool, setRolePool] = useState<ReadonlyArray<OrganizationRolePoolItem>>([]);
  const [contextLoading, setContextLoading] = useState(false);
  const [rolePoolLoading, setRolePoolLoading] = useState(false);
  const [contextError, setContextError] = useState("");
  const [rolePoolError, setRolePoolError] = useState("");
  const rolePoolRequest = useRef(0);
  const runtimeModeRequest = useRef(0);
  const runtimeCapabilities = runtimeMode === "loading" ? null : organizationUserRuntimeCapabilities(runtimeMode);

  useEffect(() => {
    const requestID = ++runtimeModeRequest.current;
    if (!active) {
      setRuntimeMode("loading");
      return;
    }
    setRuntimeMode("loading");
    void getAdminResourceSchema("roles")
      .then((roleSchema) => {
        if (runtimeModeRequest.current === requestID) {
          setRuntimeMode(resolveOrganizationUserRuntimeMode(roleSchema));
        }
      })
      .catch(() => {
        if (runtimeModeRequest.current === requestID) {
          setRuntimeMode("readonly");
        }
      });
    return () => { runtimeModeRequest.current += 1; };
  }, [active]);

  useEffect(() => {
    if (!active || !runtimeCapabilities?.useServiceObjects || !modalOpen) {
      setContextLoading(false);
      setContextError("");
      setOrgUnits([]);
      setRoleGroups([]);
      return;
    }
    let cancelled = false;
    setContextLoading(true);
    setContextError("");
    setOrgUnits([]);
    setRoleGroups([]);
    Promise.all([loadRecords("org-units"), loadRecords("role-groups")])
      .then(([organizations, groups]) => {
        if (!cancelled) {
          setOrgUnits(organizations);
          setRoleGroups(groups);
        }
      })
      .catch((error: unknown) => {
        if (!cancelled) {
          setContextError(error instanceof Error ? error.message : dictionary.rolePoolLoadFailed);
        }
      })
      .finally(() => {
        if (!cancelled) {
          setContextLoading(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [active, dictionary.rolePoolLoadFailed, loadRecords, modalOpen, runtimeCapabilities?.useServiceObjects]);

  const scopeType = String(editingRecord?.values?.scopeType || "tenant");
  const selectedOrgUnitCode = String(formValues.orgUnitCode ?? "").trim();
  const selectedOrgUnit = useMemo(
    () => orgUnits.find((record) => record.code === selectedOrgUnitCode),
    [orgUnits, selectedOrgUnitCode],
  );
  const derivedTenantCode = scopeType === "platform" ? "" : String(selectedOrgUnit?.values?.tenantCode ?? "");

  useEffect(() => {
    if (!active || !runtimeCapabilities?.useServiceObjects || resourceKey !== "users" || !modalOpen || scopeType === "platform") {
      return;
    }
    if (form.getFieldValue("tenantCode") !== derivedTenantCode) {
      form.setFieldValue("tenantCode", derivedTenantCode);
    }
  }, [active, derivedTenantCode, form, modalOpen, resourceKey, runtimeCapabilities?.useServiceObjects, scopeType]);

  useEffect(() => {
    if (!active || !runtimeCapabilities?.useServiceObjects || resourceKey !== "users" || !modalOpen || scopeType === "platform" || !selectedOrgUnitCode) {
      rolePoolRequest.current += 1;
      setRolePool([]);
      setRolePoolLoading(false);
      setRolePoolError("");
      return;
    }
    const requestID = ++rolePoolRequest.current;
    setRolePoolLoading(true);
    setRolePoolError("");
    getOrganizationRolePool(selectedOrgUnitCode)
      .then((items) => {
        if (rolePoolRequest.current === requestID) {
          setRolePool(items);
        }
      })
      .catch((error: unknown) => {
        if (rolePoolRequest.current === requestID) {
          setRolePool([]);
          setRolePoolError(error instanceof Error ? error.message : dictionary.rolePoolLoadFailed);
        }
      })
      .finally(() => {
        if (rolePoolRequest.current === requestID) {
          setRolePoolLoading(false);
        }
      });
  }, [active, dictionary.rolePoolLoadFailed, modalOpen, resourceKey, runtimeCapabilities?.useServiceObjects, scopeType, selectedOrgUnitCode]);

  const selectedTenantCode = resourceKey === "org-units"
    ? String(formValues.tenantCode ?? editingRecord?.values?.tenantCode ?? "")
    : derivedTenantCode;
  const selectedRoles = stringList(formValues.roles);
  const rolePoolCodes = useMemo(() => new Set(rolePool.map((item) => item.roleCode)), [rolePool]);
  const invalidSelectedRoles = runtimeCapabilities?.useServiceObjects
    ? selectedRoles.filter((roleCode) => !rolePoolCodes.has(roleCode))
    : [];
  const roleOptions = useMemo(
    () => mergeSelectOptions(
      rolePool.map((item) => ({
        value: item.roleCode,
        label: `${item.roleName || item.roleCode} (${item.roleCode}) · ${item.roleGroupName} (${item.roleGroupCode})`,
      })),
      selectedRoles.map((roleCode) => ({
        value: roleCode,
        label: `${roleCode} · ${dictionary.userInvalidRoleTag}`,
      })),
    ),
    [dictionary.userInvalidRoleTag, rolePool, selectedRoles],
  );
  const roleGroupOptions = useMemo(() => {
    const current = stringList(formValues.roleGroupCodes);
    const available = roleGroups
      .filter((record) => record.status === "enabled")
      .filter((record) => record.values?.scopeType === "tenant" && record.values?.tenantCode === selectedTenantCode)
      .map((record) => ({ value: record.code, label: `${record.name} (${record.code})` }));
    const fallback = current.map((code) => {
      const record = roleGroups.find((item) => item.code === code);
      return { value: code, label: record ? `${record.name} (${record.code})` : code };
    });
    return mergeSelectOptions(available, fallback);
  }, [formValues.roleGroupCodes, roleGroups, selectedTenantCode]);

  const formFields = useMemo(() => {
    if (!active || runtimeMode === "legacy") {
      return schema.fields.filter((field) => field.inForm && !field.readOnly);
    }
    if (runtimeMode === "loading" || runtimeMode === "readonly") {
      return schema.fields
        .filter((field) => field.inForm && !field.readOnly || resourceKey === "org-units" && field.key === "roleGroupCodes")
        .map((field) => ({ ...field, readOnly: true, required: false }));
    }
    return schema.fields
      .filter((field) => field.inForm && !field.readOnly || resourceKey === "org-units" && field.key === "roleGroupCodes")
      .map((field) => {
        if (resourceKey === "org-units" && field.key === "roleGroupCodes") {
          return { ...field, inForm: true, readOnly: false, options: roleGroupOptions.map(toFieldOption) };
        }
        if (resourceKey === "users" && field.key === "tenantCode") {
          return { ...field, type: "text" as const, readOnly: true };
        }
        if (resourceKey === "users" && field.key === "orgUnitCode") {
          return { ...field, required: scopeType !== "platform", options: orgUnits.filter((record) => record.status === "enabled").map(organizationTreeFieldOption) };
        }
        if (resourceKey === "users" && field.key === "roles") {
          return { ...field, options: roleOptions.map(toFieldOption) };
        }
        return field;
      });
  }, [active, orgUnits, resourceKey, roleGroupOptions, roleOptions, runtimeMode, schema.fields, scopeType]);

  const renderField = useCallback((field: AdminResourceField, fallback: ReactNode) => {
    if (!active) {
      return fallback;
    }
    if (editingRecord && field.key === "status") {
      return <Input readOnly aria-readonly="true" />;
    }
    if (runtimeMode === "legacy") {
      return fallback;
    }
    if (runtimeMode === "loading" || runtimeMode === "readonly") {
      if (resourceKey === "org-units" && field.key === "roleGroupCodes") {
        return <ReadOnlyListValue value={stringList(formValues.roleGroupCodes)} emptyText={dictionary.emptyData} />;
      }
      if (resourceKey === "users" && field.key === "roles") {
        return <ReadOnlyListValue value={selectedRoles} emptyText={dictionary.emptyData} />;
      }
      return <Input readOnly aria-readonly="true" />;
    }
    if (resourceKey === "org-units" && editingRecord && field.key === "tenantCode") {
      return <Input readOnly aria-readonly="true" />;
    }
    if (resourceKey === "org-units" && field.key === "roleGroupCodes") {
      return (
        <Select
          mode="multiple"
          allowClear
          aria-label={dictionary.organizationRoleGroupsTitle}
          disabled={!editingRecord || contextLoading || !selectedTenantCode}
          getPopupContainer={platformPopupContainer}
          maxTagCount="responsive"
          optionFilterProp="label"
          options={roleGroupOptions}
          showSearch
        />
      );
    }
    if (resourceKey === "users" && field.key === "tenantCode") {
      return <Input readOnly aria-readonly="true" placeholder={dictionary.userDerivedTenantPending} />;
    }
    if (resourceKey === "users" && scopeType === "platform" && field.key === "orgUnitCode") {
      return <Input readOnly aria-readonly="true" />;
    }
    if (resourceKey === "users" && field.key === "roles") {
      if (scopeType === "platform") {
        return <ReadOnlyListValue emptyText={dictionary.emptyData} />;
      }
      return (
        <Select
          mode="multiple"
          allowClear
          aria-describedby="organization-role-pool-status"
          aria-label={dictionary.userOrganizationRoles}
          disabled={!selectedOrgUnitCode || rolePoolLoading}
          getPopupContainer={platformPopupContainer}
          maxTagCount="responsive"
          optionFilterProp="label"
          options={roleOptions}
          showSearch
          aria-invalid={invalidSelectedRoles.length > 0}
          status={invalidSelectedRoles.length > 0 ? "error" : undefined}
        />
      );
    }
    return fallback;
  }, [active, contextLoading, dictionary, editingRecord, formValues.roleGroupCodes, invalidSelectedRoles.length, roleGroupOptions, roleOptions, rolePoolLoading, resourceKey, runtimeMode, scopeType, selectedOrgUnitCode, selectedRoles, selectedTenantCode]);

  const fieldExtra = useCallback((field: AdminResourceField, fallback: ReactNode) => {
    if (!active) {
      return fallback;
    }
    if (runtimeMode === "loading") {
      return dictionary.organizationContextLoading;
    }
    if (runtimeMode === "readonly") {
      return dictionary.rolePermissionReadonlySchemaDescription;
    }
    if (runtimeMode === "legacy") {
      return fallback;
    }
    if (resourceKey === "org-units" && field.key === "roleGroupCodes") {
      return editingRecord ? dictionary.organizationRoleGroupsHelp : dictionary.organizationCreateBeforeBinding;
    }
    if (resourceKey === "users" && field.key === "tenantCode") {
      return dictionary.userDerivedTenantHelp;
    }
    if (resourceKey === "users" && field.key === "roles" && !selectedOrgUnitCode && scopeType !== "platform") {
      return dictionary.userRolesDisabledUntilOrganization;
    }
    if (editingRecord && field.key === "status") {
      return dictionary.organizationLifecycleManagedStatus;
    }
    return fallback;
  }, [active, dictionary, editingRecord, resourceKey, runtimeMode, scopeType, selectedOrgUnitCode]);

  const submit = useCallback(async (context: ResourceExperienceSubmitContext) => {
    if (runtimeMode === "loading") {
      throw new Error(dictionary.organizationContextLoading);
    }
    return dispatchOrganizationUserWrite(runtimeMode, {
      generic: () => context.persist(context.input),
      readonly: async () => { throw new Error(dictionary.rolePermissionReadonlySchemaDescription); },
      target: () => resourceKey === "org-units"
        ? submitOrganization(context, dictionary, form, modal.confirm, contextLoading, contextError)
        : submitUser(context, dictionary, form, rolePool, scopeType, modal.confirm, contextLoading, contextError, rolePoolLoading, rolePoolError),
    });
  }, [contextError, contextLoading, dictionary, form, modal.confirm, resourceKey, rolePool, rolePoolError, rolePoolLoading, runtimeMode, scopeType]);

  const statusMessage = runtimeMode === "loading"
    ? dictionary.organizationContextLoading
    : runtimeMode === "readonly"
      ? dictionary.rolePermissionReadonlySchemaDescription
      : contextError || rolePoolError
    || (rolePoolLoading ? dictionary.userRolePoolLoading : resourceKey === "users" && selectedOrgUnitCode ? format(dictionary.userRolePoolLoaded, { count: String(rolePool.length) }) : "");
  const invalidRoleStatus = invalidSelectedRoles.length > 0
    ? format(dictionary.userInvalidRolesUnresolved, { roles: invalidSelectedRoles.join(", ") })
    : "";
  const invalidRoleNotice = resourceKey === "users" && invalidSelectedRoles.length > 0 ? (
    <Alert
      type="warning"
      showIcon
      message={dictionary.userInvalidRolesTitle}
      description={(
        <Space size={[4, 4]} wrap>
          {invalidSelectedRoles.map((role) => <Tag color="warning" key={role}>{role}</Tag>)}
        </Space>
      )}
    />
  ) : null;

  return {
    formFields,
    allowDelete: !active,
    allowStatusToggle: !active,
    initialValues: active && runtimeMode !== "legacy" && resourceKey === "users"
      ? (values, record) => record ? values : { ...values, tenantCode: "", orgUnitCode: undefined, roles: [] }
      : undefined,
    renderField,
    fieldExtra,
    submit: active ? submit : undefined,
    formSlots: active && runtimeMode !== "legacy" ? {
      header: invalidRoleNotice,
      footer: (
        <div aria-live="polite" id="organization-role-pool-status" className="organization-experience-status">
          {contextLoading ? <><Spin size="small" /> {dictionary.organizationContextLoading}</> : [statusMessage, invalidRoleStatus].filter(Boolean).join(" ")}
        </div>
      ),
    } : undefined,
    detailTab: active && runtimeCapabilities?.useServiceObjects && resourceKey === "org-units"
      ? (record) => ({
          key: "role-pool",
          label: dictionary.organizationRolePoolProvenance,
          children: <OrganizationRolePoolPanel dictionary={dictionary} record={record} />,
        })
      : undefined,
  };
}

type ConfirmModal = ReturnType<typeof App.useApp>["modal"]["confirm"];

async function submitOrganization(
  context: ResourceExperienceSubmitContext,
  dictionary: Dictionary,
  form: FormInstance<ResourceFormValues>,
  confirm: ConfirmModal,
  contextLoading: boolean,
  contextError: string,
) {
  const selectedGroups = stringList(context.values.roleGroupCodes);
  if (!context.editingRecord && selectedGroups.length > 0) {
    setFieldError(form, "roleGroupCodes", dictionary.organizationCreateBeforeBinding);
    throw new Error(dictionary.organizationCreateBeforeBinding);
  }
  const input = {
    ...context.input,
    values: omitValue(context.input.values, "roleGroupCodes"),
  };
  if (!context.editingRecord) {
    return context.persist(input);
  }
  const currentGroups = stringList(context.editingRecord.values?.roleGroupCodes);
  const bindingChanged = !sameStringSet(currentGroups, selectedGroups);
  if (!bindingChanged) {
    return context.persist(input);
  }
  if (hasMetadataChanges(context.editingRecord, context.values, new Set(["roleGroupCodes", "roleGroupCount", "effectiveRoleCount"]))) {
    setFieldError(form, "roleGroupCodes", dictionary.authorizationMetadataSeparateSave);
    throw new Error(dictionary.authorizationMetadataSeparateSave);
  }
  if (contextLoading || contextError) {
    const message = contextError || dictionary.organizationContextLoading;
    setFieldError(form, "roleGroupCodes", message);
    throw new Error(message);
  }

  const initialPreview = await prepareOrganizationRoleGroupChange(context.editingRecord.code, selectedGroups);
  const initialImpact = await getOrganizationRoleGroupChangeImpact(initialPreview.previewId);
  if (!initialImpact) {
    throw new Error(dictionary.changePreviewUnavailable);
  }
  let preview = initialPreview;
  if (initialImpact.conflictCount > 0) {
    const conflicts = await getOrganizationRoleGroupChangeConflicts(initialPreview.previewId);
    if (conflicts.length !== initialImpact.conflictCount) {
      throw new Error(dictionary.changeConflictDetailsIncomplete);
    }
    const confirmed = await confirmConflicts(confirm, dictionary, conflicts);
    if (!confirmed) {
      throw new ResourceExperienceCancelledError(dictionary.changeCancelled);
    }
    const remediations: OrganizationRoleRemediation[] = conflicts.map((conflict) => ({
      userCode: conflict.userCode,
      roleCode: conflict.roleCode,
      action: "remove-role",
    }));
    preview = await prepareOrganizationRoleGroupChange(context.editingRecord.code, selectedGroups, remediations);
    const remediatedImpact = await getOrganizationRoleGroupChangeImpact(preview.previewId);
    if (!remediatedImpact || remediatedImpact.conflictCount > 0) {
      throw new Error(dictionary.changePreviewUnavailable);
    }
  } else if (!await confirmImpact(confirm, dictionary, initialImpact.affectedUsers, initialImpact.conflictCount)) {
    throw new ResourceExperienceCancelledError(dictionary.changeCancelled);
  }
  await replaceOrganizationRoleGroups(preview);
  return recordWithValues(context.editingRecord, { roleGroupCodes: selectedGroups.join(",") });
}

async function submitUser(
  context: ResourceExperienceSubmitContext,
  dictionary: Dictionary,
  form: FormInstance<ResourceFormValues>,
  rolePool: ReadonlyArray<OrganizationRolePoolItem>,
  scopeType: string,
  confirm: ConfirmModal,
  contextLoading: boolean,
  contextError: string,
  rolePoolLoading: boolean,
  rolePoolError: string,
) {
  const orgUnitCode = String(context.values.orgUnitCode ?? "").trim();
  const roleCodes = stringList(context.values.roles);
  if (scopeType !== "platform" && !orgUnitCode) {
    setFieldError(form, "orgUnitCode", dictionary.userOrganizationRequired);
    throw new Error(dictionary.userOrganizationRequired);
  }
  const allowed = new Set(rolePool.map((item) => item.roleCode));
  const invalidSelected = scopeType === "platform" ? [] : roleCodes.filter((role) => !allowed.has(role));
  if (invalidSelected.length > 0) {
    const message = format(dictionary.userInvalidRolesUnresolved, { roles: invalidSelected.join(", ") });
    setFieldError(form, "roles", message);
    throw new Error(message);
  }
  if (!context.editingRecord) {
    return context.persist(context.input);
  }
  const currentOrgUnitCode = String(context.editingRecord.values?.orgUnitCode ?? "");
  const currentRoles = stringList(context.editingRecord.values?.roles);
  const authorizationChanged = currentOrgUnitCode !== orgUnitCode || !sameStringSet(currentRoles, roleCodes);
  if (!authorizationChanged) {
    return context.persist(context.input);
  }
  if (hasMetadataChanges(context.editingRecord, context.values, new Set(["scopeType", "tenantCode", "orgUnitCode", "role", "roles"]))) {
    setFieldError(form, "orgUnitCode", dictionary.authorizationMetadataSeparateSave);
    throw new Error(dictionary.authorizationMetadataSeparateSave);
  }
  if (scopeType === "platform") {
    throw new Error(dictionary.platformUserAuthorizationReadOnly);
  }
  if (contextLoading || rolePoolLoading || contextError || rolePoolError) {
    const message = contextError || rolePoolError || dictionary.userRolePoolLoading;
    setFieldError(form, "roles", message);
    throw new Error(message);
  }
  const invalidCurrent = currentRoles.filter((role) => !allowed.has(role));
  if (invalidCurrent.length > 0 && !await confirmRoleRemoval(confirm, dictionary, invalidCurrent)) {
    throw new ResourceExperienceCancelledError(dictionary.changeCancelled);
  }
  const remediations: OrganizationRoleRemediation[] = invalidCurrent.map((roleCode) => ({
    userCode: context.editingRecord?.code ?? "",
    roleCode,
    action: "remove-role",
  }));
  const preview = await prepareUserOrganizationChange(context.editingRecord.code, orgUnitCode, roleCodes, remediations);
  const impact = await getUserOrganizationChangeImpact(preview.previewId);
  if (!impact) {
    throw new Error(dictionary.changePreviewUnavailable);
  }
  if (!await confirmImpact(confirm, dictionary, impact.affectedUsers, impact.conflictCount)) {
    throw new ResourceExperienceCancelledError(dictionary.changeCancelled);
  }
  await changeUserOrganization(preview);
  return recordWithValues(context.editingRecord, {
    tenantCode: String(context.values.tenantCode ?? ""),
    orgUnitCode,
    role: roleCodes[0] ?? "",
    roles: roleCodes.join(","),
  });
}

function OrganizationRolePoolPanel({ dictionary, record }: { dictionary: Dictionary; record: AdminResourceRecord }) {
  const [items, setItems] = useState<ReadonlyArray<OrganizationRolePoolItem>>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError("");
    getOrganizationRolePool(record.code)
      .then((next) => {
        if (!cancelled) setItems(next);
      })
      .catch((nextError: unknown) => {
        if (!cancelled) setError(nextError instanceof Error ? nextError.message : dictionary.rolePoolLoadFailed);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [dictionary.rolePoolLoadFailed, record.code]);
  if (loading) {
    return <div className="organization-role-pool-loading" aria-live="polite"><Spin size="small" /> {dictionary.userRolePoolLoading}</div>;
  }
  if (error) {
    return <Alert type="warning" showIcon message={dictionary.rolePoolLoadFailed} description={error} />;
  }
  if (items.length === 0) {
    return (
      <section className="organization-role-pool-panel" aria-label={dictionary.organizationRolePoolSummary}>
        <div className="organization-role-pool-header">
          <div>
            <Typography.Text strong>{dictionary.organizationRolePoolSummary}</Typography.Text>
            <Typography.Text type="secondary">{dictionary.organizationRolePoolDescription}</Typography.Text>
          </div>
          <Tag>{dictionary.organizationRolePoolEmpty}</Tag>
        </div>
      </section>
    );
  }
  const groupCount = new Set(items.map((item) => item.roleGroupCode)).size;
  return (
    <section className="organization-role-pool-panel" aria-label={dictionary.organizationRolePoolSummary}>
      <div className="organization-role-pool-header">
        <div>
          <Typography.Text strong>{dictionary.organizationRolePoolSummary}</Typography.Text>
          <Typography.Text type="secondary">{dictionary.organizationRolePoolDescription}</Typography.Text>
        </div>
        <Tag color="blue">{items.length}</Tag>
      </div>
      <dl className="organization-role-pool-metrics">
        <div><dt>{dictionary.organizationRolePoolRoleCount}</dt><dd>{items.length}</dd></div>
        <div><dt>{dictionary.organizationRolePoolGroupCount}</dt><dd>{groupCount}</dd></div>
      </dl>
      <ul className="organization-role-pool-list" aria-label={dictionary.organizationRolePoolProvenance}>
        {items.map((item) => (
          <li className="organization-role-pool-item" key={item.roleCode}>
            <div><strong>{item.roleName || item.roleCode}</strong><Typography.Text code>{item.roleCode}</Typography.Text></div>
            <Typography.Text type="secondary">{item.roleGroupName} ({item.roleGroupCode})</Typography.Text>
          </li>
        ))}
      </ul>
    </section>
  );
}

function ReadOnlyListValue({ id, value = [], emptyText }: { id?: string; value?: string[]; emptyText: string }) {
  return (
    <div className="organization-readonly-list" id={id} role="textbox" aria-readonly="true" tabIndex={0}>
      {value.length > 0 ? value.map((item) => <Tag key={item}>{item}</Tag>) : <Typography.Text type="secondary">{emptyText}</Typography.Text>}
    </div>
  );
}

function confirmImpact(confirm: ConfirmModal, dictionary: Dictionary, affectedUsers: number, conflictCount: number) {
  return confirmModal(
    confirm,
    dictionary.changeImpactTitle,
    format(dictionary.changeImpactDescription, { affectedUsers: String(affectedUsers), conflictCount: String(conflictCount) }),
    dictionary.reviewAndApply,
    dictionary.cancel,
  );
}

function confirmRoleRemoval(confirm: ConfirmModal, dictionary: Dictionary, roles: string[]) {
  return confirmModal(
    confirm,
    dictionary.userInvalidRolesTitle,
    format(dictionary.userInvalidRoleRemovalConfirm, { roles: roles.join(", ") }),
    dictionary.userInvalidRoleRemove,
    dictionary.cancel,
  );
}

function confirmConflicts(confirm: ConfirmModal, dictionary: Dictionary, conflicts: ReadonlyArray<OrganizationChangeConflict>) {
  return confirmModal(
    confirm,
    dictionary.organizationBindingImpactTitle,
    <div>
      <Typography.Paragraph>{format(dictionary.organizationBindingConflictDescription, { count: String(conflicts.length) })}</Typography.Paragraph>
      <ul className="organization-conflict-list">
        {conflicts.map((conflict) => <li key={`${conflict.userCode}:${conflict.roleCode}`}>{conflict.userCode} · {conflict.roleCode}</li>)}
      </ul>
    </div>,
    dictionary.userInvalidRoleRemove,
    dictionary.cancel,
  );
}

function confirmModal(confirm: ConfirmModal, title: string, content: ReactNode, okText: string, cancelText: string): Promise<boolean> {
  return new Promise((resolve) => {
    let settled = false;
    const finish = (value: boolean) => {
      if (!settled) {
        settled = true;
        resolve(value);
      }
    };
    confirm({
      title,
      content,
      okText,
      cancelText,
      autoFocusButton: "cancel",
      onOk: () => finish(true),
      onCancel: () => finish(false),
      afterClose: () => finish(false),
    });
  });
}

function setFieldError(form: FormInstance<ResourceFormValues>, field: string, message: string) {
  form.setFields([{ name: field, errors: [message] }]);
  form.scrollToField(field, { block: "center" });
  requestAnimationFrame(() => {
    document.querySelector<HTMLElement>(`.resource-form-fields [id$="_${field}"]`)?.focus();
  });
}

function hasMetadataChanges(record: AdminResourceRecord, values: ResourceFormValues, ignoredKeys: ReadonlySet<string>) {
  for (const [key, value] of Object.entries(values)) {
    if (ignoredKeys.has(key)) {
      continue;
    }
    const current = key === "code" || key === "name" || key === "status" || key === "description"
      ? record[key]
      : record.values?.[key];
    if (normalizeComparableValue(value) !== normalizeComparableValue(current)) {
      return true;
    }
  }
  return false;
}

function normalizeComparableValue(value: unknown) {
  if (Array.isArray(value)) {
    return value.map(String).map((item) => item.trim()).filter(Boolean).sort().join(",");
  }
  return String(value ?? "").trim();
}

function recordWithValues(record: AdminResourceRecord, values: Record<string, string>): AdminResourceRecord {
  return {
    ...record,
    values: {
      ...record.values,
      ...values,
    },
  };
}

function stringList(value: unknown) {
  if (Array.isArray(value)) {
    return [...new Set(value.map(String).map((item) => item.trim()).filter(Boolean))].sort();
  }
  return [...new Set(String(value ?? "").split(",").map((item) => item.trim()).filter(Boolean))].sort();
}

function sameStringSet(left: string[], right: string[]) {
  return left.length === right.length && left.every((value, index) => value === right[index]);
}

function omitValue(values: Record<string, string> | undefined, key: string) {
  if (!values) return undefined;
  const next = { ...values };
  delete next[key];
  return Object.keys(next).length > 0 ? next : undefined;
}

function mergeSelectOptions(primary: Array<{ value: string; label: string }>, fallback: Array<{ value: string; label: string }>) {
  const seen = new Set<string>();
  return [...primary, ...fallback].filter((option) => {
    if (!option.value || seen.has(option.value)) return false;
    seen.add(option.value);
    return true;
  });
}

function toFieldOption(option: { value: string; label: string }) {
  return { value: option.value, label: { zh: option.label, en: option.label } };
}

function format(template: string, values: Record<string, string>) {
  return Object.entries(values).reduce((result, [key, value]) => result.replaceAll(`{${key}}`, value), template);
}
