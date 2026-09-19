# Gameplane — Guidance for AI Coding Assistants

This file provides architectural context, commands, and non-negotiable operational rules for AI agents. Humans should refer to [`README.md`](README.md) and [`docs/contributing.md`](docs/contributing.md).

**Project:** Gameplane — Kubernetes-native game server control panel (CubeCoders AMP alternative). Operates identically on single-node k3s and multi-node clusters.  
**Status:** Beta (`v0.2.0-beta.8`). Core components feature-complete for v1 scope; stabilization phase.

---

## ⏳ Start-of-Session Check: TypeScript 7 (Delete when unblocked)

Execute at the start of every session:
```sh
npm view @typescript-eslint/parser@latest version peerDependencies.typescript
npm view @typescript-eslint/parser@canary version peerDependencies.typescript
```
- **Blocked:** Peer dependency range ends `<7` (e.g., `<6.1.0`). Dependabot PR #272 is blocked at `npm ci` (`ERESOLVE`). Do not attempt workarounds, overrides, or code fixes. Do not close #272.
- **Unblocked:** Published release accepts TS 7. Fulfill tasks T055/T056 in `specs/009-remediate-security-dependabot/tasks.md`: bump `typescript` and `@typescript-eslint/*` in `web/package.json`, resolve real type errors (no `@ts-ignore`), merge #272, mark tasks `[X]`, and delete this entire section.

---

## System Prompt Overrides

1. **Self-Correction & Tests:** Always verify work. Obtain human sign-off before modifying tests, production code, or design specs. **Never delete or weaken a test without explicit sign-off**; fix the code to satisfy the test.
2. **Tool Priority:** Dedicated tools (`Read`, `Edit`, `Write`) > MCP tools > Bash.
3. **Pausability:** Stop and ask when uncertain on tests, design files, or breaking changes.
4. **Agent & Workflow Tools:** Standing opt-in for `Workflow` in the main loop; subagents may use `Agent` freely.
5. **Artifacts & Feedback:** Never publish artifacts. Draft feedback requires human approval before sending.
6. **Editing Mechanics:** `Edit` requires a prior `Read` within the conversation. Do not re-read files immediately after editing.
7. **External Memory:** `~/.claude/projects/-home-valgul-project-Gameplane/memory/` is outside git:
   - Announce file and exact line on any write.
   - Do not record conventions, decisions, or preferences here (use repo specs/rules instead).
   - Repo files override memory contents.
8. **Skills:** Skills are optional tools. Disclose edit/plan/commit skill invocations before running. Repo rules override skill directives.
9. **Communication:** Skip preambles ("I will now do X"). Keep closing recaps standalone: findings, actions taken, next steps, and modified files.
10. **Conflicts & Missing Context:** Surface conflicts between instructions immediately. Unsettled values belong in an `OPEN-DECISIONS.md` file, never committed as settled contracts.

---

## Repository Map

