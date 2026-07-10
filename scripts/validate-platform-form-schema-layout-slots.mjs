import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(__dirname, "..");

function argValue(name, fallback) {
  const index = process.argv.indexOf(name);
  if (index === -1) return fallback;
  return process.argv[index + 1] ?? "";
}

const contractPath = path.resolve(repoRoot, argValue("--contract", "resources/platform-form-schema-layout-slots.json"));
const requiredLayoutPresets = ["single-column", "grouped-sections", "two-column-density", "side-detail-preview"];
const requiredSlotRegions = ["form.header", "form.section.before", "form.section.after", "form.footer", "field.control", "side.preview"];
const requiredRuntimeSlotDescriptorFields = ["slotId", "region", "label", "description", "permission", "visibleWhen", "targetSection", "targetField", "dataBinding", "variant", "order"];
const requiredRuntimeSlotDataBindingModes = ["record", "formValues", "resource", "none"];
const requiredRuntimeSlotVariants = ["compact", "info", "warning", "preview", "inline"];
const forbiddenRuntimeSlotDescriptorFields = ["component", "componentName", "componentPath", "import", "script", "html"];
const requiredForbiddenPatterns = [
  "arbitrary-runtime-component-paths",
  "raw-script-slots",
  "business-page-form-forks-for-standard-crud",
  "unlocalized-slot-labels",
  "permissionless-custom-actions",
  "backend-manifest-react-component-names",
  "source-writing-without-review",
];
const requiredPromotionGates = [
  "superpowers:brainstorming",
  "product-design",
  "validate-admin-i18n",
  "validate-admin-ui-contracts",
  "admin-build",
  "browser-screenshots",
];
const requiredSchemaFields = ["Schema.FormGroups", "Schema.FormLayout", "AdminResource.FormLayout", "FieldDefinition.Group", "FieldDefinition.Help", "FieldDefinition.Validation", "FieldDefinition.Relation"];
const requiredFrontendFunctions = ["PlatformResourceForm", "normalizeFormLayoutPreset", "formModalWidth", "resourceFormSections", "FieldInput", "fieldRules", "loadRelationOptions", "mergeRelationOptions"];
const requiredRefineHooks = ["useList", "useCreate", "useUpdate", "useDelete", "useDataProvider"];

function readJSON(filePath) {
  return JSON.parse(fs.readFileSync(filePath, "utf8"));
}

function values(items) {
  return Array.isArray(items) ? items.filter(Boolean) : [];
}

function relativeExistingPath(relativePath) {
  if (!relativePath || path.isAbsolute(relativePath)) {
    return false;
  }
  const absolutePath = path.resolve(repoRoot, relativePath);
  const relative = path.relative(repoRoot, absolutePath);
  return relative !== "" && !relative.startsWith("..") && fs.existsSync(absolutePath);
}

function readRelativeFile(relativePath) {
  return fs.readFileSync(path.resolve(repoRoot, relativePath), "utf8");
}

function includesAll(actualValues, requiredValues) {
  const actual = new Set(actualValues);
  return requiredValues.every((value) => actual.has(value));
}

function uniqueErrors(items, label) {
  const errors = [];
  const seen = new Set();
  for (const item of items) {
    if (!item) {
      errors.push(`${label} contains an empty value`);
      continue;
    }
    if (seen.has(item)) {
      errors.push(`${label} contains duplicate value ${item}`);
    }
    seen.add(item);
  }
  return errors;
}

function validatePromotionState(contract, errors) {
  const state = contract.promotionState ?? {};
  if (contract.status !== "implemented") {
    errors.push("form schema layout slots status must be implemented");
  }
  if (state.runtimeSlots !== "controlled") {
    errors.push("promotionState.runtimeSlots must be controlled");
  }
  if (state.sourceLevelReactSlots !== "controlled") {
    errors.push("promotionState.sourceLevelReactSlots must be controlled");
  }
  if (state.visualImplementation !== "implemented") {
    errors.push("promotionState.visualImplementation must be implemented");
  }
  if (state.sourceWriting !== "disabled") {
    errors.push("promotionState.sourceWriting must stay disabled");
  }
  if (state.arbitraryComponentInjection !== "forbidden") {
    errors.push("promotionState.arbitraryComponentInjection must stay forbidden");
  }
}

