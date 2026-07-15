import {
  AppstoreOutlined,
  EditOutlined,
  LockOutlined,
  PlusOutlined,
  SafetyCertificateOutlined,
  StopOutlined,
  SwapOutlined,
} from "@ant-design/icons";
import { App, Button, Descriptions, Form, Input, InputNumber, Modal, Segmented, Select, Space, Tag, Typography } from "antd";
import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import {
  createAdminResource,
  queryAdminResource,
  updateAdminResource,
  type AdminResourceInput,
  type AdminResourceRecord,
} from "../api/client";
import {
  applyRoleStateOrGroupChange,
  getRolePermissionChangeImpact,
  getRoleStateOrGroupChangeConflicts,
  getRoleStateOrGroupChangeImpact,
  prepareRolePermissionChange,
  prepareRoleStateOrGroupChange,
  replaceRolePermissions,
  type OrganizationRoleRemediation,
  type RoleChangeConflict,
} from "../api/organizationRBAC";
import type { Dictionary, Language } from "../i18n";
import { hasPermission } from "../refine";
import {
  AdminActionButton,
  AdminFeedback,
  AdminFormModal,
  AdminListPanel,
  AdminPage,
  AdminTreeWorkbench,
  PlatformTreeSelect,
  PlatformTreeTransfer,
  platformPopupContainer,
  type AdminTreeWorkbenchNode,
  type PlatformTreeTransferNode,
} from "../ui";
import type { AdminResourceDefinition } from "./registry";

type RoleGovernanceConsoleProps = {
  resource: AdminResourceDefinition;
  language: Language;
  dictionary: Dictionary;
  permissions: string[];
  deniedPermissions: string[];
};

type EditorState = {
  kind: "group" | "role";
  record?: AdminResourceRecord;
  groupCode?: string;
};

type AuthorizationState = {
  role: AdminResourceRecord;
  allow: string[];
  deny: string[];
  dataScope: string;
  dataScopeOrgCodes: string[];
  dataScopeAreaCodes: string[];
};

type MetadataValues = {
  code: string;
  name: string;
  description?: string;
  scopeType?: string;
  tenantCode?: string;
  groupCode?: string;
  sortOrder?: number;
};

