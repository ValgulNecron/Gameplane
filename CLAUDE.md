# Kestrel — guidance for AI coding assistants

This file is for AI coding assistants (Claude Code and similar). It exists so a fresh agent session can plan a change without re-deriving the project's structure, commands, and house rules from scratch. Humans should read [`README.md`](README.md) and [`docs/contributing.md`](docs/contributing.md) instead — those are written for people; this is written for agents.

**Project**: Kestrel — a Kubernetes-native game server control panel. Open-source alternative to CubeCoders AMP, built on K8s primitives so the same operational model works on a single-node k3s homelab and a multi-node production cluster.

> **Status:** beta (`v0.2.0-beta.1`). CRDs, operator, API, agent, and dashboard are feature-complete for the v1 scope and stabilized for external testing; not yet recommended for unattended production. See README "Beta status & known limitations".

> **AI tooling provenance:** the project was started with Claude Code on Claude Opus 4.8 (`claude-opus-4-8`); since June 2026 development continues on Claude Fable 5 (`claude-fable-5`). This is informational only — nothing in this file is model-specific.

---

## Repo map

```
.
├── operator/                 # controller-runtime operator (Go)
│   ├── api/v1alpha1/         # CRD Go types — edit here, then `make generate manifests`
│   │   └── zz_generated.deepcopy.go    # GENERATED — do not hand-edit
│   ├── internal/controller/  # reconcilers + co-located *_envtest_test.go
│   ├── cmd/main.go           # operator entry point
│   └── config/{crd,rbac}/    # GENERATED CRD/RBAC YAML — do not hand-edit
├── api/                      # REST + WebSocket gateway (Go, chi)
│   ├── cmd/main.go           # `serve` and `bootstrap-admin` subcommands
│   └── internal/{handlers,auth,db,kube,rbac,ws}/
├── agent/                    # in-pod sidecar (Go)
│   ├── cmd/main.go
│   └── internal/{auth,console,files,heartbeat,logs,players,rcon,quiesce}/
├── web/                      # React 18 + TS strict + Vite dashboard
│   └── src/{routes,components,lib,router,styles,test}/
├── modules/                  # game template bundles (OCI artifacts)
│   ├── minecraft-java/  valheim/  terraria/
│   └── build.sh              # OCI bundle builder/pusher (uses oras ≥ 1.2.0)
├── charts/kestrel/           # Helm chart
├── deploy/kind/              # local dev cluster scripts
├── test/e2e/                 # kind-based E2E suite (build tag: e2e)
├── docs/                     # human-facing docs (architecture, contributing, security, …)
├── design.pen                # Pencil design source — encrypted, MCP only
├── go.work                   # Go workspace linking operator/api/agent/test/e2e
└── Makefile                  # canonical entry point for every command
```

The Go modules `operator`, `api`, `agent`, and `test/e2e` share one workspace via `go.work`. The `web/` tree is its own npm package.

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

`make dev-up` brings up a kind cluster from `deploy/kind/cluster.yaml` plus a local OCI registry on `localhost:5001` (cluster-internal name `kind-registry:5000`), loads the locally-built operator/api/agent images, pushes every `modules/*` directory as an OCI bundle, and installs the chart from `charts/kestrel/`.

### Build

```sh
make build                       # all components (Go + web)
make build-go                    # binaries: operator, api, agent
make build-web                   # web/dist via `npm ci && npm run build`
make images                      # docker images for all three Go services
make image-operator              # one image; same for image-api, image-agent
```

### Test (three tiers)

```sh
make test                # everything (≈ seconds)
make test-go             # Go unit tests across operator/api/agent
make test-web            # vitest for web

make test-integration    # envtest tier (operator + api) — downloads K8s 1.31 envtest assets
make test-e2e            # kind + helm + real components (≈ 10–20 min)
make test-e2e-keep       # re-run e2e against an already-up cluster
```

Per-component fallbacks when you want to focus:

```sh
cd operator && go test ./...
cd api      && go test ./...
cd agent    && go test ./...
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

Coverage gates: `operator/.testcoverage.yml` (71%), `api/.testcoverage.yml` (80%), `agent/.testcoverage.yml` (91%), `web/vitest.config.ts` (lines 84% / functions 69% / branches 80%). Don't lower thresholds without a reason; ratchet them up when adding tests.

### Codegen — mandatory after CRD type edits

```sh
make generate    # regenerates operator/api/v1alpha1/zz_generated.deepcopy.go
make manifests   # regenerates operator/config/crd/*.yaml + operator/config/rbac/*.yaml and syncs charts/kestrel/crds/
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

