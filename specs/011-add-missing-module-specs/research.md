# Research Phase: Module Specifications & Compliance Verification

This document resolves the unknowns identified during feature discovery: the contract details for missing modules (`svcutil` and `tunnel`), the canonical structure for all `specs.md` files, the implementation approach for the automated compliance check, and the current coverage audit of the repository.

---

## Decision: svcutil Coverage Gate is 90%, Not 70%

**Decision**: Correct FR-002's stated coverage gate from 70% to 90%. `svcutil/.testcoverage.yml` contains the authoritative value: `total: 90`.

**Rationale**: FR-002 claims "test coverage gate (70%)" but the svcutil module's `.testcoverage.yml` file specifies `total: 90`. The test suite itself (13 subtests in env_test.go, 5 functions in server_test.go) is designed to achieve that 90% threshold; the 10% gap accounts for hard-to-trigger shutdown-path edge cases (context timeout races). CLAUDE.md line 469 already documents this correctly: "coverage gate as 90%". The spec's citation of 70% appears to be a copy-paste error from simpler modules. `svcutil/specs.md` MUST state the correct value.

**Alternatives considered**: Rebase the coverage gate down to 70% to match the spec, but this contradicts the module's own `.testcoverage.yml` and CLAUDE.md's documented requirement. The authoritative source (the `testcoverage.yml` file itself) must be respected.

---

## Decision: svcutil Exported Contract

**Decision**: `svcutil/specs.md` documents four exported functions: `Or(key, fallback string) string`, `OrInt(key string, def int) int`, `ParseLogLevel(s string) slog.Level`, and `RunHTTP(ctx context.Context, srv *http.Server, shutdownTimeout time.Duration) error`.

**Rationale**: 
- `Or`: returns environment variable `key` if set (via `os.LookupEnv`), otherwise `fallback`. Distinguishes unset from empty-but-set variables: empty-but-set returns empty string, unset returns fallback.
- `OrInt`: parses environment variable `key` as int via `strconv.Atoi`, or `def` if unset or unparseable. Invalid values degrade to default rather than crashing startup.
- `ParseLogLevel`: maps case-insensitive log level string to `slog.Level` — recognized: 'debug', 'warn', 'error', 'info' (all case-insensitive); unknown defaults to `slog.LevelInfo`.
- `RunHTTP`: runs HTTP server until context cancellation. Starts `ListenAndServe` in goroutine, selects on listen error or `context.Done()`. On context cancellation, calls `srv.Shutdown` with deadline-bounded context using `shutdownTimeout`. Maps `http.ErrServerClosed` to nil. Returns shutdown result or listen error.

All functions are stdlib-only (no external dependencies). Key invariant: graceful fallback on invalid inputs; no startup crashes on bad configuration.

**Alternatives considered**: None — the exported API is fixed and observable from the codebase.

---

## Decision: tunnel Module Architecture & Supported Providers

**Decision**: `tunnel/specs.md` documents the relay client supervisor architecture with three supported providers: `frp` (self-hosted static public), `tailscale` (private tailnet), and `playit` (public dynamic).

**Rationale**: The tunnel module reads environment variables set by the operator, renders provider-specific config files, reads credentials from `/etc/gameplane/tunnel-auth` (read-only mount), spawns the relay binary, and supervises it with exponential backoff (1s–5min cap) capped at 300 seconds, with no jitter since a single replica per server runs. Exit codes 126 (permission denied) and 127 (command not found) are unrecoverable; all others retry. Signal handling forwards SIGTERM on cancellation with 10-second grace period before SIGKILL.

- **FRP**: serverAddr (required), serverPort (default 7000, validated 1-65535), backing service TCP ports (required format 'name:port,...'), auth token from Secret key 'token', config rendered to `/tmp/gameplane-tunnel-frpc.toml` (TOML format).
- **Tailscale**: hostname (optional, defaults to GameServer name), tags (optional, comma-separated), backing service ports, authKey from Secret, config to `/tmp/gameplane-tunnel-tailscaled.json` (JSON format), runs with `--tun=userspace-networking` due to pod hardening.
- **Playit**: tunnelName (optional, for labeling), backing service ports, secretKey from Secret, config to `/tmp/gameplane-tunnel-playit-auth` (raw secret file), reports assigned address via status subresource patch.

The operator injects images via `--tunnel-frp-image`, `--tunnel-tailscale-image`, `--tunnel-playit-image` flags; creates per-GameServer tunnel Deployment (1 replica, never sleeps).

**Alternatives considered**: Supporting additional relay providers (e.g., ngrok, Cloudflare Tunnel) was not prioritized; these three cover the primary use cases (self-hosted, private mesh, public dynamic).

---

## Decision: tunnel Coverage Gate & CRD Contracts