export function RoleGovernanceConsole({ resource, language, dictionary, permissions, deniedPermissions }: RoleGovernanceConsoleProps) {
  const { modal } = App.useApp();
  const [groups, setGroups] = useState<AdminResourceRecord[]>([]);
  const [roles, setRoles] = useState<AdminResourceRecord[]>([]);
  const [tenants, setTenants] = useState<AdminResourceRecord[]>([]);
  const [permissionCatalog, setPermissionCatalog] = useState<AdminResourceRecord[]>([]);
  const [menus, setMenus] = useState<AdminResourceRecord[]>([]);
  const [orgUnits, setOrgUnits] = useState<AdminResourceRecord[]>([]);
  const [areaCodes, setAreaCodes] = useState<AdminResourceRecord[]>([]);
  const [selectedKey, setSelectedKey] = useState("");
  const [search, setSearch] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [acting, setActing] = useState("");
  const [editor, setEditor] = useState<EditorState | null>(null);
  const [moveRole, setMoveRole] = useState<AdminResourceRecord | null>(null);
  const [moveTargetGroup, setMoveTargetGroup] = useState("");
  const [authorization, setAuthorization] = useState<AuthorizationState | null>(null);
  const [permissionMode, setPermissionMode] = useState<"allow" | "deny">("allow");
  const [menuRole, setMenuRole] = useState<AdminResourceRecord | null>(null);
  const [metadataForm] = Form.useForm<MetadataValues>();
  const authorizationTriggerRef = useRef<HTMLButtonElement | null>(null);
  const menuTriggerRef = useRef<HTMLButtonElement | null>(null);
  const governanceRequest = useRef(0);
  const canReadGroups = hasPermission(permissions, "admin:role-group:read", deniedPermissions);
  const canReadRoles = hasPermission(permissions, "admin:role:read", deniedPermissions);
  const canReadTenants = hasPermission(permissions, "admin:tenant:read", deniedPermissions);
  const canCreateGroup = hasPermission(permissions, "admin:role-group:create", deniedPermissions) && canReadTenants;
  const canUpdateGroup = hasPermission(permissions, "admin:role-group:update", deniedPermissions);
  const canCreateRole = hasPermission(permissions, "admin:role:create", deniedPermissions);
  const canUpdateRole = hasPermission(permissions, "admin:role:update", deniedPermissions);
  const canReadAuthorizationInputs = hasPermission(permissions, "admin:permission:read", deniedPermissions) && hasPermission(permissions, "admin:org-unit:read", deniedPermissions) && hasPermission(permissions, "admin:area-code:read", deniedPermissions);
  const canReadMenus = hasPermission(permissions, "admin:menu:read", deniedPermissions);

  const loadGovernance = useCallback(async (query = "", requestID = ++governanceRequest.current) => {
    if (governanceRequest.current !== requestID) return;
    setLoading(true);
    try {
      const [nextGroups, nextRoles] = await Promise.all([
        canReadGroups ? loadAllRecords("role-groups") : Promise.resolve([]),
        canReadRoles ? loadAllRecords("roles", query ? [query] : undefined) : Promise.resolve([]),
      ]);
      if (governanceRequest.current !== requestID) return;
      setGroups(nextGroups);
      setRoles(nextRoles);
      setError("");
      setSelectedKey((current) => {
        if (current && [...nextGroups, ...nextRoles].some((record) => nodeKey(record, nextGroups.includes(record) ? "group" : "role") === current)) return current;
        const preferred = resource.route === "/role-groups" ? nextGroups[0] : nextRoles[0] ?? nextGroups[0];
        return preferred ? nodeKey(preferred, nextGroups.includes(preferred) ? "group" : "role") : "";
      });
    } catch (nextError) {
      if (governanceRequest.current !== requestID) return;
      setError(errorMessage(nextError, dictionary.loadResourceFailed));
    } finally {
      if (governanceRequest.current === requestID) setLoading(false);
    }
  }, [canReadGroups, canReadRoles, dictionary.loadResourceFailed, resource.route]);

  useEffect(() => {
    const requestID = ++governanceRequest.current;
    const timer = window.setTimeout(() => void loadGovernance(search, requestID), 250);
    return () => window.clearTimeout(timer);
  }, [loadGovernance, search]);

  const groupByCode = useMemo(() => new Map(groups.map((group) => [group.code, group])), [groups]);
  const selected = useMemo(() => selectedRecord(selectedKey, groups, roles), [groups, roles, selectedKey]);
  const invalidGroups = useMemo(() => groups.filter((group) => valueOf(group, "parentCode")), [groups]);
  const invalidRoles = useMemo(() => roles.filter((role) => !valueOf(role, "groupCode") || canReadGroups && !groupByCode.has(valueOf(role, "groupCode"))), [canReadGroups, groupByCode, roles]);
  const nodes = useMemo(() => roleTreeNodes(groups, roles, search, !canReadGroups), [canReadGroups, groups, roles, search]);
  const moveSourceGroup = moveRole ? groupByCode.get(valueOf(moveRole, "groupCode")) : undefined;
  const moveTargetOptions = useMemo(
    () => moveSourceGroup
      ? groups.filter(enabled).filter((group) => group.code !== moveSourceGroup.code && sameRoleGroupBoundary(group, moveSourceGroup)).map(recordOption)
      : [],
    [groups, moveSourceGroup],
  );

  const openEditor = useCallback(async (state: EditorState) => {
    setEditor(state);
    if (state.kind === "group" && !state.record && tenants.length === 0) {
      try { setTenants(await loadAllRecords("tenants")); } catch (nextError) { setError(errorMessage(nextError, dictionary.loadResourceFailed)); }
    }
  }, [dictionary.loadResourceFailed, tenants.length]);

  useEffect(() => {
    if (!editor) return;
    const record = editor.record;
    metadataForm.setFieldsValue({
      code: record?.code ?? "",
      name: record?.name ?? "",
      description: record?.description ?? "",
      scopeType: valueOf(record, "scopeType") || "tenant",
      tenantCode: valueOf(record, "tenantCode"),
      groupCode: valueOf(record, "groupCode") || editor.groupCode,
      sortOrder: numberValue(record, "sortOrder"),
    });
  }, [editor, metadataForm]);

  const saveMetadata = async (values: MetadataValues) => {
    if (!editor) return;
    setActing("metadata");
    try {
      const input = metadataInput(editor, values);
      const result = editor.record
        ? await updateAdminResource(editor.kind === "group" ? "role-groups" : "roles", editor.record.id, input)
        : await createAdminResource(editor.kind === "group" ? "role-groups" : "roles", input);
      setEditor(null);
      setNotice(dictionary.roleGovernanceMetadataSaved);
      await loadGovernance(search);
      setSelectedKey(nodeKey(result.record, editor.kind));
    } catch (nextError) {
      setError(errorMessage(nextError, dictionary.roleGovernanceSaveFailed));
    } finally {
      setActing("");
    }
  };

  const openAuthorization = async (role: AdminResourceRecord) => {
    if (!canReadAuthorizationInputs) return;
    try {
      const [nextPermissions, nextOrgUnits, nextAreaCodes] = await Promise.all([
        permissionCatalog.length ? permissionCatalog : loadAllRecords("permissions"),
        orgUnits.length ? orgUnits : loadAllRecords("org-units"),
        areaCodes.length ? areaCodes : loadAllRecords("area-codes"),
      ]);
      setPermissionCatalog(nextPermissions);
      setOrgUnits(nextOrgUnits);
      setAreaCodes(nextAreaCodes);
      setPermissionMode("allow");
      setAuthorization({
        role,
        allow: csv(valueOf(role, "permissions")),
        deny: csv(valueOf(role, "denyPermissions")),
        dataScope: valueOf(role, "dataScope") || "all",
        dataScopeOrgCodes: csv(valueOf(role, "dataScopeOrgCodes")),
        dataScopeAreaCodes: csv(valueOf(role, "dataScopeAreaCodes")),
      });
    } catch (nextError) {
      setError(errorMessage(nextError, dictionary.loadResourceFailed));
    }
  };

  const saveAuthorization = async () => {
    if (!authorization) return;
    const overlap = authorization.allow.filter((code) => authorization.deny.includes(code));
    if (overlap.length > 0) {
      setError(dictionary.rolePermissionOverlap.replace("{permissions}", overlap.join(", ")));
      return;
    }
    setActing("authorization");
    try {
      const preview = await prepareRolePermissionChange(authorization.role.code, {
        allowPermissionCodes: authorization.allow,
        denyPermissionCodes: authorization.deny,
        dataScope: authorization.dataScope,
        dataScopeOrgCodes: authorization.dataScope === "custom_orgs" ? authorization.dataScopeOrgCodes : [],
        dataScopeAreaCodes: authorization.dataScope === "custom_areas" ? authorization.dataScopeAreaCodes : [],
      });
      const impact = await getRolePermissionChangeImpact(preview.previewId);
      if (!impact || !await confirmImpact(modal.confirm, dictionary, impact.affectedUsers, impact.conflictCount)) return;
      await replaceRolePermissions(preview);
      setAuthorization(null);
      setNotice(dictionary.roleAuthorizationSaved);
      await loadGovernance(search);
    } catch (nextError) {
      setError(errorMessage(nextError, dictionary.roleGovernanceSaveFailed));
    } finally {
      setActing("");
    }
  };

  const openMenus = async (role: AdminResourceRecord) => {
    if (!canReadMenus) return;
    try {
      if (menus.length === 0) setMenus(await loadAllRecords("menus"));
      setMenuRole(role);
    } catch (nextError) {
      setError(errorMessage(nextError, dictionary.loadResourceFailed));
    }
  };

  const executeRoleChange = async (role: AdminResourceRecord, operation: "move" | "disable", targetGroupCode?: string) => {
    setActing(operation);
    try {
      let preview = await prepareRoleStateOrGroupChange(role.code, operation, targetGroupCode);
      let impact = await getRoleStateOrGroupChangeImpact(preview.previewId);
      if (!impact) throw new Error(dictionary.changePreviewUnavailable);
      if (impact.conflictCount > 0) {
        const conflicts = await getRoleStateOrGroupChangeConflicts(preview.previewId);
        if (conflicts.length !== impact.conflictCount || !await confirmRoleConflicts(modal.confirm, dictionary, conflicts)) return;
        const remediations: OrganizationRoleRemediation[] = conflicts.map((conflict) => ({ userCode: conflict.userCode, roleCode: conflict.roleCode, action: "remove-role" }));
        preview = await prepareRoleStateOrGroupChange(role.code, operation, targetGroupCode, remediations);
        impact = await getRoleStateOrGroupChangeImpact(preview.previewId);
        if (!impact || impact.conflictCount > 0) throw new Error(dictionary.changePreviewUnavailable);
      } else if (!await confirmImpact(modal.confirm, dictionary, impact.affectedUsers, impact.conflictCount)) {
        return;
      }
      await applyRoleStateOrGroupChange(preview, operation);
      setMoveRole(null);
      setMoveTargetGroup("");
      setNotice(operation === "move" ? dictionary.roleMoveSucceeded : dictionary.roleDisableSucceeded);
      await loadGovernance(search);
    } catch (nextError) {
      setError(errorMessage(nextError, dictionary.roleGovernanceSaveFailed));
    } finally {
      setActing("");
    }
  };

  const details = selected ? (
    <RoleGovernanceDetail
      canCreateRole={canCreateRole}
      canReadAuthorizationInputs={canReadAuthorizationInputs}
      canReadMenus={canReadMenus}
      canUpdateGroup={canUpdateGroup}
      canUpdateRole={canUpdateRole}
      dictionary={dictionary}
      groupByCode={groupByCode}
      record={selected.record}
      type={selected.type}
      onAssignMenus={openMenus}
      onAssignPermissions={openAuthorization}
      onCreateRole={(groupCode) => void openEditor({ kind: "role", groupCode })}
      onDisable={(role) => void executeRoleChange(role, "disable")}
      onEdit={(kind, record) => void openEditor({ kind, record })}
      onMove={(role) => { setMoveRole(role); setMoveTargetGroup(""); }}
      authorizationTriggerRef={authorizationTriggerRef}
      menuTriggerRef={menuTriggerRef}
    />
  ) : <AdminListPanel className="role-governance-detail" title={dictionary.roleGovernanceDetail}><div className="role-governance-empty">{dictionary.emptyData}</div></AdminListPanel>;

  return (
    <AdminPage title={dictionary.roleGovernanceTitle} description={dictionary.roleGovernanceDescription}>
      {error ? <AdminFeedback className="api-alert" type="warning" message={dictionary.roleGovernanceSaveFailed} description={error} closable onClose={() => setError("")} /> : null}
      {notice ? <AdminFeedback className="api-alert" type="success" message={notice} closable onClose={() => setNotice("")} /> : null}
      {invalidGroups.length > 0 || invalidRoles.length > 0 ? (
        <AdminFeedback
          className="api-alert"
          type="error"
          message={dictionary.roleTreeInvalidTitle}
          description={dictionary.roleTreeInvalidDescription
            .replace("{groups}", invalidGroups.map((record) => record.code).join(", ") || "-")
            .replace("{roles}", invalidRoles.map((record) => record.code).join(", ") || "-")}
        />
      ) : null}
      <AdminTreeWorkbench
        actions={(
          <Space size={6} wrap>
            {canCreateGroup ? <AdminActionButton icon={<PlusOutlined />} label={dictionary.roleGroupAdd} onClick={() => void openEditor({ kind: "group" })}>{dictionary.roleGroupAdd}</AdminActionButton> : null}
            {canCreateRole ? <AdminActionButton icon={<PlusOutlined />} label={dictionary.roleAdd} type="primary" onClick={() => void openEditor({ kind: "role" })}>{dictionary.roleAdd}</AdminActionButton> : null}
          </Space>
        )}
        ariaLabel={dictionary.roleTreeAriaLabel}
        detail={details}
        emptyText={dictionary.emptyData}
        loading={loading}
        nodes={nodes}
        searchLabel={dictionary.roleTreeSearch}
        searchPlaceholder={dictionary.roleTreeSearchPlaceholder}
        searchValue={search}
        selectedKey={selectedKey}
        title={dictionary.roleTreeTitle}
        onSearchChange={setSearch}
        onSelect={setSelectedKey}
      />

      <AdminFormModal
        confirmLoading={acting === "metadata"}
        okText={dictionary.save}
        open={Boolean(editor)}
        title={editor?.kind === "group" ? dictionary.roleGroupMetadata : dictionary.roleMetadata}
        onCancel={() => setEditor(null)}
        onOk={() => metadataForm.submit()}
      >
        <Form form={metadataForm} layout="vertical" onFinish={saveMetadata}>
          <Form.Item label={dictionary.code} name="code" rules={[{ required: true }]}><Input disabled={Boolean(editor?.record)} /></Form.Item>
          <Form.Item label={dictionary.recordName} name="name" rules={[{ required: true }]}><Input /></Form.Item>
          {editor?.kind === "group" ? (
            <>
              <Form.Item label={dictionary.roleGroupScope} name="scopeType" rules={[{ required: true }]}>
                <Select disabled={Boolean(editor.record)} getPopupContainer={platformPopupContainer} options={[{ value: "platform", label: dictionary.roleGroupScopePlatform }, { value: "tenant", label: dictionary.roleGroupScopeTenant }]} />
              </Form.Item>
              <Form.Item noStyle shouldUpdate={(before, after) => before.scopeType !== after.scopeType}>
                {({ getFieldValue }) => getFieldValue("scopeType") === "tenant" ? (
                  <Form.Item label={dictionary.tenantContext} name="tenantCode" rules={[{ required: true }]}>
                    <Select disabled={Boolean(editor.record)} getPopupContainer={platformPopupContainer} optionFilterProp="label" options={tenants.map(recordOption)} showSearch />
                  </Form.Item>
                ) : null}
              </Form.Item>
              <Form.Item label={dictionary.roleGroupSortOrder} name="sortOrder"><InputNumber min={0} /></Form.Item>
            </>
          ) : (
            <Form.Item label={dictionary.roleGroupMetadata} name="groupCode" rules={[{ required: true }]}>
              <Select disabled={Boolean(editor?.record)} getPopupContainer={platformPopupContainer} optionFilterProp="label" options={groups.filter(enabled).map(recordOption)} showSearch />
            </Form.Item>
          )}
          <Form.Item label={dictionary.description} name="description"><Input.TextArea rows={3} /></Form.Item>
        </Form>
      </AdminFormModal>

      <Modal
        confirmLoading={acting === "move"}
        okText={dictionary.reviewAndApply}
        open={Boolean(moveRole)}
        title={dictionary.roleMoveTitle}
        okButtonProps={{ disabled: !moveTargetGroup }}
        onCancel={() => setMoveRole(null)}
        onOk={() => moveRole && void executeRoleChange(moveRole, "move", moveTargetGroup)}
      >
        <Select
          aria-label={dictionary.roleMoveTargetGroup}
          getPopupContainer={platformPopupContainer}
          optionFilterProp="label"
          options={moveTargetOptions}
          placeholder={dictionary.roleMoveTargetGroup}
          showSearch
          value={moveTargetGroup || undefined}
          onChange={setMoveTargetGroup}
        />
      </Modal>

      <AuthorizationModal
        acting={acting === "authorization"}
        areaCodes={areaCodes}
        authorization={authorization}
        dictionary={dictionary}
        mode={permissionMode}
        orgUnits={orgUnits}
        permissionCatalog={permissionCatalog}
        returnFocusRef={authorizationTriggerRef}
        onAuthorizationChange={setAuthorization}
        onCancel={() => setAuthorization(null)}
        onModeChange={setPermissionMode}
        onSave={() => void saveAuthorization()}
      />

      <MenuVisibilityModal
        dictionary={dictionary}
        menus={menus}
        role={menuRole}
        returnFocusRef={menuTriggerRef}
        onClose={() => setMenuRole(null)}
      />
    </AdminPage>
  );
}