function validateSupportedToday(contract, errors) {
  const supported = contract.supportedToday ?? {};
  if (!includesAll(values(supported.schemaFields), requiredSchemaFields)) {
    errors.push(`supportedToday.schemaFields must include ${requiredSchemaFields.join(", ")}`);
  }
  if (!includesAll(values(supported.frontendFunctions), requiredFrontendFunctions)) {
    errors.push(`supportedToday.frontendFunctions must include ${requiredFrontendFunctions.join(", ")}`);
  }
  if (!includesAll(values(supported.refineHooks), requiredRefineHooks)) {
    errors.push(`supportedToday.refineHooks must include ${requiredRefineHooks.join(", ")}`);
  }
  for (const control of ["Form", "Input", "InputNumber", "Select", "Switch", "PlatformTreeSelect"]) {
    if (!values(supported.antdControls).includes(control)) {
      errors.push(`supportedToday.antdControls must include ${control}`);
    }
  }
}

function validateLayouts(contract, errors) {
  const presets = values(contract.allowedLayoutPresets);
  errors.push(...uniqueErrors(presets.map((preset) => preset.id), "allowedLayoutPresets.id"));
  const byID = new Map(presets.map((preset) => [preset.id, preset]));
  for (const id of requiredLayoutPresets) {
    if (!byID.has(id)) {
      errors.push(`allowedLayoutPresets must include ${id}`);
    }
  }
  for (const id of ["single-column", "grouped-sections"]) {
    if (byID.get(id)?.status !== "implemented") {
      errors.push(`allowedLayoutPresets.${id}.status must stay implemented`);
    }
  }
  if (byID.get("single-column")?.default !== true) {
    errors.push("allowedLayoutPresets.single-column must remain the default layout");
  }
  if (byID.get("two-column-density")?.status !== "implemented") {
    errors.push("allowedLayoutPresets.two-column-density.status must stay implemented");
  }
  if (byID.get("side-detail-preview")?.status !== "implemented") {
    errors.push("allowedLayoutPresets.side-detail-preview.status must stay implemented");
  }
}

function validateSlots(contract, errors) {
  const slots = values(contract.allowedSlotRegions);
  errors.push(...uniqueErrors(slots.map((slot) => slot.id), "allowedSlotRegions.id"));
  const byID = new Map(slots.map((slot) => [slot.id, slot]));
  for (const id of requiredSlotRegions) {
    const slot = byID.get(id);
    if (!slot) {
      errors.push(`allowedSlotRegions must include ${id}`);
      continue;
    }
    if (slot.status !== "implemented") {
      errors.push(`allowedSlotRegions.${id}.status must stay implemented`);
    }
    if (!slot.owner) {
      errors.push(`allowedSlotRegions.${id} must declare owner`);
    }
    if (!values(slot.requiredCapabilities).includes("i18n")) {
      errors.push(`allowedSlotRegions.${id} must require i18n`);
    }
  }
}

function validatePatternsAndGates(contract, errors) {
  if (!includesAll(values(contract.forbiddenPatterns), requiredForbiddenPatterns)) {
    errors.push(`forbiddenPatterns must include ${requiredForbiddenPatterns.join(", ")}`);
  }
  if (!includesAll(values(contract.requiredPromotionGates), requiredPromotionGates)) {
    errors.push(`requiredPromotionGates must include ${requiredPromotionGates.join(", ")}`);
  }
}