These are the rules an agent cannot infer from reading the code. They are deliberately Kestrel-specific — generic Claude Code defaults (terse responses, no half-finished implementations, comments only when *why* is non-obvious) are already in your system prompt and don't need restating here.

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
- **Don't:** render `cluster: prod-east-1`, `5 servers online`, `Kestrel v0.4.2-rc3`, or "user `alice` not found" on `/login`.
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
- `charts/kestrel/crds/*.yaml` (synced automatically by `make manifests`)

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

---

## Architecture quick reference

The detail lives in `docs/architecture.md`; this is the index.

**`operator/`** — controller-runtime. Reconciles 7 CRDs (`gameplane.gg/v1alpha1`) into K8s objects: GameTemplate, GameServer, Backup, BackupSchedule, Restore, Module, ModuleSource. Entry: `operator/cmd/main.go`. Controllers in `operator/internal/controller/`. Inject points (agent image, CA bundle, mTLS certs) wired from CLI flags in `main.go`.

**`api/`** — chi router; REST + WebSocket. Entry: `api/cmd/main.go`, with subcommands `serve` and `bootstrap-admin`. Layout:

- `api/internal/handlers/` — REST handlers (lifecycle, users, modules, destinations, config, audit, events, resources)
- `api/internal/auth/` — local argon2id + OIDC (`coreos/go-oidc/v3`); sessions; rate limiting
- `api/internal/db/` — driver-selectable (modernc.org/sqlite **or** pgx/v5); migrations in `api/internal/db/migrations/`
- `api/internal/kube/` — Kubernetes client wrapper
- `api/internal/rbac/` — middleware enforcing the three roles (admin, operator, viewer)
- `api/internal/ws/` — WebSocket bridge for console/log streaming

**`agent/`** — sidecar that runs in every game pod. Entry: `agent/cmd/main.go`. Endpoints: console (PTY/RCON), files, logs, players, heartbeat, quiesce. Speaks token-auth + mTLS back to the operator/API.

**`web/`** — React 18 + TS strict + Vite. Entry: `web/src/main.tsx`. Routing in `web/src/router/tree.tsx` (TanStack Router). Data fetching is TanStack Query calling through the thin fetch wrapper in `web/src/lib/api.ts`; WebSocket helpers in `web/src/lib/ws.ts`. Pages in `web/src/routes/`. Shared types mirroring CRDs in `web/src/types.ts`.

**`modules/`** — game template bundles distributed as OCI artifacts. Each has `module.yaml`, `template.yaml`, `README.md`, optional `icon.png`. Built and pushed via `modules/build.sh` (uses `oras ≥ 1.2.0`). Format spec: `docs/module-authoring.md`.

**Database tables** (managed by `api/internal/db/migrations/`): `users`, `sessions`, `oidc_links`, `audit_events`, `api_tokens`, `config`. Migrations are append-only and applied at startup.

---

## Stack reference

| Layer | What's used |
|---|---|
| Go runtime | 1.25 (operator, api, agent share `go.work`) |
| K8s libs | `controller-runtime` v0.19.0, `client-go` v0.35.0, envtest 1.31 |
| HTTP / WS | `chi` v5, `coder/websocket` v1.8.12 *(README says nhooyr/websocket — README is stale; `go.mod` is canonical)* |
| Persistence | `modernc.org/sqlite` **or** `pgx/v5` (driver-selectable at runtime) |
| Auth | `argon2id` (local), `coreos/go-oidc/v3` (OIDC) |
| OCI | `oras-go/v2` for the operator pull side; `oras` CLI ≥ 1.2.0 for build/push |
| Cron | `robfig/cron/v3` (BackupSchedule) |
| Frontend core | React 18.3, TypeScript 5.6 strict, Vite 5.4 |
| Frontend libs | TanStack Router, TanStack Query, Radix + shadcn/ui, Tailwind 3.4, lucide-react, Monaco editor, xterm.js |
| Frontend tests | Vitest 2.1, `@testing-library/react`, `msw` |
| Kubernetes target | 1.28+; Helm 3.13+ |
| CRDs | `gameplane.gg/v1alpha1` — GameTemplate, GameServer, Backup, BackupSchedule, Restore, Module, ModuleSource |
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

1. New directory under `modules/<name>/` with `module.yaml`, `template.yaml`, `README.md`, optional `icon.png`. Format spec: `docs/module-authoring.md`.
2. `make modules-push` to push to the local registry.
3. The operator indexes ModuleSources within seconds — verify by checking the Modules page in the dashboard.

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
- **`Makefile`** — canonical source of every build/test/dev command (this file paraphrases it; if they disagree, `Makefile` wins).
- **`.golangci.yml`** and **`web/eslint.config.js`** — the linter rule sets that "fix, don't silence" applies to.
- **`.editorconfig`** — indentation: tabs in Go, 2 spaces elsewhere; LF line endings.
