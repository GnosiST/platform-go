import { Form, Typography, type FormProps } from "antd";
import type { Rule } from "antd/es/form";
import { type ReactNode } from "react";

export type PlatformResourceFormLayoutPreset = "single-column" | "grouped-sections" | "two-column-density" | "side-detail-preview";

export type PlatformResourceFormSection<TField extends { key: string }> = {
  key: string;
  label?: ReactNode;
  description?: ReactNode;
  fields: TField[];
};

export type PlatformResourceFormSlots<TField extends { key: string }> = {
  header?: ReactNode;
  footer?: ReactNode;
  sectionBefore?: (section: PlatformResourceFormSection<TField>) => ReactNode;
  sectionAfter?: (section: PlatformResourceFormSection<TField>) => ReactNode;
  fieldControl?: (field: TField, defaultControl: ReactNode) => ReactNode;
  sidePreview?: ReactNode;
};

export type PlatformResourceFormProps<TField extends { key: string }> = Omit<FormProps, "children" | "layout"> & {
  sections: PlatformResourceFormSection<TField>[];
  layoutPreset?: PlatformResourceFormLayoutPreset;
  slots?: PlatformResourceFormSlots<TField>;
  renderField: (field: TField) => ReactNode;
  renderFieldLabel: (field: TField) => ReactNode;
  renderFieldExtra?: (field: TField) => ReactNode;
  rules?: (field: TField) => Rule[] | undefined;
  getValuePropName?: (field: TField) => string | undefined;
};

export function PlatformResourceForm<TField extends { key: string }>({
  className,
  sections,
  layoutPreset = "single-column",
  slots,
  renderField,
  renderFieldLabel,
  renderFieldExtra,
  rules,
  getValuePropName,
  ...formProps
}: PlatformResourceFormProps<TField>) {
  const formClassName = cx("resource-form", `layout-${layoutPreset}`, className);
  const sectionNodes = sections.map((section) => (
    <section className="resource-form-section" data-section-key={section.key} key={section.key}>
      {slots?.sectionBefore?.(section)}
      {section.label ? (
        <div className="resource-form-section-header">
          <Typography.Text strong>{section.label}</Typography.Text>
          {section.description ? <Typography.Text type="secondary">{section.description}</Typography.Text> : null}
        </div>
      ) : null}
      <div className="resource-form-fields">
        {section.fields.map((field) => {
          const control = renderField(field);
          return (
            <Form.Item
              extra={renderFieldExtra?.(field)}
              key={field.key}
              label={renderFieldLabel(field)}
              name={field.key}
              rules={rules?.(field)}
              valuePropName={getValuePropName?.(field)}
            >
              {slots?.fieldControl ? slots.fieldControl(field, control) : control}
            </Form.Item>
          );
        })}
      </div>
      {slots?.sectionAfter?.(section)}
    </section>
  ));
  return (
    <Form {...formProps} className={formClassName} layout="vertical">
      {slots?.header ? <div className="resource-form-slot resource-form-slot-header">{slots.header}</div> : null}
      {layoutPreset === "side-detail-preview" && slots?.sidePreview ? (
        <div className="resource-form-side-preview">
          <div className="resource-form-main">{sectionNodes}</div>
          <aside className="resource-form-preview-rail">{slots.sidePreview}</aside>
        </div>
      ) : (
        sectionNodes
      )}
      {slots?.footer ? <div className="resource-form-slot resource-form-slot-footer">{slots.footer}</div> : null}
    </Form>
  );
}

function cx(...classes: Array<string | false | null | undefined>) {
  return classes.filter(Boolean).join(" ");
}