function validateRuntimeSlotDescriptor(contract, errors) {
  const descriptor = contract.runtimeSlotDescriptor ?? {};
  if (descriptor.status !== "implemented") {
    errors.push("runtimeSlotDescriptor.status must be implemented");
  }
  if (!includesAll(values(descriptor.fields), requiredRuntimeSlotDescriptorFields)) {
    errors.push(`runtimeSlotDescriptor must declare ${requiredRuntimeSlotDescriptorFields.join(", ")}`);
  }
  if (!includesAll(values(descriptor.requiredLocalizedFields), ["label", "description"])) {
    errors.push("runtimeSlotDescriptor.requiredLocalizedFields must include label and description");
  }
  if (!includesAll(values(descriptor.allowedRegions), requiredSlotRegions)) {
    errors.push(`runtimeSlotDescriptor.allowedRegions must include ${requiredSlotRegions.join(", ")}`);
  }
  if (!includesAll(values(descriptor.allowedDataBindingModes), requiredRuntimeSlotDataBindingModes)) {
    errors.push(`runtimeSlotDescriptor.allowedDataBindingModes must include ${requiredRuntimeSlotDataBindingModes.join(", ")}`);
  }
  if (!includesAll(values(descriptor.allowedVariants), requiredRuntimeSlotVariants)) {
    errors.push(`runtimeSlotDescriptor.allowedVariants must include ${requiredRuntimeSlotVariants.join(", ")}`);
  }
  if (!includesAll(values(descriptor.forbiddenFields), forbiddenRuntimeSlotDescriptorFields)) {
    errors.push(`runtimeSlotDescriptor.forbiddenFields must include ${forbiddenRuntimeSlotDescriptorFields.join(", ")}`);
  }
  for (const field of values(descriptor.fields)) {
    if (forbiddenRuntimeSlotDescriptorFields.includes(field)) {
      errors.push(`runtimeSlotDescriptor.fields must not include forbidden field ${field}`);
    }
  }
}

function validateSourceEvidence(contract, errors) {
  const evidence = values(contract.requiredSourceEvidence);
  if (evidence.length === 0) {
    errors.push("requiredSourceEvidence must not be empty");
  }
  for (const item of evidence) {
    const relativePath = item.path ?? "";
    if (!relativeExistingPath(relativePath)) {
      errors.push(`requiredSourceEvidence path is missing or unsafe: ${relativePath}`);
      continue;
    }
    if (!item.contains) {
      errors.push(`requiredSourceEvidence ${relativePath} must declare contains`);
      continue;
    }
    const source = readRelativeFile(relativePath);
    if (!source.includes(item.contains)) {
      errors.push(`requiredSourceEvidence ${relativePath} is missing snippet ${item.contains}`);
    }
  }
  for (const docPath of values(contract.documents)) {
    if (!relativeExistingPath(docPath)) {
      errors.push(`form schema layout slots document is missing or unsafe: ${docPath}`);
    }
  }
}

function validateBrowserEvidence(contract, errors) {
  const evidence = values(contract.browserEvidence);
  if (evidence.length < 2) {
    errors.push("browserEvidence must include desktop and mobile screenshots");
  }
  const viewports = new Set();
  for (const item of evidence) {
    const relativePath = item.path ?? "";
    if (!relativeExistingPath(relativePath)) {
      errors.push(`browserEvidence path is missing or unsafe: ${relativePath}`);
    }
    if (!item.viewport) {
      errors.push(`browserEvidence ${relativePath} must declare viewport`);
    } else {
      viewports.add(item.viewport);
    }
    if (values(item.assertions).length === 0) {
      errors.push(`browserEvidence ${relativePath} must declare assertions`);
    }
  }
  for (const viewport of ["1440x1024", "390x844"]) {
    if (!viewports.has(viewport)) {
      errors.push(`browserEvidence must include ${viewport}`);
    }
  }
}

function validate() {
  const contract = readJSON(contractPath);
  const errors = [];
  if (!contract.purpose) {
    errors.push("form schema layout slots purpose is required");
  }
  validatePromotionState(contract, errors);
  validateSupportedToday(contract, errors);
  validateLayouts(contract, errors);
  validateSlots(contract, errors);
  validatePatternsAndGates(contract, errors);
  validateRuntimeSlotDescriptor(contract, errors);
  validateSourceEvidence(contract, errors);
  validateBrowserEvidence(contract, errors);
  return { contract, errors };
}

const { contract, errors } = validate();
if (errors.length > 0) {
  console.error(errors.map((error) => `- ${error}`).join("\n"));
  process.exit(1);
}

console.log(`Validated form schema layout and slot contract in ${path.relative(repoRoot, contractPath)} (${values(contract.allowedSlotRegions).length} slot regions)`);