function RoleGovernanceDetail({
  record,
  type,
  groupByCode,
  dictionary,
  canCreateRole,
  canReadAuthorizationInputs,
  canReadMenus,
  canUpdateGroup,
  canUpdateRole,
  authorizationTriggerRef,
  menuTriggerRef,
  onEdit,
  onCreateRole,
  onMove,
  onDisable,
  onAssignPermissions,
  onAssignMenus,
}: {
  record: AdminResourceRecord;
  type: "group" | "role";
  groupByCode: Map<string, AdminResourceRecord>;
  dictionary: Dictionary;
  canCreateRole: boolean;
  canReadAuthorizationInputs: boolean;
  canReadMenus: boolean;
  canUpdateGroup: boolean;
  canUpdateRole: boolean;
  authorizationTriggerRef: React.RefObject<HTMLButtonElement>;
  menuTriggerRef: React.RefObject<HTMLButtonElement>;
  onEdit: (kind: "group" | "role", record: AdminResourceRecord) => void;
  onCreateRole: (groupCode: string) => void;
  onMove: (role: AdminResourceRecord) => void;
  onDisable: (role: AdminResourceRecord) => void;
  onAssignPermissions: (role: AdminResourceRecord) => void;
  onAssignMenus: (role: AdminResourceRecord) => void;
}) {
  const group = type === "role" ? groupByCode.get(valueOf(record, "groupCode")) : undefined;
  return (
    <AdminListPanel
      className="role-governance-detail"
      title={type === "group" ? dictionary.roleGroupMetadata : dictionary.roleMetadata}
      actions={(
        <Space size={6} wrap>
          {type === "group" && canCreateRole ? <AdminActionButton icon={<PlusOutlined />} label={dictionary.roleAdd} onClick={() => onCreateRole(record.code)}>{dictionary.roleAdd}</AdminActionButton> : null}
          {(type === "group" ? canUpdateGroup : canUpdateRole) ? <AdminActionButton icon={<EditOutlined />} label={dictionary.editRecord} onClick={() => onEdit(type, record)}>{dictionary.editRecord}</AdminActionButton> : null}
        </Space>
      )}
    >
      <div className="role-governance-detail-body">
        <div className="role-governance-detail-heading">
          <div><Typography.Title level={3}>{record.name}</Typography.Title><Typography.Text code>{record.code}</Typography.Text></div>
          <Tag color={record.status === "enabled" ? "success" : "default"}>{record.status}</Tag>
        </div>
        <Descriptions column={1} size="small">
          {type === "group" ? (
            <>
              <Descriptions.Item label={dictionary.roleGroupScope}>{valueOf(record, "scopeType") || "-"}</Descriptions.Item>
              <Descriptions.Item label={dictionary.tenantContext}>{valueOf(record, "tenantCode") || dictionary.roleGroupScopePlatform}</Descriptions.Item>
              <Descriptions.Item label={dictionary.roleGroupSortOrder}>{valueOf(record, "sortOrder") || "0"}</Descriptions.Item>
            </>
          ) : (
            <>
              <Descriptions.Item label={dictionary.roleGroupMetadata}>{group ? `${group.name} (${group.code})` : valueOf(record, "groupCode") || "-"}</Descriptions.Item>
              <Descriptions.Item label={dictionary.roleDataScope}>{valueOf(record, "dataScope") || "-"}</Descriptions.Item>
              <Descriptions.Item label={dictionary.rolePermissionAllow}>{csv(valueOf(record, "permissions")).length}</Descriptions.Item>
              <Descriptions.Item label={dictionary.rolePermissionDeny}>{csv(valueOf(record, "denyPermissions")).length}</Descriptions.Item>
            </>
          )}
          <Descriptions.Item label={dictionary.description}>{record.description || "-"}</Descriptions.Item>
        </Descriptions>
        {type === "role" ? (
          <div className="role-governance-command-bar">
            <Button disabled={!canUpdateRole || record.status !== "enabled"} icon={<SwapOutlined />} onClick={() => onMove(record)}>{dictionary.roleMove}</Button>
            <Button danger disabled={!canUpdateRole || record.status !== "enabled"} icon={<StopOutlined />} onClick={() => onDisable(record)}>{dictionary.roleDisable}</Button>
            {canReadMenus ? <Button ref={menuTriggerRef} icon={<AppstoreOutlined />} onClick={() => onAssignMenus(record)}>{dictionary.assignMenus}</Button> : null}
            {canReadAuthorizationInputs ? <Button ref={authorizationTriggerRef} disabled={!canUpdateRole || record.status !== "enabled"} icon={<SafetyCertificateOutlined />} type="primary" onClick={() => onAssignPermissions(record)}>{dictionary.assignPermissions}</Button> : null}
          </div>
        ) : (
          <Typography.Text type="secondary">{dictionary.roleGroupNoGrant}</Typography.Text>
        )}
      </div>
    </AdminListPanel>
  );
}

