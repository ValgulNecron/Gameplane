# gameaction — Specification

**Status:** beta (v0.2.0-beta.7)  
**Module / package:** `github.com/ValgulNecron/gameplane/gameaction`  
**Dependencies:** stdlib only (Go 1.25+)

## Purpose

Shared console-injection guard and command-template renderer for all transports that execute module-declared actions against a game server. Used by:
- **API** (`api/internal/ws/actions.go`) — stdin pod-attach console execution
- **Agent** (`agent/internal/actions/actions.go`) — RCON console execution

Both call the validation and rendering functions independently; each is its own trust boundary, so validation is never skipped because "the other side already checked."

## Responsibilities

1. Validate raw user-supplied action parameters against declared specifications before any rendering occurs.
2. Reject console-injection vectors (control characters) that would chain a second command on the same console line.
3. Enforce parameter types (string, int, bool, enum) and constraints.
4. Enforce a 512-character length cap on string parameters.
5. Enforce required-parameter checking and default value substitution.
6. Parse and compile command templates with strict missing-key errors.
7. Render compiled templates with validated parameters, exposing them as `.Params` in the template scope.

## Non-goals / boundaries

- Does not execute commands — only validates and renders them. Execution is the caller's responsibility (API pod-attach or agent RCON).
- Does not authenticate or authorize users — RBAC is enforced upstream.
- Does not interact with Kubernetes objects directly — callers provide the parameter declarations and values.
- Does not persist state — validation and rendering are pure functions.

## Directory & package layout

```
gameaction/
├── action.go           # Core API: Param, Resolve, Command, Compile, Render
└── action_test.go      # Unit tests (91% coverage gate)
```

Single file, no subdirectories. Part of the Go workspace (`go.work`) with netguard, operator, api, agent, and other modules.

## External interface / contracts

### Types

**`Param`** — A declared action input parameter. Mirrors the CRD's `ActionParamSpec` (in `operator/api/v1alpha1/gametemplate_types.go`) and the agent's `ActionParam`.

```go
type Param struct {
  Name        string   // Parameter identifier
  DisplayName string   // User-facing label (for UI)
  Description string   // Help text
  Type        string   // "string" | "int" | "bool" | "enum"; "" defaults to "string"
  Default     string   // Fallback if not supplied
  Enum        []string // Valid values when Type == "enum"
  Required    bool     // Reject empty values if true
}
```

**`Command`** — A compiled command template. Opaque; created by `Compile`, rendered by `Render`.

```go
type Command struct {
  tmpl *template.Template
}
```

### Functions

**`Resolve(decls []Param, got map[string]string) (map[string]string, error)`**

Validates raw user values against declared parameters. Returns a sanitized map or a validation error.

Behavior:
- Fills missing params with their declared defaults.
- Rejects parameters whose required=true and value is empty or whitespace-only.
- For non-empty values, calls `validateParam`, which:
  - **`int` type:** parses as base-10 int64 (post-trim), returns trimmed string; rejects non-integer strings.
  - **`bool` type:** accepts "true" or "false" (case-insensitive, post-trim); rejects other values.
  - **`enum` type:** rejects values not in the declared `Enum` slice (exact match, no trimming).
  - **`string` type** (or empty Type): rejects control characters (ASCII 0x00–0x1f, 0x7f), rejects strings over 512 chars, returns as-is.
- Returns the resolved map with all declared param names present (defaults may produce empty strings for optional params).

Control-character rejection (via `hasControl`):
```go
// Rejects any ASCII control character (r < 0x20 || r == 0x7f)
// Specifically blocks CR, LF, NUL, ESC, DEL — injection vectors for chaining commands.
```

**`Compile(name, command string) (*Command, error)`**

Parses a command template using Go's `text/template` package (strict mode: `missingkey=error`).

- `name` — template name, used in error messages only.
- `command` — the template string (e.g., `"say {{.Params.message}}"`).
- Returns a `*Command` or a parse error wrapping the template error.
- Template params are exposed to the template as `.Params` (a map).

