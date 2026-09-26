# Review: modules/ (consistency-only)

- **Date**: 2026-09-23
- **Reviewer tier**: sonnet (verified by opus)
- **Checked against**: `docs/module-authoring.md`, `modules/.schema/{module,gametemplate}.schema.json`, `modules/validate.py`, operator module-handling code (`operator/internal/controller/module_controller.go`, `operator/api/v1alpha1/gametemplate_types.go`)

## Scope reviewed

- Directory listing of all 30 `modules/*/` entries (confirmed each has a `module.yaml`)
- `modules/*/module.yaml` and `modules/*/template.yaml` for all 30 modules — validated structurally against `modules/.schema/module.schema.json` and `modules/.schema/gametemplate.schema.json` with a script (jsonschema against the checked-in schemas, not against a live CRD)
- `modules/.schema/README.md`, `modules/validate.py` (targeted: `LAYOUT_ENFORCED_MODULES`, `rule_directory_layout`, `rule_credential_fields_must_be_password`, the `configSchema` handling around line 982)
- `docs/module-authoring.md` config-schema section (lines ~620-670) and grepped the rest for count/schema claims
- `operator/api/v1alpha1/gametemplate_types.go` (`ConfigField`, `Lifecycle` types), `operator/internal/controller/module_controller.go` (`applyTemplate`, the `yaml.Unmarshal` call)
- Did not read: full contents of every `README.md`/`specs.md` per module (29 of them); `gp-module/` (this repo's local copy is the vendoring point, not read in depth — only used to confirm `go.work` membership, see review-docs notes)

## Method

Ran the two published JSON Schemas (`modules/.schema/*.schema.json`, which the repo states are generated from / mirror the actual CRD and `docs/module-authoring.md`) against every `module.yaml`/`template.yaml` pair to surface structural mismatches, then manually verified each flagged mismatch against the real Go CRD type and the actual operator code path that parses `template.yaml`, to distinguish a real functional bug from a schema/tooling false positive.

## Observations (no finding)

- `modules/minecraft-java`, `modules/v-rising`, `modules/nuclear-option` lack `specs.md` — this is **not** a bug: `modules/validate.py`'s `LAYOUT_ENFORCED_MODULES` set (line 1029) lists exactly the other 26 modules, and `rule_directory_layout` only requires `specs.md`/non-empty `samples/` for modules in that set (or with a `.layout-enforced` marker). The set matches the 26 modules that do have `specs.md` exactly.
- `nuclear-option/template.yaml:179` has an empty `lifecycle:` key (just a trailing comment, no nested content), which JSON-Schema-validates as `null` against a `type: object` schema. Traced this into `operator/api/v1alpha1/gametemplate_types.go:281` — `Lifecycle *LifecycleSpec \`json:"lifecycle,omitempty"\`` is a pointer field, and `sigs.k8s.io/yaml`'s JSON-number-based unmarshal accepts a JSON `null` into a pointer with no error (leaves it `nil`). This is a schema-tool false positive, not a real bug — not filed as a finding.
- `modules/validate.py`'s `rule_credential_fields_must_be_password` (historical CS2 `SRCDS_TOKEN` bug) correctly flags credential-shaped `type: string` fields; spot-checked a few modules' `configSchema` for obviously credential-named fields with the wrong type and found none currently mis-typed that way.
- The 10 modules in the finding below are otherwise schema-valid (all other `configSchema` entries, `spec.image`, `spec.ports`, `spec.capabilities`, etc. validated cleanly).

## Candidate findings

### C-modules-01: 10 modules declare an unquoted numeric `configSchema[].default` for a `type: int` field, which the operator's own YAML→struct unmarshal will reject — breaks Module reconciliation for those games

- **Location**:
  - `modules/ark-survival-evolved/template.yaml:137` (`MAX_PLAYERS`, `default: 70`)
  - `modules/arma-reforger/template.yaml:118` (`MAX_PLAYERS`, `default: 64`)
  - `modules/beammp/template.yaml:97` (`MAX_PLAYERS`, `default: 10`)
  - `modules/euro-truck-simulator-2/template.yaml:79` (`MAX_PLAYERS`, `default: 8`)
  - `modules/farming-simulator-25/template.yaml:106` (`MAX_PLAYERS`, `default: 16`)
  - `modules/hell-let-loose/template.yaml:144` (`MAX_PLAYERS`, `default: 100`)
  - `modules/mount-and-blade-2-bannerlord/template.yaml:79` (`MAX_PLAYERS`, `default: 64`)
  - `modules/squad/template.yaml:152` (`MAX_PLAYERS`, `default: 100`)
  - `modules/team-fortress-2/template.yaml:161` (`SRCDS_MAXPLAYERS`, `default: 24`)
  - `modules/the-isle/template.yaml:137` (`MAX_PLAYERS`, `default: 50`)
- **Category**: correctness (operator module handling)
- **Suggested severity**: S2
- **Observation / repro**: Each of the 10 files above has, e.g. (ark-survival-evolved):
  ```yaml
  - name: MAX_PLAYERS
    displayName: Max Players
    type: int
    default: 70
  ```
  `default: 70` (no quotes) parses as a YAML/JSON integer. `operator/api/v1alpha1/gametemplate_types.go:684-686` declares the field the CRD actually stores it in:
  ```go
  // Default is the pre-filled value (as a string).
  Default string `json:"default,omitempty"`
  ```
  `operator/internal/controller/module_controller.go:186-189` is exactly how a `Module` CR turns this `template.yaml` into a live `GameTemplate`:
  ```go
  parsed := &gameplanev1alpha1.GameTemplate{}
  if err := yaml.Unmarshal(bundle.TemplateYAML, parsed); err != nil {
      return fmt.Errorf("parse template.yaml: %w", err)
  }
  ```
  (`sigs.k8s.io/yaml`, which converts YAML→JSON then does a normal `encoding/json.Unmarshal` into the typed struct.) Unmarshaling a JSON number into a Go `string` field is a hard type error (`json: cannot unmarshal number into Go struct field ConfigField.default of type string`), so `applyTemplate` returns an error and the `Module` never reaches `Ready` — reconciliation fails every time for these 10 bundles as currently written.
- **Expected**: `default: "70"` (quoted), matching the documented pattern. `docs/module-authoring.md:632-636`'s own canonical example uses exactly this combination and gets it right:
  ```yaml
  configSchema:
    - name: MAX_PLAYERS
      displayName: Max players
      type: int
      default: "16"
  ```
  and the working `modules/minecraft-java/template.yaml` (`MAX_PLAYERS`, `type: int`, `default: "20"`) follows it correctly.
- **Actual**: 10 of the 30 shipped modules (ARK: Survival Evolved, Arma Reforger, BeamMP, Euro Truck Simulator 2, Farming Simulator 25, Hell Let Loose, Mount & Blade II: Bannerlord, Squad, Team Fortress 2, The Isle) have a `template.yaml` that the operator's own module-materialization code cannot parse without error, as currently committed. `modules/validate.py` has no rule catching this (its only `configSchema`-adjacent rule, `rule_credential_fields_must_be_password`, checks field *names*, not default-value JSON types), so nothing currently blocks this at CI/PR time either.

## Questions (not findings)

- Whether these 10 `template.yaml` files were ever successfully installed against a real cluster (i.e., whether this is a regression introduced after the last real-cluster test, or these modules have simply never been exercised end-to-end) is outside what this review's evidence can establish — `docs/game-coverage.md` only tracks join-protocol coverage, not module-install coverage, so it doesn't help pin this down.