function AuthorizationModal({
  authorization,
  mode,
  acting,
  permissionCatalog,
  orgUnits,
  areaCodes,
  dictionary,
  returnFocusRef,
  onModeChange,
  onAuthorizationChange,
  onCancel,
  onSave,
}: {
  authorization: AuthorizationState | null;
  mode: "allow" | "deny";
  acting: boolean;
  permissionCatalog: AdminResourceRecord[];
  orgUnits: AdminResourceRecord[];
  areaCodes: AdminResourceRecord[];
  dictionary: Dictionary;
  returnFocusRef: React.RefObject<HTMLElement>;
  onModeChange: (mode: "allow" | "deny") => void;
  onAuthorizationChange: (state: AuthorizationState | null) => void;
  onCancel: () => void;
  onSave: () => void;
}) {
  if (!authorization) return null;
  const nodes = permissionTreeNodes(permissionCatalog, dictionary, uniqueSorted([...authorization.allow, ...authorization.deny]));
  const selected = mode === "allow" ? authorization.allow : authorization.deny;
  const updateSelected = (next: string[]) => {
    if (mode === "allow") onAuthorizationChange({ ...authorization, allow: next, deny: authorization.deny.filter((code) => !next.includes(code)) });
    else onAuthorizationChange({ ...authorization, deny: next, allow: authorization.allow.filter((code) => !next.includes(code)) });
  };
  return (
    <Modal
      className="role-authorization-modal"
      confirmLoading={acting}
      destroyOnHidden
      okText={dictionary.reviewAndApply}
      open
      title={`${dictionary.assignPermissions}: ${authorization.role.name}`}
      width={1080}
      onCancel={onCancel}
      onOk={onSave}
    >
      <div className="role-authorization-layout">
        <div className="role-authorization-toolbar">
          <Segmented
            aria-label={dictionary.rolePermissionMode}
            options={[{ label: dictionary.rolePermissionAllow, value: "allow" }, { label: dictionary.rolePermissionDeny, value: "deny" }]}
            value={mode}
            onChange={(value) => onModeChange(value as "allow" | "deny")}
          />
          <Tag color={mode === "deny" ? "error" : "blue"}>{mode === "deny" ? dictionary.rolePermissionDeny : dictionary.rolePermissionAllow}</Tag>
        </div>
        <PlatformTreeTransfer
          ariaLabel={dictionary.assignPermissions}
          labels={transferLabels(dictionary)}
          nodes={nodes}
          returnFocusRef={returnFocusRef}
          value={selected}
          onChange={updateSelected}
        />
        <div className="role-data-scope-panel">
          <Typography.Text strong>{dictionary.roleDataScope}</Typography.Text>
          <Select
            aria-label={dictionary.roleDataScope}
            getPopupContainer={platformPopupContainer}
            options={dataScopeOptions(dictionary)}
            value={authorization.dataScope}
            onChange={(dataScope) => onAuthorizationChange({ ...authorization, dataScope })}
          />
          {authorization.dataScope === "custom_orgs" ? (
            <PlatformTreeSelect
              aria-label={dictionary.roleDataScopeOrgs}
              multiple
              options={orgUnits.map((record) => treeRecordOption(record))}
              value={authorization.dataScopeOrgCodes}
              onChange={(value) => onAuthorizationChange({ ...authorization, dataScopeOrgCodes: stringArray(value) })}
            />
          ) : null}
          {authorization.dataScope === "custom_areas" ? (
            <PlatformTreeSelect
              aria-label={dictionary.roleDataScopeAreas}
              multiple
              options={areaCodes.map((record) => treeRecordOption(record, "parentCode", "path"))}
              value={authorization.dataScopeAreaCodes}
              onChange={(value) => onAuthorizationChange({ ...authorization, dataScopeAreaCodes: stringArray(value) })}
            />
          ) : null}
        </div>
      </div>
    </Modal>
  );
}

