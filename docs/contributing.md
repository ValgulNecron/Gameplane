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
cd operator && go test ./...
cd api      && go test ./...
cd agent    && go test ./...
cd web      && npm test
```

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

## Release process

Tags matching `v*` trigger the `release.yaml` workflow, which builds
images, signs them with cosign (roadmap), and pushes the Helm chart to
the `kestrel` OCI registry.
