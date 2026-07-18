# Dependency report

This is a component-by-component inventory of Gameplane's third-party
dependencies and why each one is there. It covers the 9 Go modules that
share `go.work` (`netguard`, `gameaction`, `operator`, `api`, `agent`,
`audit-syslog-bridge`, `telemetry-receiver`, `mcp-server`, `test/e2e`), the
`web/` npm package, and the build/CI toolchain that isn't a code dependency
but is invoked by `make`/GitHub Actions.

Only **direct** dependencies (the `require` block entries without
`// indirect` in each `go.mod`, and `dependencies`/`devDependencies` in
`web/package.json`) get individual justifications; indirect/transitive
dependencies are mentioned only where the tree itself is notable (e.g. the
`k8s.io/*` and `sigstore`/`cosign` trees pull in a large indirect closure).
Every "why" below was verified against real import/call sites — where a
dependency's purpose couldn't be pinned to a call site, that's stated
plainly rather than guessed.

**How to regenerate/verify this report:** `cat <module>/go.mod` for the
direct `require` block, then `grep -rn "<import path>" <module>/ --include="*.go"`
to find call sites (or `go mod why -m <path>` from inside the module). For
`web/`, check `web/package.json` against `grep -rln "<package>" web/src`.
Toolchain versions come from `Makefile`, `.github/workflows/*.yaml`, and
`.devcontainer/post-create.sh`.

Accurate as of **2026-07-16**, repo state `v0.2.0-beta.7`.

## At a glance

| Component | Language | Direct deps | Why it exists |
|---|---|---|---|
| `netguard/` | Go | 0 (stdlib only) | Shared SSRF dial-guard used by the operator and agent |
| `gameaction/` | Go | 0 (stdlib only) | Shared console-injection guard + command-template renderer used by the agent and API |
| `operator/` | Go | 19 | controller-runtime reconciler for the 8 CRDs; module OCI pull/verify, git/OCI fetch, restic-backup scheduling |
| `api/` | Go | 17 | chi REST + WebSocket gateway: auth (local + OIDC), RBAC, K8s client, SQLite/Postgres persistence, notifications |
| `agent/` | Go | 6 | In-pod sidecar: RCON/WebRcon console, chi HTTP surface, heartbeat status patch, disk usage stats |
| `audit-syslog-bridge/` | Go | 0 (stdlib only) | Standalone HTTP-JSON → syslog relay for the audit webhook sink |
| `telemetry-receiver/` | Go | 1 | Standalone collector for the API's opt-in anonymous usage telemetry |
| `mcp-server/` | Go | 5 | Standalone, strictly read-only MCP server for AI-assisted cluster diagnostics |
| `test/e2e/` | Go | 4 | kind + Helm + real-component end-to-end suite |
| `web/` | TypeScript/React | 18 runtime + 26 dev | Dashboard: routing, data fetching, UI primitives, console/file editors, tests |

## Shared Go modules

### netguard

`netguard/go.mod` has **no `require` block at all** — it imports only the
Go standard library (`context`, `net`, `net/http`, `errors`, `strings`,
`syscall`, `time`). It is a dial-time SSRF guard: `IsAllowed` (permissive,
used by the operator for admin-configured ModuleSource fetches) and
`IsPublic` (strict, used for user-triggered downloads) both work by
inspecting the resolved IP in a `net.Dialer.Control` hook, so no client
library is needed — just `net` and `syscall` for the dial-time hook itself.

Imported by:
- `operator/internal/oci/` (module bundle OCI pulls) and `operator/internal/modsrc/` (git/http/OCI ModuleSource fetches) — permissive `IsAllowed`.
- `agent/internal/mods/mods.go` — strict `IsPublic`, for user-triggered mod-install downloads.
- `api/internal/notify/notify.go` and `deliver.go` — permissive `IsAllowed`, for admin-configured notification destinations (Discord/Slack/SMTP/webhook). This is a third importer beyond the two documented in the architecture doc; it makes sense under the same "admin-configured infrastructure endpoint" rationale as ModuleSources.

It is a local `replace` module (`replace github.com/ValgulNecron/gameplane/netguard => ../netguard` in every importer's `go.mod`), not a published module — each Dockerfile that builds an importer must `COPY netguard/` into the build context alongside the component's own source.

### gameaction

`gameaction/go.mod` also has **no `require` block** — stdlib only
(`fmt`, `strconv`, `strings`, `text/template`). It validates raw user
input against a module's declared action parameters (`Resolve`, rejecting
control characters and enforcing type/length/enum constraints — the
console-injection guard) and renders a `text/template`-based command
string from the sanitized params (`Compile`/`Render`).

