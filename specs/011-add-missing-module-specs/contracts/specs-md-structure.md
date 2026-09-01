# Document Contract: Module Specifications Format & Content

**Status**: Specification Draft  
**Feature Branch**: `011-add-missing-module-specs`  
**Applies To**: Every `specs.md` file in a module directory (Go modules in `go.work`, `web/`, and game directories under `modules/`)

---

## Title and Metadata Line

Every `specs.md` begins with a title line and metadata block:

```markdown
# <module-name> — Specification

**Status:** <current-status>  
**Module / command:** <path>  
**Dependencies:** <short-list-or-none>
```

**Format requirements**:

- Title uses level-1 heading (`#`)
- Title ends with " — Specification" (em dash, not hyphen)
- `**Status:**` indicates current development phase (e.g., "Stable", "Beta", "In Development", "Deprecated")
- `**Module / command:**` names the repo-relative path to the module's entry point or package location
- `**Dependencies:**` lists key external/internal dependencies in short form (e.g., "stdlib + chi + controller-runtime" or "none") on a separate line

---

## Required Sections (Canonical Order)

Every `specs.md` MUST contain these sections in this order. Each section title uses level-2 heading (`##`). Subsections within a section use level-3 heading (`###`) and beyond as needed.

### 1. Purpose

**Requirement**: One paragraph (2–3 sentences) stating what the module does and why it exists.

**Example**:
> The `netguard` package implements an SSRF dial-guard for outbound network connections. It blocks requests to private/loopback addresses, protecting the operator from SSRF attacks when the operator trusts the module source code but not the fetched content.

**Checklist**:
- [ ] Clearly states the module's function
- [ ] Explains the primary user/consumer
- [ ] Indicates the security or operational benefit

---

### 2. Responsibilities

**Requirement**: Bulleted list of what the module is responsible for and what it guarantees.

**Example**:
> - Validates outbound network addresses against a configurable allowlist
> - Rejects connections to private/loopback/reserved address ranges
> - Provides separate policies for different trust boundaries (e.g., ModuleSource vs. agent-initiated downloads)
> - Reports dial errors with structured information about why a connection was blocked

**Checklist**:
- [ ] Lists primary functions/roles (3–8 bullets typical)
- [ ] Focuses on what the module guarantees, not how
- [ ] Includes both positive (what it does) and negative (what it prevents) responsibilities

---

### 3. Non-goals / Boundaries

**Requirement**: Bulleted list of what the module does NOT do or is explicitly NOT responsible for.

**Example**:
> - Does not inspect packet content or protocol-level details
> - Does not manage firewall rules or iptables
> - Does not persist configuration state across restarts
> - Does not handle DNS-level attacks or TOCTOU races (assumes stable DNS responses)
> - Does not apply to connections initiated by third-party relay processes

**Checklist**:
- [ ] Lists 5–10 common misunderstandings or anti-patterns
- [ ] Uses "Does not…" phrasing
- [ ] Clarifies boundaries with other modules or concerns

---

### 4. Directory & Package Layout

**Requirement**: Description of the module's internal file structure and package organization.

**Example**:
```
netguard/
├── netguard.go           # Main dial-guard implementation
├── policies.go           # Policy decision logic
├── doc.go                # Package documentation
├── netguard_test.go      # Unit tests
└── testdata/
    └── addresses.txt     # Test fixture for address cases
```

**Checklist**:
- [ ] Shows top-level files and subdirectories
- [ ] Names key files and their responsibilities
- [ ] Indicates test file locations
- [ ] Is accurate to current codebase structure

---

### 5. External Interface / Contracts

**Requirement**: Description of the module's public API, protocols, and boundaries.

**Example**:
> **Exported Functions**:
> - `IsAllowed(network, addr string) error` — Returns nil if the address is allowed; otherwise returns a descriptive error
> - `IsPublic(addr string) bool` — Returns true if the address is internet-routable (not private/loopback)
>
> **Configuration**: Read from environment variable `PRIVATE_RANGES` (optional, defaults to RFC 1918/1928 + loopback/link-local)

**Checklist**:
- [ ] Documents public API (exported functions, types, constants)
- [ ] Specifies configuration vectors (env vars, config files, CRD fields, etc.)
- [ ] Describes any wire protocols or webhooks if applicable
- [ ] Includes error cases and return codes

---

### 6. Key Invariants

**Requirement**: Numbered or bulleted list of guarantees the module maintains and that other modules depend on.