function MenuVisibilityModal({ role, menus, dictionary, returnFocusRef, onClose }: { role: AdminResourceRecord | null; menus: AdminResourceRecord[]; dictionary: Dictionary; returnFocusRef: React.RefObject<HTMLElement>; onClose: () => void }) {
  if (!role) return null;
  const visible = legacyVisibleMenus(role, menus);
  return (
    <Modal
      className="role-menu-visibility-modal"
      cancelText={dictionary.close}
      destroyOnHidden
      footer={<Button onClick={onClose}>{dictionary.close}</Button>}
      open
      title={`${dictionary.assignMenus}: ${role.name}`}
      width={980}
      onCancel={onClose}
    >
      <AdminFeedback type="warning" message={dictionary.roleMenuLegacyReadonlyTitle} description={dictionary.roleMenuLegacyReadonlyDescription} />
      <PlatformTreeTransfer
        ariaLabel={dictionary.assignMenus}
        labels={transferLabels(dictionary)}
        nodes={menuTreeNodes(menus)}
        readOnly
        readOnlyMessage={dictionary.roleMenuLegacyReadonlyDescription}
        returnFocusRef={returnFocusRef}
        value={visible}
        onChange={() => undefined}
      />
    </Modal>
  );
}

function roleTreeNodes(groups: AdminResourceRecord[], roles: AdminResourceRecord[], search: string, includeReferencedGroups = false): AdminTreeWorkbenchNode[] {
  const normalized = search.trim().toLocaleLowerCase();
  const rolesByGroup = new Map<string, AdminResourceRecord[]>();
  for (const role of roles) {
    const groupCode = valueOf(role, "groupCode");
    rolesByGroup.set(groupCode, [...(rolesByGroup.get(groupCode) ?? []), role]);
  }
  const visibleGroups = groups.filter((group) => !normalized || `${group.name} ${group.code}`.toLocaleLowerCase().includes(normalized) || (rolesByGroup.get(group.code)?.length ?? 0) > 0);
  const nodes: AdminTreeWorkbenchNode[] = visibleGroups.flatMap((group) => [
    { key: nodeKey(group, "group"), kind: "group", label: `${group.name} (${group.code})`, childCount: rolesByGroup.get(group.code)?.length ?? 0 },
    ...(rolesByGroup.get(group.code) ?? []).map((role) => ({ key: nodeKey(role, "role"), parentKey: nodeKey(group, "group"), kind: "item" as const, label: `${role.name} (${role.code})`, status: role.status, isLeaf: true })),
  ]);
  if (!includeReferencedGroups) return nodes;
  const knownGroupCodes = new Set(groups.map((group) => group.code));
  for (const groupCode of [...rolesByGroup.keys()].filter(Boolean).filter((code) => !knownGroupCodes.has(code)).sort()) {
    const referencedRoles = rolesByGroup.get(groupCode) ?? [];
    nodes.push(
      { key: `group:${groupCode}`, kind: "group", label: groupCode, childCount: referencedRoles.length, selectable: false },
      ...referencedRoles.map((role) => ({ key: nodeKey(role, "role"), parentKey: `group:${groupCode}`, kind: "item" as const, label: `${role.name} (${role.code})`, status: role.status, isLeaf: true })),
    );
  }
  return nodes;
}

