# Gameplane — guidance for AI coding assistants

This file is for AI coding assistants (Claude Code and similar). It exists so a fresh agent session can plan a change without re-deriving the project's structure, commands, and house rules from scratch. Humans should read [`README.md`](README.md) and [`docs/contributing.md`](docs/contributing.md) instead — those are written for people; this is written for agents.

**Project**: Gameplane — a Kubernetes-native game server control panel. Open-source alternative to CubeCoders AMP, built on K8s primitives so the same operational model works on a single-node k3s homelab and a multi-node production cluster.

> **Status:** beta (`v0.2.0-beta.5`). CRDs, operator, API, agent, and dashboard are feature-complete for the v1 scope and stabilized for external testing; not yet recommended for unattended production. See README "Beta status & known limitations".

> **AI tooling provenance:** the project was started with Claude Code on Claude Opus 4.8 (`claude-opus-4-8`); since June 2026 development continues on Claude Fable 5 (`claude-fable-5`). This is informational only — nothing in this file is model-specific.

---

## Repo map

```
.
├── netguard/                 # shared SSRF dial-guard package (Go) — used by operator + agent
├── operator/                 # controller-runtime operator (Go)
│   ├── api/v1alpha1/         # CRD Go types — edit here, then `make generate manifests`
│   │   └── zz_generated.deepcopy.go    # GENERATED — do not hand-edit
│   ├── internal/controller/  # reconcilers + co-located *_envtest_test.go
│   ├── cmd/main.go           # operator entry point
│   └── config/{crd,rbac}/    # GENERATED CRD/RBAC YAML — do not hand-edit
├── api/                      # REST + WebSocket gateway (Go, chi)
│   ├── cmd/main.go           # `serve` and `bootstrap-admin` subcommands
│   └── internal/{handlers,auth,db,kube,notify,rbac,ws}/
├── agent/                    # in-pod sidecar (Go)
│   ├── cmd/main.go
│   └── internal/{auth,console,files,heartbeat,logs,players,rcon,quiesce}/
├── audit-syslog-bridge/      # optional HTTP-JSON → syslog relay image (Go), behind the audit webhook sink
├── telemetry-receiver/       # optional anonymous-usage-telemetry collector image (Go), behind the API's telemetry reporter
├── web/                      # React 18 + TS strict + Vite dashboard
│   └── src/{routes,components,lib,router,styles,test}/
├── modules/                  # GIT SUBMODULE → gameplane-module repo (game template OCI bundles)
│   ├── minecraft-java/  valheim/  terraria/
│   └── build.sh              # OCI bundle builder/pusher (uses oras ≥ 1.2.0)
├── website/                  # GIT SUBMODULE → gameplane-website repo (public marketing + docs site)
├── charts/gameplane/           # Helm chart
├── deploy/kind/              # local dev cluster scripts
├── test/e2e/                 # kind-based E2E suite (build tag: e2e)
├── docs/                     # human-facing docs (architecture, contributing, security, …)
├── design.pen                # Pencil design source — encrypted, MCP only
├── cosign.pub                # public key for verifying signed images + module bundles
├── go.work                   # Go workspace linking netguard/operator/api/agent/audit-syslog-bridge/telemetry-receiver/test/e2e
└── Makefile                  # canonical entry point for every command
```

The Go modules `netguard`, `operator`, `api`, `agent`, `audit-syslog-bridge`, `telemetry-receiver`, and `test/e2e` share one workspace via `go.work`. The `web/` tree is its own npm package.

`modules/` is a **git submodule** pointing at the separate `gameplane-module` repo. After a fresh clone, run `git submodule update --init` (or clone with `--recurse-submodules`) before `make dev-up` / `make modules-push` — otherwise `modules/` is an empty directory and those targets find no `build.sh`.