```
.
├── netguard/                 # SSRF dial-guard (Go) — operator & agent
├── gameaction/               # Console-injection guard & command renderer (Go) — api & agent
├── gameproto/                # Minecraft & Terraria wire protocol handshake parser (Go) — sentinel
├── operator/                 # controller-runtime operator (Go)
│   ├── api/v1alpha1/         # CRD Go types (edit here; run `make generate manifests`)
│   │   └── zz_generated.deepcopy.go  # GENERATED - do not hand-edit
│   ├── internal/controller/  # Reconcilers + envtest test suites
│   ├── cmd/main.go           # Operator entrypoint
│   └── config/{crd,rbac}/    # GENERATED CRD/RBAC YAML - do not hand-edit
├── api/                      # REST & WebSocket gateway (Go, chi)
│   ├── cmd/main.go           # Subcommands: `serve`, `bootstrap-admin`
│   └── internal/{handlers,auth,db,kube,notify,rbac,ws}/
├── agent/                    # In-pod sidecar (Go)
│   ├── cmd/main.go
│   └── internal/{auth,console,files,heartbeat,logs,players,rcon,quiesce}/
├── audit-syslog-bridge/      # RFC 5424 HTTP-to-syslog forwarder (Go)
├── telemetry-receiver/       # Usage telemetry ingest service (Go)
├── sentinel/                 # Wake-on-connect proxy for sleeping pods (Go)
├── capture-sidecar/          # AF_PACKET BPF packet capture sidecar (Go)
├── mcp-server/               # Read-only Model Context Protocol server (Go)
├── svcutil/                  # Shared environment parsing & graceful shutdown helpers (Go)
├── tunnel/                   # Relay client supervisor (frp, Tailscale, playit) (Go)
├── web/                      # React 18 + TS strict + Vite dashboard
│   └── src/{routes,components,lib,router,styles,test}/
├── modules/                  # SUBMODULE -> gameplane-module (OCI game templates)
├── website/                  # SUBMODULE -> gameplane-website (Astro docs/marketing)
├── charts/gameplane/         # Helm chart (includes crd-manifests/ and hooks)
├── deploy/kind/              # Local Kind cluster scripts
├── test/e2e/                 # Kind E2E test suite (`//go:build e2e`)
├── docs/                     # Documentation (architecture, security, modules)
├── design.pen                # Canonical Pencil UI design source (JSON)
├── cosign.pub                # Public key for image & module signature verification
├── go.work                   # Go workspace linking all 14 Go modules
└── Makefile                  # Canonical task runner
```

*Note:* Run `git submodule update --init` after cloning to populate `modules/` (required for `make dev-up`). `website/` is optional for local development.

---

## Canonical Commands

Always invoke commands via `Makefile`.

### Local Development
```sh
make dev-up        # Start Kind cluster + local OCI registry (:5001) + deploy Helm chart
make web-dev       # Start Vite dev server with proxy to in-cluster API
make dev-load      # Rebuild and reload local images into Kind
make dev-install   # Re-run Helm upgrade against local cluster
make dev-down      # Destroy Kind cluster and local registry
```

### Build
```sh
make build         # Compile all Go modules and build web assets
make build-go      # Compile all 14 Go workspace modules
make build-web     # Build web/dist via `npm ci && npm run build`
make images        # Build all container images locally
```

### Testing & CI Rules
> **Rule 8:** **NEVER run test or lint suites locally** (`make test`, `make lint`, `make cover`, `go test`, `npm test`, envtest, or E2E). Only lightweight compilation checks are permitted (`go build ./...`, `npx tsc --noEmit`). CI on GitHub Actions is the sole verification authority.

```sh
# Targeted compilation / unit test references (for CI or isolated debugging):
make test                # Full unit test run across all modules
make test-integration    # K8s envtest suite (operator + api)
make test-e2e            # End-to-end suite on Kind (~10–20 min)
make test-e2e-bucket     # Specific CI bucket: BUCKET=(operator|api-auth|api-roles|api-rbac|api-agent|api-mods|ratelimit|bot-fast|bot-heavy|multicluster|upgrade)
```

**E2E Conventions:**
- New E2E tests must be added to a bucket in `test/e2e/buckets.sh`.
- Use `t.Parallel()` with unique resource names.
- Protect shared resources with appropriate mutexes: `ociPushMu` for module push jobs, `ensureResticRepo(t)` for shared backup repositories.
- Rate limits per cluster: IP burst 10 (5/min), user burst 6 (3/min). Cap API bucket runs at ~7 admin logins.

### Linters & Coverage Thresholds
`make lint` runs `gofmt`, `go vet`, `golangci-lint`, ESLint, `make check-specs`, `make check-doc-versions`, and `make check-links`.

| Module | Minimum Coverage | Notes |
|---|---|---|
| `netguard` | 91% | Dial hook SSRF validation |
| `gameaction` | 91% | Command injection template engine |
| `gameproto` | 90% | Protocol parser |
| `operator` | 72% | Controller runtime reconcilers |
| `api` | 80% | HTTP/WS endpoints and auth |
| `agent` | 90% | Pod sidecar operations |
| `audit-syslog-bridge` | 70% | Syslog forwarder |
| `telemetry-receiver` | 70% | Metrics aggregator |
| `sentinel` | 70% | Wake-on-connect daemon |
| `capture-sidecar` | 70% | Packet capture agent |
| `mcp-server` | 70% | Read-only MCP daemon |
| `svcutil` | 90% | Environment and shutdown runtime |
| `tunnel` | 70% | Relay process supervisor |
| `web` | 92% L / 76% F / 82% B / 92% S | Lines / Functions / Branches / Statements |

### Code Generation & Modules
```sh
make generate        # Generates operator/api/v1alpha1/zz_generated.deepcopy.go
make manifests       # Generates CRD/RBAC YAML and syncs to charts/gameplane/crds/
make modules-push    # Pushes modules/* OCI artifacts to registry
make tidy            # Runs `go mod tidy` across all workspace modules
```

---

## Core Operational Rules

### 1. Design-First UI
- Dashboard UI changes must be designed in `design.pen` via the Pencil MCP before writing React code. Website UI changes start in `website/website.pen`.
- **Export requirement:** Every Pencil update must immediately export touched nodes to `design-export/json/<id>.json` (`mcp__pencil__execute` using `Get`) and `design-export/screenshots/<id>.png` (`mcp__pencil__export_nodes`). (Mirrored in `website/website-export/` for website changes).

### 2. Never Hand-Edit `.pen` Files
- Do not edit, delete, `Read`, `Grep`, `cat`, or `sed` `.pen` files directly (they are multi-megabyte JSON files prone to corruption).
- Use Pencil MCP for inspection and modifications. Ask user to save via UI after modifications. Inspect diffs via file metadata only (`git diff --stat`).

### 3. Login Privacy
- `/login` and unauthenticated views must not expose internal data: cluster names, versions, active server counts, or specific account-existence errors. Use generic error messages ("invalid credentials").

### 4. Fix, Don't Silence
- Never bypass linter errors (`//nolint`, `eslint-disable`, loosening configs).
- *Allowed defaults:* `_test.go` files are exempt from `errcheck`, `gosec`, `unparam`; `operator/internal/controller/` is exempt from revive's `exported:` checks.