function permissionTreeNodes(records: AdminResourceRecord[], dictionary: Dictionary, historicalCodes: string[]): PlatformTreeTransferNode[] {
  const nodes: PlatformTreeTransferNode[] = [];
  const branches = new Set<string>();
  const historicalCodeSet = new Set(historicalCodes);
  const catalogCodes = new Set(records.map((record) => record.code));
  for (const permission of records.filter((record) => enabled(record) || historicalCodeSet.has(record.code))) {
    const availableDisabledReason = enabled(permission) ? undefined : dictionary.rolePermissionHistoricalDisabled;
    const type = valueOf(permission, "resourceType") || "api";
    const typeKey = `permission-type:${type}`;
    const group = valueOf(permission, "capability") || valueOf(permission, "resource") || dictionary.uncategorized;
    const groupKey = `${typeKey}:${group}`;
    if (!branches.has(typeKey)) {
      branches.add(typeKey);
      nodes.push({ key: typeKey, kind: "branch", label: type === "page-button" ? dictionary.permissionTypePageButton : dictionary.permissionTypeAPI });
    }
    if (!branches.has(groupKey)) {
      branches.add(groupKey);
      nodes.push({ key: groupKey, parentKey: typeKey, kind: "branch", label: group });
    }
    nodes.push({
      key: permission.code,
      parentKey: groupKey,
      kind: "leaf",
      label: permission.name || permission.code,
      code: permission.code,
      status: permission.status,
      availableDisabledReason,
    });
  }
  const missingCodes = historicalCodes.filter((code) => !catalogCodes.has(code));
  if (missingCodes.length > 0) {
    const typeKey = "permission-type:historical";
    nodes.push({ key: typeKey, kind: "branch", label: dictionary.permissionTypeHistorical });
    nodes.push(...missingCodes.map((code) => ({
      key: code,
      parentKey: typeKey,
      kind: "leaf" as const,
      label: code,
      code,
      availableDisabledReason: dictionary.rolePermissionHistoricalMissing,
    })));
  }
  return nodes;
}