**Decision**: `tunnel/specs.md` documents the 70% coverage gate (confirmed in `tunnel/.testcoverage.yml`), and the CRD fields: `spec.networking.tunnel.enabled` (bool), `.provider` (enum frp|tailscale|playit), `.credentialsSecretRef` (optional), provider-specific nested fields (`.frp`, `.tailscale`, `.playit`).

**Rationale**: The test suite (21 test functions in main_test.go, 967 lines) covers configuration loading, credential reading, command building, supervision loop, signal handling, and backoff calculation. The 30% gap accounts for hard-to-trigger error paths and platform-specific signal handling edge cases. The CRD defines the operator's contract with end users; `svcutil/specs.md` mirrors the operator's contract with the tunnel pod via environment variables and Secrets. Playit-only RBAC grants the tunnel pod permission to patch the GameServer's status subresource with assigned addresses.

**Alternatives considered**: Running at 80% or 90% coverage would require mocking system calls and environment interactions at greater complexity; 70% is the project standard for components with external dependencies.

---

## Decision: Canonical specs.md Structure & FR-005 Alignment

**Decision**: All `specs.md` files (existing and new) conform to the canonical ordered structure:

1. **Title** — `# <module> — Specification` followed by `**Status:**`, `**Module / command:**`, `**Dependencies:**` (brief list)
2. **Purpose** — concise summary of the module's role
3. **Responsibilities** — bullet list of what the module does
4. **Non-goals / boundaries** — explicit "Does NOT..." bullets stating what the module is **not** responsible for
5. **Directory & package layout** — directory tree + package structure (for Go modules) or file/component layout (for web, Helm)
6. **External interface / contracts** — exposed APIs, environment variables, CRD fields, wire protocols, configuration schema (using markdown tables and code fences)
7. **Key invariants** — numbered or bulleted list of assumptions/guarantees other modules depend on
8. **Dependencies** — subsections for Internal and External; lists what the module imports and why
9. **[Optional: Data & persistence]** — if stateful, where and how state is stored, schema details
10. **Security considerations** — SSRF/egress boundaries, mTLS, RBAC, threat model, DoS mitigations, sensitive data handling
11. **Testing & coverage** — test structure (unit/integration/e2e breakdown), key test cases, coverage gate value, uncovered paths explanation, references to CI/Makefile commands
12. **References** — importers/dependent modules with brief description of usage, cross-references to related specs.md or docs/

This structure maps directly onto FR-005's requirement: "follow the standard structure established by existing module specifications." Deviation is permitted only for domain-specific subsections (e.g., `operator/specs.md` adds CRD/Reconciler sections; `web/specs.md` adds Routing & API Client structure; `test/e2e/internal/specs.md` adds wire-protocol sections).

**Rationale**: Consistency across 13+ existing module specs.md files enables cross-module navigation and establishes reader expectations. The canonical order (Purpose first, References last) matches the observed practice in 10+ modules. Optional sections (Data & persistence) are included only when the module has stateful concerns. Complex modules average 290–560 lines; simpler modules 75–120 lines. This is a template, not a constraint; modules adapt the structure to their complexity (e.g., `gameproto/specs.md` reorganizes with Handshake Codecs sections; `test/e2e/internal/specs.md` defines probe protocol specifications).

**Alternatives considered**: A more rigid checklist-style template would force artificial sections onto simple modules; the canonical order provides guidance without rigidity.

---

## Decision: Check Implementation — Shell Script in hack/, Module List from go.work

**Decision**: The automated compliance check is implemented as a POSIX shell script at `hack/check-specs.sh`, invoked by a new Makefile target `make check-specs`. The existing `lint` Makefile target is updated to depend on `check-specs` so the compliance check runs as part of CI's lint job.

**Rationale**: Maintainer ruling D1 specifies this location and wiring. A shell script (not Python, not Go) keeps the check runnable in CI without additional build steps or toolchain dependencies. The module list is derived from `go.work` (lines 4–17), which is already authoritative and machine-readable. Parsing approach: read `go.work`, extract the `use (...)` block, split on whitespace, filter out comments, and iterate over each line to check for corresponding `specs.md` file. This avoids hardcoding the module list and auto-adapts if `go.work` changes.

Example parsing (pseudocode):
```sh
awk '/^use \(/,/^\)/' go.work | grep -v '^use\|^)' | tr -d ' \t' | while read module; do
  [ -n "$module" ] && check_specs "$module"
done
```

The check also verifies `web/specs.md` (handled as a special case after the go.work loop).

**Alternatives considered**: 
- Reading `go.work` with a dedicated Go tool would require compilation; shell is simpler.
- Hardcoding the module list would require manual updates when `go.work` changes.
- Storing the module list in a separate `.txt` file would add a new artifact to maintain.

---