`website/` is a **git submodule** pointing at the separate `gameplane-website` repo — the public marketing + docs site (Astro + Tailwind 4, deployed to GitHub Pages at <https://valgulnecron.github.io/gameplane-website/>). Nothing in this repo's build depends on it; it's safe to leave uninitialized.

---

## Common commands

The `Makefile` is the source of truth — these are the targets you'll actually use. Don't run lower-level `go build`/`npm run` recipes unless the Make target doesn't cover what you need.

### Local dev cluster

```sh
make dev-up        # creates kind cluster + local OCI registry + installs Helm chart
make web-dev       # starts the Vite dev server with proxy to the in-cluster API
make dev-down      # tears it all down
make dev-load      # rebuild and load images into kind
make dev-install   # re-run helm upgrade against the local cluster
```

`make dev-up` brings up a kind cluster from `deploy/kind/cluster.yaml` plus a local OCI registry on `localhost:5001` (cluster-internal name `kind-registry:5000`), loads the locally-built operator/api/agent images, pushes every `modules/*` directory as an OCI bundle, and installs the chart from `charts/gameplane/`.

### Build

```sh
make build                       # all components (Go + web)
make build-go                    # compiles every Go module: netguard, operator, api, agent, audit-syslog-bridge, telemetry-receiver
make build-web                   # web/dist via `npm ci && npm run build`
make images                      # docker images: operator, api, agent, audit-syslog-bridge, telemetry-receiver
make image-operator              # one image; same for image-api, image-agent, image-audit-syslog
```

### Test (three tiers)

```sh
make test                # everything (≈ seconds)
make test-go             # Go unit tests across netguard, operator, api, agent, audit-syslog-bridge, telemetry-receiver
make test-web            # vitest for web

make test-integration    # envtest tier (operator + api) — downloads K8s 1.31 envtest assets
make test-e2e            # kind + helm + real components (≈ 10–20 min)
make test-e2e-keep       # re-run e2e against an already-up cluster
make test-e2e-bucket     # one CI bucket (BUCKET=operator|api-auth|api-rbac|api-agent|ratelimit|bot)
```

**e2e test conventions** (CI runs the suite as parallel per-bucket jobs, one kind cluster each):

- A new e2e test MUST be added to a bucket in `test/e2e/buckets.sh` — the `e2e bucket coverage` CI job fails on any unbucketed test.
- New tests call `t.Parallel()` and use per-test unique resource names. Guards for shared state: `ociPushMu` (module tests sharing the fixed-name oras-push Job), `ensureResticRepo(t)` (anything running a backup against the shared restic repo).
- Budget API tests by logins, not CPU: each job's cluster rate-limits logins per IP (burst 10, 5/min) and per user (burst 6, 3/min), and every test in a job shares one IP through the port-forward. Keep an api bucket at ~7 admin logins. Tests observing raw login status codes must stay non-parallel.

Per-component fallbacks when you want to focus:

```sh
cd netguard && go test ./...
cd operator && go test ./...
cd api      && go test ./...
cd agent    && go test ./...
cd audit-syslog-bridge && go test ./...
cd telemetry-receiver && go test ./...
cd web      && npm test
```

### Lint & coverage

```sh
make lint            # gofmt + go vet + golangci-lint + ESLint
make lint-go         # only Go
make lint-web        # only web

make cover           # full coverage with threshold gates (CI-equivalent)
make cover-ratchet   # measured-vs-threshold delta per module
```

Coverage gates: `netguard/.testcoverage.yml` (91%), `operator/.testcoverage.yml` (72%), `api/.testcoverage.yml` (80%), `agent/.testcoverage.yml` (90% — re-baselined down from 91% when the SSRF dial guard moved into `netguard`, which now carries and gates that coverage instead), `audit-syslog-bridge/.testcoverage.yml` (70%), `telemetry-receiver/.testcoverage.yml` (70%), `web/vitest.config.ts` (lines 92% / functions 76% / branches 82% / statements 92%). Don't lower thresholds without a reason; ratchet them up when adding tests.

### Codegen — mandatory after CRD type edits

```sh
make generate    # regenerates operator/api/v1alpha1/zz_generated.deepcopy.go
make manifests   # regenerates operator/config/crd/*.yaml + operator/config/rbac/*.yaml and syncs charts/gameplane/crds/
```

Forgetting these leaves the CRD YAML out of sync with the Go types — CI will catch it, but your envtest run will fail mysteriously first.

### Modules and miscellany

```sh
make modules-push     # builds + pushes every modules/* dir to MODULE_REGISTRY
make tidy             # `go mod tidy` across all Go modules
make clean            # remove bin/, dist/, web/dist
```

---

## Project-specific rules

These are the rules an agent cannot infer from reading the code. They are deliberately Gameplane-specific — generic Claude Code defaults (terse responses, no half-finished implementations, comments only when *why* is non-obvious) are already in your system prompt and don't need restating here.

### 1. Design-first for UI changes

Any change to the web dashboard's visual surface starts in **`design.pen`** (Pencil), not in code. Update the relevant screen via the `pencil` MCP server, then translate to React.

- *Why:* `design.pen` holds 18 designed screens that are the source of truth. Code-led redesigns get reverted.
- *How:* `mcp__pencil__open_document` → `mcp__pencil__get_editor_state` → edit the relevant frame → translate to React.
- Backend, API, and operator changes do **not** need a Pencil pass.

### 2. Never delete or text-edit `design.pen`

The file is encrypted — only the `pencil` MCP server can read/write it.

- **Don't:** `Read`, `Grep`, `sed`, or `cat` `design.pen`. Don't `rm` it. Don't `git rm` it.
- **Do:** use `mcp__pencil__open_document`, `mcp__pencil__batch_get`, `mcp__pencil__batch_design`, `mcp__pencil__get_screenshot`.

### 3. Login privacy: no pre-auth telemetry surface

The login page (`web/src/routes/Login.tsx`) and any unauthenticated screen must not display internal metrics, counts, hostnames, version strings, cluster names, or anything that aids user enumeration.

- *Why:* the login page is internet-reachable on most installs (`docs/security.md` covers the threat model).
- **Don't:** render `cluster: prod-east-1`, `5 servers online`, `Gameplane v0.4.2-rc3`, or "user `alice` not found" on `/login`.
- **Do:** keep it to brand + form + neutral error copy ("invalid credentials" — never "wrong password" vs "no such user").

### 4. Fix, don't silence

When `golangci-lint` or ESLint flags something, **fix the code** — do not add suppression directives or remove rules from config.

- **Don't:** `//nolint:errcheck`, `// eslint-disable-next-line`, deleting linters from `.golangci.yml`, loosening `web/eslint.config.js`.
- **Do:** fix the underlying issue. If a rule is genuinely wrong for a justified case, raise it with the maintainer rather than silencing it inline.

Existing exemptions you don't need to re-derive (already in `.golangci.yml`):

- `_test.go` files are exempt from `errcheck`, `gosec`, and `unparam`.
- `operator/internal/controller/` is exempt from revive's `exported:` rule (controller builder helpers don't need godoc strings).