function menuTreeNodes(records: AdminResourceRecord[]): PlatformTreeTransferNode[] {
  const parentCodes = new Set(records.map((record) => valueOf(record, "parentCode")).filter(Boolean));
  return records.map((record) => ({
    key: record.code,
    parentKey: valueOf(record, "parentCode") || undefined,
    kind: parentCodes.has(record.code) ? "branch" : "leaf",
    label: record.name || record.code,
    code: record.code,
    status: record.status,
  }));
}

function legacyVisibleMenus(role: AdminResourceRecord, menus: AdminResourceRecord[]) {
  const allows = csv(valueOf(role, "permissions"));
  const denies = csv(valueOf(role, "denyPermissions"));
  const directlyVisible = new Set(menus.filter(enabled).filter((menu) => permissionAllows(allows, denies, valueOf(menu, "permission"))).map((menu) => menu.code));
  const byCode = new Map(menus.map((menu) => [menu.code, menu]));
  for (const code of [...directlyVisible]) {
    let parentCode = valueOf(byCode.get(code), "parentCode");
    while (parentCode) {
      directlyVisible.add(parentCode);
      parentCode = valueOf(byCode.get(parentCode), "parentCode");
    }
  }
  return [...directlyVisible];
}

function permissionAllows(allows: string[], denies: string[], permission: string) {
  if (!permission) return false;
  if (denies.some((value) => permissionMatch(value, permission))) return false;
  return allows.some((value) => permissionMatch(value, permission));
}

function permissionMatch(granted: string, permission: string) {
  return granted === "*" || granted === permission || granted.endsWith(":*") && permission.startsWith(granted.slice(0, -1));
}

async function loadAllRecords(resource: string, keywords?: string[]) {
  const records: AdminResourceRecord[] = [];
  const pageSize = 1000;
  for (let page = 1; ; page += 1) {
    const result = await queryAdminResource(resource, { keywords, page, pageSize, sort: [{ field: "name", order: "asc" }] });
    records.push(...result.items);
    if (result.items.length < pageSize || records.length >= result.total) return records;
  }
}