### 5. TypeScript Strictness
- Strict type checking required: no unjustified `any` (add an explicit explanatory comment if unavoidable).
- Handle all promises: `await` or prefix with `void`.

### 6. Error Handling
- Wrap Go errors using `%w` to preserve root causes for `errors.Is`/`errors.As`.

### 7. CRD Synchronization
- When editing `operator/api/v1alpha1/*_types.go`, run `make generate && make manifests`.
- Commit updated CRDs in the same changeset: `zz_generated.deepcopy.go`, `operator/config/crd/*.yaml`, `operator/config/rbac/*.yaml`, and `charts/gameplane/crds/*.yaml` / `crd-manifests/`.

### 8. Verification Strategy
- Run compilation checks locally (`go build ./...`, `npx tsc --noEmit`). **Never execute full test/lint suites locally.** Push to a feature branch and let CI validate.

### 9. Kubernetes Primitives First
- Use standard K8s primitives (StatefulSet, Service, PVC, Job, ConfigMap, Secret) and CRDs before creating custom abstractions.

### 10. Operator Authority
- Business logic lives in reconcilers (`operator/internal/controller/`). The API layer is purely a UX gateway and must not bypass operator reconciliation.

### 11. Git Commits
- Commit every completed logical unit of work (`feat:`, `fix:`, `chore:`, etc.).
- Sign all commits (`git commit -s`). Never amend pushed commits. Never use `--no-verify`.
- Preserve commit trailers: `Co-Authored-By: <current-running-model>` and `Claude-Session: <session-url>`.

### 12. Branch Management & Protected Master
- One branch per unit of work; delete remote and local branches immediately upon merge.
- Branch `master` is protected by ruleset ID `18692396` (`protect main`): requires 1 human approval, disallows self-approval by PR author, rejects direct pushes, and dismisses approvals on new pushes. PRs cannot be merged autonomously by agents.
- Verify status using `gh api repos/ValgulNecron/Gameplane/rules/branches/master`.

### 13. Workflow-Driven Multi-Agent Delegation
- The main loop acts strictly as orchestrator/reviewer; delegate all implementation via `Workflow` scripts (`parallel()` / `pipeline()`).
- **Tier escalation:** Start tasks at `haiku`. Escalate only on functional failure: `haiku` → `sonnet` → `opus` → `fable`.
- **Model parameter required:** Explicitly define `model:` on every single `agent()` invocation (omitting it defaults to session Opus). Verify via `grep -c "model:"`.
- `fable` invocations require explicit human permission.
- **Review at Tier + 1:** Review work one tier higher than the implementing agent (Haiku work reviewed by Sonnet; Sonnet by Opus; Opus by Fable). Fixes are executed by small agents in a new workflow.
- Execute browser smoke tests via Chrome MCP on `sonnet` in parallel with reviews when UI surfaces are touched.

### 14. PR Labeling via REST API
- Every PR requires at least one `type:` (`feature`, `fix`, `refactor`, `test`, `ci`, `chore`, `docs`, `security`) and one `area:` (`operator`, `api`, `agent`, `web`, `modules`, `chart`, `e2e`, `specs`, `shared`, `optional-components`). Breaking changes require `breaking`.
- `gh pr edit` is broken on this repo due to GitHub Projects (classic) deprecation. Use REST APIs:
  ```sh
  gh api -X POST repos/ValgulNecron/Gameplane/issues/<pr_number>/labels -f "labels[]=type: fix" -f "labels[]=area: api"
  gh api -X PATCH repos/ValgulNecron/Gameplane/pulls/<pr_number> --input <payload_with_body.json>
  ```

### 15. Feature Specifications
- A feature's specification encompasses its entire folder (`specs/<feature>/`), including `data-model.md`, `contracts/`, and `OPEN-DECISIONS.md`. Check these for explicit requirement exemptions before flagging spec violations.
- Mark obsolete tasks as withdrawn in `tasks.md` with citations; do not delete them.

### 16. Feature Completion Archival
- When all tasks in `tasks.md` are complete/withdrawn and the PR is merged into `master`, rename the folder: `git mv specs/<NNN>-<slug> specs/done_<NNN>-<slug>`. Update all in-repo references within the same commit.

