# Validation Rules & Diagnostics Contract

**Branch**: `010-easy-module-building` | **Date**: 2026-08-27 | **Spec**: [spec.md](../spec.md)

This contract defines the rule catalog, severity levels, diagnostic message formats, and machine-readable JSON schema for the offline validation engine in `gp-module validate`.

---

## 1. Rule Catalog

| Rule ID | Severity | Scope | Condition & Description | Remediation Advice |
| :--- | :---: | :--- | :--- | :--- |
| `missing-required-file` | **ERROR** | Directory | A required bundle file (`module.yaml`, `template.yaml`, `README.md`) does not exist in the module directory. | Create the missing required file or re-scaffold using `gp-module init`. |
| `invalid-module-name` | **ERROR** | `module.yaml` | `name` does not conform to DNS-1123 label regex `^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`, exceeds 63 characters, or differs from directory name. | Rename the directory or update `name` in `module.yaml` to match lower-case alphanumeric DNS-1123 format. |
| `metadata-schema-violation` | **ERROR** | `module.yaml` | Required field missing (`apiVersion`, `displayName`, `version`, `game`, `summary`) or field fails JSON Schema validation. | Review error line and correct invalid field format according to `module.schema.json`. |
| `template-schema-violation` | **ERROR** | `template.yaml` | `template.yaml` structure does not conform to `gametemplate.schema.json`. | Correct the malformed YAML field matching the schema definition. |
| `image-unpinned` | **ERROR** | `template.yaml` | Default `spec.image` does not include an `@sha256:...` digest pin and lacks the `# gameplane:floating` comment opt-out. | Resolve image digest using `crane digest <image>` or `docker inspect` and append `@sha256:...`, or append `# gameplane:floating` if intentionally dynamic. |
| `unrecognized-category` | **WARN** | `module.yaml` | A category declared in `categories` is not in the canonical taxonomy list (`Survival`, `Sandbox`, `Shooter`, `Simulation`, `Building`, `Adventure`, `Horror`, `Co-op`, `PvP`, `Modded`, `Creative`). | Use a canonical category where possible to align with catalog filter chips. |
| `invalid-port-number` | **ERROR** | `template.yaml` | `containerPort` is outside the allowable TCP/UDP port range of 1 to 65535. | Change `containerPort` to an integer between 1 and 65535. |
| `invalid-port-protocol` | **ERROR** | `template.yaml` | `protocol` is not `TCP` or `UDP`. | Change `protocol` to either `TCP` or `UDP`. |
| `duplicate-port-collision` | **ERROR** | `template.yaml` | Two port entries declare the same `containerPort` and `protocol` combination. | Assign unique port numbers or change protocol. |
| `invalid-config-type` | **ERROR** | `template.yaml` | `configSchema[].type` is not one of `string`, `int`, `enum`, `boolean`, `password`. | Change field type to one of the supported Gameplane types. |
| `invalid-memory-percent` | **ERROR** | `template.yaml` | `autoFromMemoryLimit.percent` is not an integer between 1 and 100. | Set percent between 1 and 100 (e.g. 75 for 75%). |
| `credential-field-not-password` | **ERROR** | `template.yaml` | Field name contains credential substrings (`PASSWORD`, `TOKEN`, `SECRET`, `KEY`, `AUTH`) but `type` is not `password`. | Set `type: password` so the operator stores value securely in a Secret instead of plaintext CR. |
| `excessive-asset-size` | **WARN** | Assets | Asset `icon.png` exceeds 512 KiB or total directory size exceeds 1 MiB. | Compress asset or reduce resolution to optimize OCI bundle transfer. |

---

## 2. Structured JSON Output Schema

When invoked with `--json`, `gp-module validate` outputs:

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "ValidationReport",
  "type": "object",
  "required": ["timestamp", "clean", "errorCount", "warningCount", "modules"],
  "properties": {
    "timestamp": { "type": "string", "format": "date-time" },
    "clean": { "type": "boolean" },
    "errorCount": { "type": "integer", "minimum": 0 },
    "warningCount": { "type": "integer", "minimum": 0 },
    "modules": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["name", "path", "clean", "findings"],
        "properties": {
          "name": { "type": "string" },
          "path": { "type": "string" },
          "clean": { "type": "boolean" },
          "findings": {
            "type": "array",
            "items": {
              "type": "object",
              "required": ["level", "ruleId", "file", "line", "message", "remediation"],
              "properties": {
                "level": { "enum": ["ERROR", "WARN"] },
                "ruleId": { "type": "string" },
                "file": { "type": "string" },
                "line": { "type": "integer" },
                "field": { "type": "string" },
                "message": { "type": "string" },
                "remediation": { "type": "string" }
              }
            }
          }
        }
      }
    }
  }
}
```