## Decision: Check Implementation — Whitespace-Only Detection

**Decision**: A `specs.md` file is considered **missing** if:
1. The file does not exist at the expected path (`<module>/specs.md`), or
2. The file exists but is empty (0 bytes), or
3. The file exists but contains only whitespace (spaces, tabs, newlines).

Detection approach: `test -f <file> && [ -s <file> ]` for existence and non-empty, plus a regex or `grep` check: `grep -q '[^[:space:]]' <file>` to verify at least one non-whitespace character.

**Rationale**: Maintainer ruling D3 specifies that empty or whitespace-only specs.md counts as missing. During spec authoring, a file may be created but left empty or partially filled; the check must catch this state and flag it as noncompliant, treating it identically to a missing file. This ensures the check prevents incomplete work from being merged.

**Alternatives considered**: Trusting file existence alone would allow empty files to pass; that violates D3.

---

## Decision: Check Output Format & Exit Codes

**Decision**: The check outputs:
- A list of all checked modules (e.g., "Checked: agent, api, audit-syslog-bridge, ..., web (15 modules)").
- For each missing or empty `specs.md`, an error line: `ERROR: <module>/specs.md is missing or empty`.
- A final summary line: `N file(s) missing` or `All specs.md files present and non-empty`.
- Exit code 0 if all modules have valid specs.md; exit code 1 if any module lacks a valid specs.md.

Example output:
```
Checked: agent, api, audit-syslog-bridge, ..., web (15 modules)
ERROR: svcutil/specs.md is missing or empty
ERROR: tunnel/specs.md is missing or empty
2 file(s) missing
```

**Rationale**: Clear, machine-parseable output enables CI to report which modules are noncompliant. The exit code enables CI conditional logic (e.g., "lint job fails if check exits 1"). The summary line provides human-readable feedback at a glance.

**Alternatives considered**: JSON output would be more machine-friendly but adds complexity; plain text with error prefixes is sufficient for CI integration.

---

## Decision: Check Wiring into make lint & CI Integration

**Decision**: 
- Create a new Makefile target `.PHONY: check-specs` that invokes `hack/check-specs.sh`.
- Update the existing `make lint` target (currently at line 229 in Makefile) to depend on `check-specs` as a prerequisite. This enables local `make lint` to run the check during development (subject to D6 restrictions on `make lint` itself).
- In `.github/workflows/ci.yaml`, add a new step inside the existing per-module `lint` matrix job, gated `if: matrix.module == 'netguard'` (using the same pattern as the existing "verify lint gate configuration" step), that runs `make check-specs` or invokes `hack/check-specs.sh` directly. This ensures CI enforces the check without creating a separate CI job.

**Rationale**: Maintainer ruling D5 (2026-09-01, post-review) explicitly authorizes the CI wiring. D5 supersedes the CI half of D1, clarifying that the Makefile's `check-specs` target must be linked into CI via a dedicated step in the existing lint job, since CI does **not** invoke `make lint` — the lint job (`.github/workflows/ci.yaml` lines 327–388) directly calls `golangci/golangci-lint-action` for Go modules and `npm run lint` for web. To enforce the check in CI, a dedicated step must be added to an existing job (the lint job) rather than relying on an indirect `make lint` call that does not exist in CI. By adding a step to the existing lint job, gated on `matrix.module == 'netguard'` (ensuring a single execution, not 14 redundant runs), we enforce the check in CI while satisfying the "no new CI job" constraint (it is a step within an existing job, not a new job). The Makefile's `check-specs` target remains available for local development, where it may be invoked directly (subject to D6 restrictions).

**Alternatives considered**: 
- Creating a separate CI job would add another job to the CI matrix, increasing latency; adding a step to the existing lint job is faster and simpler.
- Relying solely on the Makefile dependency would not cause CI to run the check, since CI does not call `make lint` for Go modules.

---

## Decision: Local Execution of hack/check-specs.sh is Permitted

**Decision**: Running `hack/check-specs.sh` locally (via `make check-specs` or directly via bash) is explicitly permitted as a read-only static file inspection, qualifying as a **compile check exception** under CLAUDE.md rule 8.

**Rationale**: Maintainer ruling D6 (2026-09-01) clarifies that the check script is a POSIX shell script performing only static file existence and non-emptiness validation — no test framework, no network, no container runtime, no compilation. This is functionally equivalent to `go build ./...` or `tsc --noEmit` (compile checks), which CLAUDE.md rule 8 explicitly permits for catching obviously broken code before pushing. Running `hack/check-specs.sh` locally does not violate the rule prohibiting local execution of test/lint suites; it is a pre-flight check, not a full suite. In contrast, the full `make lint` target (which includes `golangci-lint`, `npm run lint`, and the specs check) remains CI-only per CLAUDE.md rule 8 and constitution Principle VI — the Makefile's `lint: check-specs lint-go lint-web` target should not be invoked locally. Developers may run `make check-specs` or `hack/check-specs.sh` directly to verify compliance before pushing, but must not run the full `make lint`.

