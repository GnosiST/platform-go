import { Layout, Menu, Typography } from "antd";
import type { ReactNode } from "react";
import type { AdminResourceDefinition } from "../resources/registry";

type AdminShellProps = {
  resources: AdminResourceDefinition[];
  children: ReactNode;
};

export function AdminShell({ resources, children }: AdminShellProps) {
  return (
    <Layout className="platform-shell">
      <Layout.Sider width={248} className="platform-sider">
        <div className="platform-brand">
          <div className="platform-logo">P</div>
          <div>
            <Typography.Text className="platform-title">platform-go</Typography.Text>
            <Typography.Text className="platform-subtitle">Capability Admin</Typography.Text>
          </div>
        </div>
        <Menu
          mode="inline"
          selectedKeys={[resources[0]?.route ?? "/"]}
          items={resources.map((resource) => ({
            key: resource.route,
            label: resource.title,
          }))}
        />
      </Layout.Sider>
      <Layout>
        <header className="platform-topbar">
          <Typography.Text className="platform-topbar-title">通用平台底座</Typography.Text>
        </header>
        <Layout.Content className="platform-content">{children}</Layout.Content>
      </Layout>
    </Layout>
  );
}