Imported by:
- `agent/internal/actions/actions.go` — the RCON quick-action transport.
- `api/internal/ws/actions.go` — the stdin pod-attach quick-action transport.

Both importers call `Resolve` independently before rendering, since each
is its own trust boundary (per the package doc comment). Also a local
`replace` module; same Dockerfile `COPY` requirement as netguard.

## Services

### operator

controller-runtime operator reconciling GameServer, GameTemplate, Backup,
BackupSchedule, Restore, Module, and ModuleSource. Direct deps from
`operator/go.mod` (excluding the local `netguard` replace, covered above):

| Dependency | Version | Why | Status |
|---|---|---|---|
| `sigs.k8s.io/controller-runtime` | v0.19.0 | Core reconciler framework — every controller in `internal/controller/*` embeds `client.Client`; `cmd/main.go` builds the manager via `ctrl.NewManager` | direct-runtime |
| `k8s.io/api` | v0.35.0 | Core/apps/batch typed API objects used across `api/v1alpha1/*_types.go` and controllers to build Pod/PVC/Job/Service/RBAC objects | direct-runtime |
| `k8s.io/apimachinery` | v0.35.0 | `metav1.ObjectMeta`, `runtime.Scheme`, `types.NamespacedName`, etc. — pervasive across CRD types and controllers | direct-runtime |
| `k8s.io/client-go` | v0.35.0 | `internal/controller/cluster_controller.go`, `gameserver_stop_attach.go` (pod exec/attach via `kubernetes.Clientset`), `cmd/main.go` scheme/clientset setup | direct-runtime |
| `github.com/go-git/go-git/v5` | v5.19.1 | `internal/modsrc/git.go` — clones a ModuleSource's git repo/ref (HTTP or SSH) in-memory to discover module directories | direct-runtime |
| `github.com/go-git/go-billy/v5` | v5.9.0 | `internal/modsrc/git.go` — in-memory filesystem (`memfs`) backing the git clone above, avoiding disk writes for an admin-configured but externally-controlled fetch | direct-runtime |
| `golang.org/x/crypto` | v0.50.0 | `internal/modsrc/git.go` — SSH key/host-key handling for git-over-SSH ModuleSource cloning, alongside `go-git`'s SSH transport | direct-runtime |
| `github.com/google/go-containerregistry` | v0.20.7 | `internal/verify/verify.go` — `name.NewDigest` + `remote.WithAuth/WithContext` resolve and authenticate the image ref being cosign-verified | direct-runtime |
| `oras.land/oras-go/v2` | v2.6.0 | `internal/oci/client.go` — pulls Module OCI bundle artifacts from the registry (`registry/remote.Repository` + retry transport) | direct-runtime |
| `github.com/opencontainers/image-spec` | v1.1.1 | `internal/oci/client.go` — `ocispec.Manifest`/`ocispec.Descriptor` types for parsing OCI manifests when pulling module bundles | direct-runtime |
| `github.com/sigstore/cosign/v2` | v2.6.3 | `internal/verify/verify.go` — `cosign.VerifyImageSignatures`/`CheckOpts` verify Module OCI bundle signatures, keyed (against `cosign.pub`) or keyless (Fulcio/Rekor) | direct-runtime |
| `github.com/sigstore/sigstore` | v1.10.8 | `internal/verify/verify.go` — `pkg/fulcioroots` supplies Fulcio root certs for keyless cosign verification | direct-runtime |
| `golang.org/x/mod` | v0.35.0 | `semver.Compare`/`semver.IsValid` in `internal/controller/semver.go`, `module_controller.go`, `internal/modsrc/oci.go`, `internal/oci/client.go` — sorts/validates Module version tags | direct-runtime |
| `sigs.k8s.io/yaml` | v1.6.0 | `internal/modsrc/bundle.go` parses `module.yaml`; `internal/controller/module_controller.go` parses the bundled `template.yaml` into JSON-tagged Go structs | direct-runtime |
| `github.com/robfig/cron/v3` | v3.0.1 | `internal/controller/backupschedule_controller.go` — parses a BackupSchedule's cron expression and computes the next run time | direct-runtime |
| `github.com/kubernetes-csi/external-snapshotter/client/v8` | v8.0.0 | `internal/controller/backup_volumesnapshot.go`/`backup_controller.go` — the CSI `VolumeSnapshot` types backing the volume-snapshot Backup/Restore strategy | direct-runtime |
| `github.com/prometheus/client_golang` | v1.23.2 | `internal/controller/metrics.go` — custom GameServer-phase and Backup Prometheus collectors registered with controller-runtime's metrics registry | direct-runtime |
| `github.com/go-logr/logr` | v1.4.3 | No non-test import found; only used in `internal/controller/helpers_envtest_test.go` (`logr.Discard()`). Runtime logging goes through controller-runtime's zap wrapper, which satisfies the `logr.Logger` interface without a direct import | **test-only** |
| `github.com/opencontainers/go-digest` | v1.0.0 | Only imported by `internal/oci/testregistry_test.go` (a fake OCI registry double for tests) | **test-only** |

