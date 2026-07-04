import { Alert, Card, List, Spin, Typography } from "antd";
import { useEffect, useState } from "react";
import { listCapabilities, type CapabilityItem } from "./platform/api/client";
import { coreResources } from "./platform/resources/registry";
import { AdminShell } from "./platform/shell/AdminShell";

export default function App() {
  const [capabilities, setCapabilities] = useState<CapabilityItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    listCapabilities()
      .then((items) => {
        setCapabilities(items);
        setError("");
      })
      .catch((nextError: unknown) => {
        setError(nextError instanceof Error ? nextError.message : "能力清单加载失败");
      })
      .finally(() => setLoading(false));
  }, []);

  return (
    <AdminShell resources={coreResources}>
      <section className="platform-page">
        <Typography.Title level={2}>能力清单</Typography.Title>
        {error ? <Alert type="warning" message="无法连接平台 API" description={error} showIcon /> : null}
        <Card title="已启用能力">
          {loading ? (
            <Spin />
          ) : (
            <List
              dataSource={capabilities}
              renderItem={(item) => (
                <List.Item>
                  <List.Item.Meta title={item.name || item.id} description={`${item.id} / ${item.version || "0.1.0"}`} />
                </List.Item>
              )}
            />
          )}
        </Card>
      </section>
    </AdminShell>
  );
}
