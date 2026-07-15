import type { ReactNode } from "react";
import type { AdminResourceField, AdminResourceInput, AdminResourceRecord } from "../api/client";
import type { PlatformResourceFormSlots } from "../ui";

export type ResourceExperienceKey = "organization-user";

export type ResourceFormValues = Record<string, string | string[] | boolean | number | undefined>;

export class ResourceExperienceCancelledError extends Error {}

export type ResourceExperienceSubmitContext = {
  editingRecord: AdminResourceRecord | null;
  input: AdminResourceInput;
  values: ResourceFormValues;
  persist: (input: AdminResourceInput) => Promise<AdminResourceRecord>;
};

export type ResourceExperienceDetailTab = {
  key: string;
  label: ReactNode;
  children: ReactNode;
};

export type ResourceExperienceController = {
  formFields: AdminResourceField[];
  formSlots?: PlatformResourceFormSlots<AdminResourceField>;
  allowDelete: boolean;
  allowStatusToggle: boolean;
  initialValues?: (values: ResourceFormValues, editingRecord: AdminResourceRecord | null) => ResourceFormValues;
  renderField: (field: AdminResourceField, fallback: ReactNode) => ReactNode;
  fieldExtra?: (field: AdminResourceField, fallback: ReactNode) => ReactNode;
  submit?: (context: ResourceExperienceSubmitContext) => Promise<AdminResourceRecord>;
  detailTab?: (record: AdminResourceRecord) => ResourceExperienceDetailTab | undefined;
};