**Example**:
> 1. A connection to `127.0.0.1:*` is always blocked
> 2. A connection to an address in RFC 1918 range is blocked unless explicitly allowlisted
> 3. A blocked connection returns a non-nil error with `IsAllowed` as the error type
> 4. The decision to block/allow is made before any network I/O occurs (dial is safe)

**Checklist**:
- [ ] Lists 3–10 invariants that code can rely on
- [ ] Covers edge cases (nil inputs, empty ranges, etc.)
- [ ] Stated in present tense ("always", "never", "ensures")
- [ ] Testable and verifiable

---

### 7. Dependencies

**Requirement**: Structured list of what the module depends on.

**Example**:
> **Internal**: None  
> **External**: Go stdlib only (`net`, `fmt`, `errors`)  
> **Go version**: 1.26+

Or for modules with external deps:

> **Internal**: `netguard`, `svcutil`  
> **External**: `controller-runtime/v0.19.0`, `client-go/v0.35.0`, Kubernetes 1.28+

**Checklist**:
- [ ] Clearly separates internal (same go.work) and external (third-party, stdlib)
- [ ] Lists major version constraints if any
- [ ] Mentions Go version minimum if not already in CLAUDE.md
- [ ] Mentions K8s version minimum for controllers/operators

---

### 8. (Optional) Data & Persistence

**Requirement** (if applicable): If the module maintains state, databases, or persistent data, describe the persistence strategy.

**Present in**: agent, api, operator, capture-sidecar, and other stateful modules  
**Omit if**: The module is purely functional (netguard, gameaction, gameproto, svcutil, mcp-server) or stateless (tunnel, sentinel, audit-syslog-bridge)

**Example**:
> The API uses a driver-selectable database backend (SQLite by default, PostgreSQL optional) to persist users, sessions, audit events, and configuration. Migrations are applied at startup; no manual DDL is required. See `api/internal/db/migrations/` for the current schema.

**Checklist**:
- [ ] Explains what is persisted and why
- [ ] Names storage backend(s)
- [ ] Describes initialization/migration strategy
- [ ] Notes any backup/recovery considerations

---

### 9. Security Considerations

**Requirement**: Discussion of security boundaries, threat model, and mitigations.

**Example**:
> - **SSRF Prevention**: Validates network addresses before dial to block private/loopback access
> - **mTLS**: Agent←→Operator communication uses certificate-based mutual authentication
> - **RBAC**: API enforces role-based access control (admin, operator, viewer) on all endpoints
> - **Secret Handling**: Credentials are mounted read-only, never passed via command-line arguments to subprocesses
> - **Signal Forwarding**: Graceful shutdown (SIGTERM) is forwarded to child processes; unresponsive children are killed (SIGKILL) after a timeout

**Checklist**:
- [ ] Identifies trust boundaries (what/who the module trusts)
- [ ] Covers authentication and authorization (if applicable)
- [ ] Addresses data sensitivity (secrets, logs, telemetry)
- [ ] Mentions DoS/resource-exhaustion mitigations
- [ ] References related threat model docs (e.g., `docs/security.md`)

---

### 10. Testing & Coverage

**Requirement**: Overview of the test structure and coverage thresholds.

**Example**:
> **Test Structure**:
> - Unit tests in `_test.go` files alongside source
> - Integration tests in `*_envtest_test.go` for CRD reconciliation
> - E2E tests in `test/e2e/` for end-to-end workflows
>
> **Coverage Gate**: 90% (defined in `.testcoverage.yml`)  
> **Uncovered Paths**: Graceful shutdown race conditions (edge cases in signal handling that are difficult to trigger reliably)
>
> **Key Test Cases**:
> - Valid / invalid network addresses
> - Private / public IP ranges (RFC 1918, loopback, link-local)
> - Custom allowlist override behavior
> - Error message clarity

**Checklist**:
- [ ] Describes unit, integration, and/or E2E test tiers present in the module
- [ ] States the coverage gate percentage (from `.testcoverage.yml` or `vitest.config.ts`)
- [ ] Explains any uncovered gaps and why they are acceptable
- [ ] Lists key test cases by feature/concern
- [ ] References how to run tests (`make test-go`, `make test-web`, etc.)

---

### 11. References

**Requirement**: List of related documentation, dependent modules, and related code.