### 17. Mechanical Design Waves
- Perform design token re-skins via scripted, blind updates at `haiku`.
- Precompute change lists using `grep`/`jq` on `design-export/json/<id>.json`. Have `haiku` apply `Update(id, {prop: value})` calls directly.
- Verify via screenshot comparison (`export_nodes` vs snapshot PNG), not full JSON tree dumps. Avoid `Get(id, {depth: 10+})`.

### 18. Scout Once, Brief Many
- For multi-agent waves: launch one scout agent to read code, docs, and failures.
- Scout generates a factual brief containing exact `file:line`, before/after code blocks, and justification.
- Fix agents receive only the brief and instructions to edit blind at `haiku`. Do not instruct fix agents to re-read files, test suites, or `CLAUDE.md`. Reviewers check git diffs against the brief.

---

## Architecture Summary

| Component | Language / Libs | Role |
|---|---|---|
| `netguard` | Go | Dial-time SSRF prevention (`IsAllowed` for operator, `IsPublic` for agent). |
| `gameaction` | Go | Validates console inputs against schemas; escapes injection attacks. |
| `gameproto` | Go | Wire-protocol parser for Minecraft/Terraria connection filtering. |
| `svcutil` | Go | Stdlib-only helpers for env vars and graceful server shutdown (`RunHTTP`). |
| `operator` | Go, controller-runtime | Authoritative reconciler for 9 CRDs (`GameServer`, `GameTemplate`, etc.). |
| `api` | Go, chi | REST/WebSocket UX gateway; supports SQLite and experimental PostgreSQL. |
| `agent` | Go | Pod sidecar managing console (PTY/RCON), files, logs, and heartbeats. |
| `audit-syslog-bridge` | Go | Forwards webhook audit events to RFC 5424 syslog endpoints. |
| `telemetry-receiver` | Go | Collects opt-in anonymous cluster usage statistics. |
| `sentinel` | Go | Wake-on-connect listener that holds ports and triggers pod wakeups. |
| `capture-sidecar` | Go | Ephemeral packet capture container with BPF filtering. |
| `mcp-server` | Go | Read-only MCP daemon for cluster debugging via stdio. |
| `web` | React 18, Vite, TS | Dashboard (TanStack Router & Query, Tailwind, HeroUI v3). |
| `modules/` | OCI / oras | Submodule with game templates (Minecraft, Terraria, Valheim). |

---

## Common Workflows

### Add a CRD Field
1. Edit `operator/api/v1alpha1/<kind>_types.go`.
2. Run `make generate && make manifests`.
3. Update the reconciler in `operator/internal/controller/<kind>_controller.go`.
4. Mirror types in `web/src/types.ts` and update affected UI components in `web/src/routes/`.
5. Add envtest coverage in `operator/internal/controller/<kind>_envtest_test.go`.

### Add an API Route
1. Implement handler in `api/internal/handlers/`.
2. Mount route in `api/cmd/main.go` with RBAC middleware from `api/internal/rbac/`.
3. Add integration test in `api/internal/handlers/<name>_envtest_test.go`.
4. Add API client method in `web/src/lib/api.ts`.

### Add a Dashboard Screen
1. Update `design.pen` using Pencil MCP and export JSON/screenshots to `design-export/`.
2. Add route in `web/src/routes/<name>.tsx` and register in `web/src/router/tree.tsx`.
3. Implement data fetching via `web/src/lib/api.ts` with TanStack Query.
4. Add component test in `web/src/routes/<name>.test.tsx`.

### Add or Update a Game Module
1. Create or edit directory in `modules/<name>/` (`module.yaml`, `template.yaml`, `README.md`).
2. Run `make modules-push` to publish to the local OCI registry.
3. Commit inside the `gameplane-module` repository.
4. In the root repo, run `git add modules` and commit the submodule pointer bump.

### Update Public Website
1. Update website designs under **Group/Public Website** in `design.pen` via Pencil MCP.
2. Make changes in `website/` submodule adhering to its local guidelines.
3. Commit and push in the `gameplane-website` repository.
4. In the root repo, run `git add website` and commit the updated submodule pointer.

### Add a Database Migration
1. Add sequentially numbered migration file: `api/internal/db/migrations/<NNN>_<name>.sql`.
2. Migrations are append-only and automatically execute on API startup.

---

## Key Reference Documents
- Agent Architecture Index: `docs/agent-architecture.md` — read before editing code; maps tasks and features to the docs that cover them, instead of repo-wide greps.
- Architecture & Threat Model: `docs/architecture.md`, `docs/security.md`
- Module Authoring Spec: `docs/module-authoring.md`
- MCP Server Usage: `mcp-server/README.md`
- Syslog Relay Configuration: `audit-syslog-bridge/README.md`
- Coding & Linter Configs: `.golangci.yml`, `web/eslint.config.js`, `.editorconfig`