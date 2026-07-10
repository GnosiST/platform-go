import { Spin } from "antd";
import { useCan, useResourceParams } from "@refinedev/core";
import type { Dictionary, Language } from "../i18n";
import { GenericResourceConsole } from "../resources/GenericResourceConsole";
import type { AdminResourceDefinition } from "../resources/registry";
import { AdminFeedback } from "../ui";

type ResourceRoutePageProps = {
  resource: AdminResourceDefinition;
  availableResourceRoutes: string[];
  language: Language;
  dictionary: Dictionary;
  permissions: string[];
  deniedPermissions: string[];
};

export function ResourceRoutePage({ resource, availableResourceRoutes, language, dictionary, permissions, deniedPermissions }: ResourceRoutePageProps) {
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

  return (
    <GenericResourceConsole
      resource={resource}
      availableResourceRoutes={availableResourceRoutes}
      language={language}
      dictionary={dictionary}
      permissions={permissions}
      deniedPermissions={deniedPermissions}
    />
  );
}