Don't add new exemptions on top of these.

### 5. TypeScript strict; no unjustified `any`

`web/tsconfig.json` enables `strict`, `noUnusedLocals`, `noUnusedParameters`, `noFallthroughCasesInSwitch`. ESLint enforces `@typescript-eslint/no-explicit-any: error` and `@typescript-eslint/no-floating-promises: error`.

- If `any` is genuinely unavoidable (interop with an untyped lib), leave a one-line comment stating *why*.
- To satisfy `no-floating-promises`: either `await` the Promise or prefix it with `void`. Don't disable the rule.

### 6. Go errors wrap with `%w`

Always preserve the cause so `errors.Is` / `errors.As` keep working up the stack.

```go
// good
return fmt.Errorf("reconcile gameserver %s: %w", gs.Name, err)

// bad — discards the cause
return errors.New("reconcile failed: " + err.Error())

// bad — strips the cause silently
return fmt.Errorf("reconcile failed")
```

### 7. After touching CRD Go types, regenerate

CRD types live in `operator/api/v1alpha1/*_types.go`. After any edit there:

```sh
make generate && make manifests
```

Commit the regenerated files in the same change:

- `operator/api/v1alpha1/zz_generated.deepcopy.go`
- `operator/config/crd/*.yaml`
- `operator/config/rbac/*.yaml`
- `charts/gameplane/crds/*.yaml` (synced automatically by `make manifests`)

