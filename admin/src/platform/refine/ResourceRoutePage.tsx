import { Spin } from "antd";
import { useCan, useResourceParams } from "@refinedev/core";
import type { Dictionary, Language } from "../i18n";
import { GenericResourceConsole } from "../resources/GenericResourceConsole";
import { MenuGovernanceConsole } from "../resources/MenuGovernanceConsole";
import { PermissionGovernanceConsole } from "../resources/PermissionGovernanceConsole";
import { RoleGovernanceConsole } from "../resources/RoleGovernanceConsole";
import { SessionConsole } from "../resources/SessionConsole";
import type { AdminResourceDefinition } from "../resources/registry";
import type { AdminSensitiveRevealFactorComplete } from "../api/client";
import type { SensitiveRevealOIDCResume } from "../security/sensitiveRevealOIDC";
import { AdminFeedback } from "../ui";

type ResourceRoutePageProps = {
  resource: AdminResourceDefinition;
  availableResourceRoutes: string[];
  language: Language;
  dictionary: Dictionary;
  permissions: string[];
  deniedPermissions: string[];
  sensitiveRevealOIDCResume: SensitiveRevealOIDCResume<AdminSensitiveRevealFactorComplete> | null;
  onSensitiveRevealOIDCResumeConsumed: () => void;
};

export function ResourceRoutePage({ resource, availableResourceRoutes, language, dictionary, permissions, deniedPermissions, sensitiveRevealOIDCResume, onSensitiveRevealOIDCResumeConsumed }: ResourceRoutePageProps) {
  const { resource: refineResource } = useResourceParams({ resource: resource.name, action: "list" });
  const readAccess = useCan({
    resource: resource.name,
    action: "list",
    params: { resource: refineResource },
  });

  if (readAccess.isLoading) {
    return (
      <div className="loading-panel">
        <Spin />
      </div>
    );
  }

  if (readAccess.data && !readAccess.data.can) {
    return <AdminFeedback type="warning" message={dictionary.noPermission} description={readAccess.data.reason} />;
  }

  if (resource.route === "/roles" || resource.route === "/role-groups") {
    return (
      <RoleGovernanceConsole
        resource={resource}
        language={language}
        dictionary={dictionary}
        permissions={permissions}
        deniedPermissions={deniedPermissions}
      />
    );
  }

  if (resource.route === "/menus") {
    return (
      <MenuGovernanceConsole
        resource={resource}
        availableResourceRoutes={availableResourceRoutes}
        language={language}
        dictionary={dictionary}
        permissions={permissions}
        deniedPermissions={deniedPermissions}
      />
    );
  }

  if (resource.route === "/permissions") {
    return (
      <PermissionGovernanceConsole
        resource={resource}
        language={language}
        dictionary={dictionary}
        permissions={permissions}
        deniedPermissions={deniedPermissions}
      />
    );
  }

  if (resource.route === "/sessions") {
    return <SessionConsole resource={resource} language={language} dictionary={dictionary} />;
  }

  return (
    <GenericResourceConsole
      resource={resource}
      availableResourceRoutes={availableResourceRoutes}
      language={language}
      dictionary={dictionary}
      experienceKey={resource.route === "/org-units" || resource.route === "/users" ? "organization-user" : undefined}
      permissions={permissions}
      deniedPermissions={deniedPermissions}
      oidcResume={sensitiveRevealOIDCResume}
      onOIDCResumeConsumed={onSensitiveRevealOIDCResumeConsumed}
    />
  );
}