Also declared as a `tool` directive (Go 1.24+ syntax, not a runtime
dependency): `sigs.k8s.io/controller-tools/cmd/controller-gen` — invoked by
`make generate`/`make manifests` to regenerate `zz_generated.deepcopy.go`
and the CRD/RBAC YAML.

The indirect closure is unusually large (~150 entries) because
`sigstore/cosign` and `go-git` each pull in substantial trees of their own
(TUF, Rekor, PKCS11, credential helpers, etc.) — normal for those
libraries, not a sign of dependency sprawl in Gameplane's own code.

### api

chi-based REST + WebSocket gateway. Direct deps from `api/go.mod`
(excluding the local `netguard`/`gameaction` replaces, covered above):

| Dependency | Version | Why | Status |
|---|---|---|---|
| `github.com/go-chi/chi/v5` | v5.1.0 | `cmd/main.go` router setup and nearly every file under `internal/handlers/` and `internal/ws/` — the HTTP router/middleware stack for the whole REST surface | direct-runtime |
| `github.com/coder/websocket` | v1.8.12 | `internal/ws/attach.go`, `podlogs.go`, `dialer.go` — upgrades HTTP to WebSocket for exec/attach terminal streaming and pod-log tailing, and dials the outbound WS proxy path | direct-runtime |
| `github.com/coreos/go-oidc/v3` | v3.11.0 | `internal/auth/oidc.go` — `oidc.NewProvider` + `IDTokenVerifier` for OIDC discovery and ID-token verification in the login flow | direct-runtime |
| `golang.org/x/oauth2` | v0.30.0 | `internal/auth/oidc.go` — `oauth2.Config` drives the OIDC authorization-code exchange | direct-runtime |
| `golang.org/x/crypto` | v0.53.0 | `internal/auth/password.go` — `argon2.IDKey` hashes/verifies local-login passwords (argon2id), plus a constant-time dummy-verify path to avoid a user-enumeration timing leak | direct-runtime |
| `k8s.io/api`, `k8s.io/apimachinery`, `k8s.io/client-go` | v0.35.0 | `internal/kube/*.go` (client wrapper, kubeconfig loading, watches, exec streaming), `internal/handlers/*.go`, `internal/notify/sinks.go`, `internal/rbac/rbac.go` — reading/writing CRDs and core objects | direct-runtime |
| `sigs.k8s.io/controller-runtime` | v0.23.3 | `cmd/main.go` — only `ctrl.GetConfig()` is used, to load the kubeconfig/in-cluster `*rest.Config` at startup; not the manager/reconciler machinery (that's the operator's job — see rule "the operator is authoritative"). `envtest.Environment` in `internal/handlers/suite_envtest_test.go` is test-only | direct-runtime (minimal) |
| `sigs.k8s.io/yaml` | v1.6.0 | `internal/handlers/module_upload.go` — parses `module.yaml` metadata and `template.yaml` from uploaded module bundles | direct-runtime |
| `github.com/minio/minio-go/v7` | v7.2.1 | `internal/audit/s3.go` — S3-compatible client (`minio.New`, `PutObject`) backing the S3 audit-log sink | direct-runtime |
| `github.com/prometheus/client_golang` | v1.23.2 | `cmd/main.go` (`promhttp` handler at `/metrics`); `promauto` counters in `internal/notify/notify.go`, `internal/audit/s3.go`, `internal/audit/audit.go` | direct-runtime |
| `modernc.org/sqlite` | v1.34.1 | `internal/db/db.go` — blank-imported to register the `"sqlite"` `database/sql` driver, the default persistence backend | direct-runtime |
| `github.com/jackc/pgx/v5` | v5.5.4 | `internal/db/db_postgres.go` (behind `//go:build postgres`) — registers `pgx/v5/stdlib` as the `"pgx"` driver when built with `-tags postgres`; the alternative persistence backend | direct-runtime (opt-in build tag) |
| `golang.org/x/mod` | v0.37.0 | `internal/handlers/semver.go` — `semver.Compare` orders module versions for the version-switch/update-detection UI | direct-runtime |
| `golang.org/x/sync` | v0.21.0 | `internal/registry/registry.go` — `singleflight.Group` collapses concurrent DB + live-Secret registry-credential lookups into one in-flight call | direct-runtime |
| `github.com/go-jose/go-jose/v4` | v4.0.2 | No non-test import found; used only in `internal/auth/oidc_issuer_test.go`/`oidc_more_test.go` to build fake JWKS/signed JWTs for testing the OIDC verifier | **test-only** |
| `github.com/prometheus/client_model` | v0.6.2 | No non-test import found; used in `internal/audit/audit_webhook_test.go`/`s3_test.go` (`dto "prometheus/client_model/go"`) to assert counter values in tests | **test-only** |