Note: Helm only installs `crds/` on first install — `helm upgrade` never updates CRDs, so e2e/dev clusters must be recreated after CRD schema changes.

### 8. Do NOT run the test or lint suites locally — CI is the source of truth

**This project's tests must run on GitHub Actions, not on the maintainer's machine.** Do not run `make test`, `make lint`, `make cover`, `go test`, `npm test`/`vitest`, or any envtest/kind/e2e suite locally. Instead: write the code, commit per logical unit, push to a feature branch, and let GitHub Actions run the full suite. Watch the run with the `gh` CLI and fix failures with follow-up commits.

- A quick **compile** check is fine — `go build ./...` or `tsc --noEmit` is a compilation, not a test — to avoid pushing obviously-broken code. Running the *test/lint suites* is not.
- Sign commits (`git commit -s`). For UI work, include the Pencil node id(s) you touched in the PR description.

### 9. K8s-native by default

New features should compose CRDs, controllers, and stock primitives (StatefulSet, PVC, Service, Job, ConfigMap, Secret) before reaching for custom plumbing. The same control plane has to work on a single-node k3s and a multi-node prod cluster — anything that assumes a particular host, filesystem layout, or process model breaks the scaling promise. If a desired behavior doesn't fit a CRD/controller cleanly, that's signal to discuss the design before writing code, not signal to bolt on a side-channel.

### 10. The operator is authoritative

The API server is a **UX layer**. It reads CRDs and writes them through, but the controller-runtime operator owns reconciliation. A user must be able to `kubectl apply` a `GameServer` and get the same outcome as creating it through the dashboard.

- **Don't** put business logic in `api/internal/handlers/` that should live in a reconciler (e.g., "when GameServer is created, also create a default Backup").
- **Do** put the logic in the relevant `operator/internal/controller/*_controller.go` and let the API just write the CR.

### 11. Commit regularly — this overrides the default "only commit when asked"

This project standing-orders agents to commit after each logical unit of work. Treat that as a default to follow, not a request you wait for.

