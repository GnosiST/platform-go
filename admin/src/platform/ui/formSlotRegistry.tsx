import { Tag, Typography } from "antd";
import { type ReactNode } from "react";
import type {
  AdminResourceField,
  AdminResourcePermissions,
  AdminResourceRecord,
  AdminResourceRuntimeSlot,
} from "../api/client";
import type { Dictionary, Language } from "../i18n";
import { PlatformOverflowText } from "./AdminPrimitives";
import type { PlatformResourceFormSection } from "./PlatformResourceForm";

export type AdminFormRuntimeSlotDescriptor = AdminResourceRuntimeSlot;

export type AdminFormRuntimeSlotRendererProps = {
  descriptor: AdminFormRuntimeSlotDescriptor;
  dictionary: Dictionary;
  language: Language;
  fields: AdminResourceField[];
  sections: PlatformResourceFormSection<AdminResourceField>[];
  record?: AdminResourceRecord | null;
  formValues: Record<string, string | string[] | boolean | number | undefined>;
  permissions: AdminResourcePermissions;
  defaultControl?: ReactNode;
  field?: AdminResourceField;
};

export type AdminFormRuntimeSlotRenderer = (props: AdminFormRuntimeSlotRendererProps) => ReactNode;

export type AdminFormRuntimeSlotRegistry = {
  render: (descriptor: AdminFormRuntimeSlotDescriptor, props: Omit<AdminFormRuntimeSlotRendererProps, "descriptor">) => ReactNode;
};

export function createAdminFormSlotRegistry(renderers: Record<string, AdminFormRuntimeSlotRenderer>): AdminFormRuntimeSlotRegistry {
  return {
    render(descriptor, props) {
      const renderer = renderers[descriptor.slotId];
      if (renderer) {
        return renderer({ ...props, descriptor });
      }
      if (descriptor.region === "field.control" && props.defaultControl) {
        return props.defaultControl;
      }
      return <RuntimeSlotUnavailable descriptor={descriptor} dictionary={props.dictionary} language={props.language} />;
    },
  };
}

export const defaultAdminFormSlotRegistry = createAdminFormSlotRegistry({
  "platform.record-summary": RecordSummarySlot,
  "platform.permission-summary": PermissionSummarySlot,
  "platform.localized-preview": LocalizedPreviewSlot,
});

function RecordSummarySlot(props: AdminFormRuntimeSlotRendererProps) {
  const fields = selectedFields(props);
  if (fields.length === 0) {
    return <RuntimeSlotCard descriptor={props.descriptor} dictionary={props.dictionary} language={props.language} empty />;
  }
  return (
    <RuntimeSlotCard descriptor={props.descriptor} dictionary={props.dictionary} language={props.language}>
      <div className="runtime-slot-list">
        {fields.map((field) => (
          <div key={field.key}>
            <span>{localizedText(field.label, props.language)}</span>
            <PlatformOverflowText value={slotFieldValue(field, props)} />
          </div>
        ))}
      </div>
    </RuntimeSlotCard>
  );
}

function PermissionSummarySlot(props: AdminFormRuntimeSlotRendererProps) {
  return (
    <RuntimeSlotCard descriptor={props.descriptor} dictionary={props.dictionary} language={props.language}>
      <div className="runtime-slot-permissions">
        {Object.entries(props.permissions).map(([action, permission]) => (
          <Tag key={action}>
            {action}: {permission}
          </Tag>
        ))}
      </div>
    </RuntimeSlotCard>
  );
}

function LocalizedPreviewSlot(props: AdminFormRuntimeSlotRendererProps) {
  const fields = selectedFields(props).filter((field) => field.localizable || field.key.endsWith("Zh") || field.key.endsWith("En"));
  if (fields.length === 0) {
    return <RuntimeSlotCard descriptor={props.descriptor} dictionary={props.dictionary} language={props.language} empty />;
  }
  return (
    <RuntimeSlotCard descriptor={props.descriptor} dictionary={props.dictionary} language={props.language}>
      <div className="runtime-slot-list">
        {fields.map((field) => (
          <div key={field.key}>
            <span>{localizedText(field.label, props.language)}</span>
            <PlatformOverflowText value={slotFieldValue(field, props)} />
          </div>
        ))}
      </div>
    </RuntimeSlotCard>
  );
}

function RuntimeSlotUnavailable({
  descriptor,
  dictionary,
  language,
}: {
  descriptor: AdminFormRuntimeSlotDescriptor;
  dictionary: Dictionary;
  language: Language;
}) {
  return (
    <RuntimeSlotCard descriptor={descriptor} dictionary={dictionary} language={language}>
      <Typography.Text type="secondary">{dictionary.runtimeSlotUnavailable}</Typography.Text>
    </RuntimeSlotCard>
  );
}

function RuntimeSlotCard({
  descriptor,
  dictionary,
  language,
  empty,
  children,
}: {
  descriptor: AdminFormRuntimeSlotDescriptor;
  dictionary: Dictionary;
  language: Language;
  empty?: boolean;
  children?: ReactNode;
}) {
  const title = localizedText(descriptor.label, language) || slotFallbackTitle(descriptor.slotId, dictionary);
  const description = localizedText(descriptor.description, language);
  return (
    <section className={`runtime-slot-card variant-${descriptor.variant ?? "compact"}`}>
      <div className="runtime-slot-card-header">
        <Typography.Text strong>{title}</Typography.Text>
        {description ? <Typography.Text type="secondary">{description}</Typography.Text> : null}
      </div>
      {empty ? <Typography.Text type="secondary">{dictionary.runtimeSlotNoData}</Typography.Text> : children}
    </section>
  );
}

function selectedFields(props: AdminFormRuntimeSlotRendererProps) {
  const keys = props.descriptor.dataBinding?.fields ?? [];
  if (keys.length === 0) {
    return props.fields.slice(0, 4);
  }
  const byKey = new Map(props.fields.map((field) => [field.key, field]));
  return keys.map((key) => byKey.get(key)).filter((field): field is AdminResourceField => Boolean(field));
}

function slotFieldValue(field: AdminResourceField, props: AdminFormRuntimeSlotRendererProps) {
  const formValue = props.formValues[field.key];
  if (formValue !== undefined && formValue !== "") {
    return formatSlotValue(formValue);
  }
  if (field.source === "values") {
    return props.record?.values?.[field.key] ?? "";
  }
  const recordValue = props.record?.[field.key as keyof AdminResourceRecord];
  return typeof recordValue === "string" ? recordValue : "";
}

function formatSlotValue(value: string | string[] | boolean | number) {
  if (Array.isArray(value)) {
    return value.join(", ");
  }
  if (typeof value === "boolean") {
    return String(value);
  }
  return `${value}`;
}

function slotFallbackTitle(slotId: string, dictionary: Dictionary) {
  switch (slotId) {
  case "platform.record-summary":
    return dictionary.runtimeSlotRecordSummary;
  case "platform.permission-summary":
    return dictionary.runtimeSlotPermissionSummary;
  case "platform.localized-preview":
    return dictionary.runtimeSlotLocalizedPreview;
  default:
    return slotId;
  }
}

function localizedText(value: { zh: string; en: string } | undefined, language: Language) {
  if (!value) {
    return "";
  }
  return value[language] || value.zh || value.en;
}