**Confirmed policy**: `k8s.io/metrics` is not imported anywhere in `api/`
(`grep -rn "k8s.io/metrics" api/` returns nothing but the explanatory
comments). `internal/handlers/cluster.go` reads the metrics-server
aggregated API (`/apis/metrics.k8s.io/v1beta1/nodes`) via
`h.k.Typed.CoreV1().RESTClient().Get().AbsPath(...).DoRaw(ctx)` — a raw
REST call over the existing typed client-go clientset — specifically to
avoid adding a whole extra module for one read-only GET. See "Notable
policies" below.

### agent

In-pod sidecar handling console/RCON, files, logs, players, heartbeat,
and quiesce. Direct deps from `agent/go.mod` (excluding the local
`netguard`/`gameaction` replaces, covered above):

| Dependency | Version | Why | Status |
|---|---|---|---|
| `github.com/go-chi/chi/v5` | v5.1.0 | `cmd/main.go` — router for the agent's whole HTTP surface; every feature package (console, logs, files, players, status, actions, quiesce, lifecycle, mods) exposes `Mount(r chi.Router, ...)` onto it | direct-runtime |
| `github.com/coder/websocket` | v1.8.12 | Dual role: server-side WS upgrade for `internal/console/console.go` (RCON console stream) and `internal/logs/logs.go` (live log tail); client-side outbound dial in `internal/rcon/websocket.go` implementing the Rust dedicated server's WebRcon protocol | direct-runtime |
| `k8s.io/apimachinery` | v0.31.1 | `internal/heartbeat/heartbeat.go` only — `metav1.Now()`/`PatchOptions{}`, `types.MergePatchType` to build the JSON merge-patch that updates `status.agent` | direct-runtime |
| `k8s.io/client-go` | v0.31.1 | `internal/heartbeat/heartbeat.go` only — `rest.InClusterConfig()` then `dynamic.NewForConfig(...).Resource(gvr).Namespace(...).Patch(...)` to patch the owning GameServer's status every tick; only the `dynamic` + `rest` subpackages are used, no typed clientset | direct-runtime |
| `github.com/prometheus/client_golang` | v1.20.5 | `cmd/main.go` — serves `promhttp.Handler()` at `/metrics`; only the `promhttp` subpackage, no custom collectors | direct-runtime |
| `golang.org/x/sys` | v0.22.0 | `internal/usage/usage.go` — `unix.Statfs`/`unix.Statfs_t` to report the game data volume's disk usage over heartbeat. **Not** used for a PTY: `internal/console/console.go`'s doc comment states the agent's console is RCON-only by design ("no real PTY"); a `GameTemplate.spec.consoleMode: "pty"` game is bridged instead through `api/internal/ws/attach.go` against the Kubernetes pod-attach API, with no agent involvement at all | direct-runtime |

Note `k8s.io/apimachinery`/`k8s.io/client-go` are pinned at v0.31.1 here
versus v0.35.0 in `operator`/`api`/`mcp-server` — each Go module in the
workspace resolves its own client-go version independently (there's no
shared root `go.mod`); the agent's use is narrow enough (one dynamic-client
patch call) that this drift hasn't mattered in practice, but it's worth
knowing the versions aren't lockstepped across the workspace.

### audit-syslog-bridge

`audit-syslog-bridge/go.mod` has **no `require` block** — stdlib only
(`crypto/subtle`, `crypto/tls`, `log/slog`, `net`, `net/http`, etc.). It's
a small, schema-agnostic HTTP-JSON → syslog (RFC 5424) relay: it accepts
one JSON POST body, frames it as an RFC 5424/6587 message, and forwards it
over a lazily-dialed TCP/TCP+TLS/UDP connection (`main.go`'s `forwarder`
type). Sits behind the API's audit webhook sink
(`api.audit.webhook.syslogBridge.enabled`). No third-party library needed
because the entire job is HTTP-in, hand-formatted-string-out.

### telemetry-receiver

`telemetry-receiver/go.mod` has exactly one direct dependency:

| Dependency | Version | Why |
|---|---|---|
| `github.com/prometheus/client_golang` | v1.23.2 | `main.go` — `prometheus.NewRegistry()` + `CounterVec`/`Histogram` for `gameplane_telemetry_reports_total`/`_servers`/`_templates`, exposed via `promhttp.HandlerFor` at `/metrics` |

