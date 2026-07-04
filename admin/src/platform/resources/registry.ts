export type AdminResourceDefinition = {
  name: string;
  route: string;
  title: string;
  description: string;
  permission: string;
};

export const coreResources: AdminResourceDefinition[] = [
  {
    name: "capabilities",
    route: "/capabilities",
    title: "能力清单",
    description: "查看当前平台启用的能力包。",
    permission: "admin:capability:read",
  },
];