- *Why:* without this rule, agents accumulate hundreds of mixed-concern files into a single mega-commit (it has happened on this repo). That destroys reviewability, makes `git bisect` useless, and turns rollbacks into a research project.
- **A "logical unit" is**: one bug fix, one feature slice, one refactor step, one CRD/codegen pair, one passing test addition. Roughly: if you can describe it in one short conventional-commit subject line, commit it.
- **Cadence**: commit before switching topics, before starting a risky change, and at meaningful checkpoints (a compiling, logically-complete unit — see rule 8, tests run on CI not locally). Don't end a working session with > ~10 modified files staged but uncommitted.
- **Mechanics**: sign every commit (`git commit -s`), use conventional-commit prefixes (`feat:`, `fix:`, `chore:`, `refactor:`, `test:`, `docs:`, `ci:`). Never `--amend` a commit you've already pushed; never `--no-verify` to skip hooks. If a pre-commit hook fails, fix the underlying issue and create a new commit. Codegen output goes in the same commit as the source change that triggered it (rule 7).
- **When *not* to commit**: known-broken state (compile errors, failing tests you haven't addressed), partial CRD edits without their regenerated artifacts, anything containing secrets/credentials, or unreviewed bulk reformatting. In those cases, finish the unit first.
- **Pushing**: push at natural checkpoints so work isn't stranded locally, but do **not** force-push `main` and do **not** push obviously broken commits.

### 12. One branch per unit of work — delete it once merged

Every piece of work goes on its own branch (rule 8). The moment that branch is merged into `main`, **delete it** — both the remote (`git push origin --delete <branch>`) and any local copy (`git branch -d <branch>`). Don't leave merged branches lying around.

- *Why:* stale merged branches pile up and make the branch list useless — 53 had accumulated here (49 already merged but never deleted) before this rule. A clean branch list should show only `main` plus genuinely in-progress work.
- **Mechanics:** finish the branch → get it merged into `main` (PR-merge, or — since `main` is unprotected — a `--no-ff` merge pushed to main; CI also runs on `push: [main]`) → immediately delete the branch remote + local. Before ending a session, confirm no merged branch is left behind.
- Never delete a branch whose work is **not** yet in `main`, and never `--delete-branch` a stacked child whose descendants still depend on it (merge bottom-up first).

### 13. Delegate to subagents, smallest model first

Prefer handing work to subagents over doing it in the main loop — exploration, design.pen/Pencil edits, doc passes, mechanical refactors, test writing, conflict grunt-work. The main loop's job is orchestration, judgment, and verification, not the legwork.

- **Size the model bottom-up.** Start every delegated task at the smallest model (`haiku`), and escalate step by step (`haiku` → `sonnet` → `opus`/`fable`) *only* when the smaller model's output is actually inadequate. There's no need for a top-tier model on design/scan/mechanical work that a smaller one handles fine.
- **Mechanics:** pass `model: "haiku"` on the first `Agent` attempt for a well-scoped task, verify the result (screenshot / diff / compile), and re-run one tier up only if needed. Reserve the top model for the main loop's decisions.
- *Why:* cost and latency — the biggest model adds nothing on well-scoped tasks, and concurrent cheap subagents finish the breadth faster.

---

## Architecture quick reference

The detail lives in `docs/architecture.md`; this is the index.

**`netguard/`** — shared Go package: the SSRF dial-guard used by both the operator (`IsAllowed`, permissive — ModuleSource `git`/`http` fetches, since self-hosted registries legitimately live on private/loopback addresses) and the agent (`IsPublic`, strict — `capabilities.mods.install` downloads, which are less trusted). Enforcement happens at dial time via a `net.Dialer.Control` hook, defeating DNS rebinding past a name-based allowlist. See the package doc comment for why the two policies must stay separately selectable.

**`operator/`** — controller-runtime. Reconciles 7 CRDs (`gameplane.local/v1alpha1`) into K8s objects: GameTemplate, GameServer, Backup, BackupSchedule, Restore, Module, ModuleSource. Entry: `operator/cmd/main.go`. Controllers in `operator/internal/controller/`. Inject points (agent image, CA bundle, mTLS certs) wired from CLI flags in `main.go`.

**`api/`** — chi router; REST + WebSocket. Entry: `api/cmd/main.go`, with subcommands `serve` and `bootstrap-admin`. Layout:

- `api/internal/handlers/` — REST handlers (lifecycle, users, modules, destinations, config, audit, events, resources)
- `api/internal/auth/` — local argon2id + OIDC (`coreos/go-oidc/v3`); sessions; rate limiting
- `api/internal/db/` — driver-selectable (modernc.org/sqlite **or** pgx/v5); migrations in `api/internal/db/migrations/`
- `api/internal/kube/` — Kubernetes client wrapper
- `api/internal/notify/` — notification delivery: watches GameServer/Backup/Restore status transitions and pushes events to admin-configured sinks (Discord/Slack/SMTP/webhook); see `docs/notifications.md`
- `api/internal/rbac/` — middleware enforcing the three roles (admin, operator, viewer)
- `api/internal/ws/` — WebSocket bridge for console/log streaming

**`agent/`** — sidecar that runs in every game pod. Entry: `agent/cmd/main.go`. Endpoints: console (PTY/RCON), files, logs, players, heartbeat, quiesce. Speaks token-auth + mTLS back to the operator/API.

**`audit-syslog-bridge/`** — optional, schema-agnostic HTTP-JSON → syslog (RFC 5424) relay image. Sits behind the API's audit webhook sink (`api.audit.webhook.syslogBridge.enabled`) to forward audit events to a syslog/SIEM collector; forwards any JSON webhook body verbatim, so it isn't Gameplane-specific. See `audit-syslog-bridge/README.md`.

**`telemetry-receiver/`** — optional collector for the anonymous usage telemetry the API reports daily (`{version, servers, templates}`; opt-in via the admin toggle). Validates, logs, and aggregates into Prometheus metrics; deployed via `api.telemetry.receiver.enabled` (API auto-pointed at it) or run standalone for a public endpoint. See `telemetry-receiver/README.md`.

**`web/`** — React 18 + TS strict + Vite. Entry: `web/src/main.tsx`. Routing in `web/src/router/tree.tsx` (TanStack Router). Data fetching is TanStack Query calling through the thin fetch wrapper in `web/src/lib/api.ts`; WebSocket helpers in `web/src/lib/ws.ts`. Pages in `web/src/routes/`. Shared types mirroring CRDs in `web/src/types.ts`.

**`modules/`** — a **git submodule** (the standalone `gameplane-module` repo) holding the official game template bundles distributed as OCI artifacts. Each has `module.yaml`, `template.yaml`, `README.md`, optional `icon.png`. Built and pushed via `modules/build.sh` (uses `oras ≥ 1.2.0`). Format spec: `docs/module-authoring.md`. Run `git submodule update --init` after clone to populate it.

**Database tables** (managed by `api/internal/db/migrations/`): `users`, `sessions`, `oidc_links`, `audit_events`, `api_tokens`, `config`. Migrations are append-only and applied at startup.

---

## Stack reference

| Layer | What's used |
|---|---|
| Go runtime | 1.25 (netguard, operator, api, agent, audit-syslog-bridge, telemetry-receiver share `go.work`) |
| K8s libs | `controller-runtime` v0.19.0, `client-go` v0.35.0, envtest 1.31 |
| HTTP / WS | `chi` v5, `coder/websocket` v1.8.12 |
| Persistence | `modernc.org/sqlite` **or** `pgx/v5` (driver-selectable at runtime) |
| Auth | `argon2id` (local), `coreos/go-oidc/v3` (OIDC) |
| OCI | `oras-go/v2` for the operator pull side; `oras` CLI ≥ 1.2.0 for build/push |
| Supply chain | `sigstore/cosign` — keyed/offline (no Rekor) signing of published images and official module bundles; verify with the repo-root `cosign.pub` |
| Cron | `robfig/cron/v3` (BackupSchedule) |
| Frontend core | React 18.3, TypeScript 5.6 strict, Vite 5.4 |
| Frontend libs | TanStack Router, TanStack Query, Radix + shadcn/ui, Tailwind 3.4, lucide-react, Monaco editor, xterm.js |
| Frontend tests | Vitest 2.1, `@testing-library/react`, `msw` |
| Kubernetes target | 1.28+; Helm 3.13+ |
| CRDs | `gameplane.local/v1alpha1` — GameTemplate, GameServer, Backup, BackupSchedule, Restore, Module, ModuleSource |
| License | AGPL-3.0-or-later |

---

## Common workflows

A short cookbook for recurring tasks. Each entry lists the exact files to touch.

### Add a field to a CRD

1. Edit the type in `operator/api/v1alpha1/<kind>_types.go` — files are `gameserver_types.go`, `gametemplate_types.go`, `backup_types.go`, `backupschedule_types.go`, `restore_types.go`, `module_types.go`, `modulesource_types.go`.
2. `make generate && make manifests`.
3. Update the reconciler in `operator/internal/controller/<kind>_controller.go` to honor the new field.
4. If the field is exposed in the dashboard, mirror it in `web/src/types.ts` and update the relevant `web/src/routes/*.tsx`.
5. Add an envtest case in `operator/internal/controller/<kind>_envtest_test.go`.

### Add a new API route

1. New handler under `api/internal/handlers/`.
2. Mount it in `api/cmd/main.go` with the right RBAC middleware from `api/internal/rbac/`.
3. Add an integration test (`api/internal/handlers/<name>_envtest_test.go`) and a unit test for the handler logic.
4. Add the matching client call in `web/src/lib/api.ts` (the web client lives in `web/src/lib/`, not `web/src/api/`).

### Add a new dashboard page

1. **First**: update `design.pen` via the `pencil` MCP server. Don't open the editor on it; use `open_document`.
2. Add the route file in `web/src/routes/<name>.tsx` and register it in `web/src/router/tree.tsx`.
3. Call the API via the existing client (`web/src/lib/api.ts`) wrapped in TanStack Query at the call site; use the WebSocket helper in `web/src/lib/ws.ts` for streams. Add new helpers in `web/src/lib/` only if needed.
4. Co-locate a `<name>.test.tsx` next to the route file.

### Add a new game module

Modules live in the **`gameplane-module`** repo, checked out here as the `modules/` submodule — so module changes are committed in that repo, then the submodule pointer is bumped in this one.

1. New directory under `modules/<name>/` (i.e. in the `gameplane-module` repo) with `module.yaml`, `template.yaml`, `README.md`, optional `icon.png`. Format spec: `docs/module-authoring.md`.
2. `make modules-push` to push to the local registry.
3. The operator indexes ModuleSources within seconds — verify by checking the Modules page in the dashboard.
4. Commit in `gameplane-module`, then `git add modules` here and commit the bumped submodule pointer.

### Update the public website

The site lives in the **`gameplane-website`** repo, checked out here as the `website/` submodule — website changes are committed there, then the submodule pointer is bumped in this repo.

1. **Design first**: the site's screens are the **Group/Public Website** frames in `design.pen` (same Pencil MCP workflow as rule 1). Backend-only or copy-only tweaks don't need a Pencil pass.
2. Commit in `website/` (same conventions: signed, conventional prefixes; its own `AGENTS.md` has the specifics — semantic tokens only, every internal link through `withBase()`).
3. Push to the website repo's `main` — its `deploy.yaml` publishes GitHub Pages automatically; `ci.yaml` gates PRs with lint + `astro check` + build.
4. `git add website` here and commit the bumped submodule pointer.
5. **On each release**: update the website's `src/content/docs/changelog.mdx` and the `VERSION` constant in `src/config.ts` (the changelog page is a snapshot of `CHANGELOG.md`, synced manually).

### Add a database migration

1. New file under `api/internal/db/migrations/` — number it sequentially (e.g., `003_<name>.sql`). Migrations are append-only.
2. Apply on startup; no manual command needed for local dev. For production rollouts, the operator restarts the API pod and migrations run automatically.

---

## Where to read deeper

- **`README.md`** — project pitch and quickstart.
- **`docs/architecture.md`** — components, data flow, security boundaries, and the "operator is authoritative" rationale.
- **`docs/contributing.md`** — full code style, test tiers, PR process, signed commits.
- **`docs/security.md`** — auth, RBAC, threat model, pod security defaults, and the pre-auth privacy rule.
- **`docs/install.md`** — Helm values, K8s/Helm prerequisites, OIDC setup.
- **`docs/module-authoring.md`** — OCI bundle format for game templates.
- **`audit-syslog-bridge/README.md`** — the syslog relay's config vars, transport tradeoffs (why TCP over UDP), and standalone `docker run` usage.
- **`Makefile`** — canonical source of every build/test/dev command (this file paraphrases it; if they disagree, `Makefile` wins).
- **`.golangci.yml`** and **`web/eslint.config.js`** — the linter rule sets that "fix, don't silence" applies to.
- **`.editorconfig`** — indentation: tabs in Go, 2 spaces elsewhere; LF line endings.