Deliberately kept to "stdlib + client_golang only" (per the package doc
comment) so it can be deployed standalone on a public host collecting
reports from many installs, independent of Gameplane's release cadence.
It validates the API's daily `{version, servers, templates}` telemetry
POST, logs it structurally, and aggregates it into the metrics above —
never storing raw per-install reports.

### mcp-server

Strictly read-only Model Context Protocol server for AI-assisted cluster
diagnostics. Direct deps from `mcp-server/go.mod`:

| Dependency | Version | Why |
|---|---|---|
| `github.com/modelcontextprotocol/go-sdk` | v0.8.0 | `main.go`/`tools.go` — `mcp.NewServer`, `mcp.AddTool`, `mcp.StdioTransport` implement the MCP (JSON-RPC 2.0) protocol itself; every registered tool (`list_gameplane_resources`, `get_gameplane_resource`, `list_pods`, `get_pod`, `list_events`, `get_pod_logs`, `propose_fix`) is built against this SDK's types |
| `k8s.io/client-go` | v0.35.0 | `internal/kube/client.go` — `kubernetes.NewForConfig`/`dynamic.NewForConfig` build the typed and dynamic clientsets, kept as unexported fields on `Client` so no exported method can reach a mutating verb |
| `k8s.io/api` | v0.35.0 | `internal/kube/client.go` — `corev1` types for the typed Pod/Event reads (`ListPods`, `GetPod`, `ListEvents`, `PodLogs`) |
| `k8s.io/apimachinery` | v0.35.0 | `internal/kube/client.go` — `unstructured.Unstructured(List)`, `runtime.Scheme`, `schema.GroupVersionResource` back the dynamic-client reads of the 7 Gameplane CRDs (redeclared GVK/GVR locally rather than importing the operator module's generated types, to stay standalone) |
| `sigs.k8s.io/controller-runtime` | v0.23.3 | `main.go` — only `ctrl.GetConfig()`, to load the kubeconfig (in-cluster, falling back to `KUBECONFIG`/`~/.kube/config`) that builds the `kube.Client` above |

The read-only guarantee is structural (only List/Get-shaped methods are
exported from `internal/kube`, so `tools.go`'s handlers have no way to
reach a mutating verb even by mistake) and RBAC-backed (a
`get`/`list`/`watch`-only ClusterRole) — the go-sdk itself has no bearing
on that guarantee; it's purely the protocol transport.

## Frontend

### web

React 18 + TypeScript strict + Vite dashboard (`web/package.json` version
`0.2.0-beta.7`, matching the repo version):

**Framework / core**
- `react` `^18.3.1`, `react-dom` `^18.3.1` — base UI framework, rendered via `createRoot` in `src/main.tsx`, used throughout every component.

**Routing / data fetching**
- `@tanstack/react-router` `^1.75.0` — file-based route tree; `createRouter`/`RouterProvider` in `src/main.tsx`, tree built in `src/router/tree.tsx`, `Link`/`useNavigate` used across `src/routes/*.tsx`.
- `@tanstack/react-query` `^5.59.0` — server-state cache; `QueryClient`/`QueryClientProvider` in `src/main.tsx`, `useQuery`/`useMutation` used pervasively across routes and `src/lib/` — the dominant data-fetching layer.
- `@tanstack/react-virtual` `^3.10.8` — `useVirtualizer` in `src/routes/tabs/Logs.tsx`, virtualizes the log-line list.
- `@tanstack/router-devtools` (dev) — **no import found anywhere in `src/`**. Installed but apparently unused; flag as a candidate for removal or an intentionally-deferred addition.

**UI primitives (Radix + shadcn-style wrappers)**
- `@radix-ui/react-dialog` `^1.1.2` — underlies `src/components/ui/confirm-dialog.tsx` and every modal/drawer (`UploadModuleDialog`, `InstallDialog`, `CloneServerDialog`, `TransferServerDialog`, `SourceDialog`, `RestoreDialog`, `BackupDetailDrawer`, and more).
- `@radix-ui/react-dropdown-menu` `^2.1.2` — wrapped in `src/components/ui/dropdown-menu.tsx`, used by `ClusterSelector.tsx` and others.
- `@radix-ui/react-slot` `^1.1.0` — `Slot` in `src/components/ui/button.tsx`, supports the Button's `asChild` polymorphism.
- `class-variance-authority` `^0.7.0` — `cva()` in `src/components/ui/button.tsx` defines button variant/size classes.
- `clsx` `^2.1.1` + `tailwind-merge` `^2.5.2` — combined into the `cn()` helper in `src/lib/utils.ts`, used app-wide for conditional/merged Tailwind classes.
- `lucide-react` `^0.445.0` — icon set, imported in roughly 35 files (`AppLayout`, `Dashboard`, `Login`, dialogs, etc.).
- `@radix-ui/react-label` `^2.1.0` — **no import found**; forms use a plain `<label>` in `field.tsx`/`password-input.tsx` instead.
- `@radix-ui/react-tabs` `^1.1.1` — **no import found**; `src/components/ui/tabs.tsx` is a hand-rolled tab bar over plain `<button>`s, not Radix Tabs.
- `@radix-ui/react-toast` `^1.2.2` — **no import found**; there is no toast/notification component in `src/` currently.

**Editors / terminals**
- `@monaco-editor/react` `^4.6.0` — the `Editor` component in `src/routes/tabs/Files.tsx` (in-browser file editor) and `Placement.tsx`; given its own `manualChunks` bundle in `vite.config.ts`.
- `@xterm/xterm` `^5.5.0` + `@xterm/addon-fit` `^0.10.0` — `Terminal`/`FitAddon` in `src/routes/tabs/useConsoleTerminal.ts`, driving the live server console (`Console.tsx`); also chunked separately in `vite.config.ts`.

**Styling / build**
- `tailwindcss` `^3.4.13`, `autoprefixer` `^10.4.20`, `postcss` `^8.4.47` (dev) — `postcss.config.js` wires Tailwind + autoprefixer; `tailwind.config.ts` defines the design tokens used throughout `src/styles` and Tailwind classes app-wide.
- `vite` `^5.4.8`, `@vitejs/plugin-react` `^4.3.1` (dev) — `vite.config.ts` drives the dev server (with the API/WS proxy to the in-cluster API used by `make web-dev`), production build, and is the engine behind `vitest.config.ts`.
- `typescript` `^5.6.2` (dev) — strict-mode compile per `tsconfig.json`; `npm run build` is `tsc -b && vite build`.

**Lint** (dev, all wired directly in `eslint.config.js`)
- `eslint` `^9.11.1`, `@eslint/js`, `@typescript-eslint/eslint-plugin`, `@typescript-eslint/parser`, `eslint-plugin-react`, `eslint-plugin-react-hooks`, `globals` — flat-config ESLint enforcing `strict`, `@typescript-eslint/no-explicit-any: error`, `@typescript-eslint/no-floating-promises: error`, run via `npm run lint`.

**Testing — unit tier (Vitest)**
- `vitest` `^2.1.1`, `@vitest/coverage-v8` (dev) — `vitest.config.ts` (jsdom environment, coverage thresholds), run via `npm test`/`test:cover`.
- `jsdom` `^25.0.1` (dev) — the test environment itself.
- `@testing-library/react`, `@testing-library/user-event`, `@testing-library/jest-dom` (dev) — `render`/`screen`, interaction simulation, and DOM matchers used across the co-located `*.test.tsx` files.
- `msw` `^2.14.4` — dual role: unit-test request mocking (`src/test/server.ts`/`handlers.ts`, used via `setupServer` in most `*.test.tsx` files) **and** browser-mode mocking for Playwright's mock tier (`src/test/browser-msw.ts` uses `msw/browser`'s `setupWorker`, loaded by `main.tsx` when `VITE_E2E_MOCK` is set).
- `vitest-websocket-mock` (dev) — `src/test/ws.ts`, mocks the WebSocket used by `src/lib/ws.ts`/console streaming in unit tests.