**Alternatives considered**: 
- Disallowing all local script execution would require developers to push speculatively and wait for CI feedback, increasing iteration latency.
- Treating the check as equivalent to the full lint suite would artificially gate a low-cost, high-signal verification that can run locally without CI infrastructure.

---

## Decision: No Iteration of modules/* Directories

**Decision**: The check does **not** iterate over `modules/<game>/` directories to verify they contain specs.md files. The `modules/` directory is a git submodule pointing to the external `gameplane-module` repo; specifications for game modules are the responsibility of that separate repository's own CI.

**Rationale**: Maintainer ruling D2 specifies this boundary. The `gameplane-module` repo has its own CI and its own documentation requirements. The guideline that each `modules/<game>/` directory should contain a specs.md is codified in `docs/module-authoring.md` (as a written guideline, not a repo-level enforcement rule) and enforced in that repo's own CI. Attempting to validate game module specs from the Gameplane repo would:
1. Require reading into a submodule's directory, complicating the check.
2. Create a cross-repo coupling that the maintainer explicitly did not ask for.
3. Add scope creep beyond the feature's intent (Gameplane's own modules + web).

**Alternatives considered**: Include a comment in the check script explaining why modules/ is excluded, for clarity.

---

## Decision: Audit Results — Current specs.md Coverage

**Decision**: Current state (as of 2026-09-01):
- **Modules in go.work with valid specs.md**: agent, api, audit-syslog-bridge, capture-sidecar, gameaction, gameproto, mcp-server, netguard, operator, sentinel, telemetry-receiver, test/e2e (12 of 14 modules).
- **Modules in go.work without specs.md**: **svcutil, tunnel** (2 missing).
- **web/specs.md**: Present and valid.
- **Modules submodule** (`modules/minecraft-java`, `modules/valheim`, etc.): Zero specs.md files in any game directory (as expected, per D2).

Implication: Upon completion of FR-001 and FR-003 (writing svcutil/specs.md and tunnel/specs.md), the repository will be 100% compliant with FR-001 (all 14 go.work modules + web = 15 total, 100% coverage).

**Rationale**: The audit confirms that only svcutil and tunnel are missing; no other active modules lack specs.md. The check's negative test case (simulated missing/empty specs.md → non-zero exit) will be validated in CI by temporarily removing or emptying a file during the check script's own tests (if such tests are added to CI).

**Alternatives considered**: Pre-creating empty placeholder files at svcutil/specs.md and tunnel/specs.md to "pass" the check would violate the spirit of the feature; writing proper specs is the whole point.

---

## Decision: modules/<game>/specs.md Guideline Placement

**Decision**: The guideline requiring each `modules/<game>/` directory to contain a specs.md file is documented in `docs/module-authoring.md`, as a written best-practice recommendation and enforcement rule for the `gameplane-module` repo's own CI, not as a requirement checked by Gameplane's lint job.

**Rationale**: Maintainer ruling D2 specifies that modules/<game>/specs.md requirement is a guideline in docs/module-authoring.md, not codified in this repo's CI. The gameplane-module repo is a separate repository with separate CI; it is not Gameplane's responsibility to enforce its specs.md requirement. However, Gameplane's own documentation should clarify that game modules are expected to include specs.md, so maintainers and contributors of game modules know the standard. A brief addition to `docs/module-authoring.md` in a subsection like "Module Documentation" or "specs.md" states: "Each game module directory should include a `specs.md` file documenting the module's purpose, console/RCON protocol details, configuration parameters, and any game-specific implementation notes. See the constitution and other module examples for the expected structure."

**Alternatives considered**: Leaving game module specs.md as unwritten convention would risk inconsistency across the gameplane-module repo; documenting it in Gameplane's authoring guide makes the expectation explicit without enforcing it here.

---

## Summary

The feature is fully specified once:
1. `svcutil/specs.md` is written, correcting FR-002's coverage gate to 90%.
2. `tunnel/specs.md` is written, documenting the relay supervisor and CRD contracts.
3. Both follow the canonical structure defined in this research.
4. `hack/check-specs.sh` is implemented, parsing go.work and verifying all 15 modules.
5. `make check-specs` is added to the Makefile and integrated into `make lint`.
6. `docs/module-authoring.md` is updated with the modules/<game>/specs.md guideline.

All decisions are grounded in maintainer rulings D1–D6 (issued 2026-09-01, with D5 and D6 issued post-review) and the constitution's principles (Principle IV: spec-driven development; Principle VI: CI bears the heavy lifting).
