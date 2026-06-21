import type { GameTemplate } from "@/types";

export type ConfigField = NonNullable<GameTemplate["spec"]["configSchema"]>[number];

const DNS_LABEL = /^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/;
const QUANTITY = /^(\d+)(\.\d+)?(Ki|Mi|Gi|Ti|Pi|Ei|m|k|M|G|T|P|E)?$/;

export function isValidK8sName(s: string): boolean {
  return s.length > 0 && s.length <= 63 && DNS_LABEL.test(s);
}

export function isValidQuantity(s: string): boolean {
  const trimmed = s.trim();
  if (trimmed === "" || trimmed !== s) return false;
  return QUANTITY.test(trimmed);
}

// isValidVersion checks a chosen version id against a template's catalog.
// When the template declares no versions, the version is irrelevant (true).
// When it does, a value must be supplied and must match a catalog id.
export function isValidVersion(
  template: GameTemplate | undefined,
  version: string | undefined,
): boolean {
  const versions = template?.spec.versions;
  if (!versions || versions.length === 0) return true;
  if (!version) return false;
  return versions.some((v) => v.id === version);
}

// defaultVersionId returns the template's pre-selected version: the entry
// marked default, else the first, else undefined when no catalog exists.
export function defaultVersionId(template: GameTemplate | undefined): string | undefined {
  const versions = template?.spec.versions;
  if (!versions || versions.length === 0) return undefined;
  return (versions.find((v) => v.default) ?? versions[0]).id;
}

export interface ConfigError {
  name: string;
  message: string;
}

export function validateConfig(
  schema: ConfigField[],
  values: Record<string, string>,
): ConfigError[] {
  const errors: ConfigError[] = [];
  for (const field of schema) {
    const raw = values[field.name];
    const provided = raw ?? field.default ?? "";
    if (field.required && provided === "") {
      errors.push({
        name: field.name,
        message: `${field.displayName ?? field.name} is required`,
      });
      continue;
    }
    if (provided !== "" && field.type === "enum" && field.enum && !field.enum.includes(provided)) {
      errors.push({
        name: field.name,
        message: `${field.displayName ?? field.name} must be one of: ${field.enum.join(", ")}`,
      });
    }
  }
  return errors;
}