**Testing — e2e tier (Playwright)**
- `@playwright/test` `^1.59.1` (dev) — `playwright.config.ts` defines mock vs. live run modes (`GAMEPLANE_E2E_TARGET`), driven by `make test-web-e2e-mock`/`test-web-e2e-live`. Live mode hits a real kind cluster via a `kubectl port-forward` globalSetup; mock mode reuses the same `msw` handlers as the unit tier.

**Types**
- `@types/react`, `@types/react-dom`, `@types/node` (dev) — type definitions consumed implicitly by TypeScript, no direct import.

## Test-only

### test/e2e

kind + Helm + real-component end-to-end suite (build tag `e2e`), run
against a throwaway kind cluster per CI bucket or a reused
cluster/kubelab via `GAMEPLANE_E2E_REUSE_CLUSTER`. Direct deps from
`test/e2e/go.mod`:

| Dependency | Version | Why |
|---|---|---|
| `github.com/coder/websocket` | v1.8.14 | `api_ws_e2e_test.go` — a real WebSocket client dialing the API's WS endpoints (console/logs) end-to-end, exercising the same protocol the dashboard's `web/src/lib/ws.ts` uses |
| `k8s.io/client-go` | v0.31.1 | Used across most `*_e2e_test.go` files to build a real cluster client, apply/watch objects, and assert on cluster state after driving the API/dashboard |
| `k8s.io/apimachinery` | v0.31.1 | 28 files reference it — `metav1`, `types`, `runtime` types used throughout the assertions and helper builders (`env.go`, `test_helpers_e2e_test.go`) |
| `k8s.io/api` | v0.31.1 | Core/apps typed objects used to construct and inspect Pods/Deployments/etc. during the e2e flows |

