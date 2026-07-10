# Contributing

## Dev loop

```sh
make dev-up      # creates kind cluster, builds images, installs Helm chart
make web-dev     # starts the Vite dev server with proxy to in-cluster API
```

## Design first for UI changes

**Any change to the web dashboard's visual surface starts in
`design.pen` (Pencil), not in code.** Open `design.pen`, update the
relevant screen, review with the maintainers, *then* translate to
React. This keeps the 18 designed screens the source of truth and
prevents the dashboard from drifting into code-led redesigns.

Backend/operator/API changes do not need a Pencil pass.

## Code style

- **Go**: `gofmt`, `go vet`, `golangci-lint`. Errors are wrapped with `%w`.
- **TypeScript**: strict mode on. ESLint + Prettier. No `any` without a justification comment.
- **Comments**: default to writing none — let naming carry the weight. Add one only when the *why* is non-obvious (hidden invariant, workaround, or constraint a reader would ask about).

## Testing

Run the whole suite:

```sh
make test
```

Per-component:

```sh
cd netguard && go test ./...
cd operator && go test ./...
cd api      && go test ./...
cd agent    && go test ./...
cd audit-syslog-bridge && go test ./...
cd web      && npm test
```

## E2E testing

The e2e suite runs against a real kind cluster. CI splits it into parallel
buckets defined in `test/e2e/buckets.sh` — the `e2e bucket coverage` job fails on
any test that isn't in one, so add new tests to a bucket.

The two game-bot tests (`TestGameServer_MinecraftBotConnects`,
`TestGameServer_TerrariaBotConnects`) boot a real game server and run a
headless protocol bot as a **Job inside the cluster**, dialing the game
Service's DNS name — the same network path a real in-cluster client uses —
rather than tunnelling through `kubectl port-forward`.

The games namespace carries a `default-deny-egress` NetworkPolicy whose
`podSelector: {}` matches every pod in it. This allows DNS resolution but
blocks outbound connections, so an in-cluster helper pod placed there cannot
connect to the game Service. The probe Job therefore runs in the `default`
namespace, where no NetworkPolicy restricts it. Note that `kubectl
port-forward` bypasses NetworkPolicy entirely (it tunnels kubelet→pod), so
"it works via port-forward" is not evidence that in-cluster pod→game traffic
works.

The bot is `test/e2e/cmd/gameprobe`, built into `gameplane-test/gameprobe:e2e`
by the `e2e-gameprobe` bake target. That target sits outside the `e2e` bake
group so only the game-bot job pays to build it, and `deploy/kind/e2e.sh`
side-loads it into the cluster when present. The `e2e game bot (kind)` CI job
is now blocking rather than advisory.

## AI-assisted development

Much of this codebase is developed with AI coding assistants (Claude
Code). The project was started on Claude Opus 4.8 (`claude-opus-4-8`);
since June 2026 development continues on Claude Fable 5
(`claude-fable-5`). Agent-facing guidance lives in
[`CLAUDE.md`](../CLAUDE.md) — keep it current when project structure or
house rules change. All contributions, AI-assisted or not, go through
the same review, lint, and test gates below.

## Submitting a change

1. Fork + feature branch
2. Ensure `make lint && make test` pass
3. For UI work, include the Pencil node id(s) touched in the PR description
4. Sign commits (`git commit -s`)

Game-module changes (`modules/`) belong in the separate **`gameplane-module`**
repo, which this repo vendors as a submodule. Open the module PR there; once it
merges, bump the submodule pointer here (`git add modules`) in a follow-up PR.

## Release process

Tags matching `v*` trigger the `release.yaml` workflow, which builds the
component images, pushes the Helm chart to the `gameplane` OCI registry,
and — when a signing key is configured — keyed-cosign-signs the images by
digest and pushes and signs the official `modules/*` bundles.

Module signing is gated on a one-time key setup: run `cosign
generate-key-pair`, set `COSIGN_PRIVATE_KEY`/`COSIGN_PASSWORD` as CI
secrets, and publish `cosign.pub`. Until then the `modules` job no-ops and
the release still succeeds. See
[`module-authoring.md`](module-authoring.md#signing-official-bundles).