**`(*Command).Render(params map[string]string) (string, error)`**

Executes the compiled template with the resolved parameters.

- Returns the rendered command string, trimmed of leading/trailing whitespace.
- Returns an error if the template references a missing key (enforced by `missingkey=error`) or if execution fails.
- Result is ready to pass to the console (RCON or pod-attach stdin).

## Key invariants

1. **Independent validation:** API and agent each call `Resolve` independently before rendering. Neither relies on "the other side already checked" — validation is not skipped in the agent even if the API validated first, and vice versa. This is a hard security boundary.

2. **Control-character rejection:** Every string parameter is scanned for ASCII control characters (0x00–0x1f, 0x7f) before rendering. This blocks console-injection attacks where a parameter value could contain a newline or carriage return to chain a second command.

3. **512-char limit:** String parameters are capped at 512 characters. Limit is enforced *before* template rendering, rejecting the parameter value itself, not the rendered output.

4. **Type enforcement:** Enums reject values outside the declared set. Ints reject non-numeric strings. Bools reject values outside {true, false}. Types are case-insensitive for bool (before lowercasing) and trimmed for int, but exact-match for enum.

5. **Required params:** A param with `Required: true` rejects empty or whitespace-only values, even with a default. If no default is provided, the value is required. If a default is provided, a missing value is filled with the default, then checked for emptiness.

6. **Template strictness:** `Compile` uses `missingkey=error`, so any template reference to a param that wasn't provided to `Resolve` (or declared) will fail at render time.

7. **Rendering is deterministic:** Template execution is pure — given the same template and params, rendering always produces the same output (modulo non-determinism in the template itself, e.g., `rand`).

## Dependencies

**Stdlib only:**
- `fmt` — error formatting
- `strconv` — integer parsing
- `strings` — trimming, splitting, contains checks
- `text/template` — command template compilation and rendering

No external modules.

## Security considerations

1. **Console injection:** The control-character guard is the primary defense against chaining commands on the same console line. Rejecting 0x00–0x1f and 0x7f stops CR, LF, NUL, ESC, DEL, and related codes that could terminate a command or start a new one.

2. **Template injection:** Compiling with `missingkey=error` prevents silent fallthrough to empty strings if a template references a param that `Resolve` didn't provide. This makes template errors visible and loud.

3. **Type confusion:** Requiring explicit type declarations (int, bool, enum) prevents a user-supplied string from being misinterpreted as a different type by the command renderer or console.

4. **Length limits:** The 512-char cap prevents resource exhaustion (very long strings passed to RCON or console stdin).

5. **Trust boundary:** Each caller (API and agent) validates independently. This is not just a convenience — it's a security boundary, because the API and agent run in different processes/containers and may have different threat models.

## Testing & coverage

**Test file:** `action_test.go`

**Test coverage:** 91% (enforced by `.testcoverage.yml`; total threshold 91%).

**Test cases cover:**
- Default value substitution (`TestResolve_Defaults`)
- Required param enforcement (`TestResolve_RequiredMissing`)
- Control character rejection for all injection vectors: `\n`, `\r`, `\x00`, `\x1b`, `\x7f` (`TestResolve_RejectsControlChars`)
- Type validation: int, bool, enum, string (`TestResolve_IntBoolEnum`)
- String length limit at 512 chars (`TestResolve_TooLong`)
- Template parse errors (`TestCompile_BadTemplate`)
- Parameter substitution in templates (`TestRender_Params`)
- Missing-key errors in templates (`TestRender_MissingKey`)

## References

- **CRD types:** `operator/api/v1alpha1/gametemplate_types.go` — `ActionParamSpec`, `ActionSpec`
- **API usage:** `api/internal/ws/actions.go` — WebSocket action execution
- **Agent usage:** `agent/internal/actions/actions.go` — RCON action execution
- **Module authoring:** `docs/module-authoring.md` — defines the `actions` field in `template.yaml`
- **CLAUDE.md:** "The operator is authoritative" + "both importers call it independently"
