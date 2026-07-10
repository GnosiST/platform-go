import type { LocalizedText } from "../api/client";

export type DashboardPluginRow = {
  id: string;
  title: LocalizedText;
  description: LocalizedText;
  price: string;
};

export type DashboardUpdateRow = {
  id: string;
  content: LocalizedText;
  submitter: string;
  time: string;
};

export type DashboardAnnouncement = {
  id: string;
  type: "notice" | "compliant" | "service";
  title: LocalizedText;
  time: LocalizedText;
};

export const dashboardPlugins: DashboardPluginRow[] = [
  {
    id: "bbs",
    title: { zh: "BBS 社区插件", en: "BBS Community Plugin" },
    description: { zh: "面向 Gin + Refine + Ant Design 后台的论坛社区、用户、帖子、评论与站内通知能力。", en: "Forum, user, post, comment, and notification capabilities for Gin, Refine, and Ant Design admin systems." },
    price: "¥ 3688",
  },
  {
    id: "ai-admin",
    title: { zh: "AI 管理助手", en: "AI Admin Assistant" },
    description: { zh: "为运营台接入智能问答、批量生成和流程建议。", en: "Adds Q&A, batch generation, and workflow suggestions to operations consoles." },
    price: "¥ 599",
  },
  {
    id: "wechat",
    title: { zh: "微信公众号管理", en: "WeChat Account Management" },
    description: { zh: "公众号素材、消息通知和后台配置工具。", en: "Official account assets, notification messages, and admin configuration tools." },
    price: "¥ 188",
  },
];

export const dashboardUpdates: DashboardUpdateRow[] = [
  {
    id: "2225",
    content: { zh: "合并资源表格分页和列设置交互优化", en: "Merged resource table pagination and column setting improvements" },
    submitter: "PixelMax",
    time: "2026-06-30 17:34:09",
  },
  {
    id: "2218",
    content: { zh: "修复数据库迁移退出码和启动链路", en: "Fixed database migration exit code and startup chain" },
    submitter: "PixelMax",
    time: "2026-06-30 17:33:42",
  },
  {
    id: "2219",
    content: { zh: "补充动态菜单权限码和缓存字段", en: "Added dynamic menu permission code and cache fields" },
    submitter: "lanxi",
    time: "2026-06-30 15:42:28",
  },
];

export const dashboardAnnouncements: DashboardAnnouncement[] = [
  {
    id: "support",
    type: "notice",
    title: { zh: "购买商业授权后可进入专属技术支持通道，加快问题排查和版本升级效率。", en: "Commercial licensing unlocks a dedicated support channel for faster issue triage and upgrades." },
    time: { zh: "2026-07-05", en: "2026-07-05" },
  },
  {
    id: "risk",
    type: "compliant",
    title: { zh: "未授权商用存在合规风险，建议团队尽快完成授权以保障项目持续交付。", en: "Unauthorized commercial use carries compliance risk; finish licensing to protect delivery." },
    time: { zh: "2026-07-02", en: "2026-07-02" },
  },
  {
    id: "service",
    type: "service",
    title: { zh: "授权用户可获得官方长期维护承诺，包含安全修复与关键版本升级支持。", en: "Licensed users receive long-term maintenance, security fixes, and key version upgrade support." },
    time: { zh: "2026-06-30", en: "2026-06-30" },
  },
];