function metadataInput(editor: EditorState, values: MetadataValues): AdminResourceInput {
  const existing = editor.record?.values ?? {};
  if (editor.kind === "group") {
    return {
      code: values.code,
      name: values.name,
      status: editor.record?.status ?? "enabled",
      description: values.description,
      values: { ...existing, parentCode: "", scopeType: values.scopeType ?? "tenant", tenantCode: values.scopeType === "platform" ? "" : values.tenantCode ?? "", sortOrder: String(values.sortOrder ?? 0) },
    };
  }
  return {
    code: values.code,
    name: values.name,
    status: editor.record?.status ?? "enabled",
    description: values.description,
    values: { ...existing, groupCode: values.groupCode ?? editor.groupCode ?? "" },
  };
}

function selectedRecord(key: string, groups: AdminResourceRecord[], roles: AdminResourceRecord[]) {
  if (key.startsWith("group:")) {
    const record = groups.find((candidate) => candidate.code === key.slice(6));
    return record ? { type: "group" as const, record } : undefined;
  }
  const record = roles.find((candidate) => candidate.code === key.slice(5));
  return record ? { type: "role" as const, record } : undefined;
}

function nodeKey(record: AdminResourceRecord, type: "group" | "role") { return `${type}:${record.code}`; }
function valueOf(record: AdminResourceRecord | null | undefined, key: string) { return record?.values?.[key] ?? ""; }
function numberValue(record: AdminResourceRecord | undefined, key: string) { const value = Number(valueOf(record, key)); return Number.isFinite(value) ? value : 0; }
function csv(value: string) { return [...new Set(value.split(",").map((item) => item.trim()).filter(Boolean))].sort(); }
function uniqueSorted(values: string[]) { return [...new Set(values)].sort(); }
function enabled(record: AdminResourceRecord) { return record.status === "enabled"; }
function sameRoleGroupBoundary(left: AdminResourceRecord, right: AdminResourceRecord) {
  return valueOf(left, "scopeType") === valueOf(right, "scopeType") && valueOf(left, "tenantCode") === valueOf(right, "tenantCode");
}
function recordOption(record: AdminResourceRecord) { return { value: record.code, label: `${record.name} (${record.code})` }; }
function treeRecordOption(record: AdminResourceRecord, parentKey = "parentCode", pathKey?: string) { return { value: record.code, label: `${record.name} (${record.code})`, parentValue: valueOf(record, parentKey), pathValue: pathKey ? valueOf(record, pathKey) : undefined }; }
function stringArray(value: unknown) { return Array.isArray(value) ? value.map(String) : []; }
function errorMessage(error: unknown, fallback: string) { return error instanceof Error ? error.message : fallback; }

function transferLabels(dictionary: Dictionary) {
  return {
    available: dictionary.transferAvailable,
    selected: dictionary.transferSelected,
    search: dictionary.transferSearch,
    empty: dictionary.emptyData,
    selectAllFiltered: dictionary.transferSelectAllFiltered,
    clear: dictionary.clearSelection,
    selectedCount: (count: number) => dictionary.transferSelectedCount.replace("{count}", String(count)),
    disabledReason: (reason: string) => dictionary.transferDisabledReason.replace("{reason}", reason),
  };
}

function dataScopeOptions(dictionary: Dictionary) {
  return [
    ["all", dictionary.dataScopeAll],
    ["current_org", dictionary.dataScopeCurrentOrg],
    ["current_and_children", dictionary.dataScopeCurrentAndChildren],
    ["custom_orgs", dictionary.dataScopeCustomOrgs],
    ["current_area", dictionary.dataScopeCurrentArea],
    ["current_and_children_areas", dictionary.dataScopeCurrentAndChildrenAreas],
    ["custom_areas", dictionary.dataScopeCustomAreas],
    ["self", dictionary.dataScopeSelf],
  ].map(([value, label]) => ({ value, label }));
}

type ConfirmModal = ReturnType<typeof App.useApp>["modal"]["confirm"];

function confirmImpact(confirm: ConfirmModal, dictionary: Dictionary, affectedUsers: number, conflictCount: number) {
  return confirmPromise(confirm, dictionary.changeImpactTitle, dictionary.changeImpactDescription.replace("{affectedUsers}", String(affectedUsers)).replace("{conflictCount}", String(conflictCount)), dictionary.reviewAndApply, dictionary.cancel);
}

function confirmRoleConflicts(confirm: ConfirmModal, dictionary: Dictionary, conflicts: ReadonlyArray<RoleChangeConflict>) {
  return confirmPromise(confirm, dictionary.roleConflictTitle, <div><Typography.Paragraph>{dictionary.roleConflictDescription.replace("{count}", String(conflicts.length))}</Typography.Paragraph><ul className="role-conflict-list">{conflicts.map((conflict) => <li key={`${conflict.userCode}:${conflict.roleCode}`}>{conflict.userCode} · {conflict.roleCode}</li>)}</ul></div>, dictionary.userInvalidRoleRemove, dictionary.cancel);
}

function confirmPromise(confirm: ConfirmModal, title: string, content: ReactNode, okText: string, cancelText: string): Promise<boolean> {
  return new Promise((resolve) => {
    let settled = false;
    const finish = (value: boolean) => { if (!settled) { settled = true; resolve(value); } };
    confirm({ title, content, okText, cancelText, icon: <LockOutlined />, onOk: () => finish(true), onCancel: () => finish(false), afterClose: () => finish(false) });
  });
}