**Example**:
> - **Architecture**: `docs/architecture.md` § "netguard"
> - **Consumers**: `operator/internal/controller/` (ModuleSource fetch), `agent/internal/files/` (capability install)
> - **Related Specs**: `operator/specs.md`, `agent/specs.md`
> - **Configuration**: `.golangci.yml` (linter config), `svcutil/` (stdlib helpers)

**Checklist**:
- [ ] Lists paths to documentation that explain architecture or threat model
- [ ] Names 2–5 modules that import or depend on this module
- [ ] References related spec files (`<module>/specs.md`)
- [ ] Includes related CLAUDE.md rules if significant

---

## Formatting Conventions

All `specs.md` files MUST observe these formatting conventions:

- **Markdown tables**: For environment variables, API endpoints, or configuration options, use GitHub-flavored Markdown tables
  ```markdown
  | Variable | Type | Default | Description |
  |---|---|---|---|
  | `GAMESERVER_NAME` | string | required | Name of the GameServer resource |
  ```

- **Code fences with language tags**: Use triple backticks with a language identifier
  ```markdown
  ```json
  { "example": "value" }
  ```
  ```

- **Bold + code for symbols**: Refer to function names, environment variables, and constants as `**\`functionName()\`**` or `**\`ENV_VAR\`**`

- **Heading depth**: Limit to `##` for major sections, `###` for subsections, `####` only for sub-subsections (rare)

- **Lists**: Use hyphens (`-`) for bulleted lists, numbers (`1.`, `2.`) for ordered lists

- **Line length**: Preferably wrap at 100 characters for readability; no hard requirement

---

## Module-Specific Content Requirements

### `svcutil/specs.md` (Feature Requirement FR-002)

In addition to the standard sections above, `svcutil/specs.md` MUST include:

**Purpose section**: Must explicitly state it is a shared stdlib-only package for environment parsing and graceful HTTP server shutdown.

**External Interface / Contracts section**: MUST document the four exported functions:

| Function | Signature | Behavior |
|---|---|---|
| `Or` | `Or(key, fallback string) string` | Returns environment variable if set; otherwise returns fallback. Distinguishes unset from empty-but-set. |
| `OrInt` | `OrInt(key string, def int) int` | Returns environment variable parsed as integer; returns default if unset or invalid. Non-crashing fallback semantics. |
| `ParseLogLevel` | `ParseLogLevel(s string) slog.Level` | Maps case-insensitive log-level string to `slog.Level`. Unknown values default to `slog.LevelInfo`. |
| `RunHTTP` | `RunHTTP(ctx context.Context, srv *http.Server, shutdownTimeout time.Duration) error` | Starts HTTP server, gracefully shuts down on context cancellation with bounded timeout. Maps `http.ErrServerClosed` to nil. |

**Key Invariants section**: MUST include:
- `OrInt` and `ParseLogLevel` never panic on invalid input; they degrade gracefully to sensible defaults
- Environment variable fallback semantics are consistent: unset vars use fallback, empty-but-set vars return empty string (no fallback)
- Graceful shutdown waits bounded by `shutdownTimeout` before force-killing the server

**Testing & Coverage section**: 
- MUST state coverage gate as **90%** (not 70%)
- MUST list uncovered gaps as graceful shutdown race conditions (difficult to trigger reliably)

**Checklist**:
- [ ] Clearly identifies as stdlib-only shared utility package
- [ ] Documents all four exported functions with correct signatures and behavior
- [ ] Lists fallback behavior for each function
- [ ] Specifies no-panic guarantee for invalid inputs
- [ ] States 90% coverage gate (corrected from FR-002 typo of 70%)
- [ ] Explains why shutdown races are uncovered
- [ ] Cites svcutil as a consumer in relevant specs (operator, api, agent, audit-syslog-bridge, telemetry-receiver, sentinel, mcp-server)

---

### `tunnel/specs.md` (Feature Requirement FR-004)

In addition to the standard sections above, `tunnel/specs.md` MUST include:

**Purpose section**: Must explicitly state it is a relay client supervisor that configures and manages third-party tunnel processes.

**External Interface / Contracts section**: MUST document:

| Environment Variable | Type | Required | Description |
|---|---|---|---|
| `GAMESERVER_NAME` | string | yes | Name of the GameServer resource |
| `GAMESERVER_NAMESPACE` | string | yes | Namespace of the GameServer resource |
| `TUNNEL_TYPE` | enum (frp\|tailscale\|playit) | yes | Provider type |
| `BACKING_SERVICE_DNS` | string | yes | DNS name of the game pod (`<gs-name>.<namespace>.svc`) |
| `FRP_SERVER_ADDR` | string | if frp | FRP server address |
| `FRP_SERVER_PORT` | int | no (default 7000) | FRP server port |
| `TAILSCALE_HOSTNAME` | string | no | Hostname for tailnet (defaults to GameServer name) |
| `PLAYIT_TUNNEL_NAME` | string | no | Label for playit tunnel (defaults to GameServer name) |