Note the `k8s.io/*` trio here is pinned older (v0.31.1) than every other
module in the workspace (v0.35.0) — this module drives a real cluster
rather than parsing CRD schemas, so the version skew has been low-risk in
practice, but it's another instance of the workspace's per-module,
independently-resolved `k8s.io/*` versions (see the agent section above).

## Toolchain / build-time

These aren't code dependencies of any component; they're invoked by
`Makefile` targets and GitHub Actions workflows.

- **`sigs.k8s.io/controller-tools/cmd/controller-gen`** (`tool` directive in `operator/go.mod`, pulling in `sigs.k8s.io/controller-tools v0.20.1` indirect) — `make generate`/`make manifests` regenerate `zz_generated.deepcopy.go` and the CRD/RBAC YAML.
- **`github.com/vladopajic/go-test-coverage/v2@v2.11.0`** — pinned in `Makefile`'s `GO_TEST_COVERAGE_PKG`, run via `go run` (no install step) by `make cover-go-check` and CI's `go` job to enforce each module's `.testcoverage.yml` threshold.
- **`github.com/wadey/gocovmerge@latest`** — pinned in `Makefile`'s `GOCOVMERGE_PKG`, merges each module's `unit.out` + `envtest.out` coverage profiles before the threshold gate. **Unpinned** (`@latest`) unlike every other tool version in this list — worth pinning for build reproducibility.
- **`sigs.k8s.io/controller-runtime/tools/setup-envtest@release-0.19`** — installed by `make envtest-bin`; downloads the K8s 1.31.0 envtest binaries (`kube-apiserver`, `etcd`) that back `operator`'s and `api`'s `-tags=envtest` integration tests.
- **`golangci-lint` v1.64.8** — pinned in `.devcontainer/post-create.sh` (built from source, not the release binary, because the release binary is built with an older Go than the repo's `go 1.25.0` toolchain wants). Configured by `.golangci.yml` (bodyclose, errcheck, gosec, govet, ineffassign, staticcheck, unused, misspell, gofmt, goimports, revive, unparam, nilerr, noctx, errorlint, contextcheck). **Not currently invoked by CI** — `ci.yaml`'s `go` job runs only `go vet`, not `golangci-lint run`; `make lint-go` exists for local/devcontainer use and is called out in `docs/contributing.md` as a pre-PR step, but nothing in `.github/workflows/` gates on it. Same for the web job: it runs `npm run build` and `npm run test:cover`, not `npm run lint` — ESLint isn't CI-gated either.
- **`oras` CLI ≥ 1.2.0** (pinned `1.2.0` in the devcontainer; `oras-project/setup-oras@v1` action in `release.yaml`) — `modules/build.sh` pushes each `modules/<name>/` directory as an OCI module bundle (`make modules-push`); this is a *different* thing from `operator`'s `oras.land/oras-go/v2` **library** dependency, which is the operator's own pull-side client, not the CLI.
- **`cosign` CLI v2.4.3** (`sigstore/cosign-installer@v3`, pinned `cosign-release: 'v2.4.3'`) — used in `publish-edge.yaml`/`release.yaml` to keyed-sign (offline, `--tlog-upload=false`, no Rekor) published container images and module bundles against `COSIGN_PRIVATE_KEY`, and to round-trip-verify against the derived public key before publishing. Distinct from `operator`'s `sigstore/cosign/v2` **library** dependency, which only ever *verifies* (never signs) at reconcile time.
- **`kind`** v0.24.0 (devcontainer pin) / `helm/kind-action@v1` (`install_only: true` in CI) — spins up the ephemeral clusters for `make dev-up`, `make test-e2e`, and every `e2e-*` CI job.
- **`helm`** (`azure/setup-helm@v4` in CI; `kubectl-helm-minikube` devcontainer feature, `helm: latest`) — chart lint (`helm lint`), chart render sanity-check, `helm template`, and `helm upgrade --install` in `deploy/kind/*.sh`/`Makefile`'s `dev-install`.
- **`docker buildx bake`** (`docker-bake.hcl`, `docker/bake-action@v6`) — builds the operator/api/agent e2e images concurrently with a shared GitHub Actions layer cache (`.github/actions/e2e-images`); also builds the `e2e-gameprobe` headless bot image used only by the game-bot CI job.
- **`docker/build-push-action@v6` + `docker/metadata-action@v5`** — the actual multi-arch (`linux/amd64,linux/arm64`) image builds and GHCR tagging in `publish-edge.yaml` (main → `:edge`) and `release.yaml` (`v*` tags → versioned images + `oci://ghcr.io/<owner>/charts` Helm push).

The Helm chart itself (`charts/gameplane/Chart.yaml`) declares **no chart
`dependencies:`** — it's a single chart with no subcharts.

## Notable policies

- **No `k8s.io/metrics` module.** `api/` deliberately avoids adding
  `k8s.io/metrics` as a dependency for the one place it needs
  metrics-server data (`internal/handlers/cluster.go`'s node CPU/mem
  panel): it issues a raw `GET /apis/metrics.k8s.io/v1beta1/nodes` through
  the existing typed client-go clientset's `RESTClient()` instead. Adding a
  new module for a single read-only GET was judged not worth the extra
  dependency surface — the same reasoning that keeps `netguard`,
  `gameaction`, `audit-syslog-bridge`, and `telemetry-receiver` this lean.
- **Persistence is driver-selectable, but Postgres needs a rebuild.**
  `api/internal/db` registers `modernc.org/sqlite` (pure-Go, default)
  unconditionally, and compiles in `github.com/jackc/pgx/v5`'s `stdlib`
  driver only under the `postgres` build tag (`db_postgres.go`); without
  that tag, `db_nopostgres.go` stands in and `Open(ctx, "postgres", dsn)`
  returns `"postgres support not compiled in — rebuild with -tags
  postgres"`. Confirmed: `api/Dockerfile` (and thus the published
  `ghcr.io/.../api` image) does **not** pass `-tags postgres` — this is
  intentional and documented, not an oversight: `charts/gameplane/values.yaml`'s
  `api.db.driver` comment explicitly tells an operator who wants Postgres
  to set `dsn` *and* rebuild the image with `-tags postgres` themselves.
  So `pgx/v5` is a real, live dependency, but it ships dormant in the
  default binary.
- **netguard's two policies, three importers.** The architecture doc
  documents `operator` (`IsAllowed`, permissive) and `agent`
  (`IsPublic`, strict) as netguard's two consumers. `api/internal/notify`
  is a third, using the permissive `IsAllowed` policy for admin-configured
  notification destinations (Discord/Slack/SMTP/webhook) — the same
  "admin-configured infrastructure, not user-supplied" rationale as
  ModuleSources.
- **Local `replace` modules need explicit Docker `COPY`s.** `netguard` and
  `gameaction` are both resolved via `replace ... => ../netguard` /
  `../gameaction` directives, not published modules. Every Dockerfile that
  builds an importer (`operator/Dockerfile`, `api/Dockerfile`,
  `agent/Dockerfile`) must `COPY` those directories into the build context;
  a Dockerfile edit that forgets this breaks the build with an unresolved
  module error, not a subtle bug — but it's a manual step to remember when
  adding a new local module.
- **Signing has two independent code paths that must be kept in sync.**
  Verification (`operator`'s `sigstore/cosign/v2` **library**, checked at
  Module-reconcile time against `cosign.pub`) and signing (the `cosign`
  **CLI**, `sigstore/cosign-installer@v3` in CI, gated on
  `COSIGN_PRIVATE_KEY` being set) are deliberately pinned to the same
  cosign v2 line (`v2.6.3` library / `v2.4.3` CLI) because cosign v3
  dropped the offline `--tlog-upload=false` flag this project's keyed,
  no-Rekor signing model depends on — a bump to either side needs to keep
  that constraint in mind.
- **Lint is configured but not CI-enforced.** `.golangci.yml` and
  `web/eslint.config.js` are both real, actively-maintained rule sets (see
  "Fix, don't silence" in `CLAUDE.md`), but neither `golangci-lint run` nor
  `npm run lint` currently appears in `.github/workflows/ci.yaml` — only
  `go vet` (Go) and `npm run build`/`test:cover` (web) are CI-gated. Lint
  is a devcontainer/local (`make lint`) and `docs/contributing.md`
  pre-PR-checklist step today, not an automated CI gate.
- **Two workspace-wide `k8s.io/*` version cohorts.** `operator`, `api`,
  and `mcp-server` are on `k8s.io/api|apimachinery|client-go v0.35.0` +
  `sigs.k8s.io/controller-runtime v0.23.3` (`operator` itself is on
  controller-runtime v0.19.0, an outlier even within that cohort).
  `agent` and `test/e2e` are still on `v0.31.1`. Because `go.work` links
  independent modules rather than a single root `go.mod`, nothing forces
  these to move together — each module upgrades on its own schedule.