**Credentials & Secrets**: MUST describe:
- Read-only Secret mount at `/etc/gameplane/tunnel-auth`
- Credential keys per provider: `token` (frp), `authKey` (Tailscale), `secretKey` (playit)

**Config Rendering Paths**: MUST document:
- FRP TOML config at `/tmp/gameplane-tunnel-frpc.toml`
- Tailscale JSON config at `/tmp/gameplane-tunnel-tailscaled.json`
- Playit secret file at `/tmp/gameplane-tunnel-playit-auth`

**Process Supervision**: MUST describe:
- Exponential backoff retry strategy (base 2^n seconds, capped at 5 minutes)
- Signal forwarding (SIGTERM for graceful shutdown, 10-second grace before SIGKILL)
- Exit code classification (126/127 and "permission denied" are unrecoverable; others retry)

**Key Invariants section**: MUST include:
- Tunnel pod never sleeps (always running, survives GameServer idle sleep)
- Credentials are never passed via command-line arguments (file-based only, gosec G204 compliance)
- Tunnel keeps advertising assigned public/relay addresses even while game pod may cycle

**Testing & Coverage section**:
- MUST state coverage gate as **70%** per `.testcoverage.yml`
- MUST list per-provider config rendering tests and supervised process lifecycle tests

**Checklist**:
- [ ] Clearly identifies as relay client supervisor
- [ ] Documents all three supported providers (frp, tailscale, playit)
- [ ] Lists environment variables by provider (required vs. optional)
- [ ] Describes credential mounting at `/etc/gameplane/tunnel-auth`
- [ ] Documents rendered config file paths
- [ ] Explains exponential backoff and retry semantics
- [ ] Describes signal forwarding (SIGTERM → 10s grace → SIGKILL)
- [ ] Lists unrecoverable vs. retryable exit codes
- [ ] States 70% coverage gate per `.testcoverage.yml`
- [ ] Covers per-provider integration tests and supervisor lifecycle

---

## Acceptance Criteria for Compliance Review

When reviewing a new or updated `specs.md`, verify the following:

### Structure
- [ ] File begins with title line (`# <module> — Specification`)
- [ ] Metadata block includes Status, Module/command, Dependencies
- [ ] All 11 required sections present (or 10 if Data & Persistence is omitted)
- [ ] Sections appear in canonical order (Purpose, Responsibilities, Non-goals, …)
- [ ] Each section uses level-2 heading (`##`)

### Content Quality
- [ ] Purpose is 2–3 sentences, actionable
- [ ] Responsibilities are 3–8 bullets, focus on guarantees not implementation
- [ ] Non-goals clearly delineate scope boundaries
- [ ] Directory layout is accurate to current codebase
- [ ] External interface documents public API (functions, env vars, CRDs, etc.)
- [ ] Key invariants are testable and enforceable (not aspirational)
- [ ] Dependencies list is complete and accurate
- [ ] Security section covers trust boundaries, auth, secrets, DoS mitigations
- [ ] Testing section states coverage gate percentage and explains gaps
- [ ] References section names related docs and consumer modules

### Formatting
- [ ] Markdown is syntactically valid
- [ ] Code blocks use language-tagged fences
- [ ] Tables use GitHub-flavored Markdown
- [ ] Symbols (functions, env vars, constants) are in bold+code

### Module-Specific (svcutil)
- [ ] All four exported functions documented with signatures
- [ ] No-panic, graceful-fallback guarantees stated
- [ ] Coverage gate listed as 90% (not 70%)
- [ ] Shutdown race uncovered-gaps explanation present

### Module-Specific (tunnel)
- [ ] All three providers (frp, tailscale, playit) documented
- [ ] Environment variable table complete and accurate
- [ ] Credentials Secret mount path and keys documented
- [ ] Config rendering paths (3 files) documented
- [ ] Exponential backoff strategy described
- [ ] Signal forwarding (SIGTERM/SIGKILL) explained
- [ ] Exit code classification (recoverable vs. unrecoverable) listed
- [ ] Coverage gate listed as 70% per `.testcoverage.yml`
