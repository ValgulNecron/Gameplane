# Review: hack/ (plus the Makefile targets CI calls and the CLAUDE.md commands section)

- **Date**: 2026-09-24
- **Reviewer tier**: opus (verification pending)
- **Checked against**: `specs/012-docs-refresh-and-outreach/contracts/docs-audit.md` ("Standard FR-010: Version Strings", "Standard FR-010: Internal Links"), each script's own header contract, the `Makefile` help strings, `CLAUDE.md:96-163` (Canonical Commands, including Linters & Coverage Thresholds), `docs/contributing.md`

## Scope reviewed

Read in full:
- `hack/check-doc-versions.sh`, `hack/check-links.sh`, `hack/check-specs.sh`, `hack/gen-module-schema.py`. These are all the files in `hack/`.
- `Makefile`
- `CLAUDE.md` Canonical Commands and Linters & Coverage Thresholds sections

Read to cross-check a claim:
- `go.work`, `charts/gameplane/Chart.yaml`, `web/vitest.config.ts`
- every `<module>/.testcoverage.yml` (`total:` line only)
- `specs/012-docs-refresh-and-outreach/contracts/docs-audit.md:68-200`
- `deploy/kind/up.sh` (grep for image loading only)
- `charts/gameplane/values.yaml:13-22,55-60`, `operator/internal/controller/gameserver_sentinel.go:228-230`

Not done: none of the lint scripts were run (CLAUDE.md rule 8). Their behaviour was read from the code. The only execution was two small read-only probes:
- the `github_slug`/`strip_heading_marker` functions copied out of `check-links.sh` into the scratchpad and run on four headings
- an in-memory regeneration of the module schema compared with the committed file, without writing it

## Method

- Compared each script with its header contract and with the spec 012 contract it cites.
- Checked the two doc checkers use the same 18-file list (diffed the arrays: identical).
- Checked `check-specs.sh` against the actual `go.work`, and every listed module plus `web/` for a non-empty `specs.md`.
- Compared CLAUDE.md command descriptions and the coverage table with the Makefile recipes and the `.testcoverage.yml` / `vitest.config.ts` thresholds.

## Observations (no finding)

- **`check-specs.sh`** parses the block-form `use (…)` in `go.work`, which has 15 entries including `gp-module` and `test/e2e`, then adds `web`. All 16 have a non-empty `specs.md` today.
- **`gen-module-schema.py`** output matches the committed `modules/.schema/gametemplate.schema.json` byte for byte (regenerated in memory, not written).
- **Shared file list.** `check-doc-versions.sh` and `check-links.sh` audit the same 18 files, in the same order.
- **Makefile targets CI calls** (`check-links`, `check-doc-versions`, `check-specs`, `cover-go-merge`, `cover-go-check`; `ci.yaml:398-406,547`). They are thin wrappers. `cover-go-check` skips a module that has no profile, so each CI `go` leg gates only the module it just tested (`Makefile:126-140`).
- **CLAUDE.md coverage values.** The 13 Go rows and the web row match `<module>/.testcoverage.yml` and `web/vitest.config.ts:37-42` exactly.
- **CLAUDE.md `make build-go`.** "Compile all 14 Go workspace modules" matches `GO_MODULES` (`Makefile:35`), which has 14 entries because it excludes `test/e2e`.
- **`check-links.sh` fences.** Fence detection only handles ```` ``` ```` with up to 3 leading spaces. None of the 18 audited files use `~~~` fences or fences indented 4+ spaces (checked with grep), so there is no effect today.

## Candidate findings

### C-hack-01: `check-links.sh` keeps non-ASCII punctuation in heading slugs, breaking its own slug rule

- **Location**: `hack/check-links.sh:166` (implementation), compared with the rule it documents at `hack/check-links.sh:48-52` and with `specs/012-docs-refresh-and-outreach/contracts/docs-audit.md:185`
- **Category**: correctness
- **Suggested severity**: S4
- **Observation / repro**:
  1. Rule 4 in the header: "Remove backticks and any character that is not a letter (including Unicode letters), digit, space, hyphen, or underscore". The contract says "special chars removed".
  2. The implementation strips only `[[:punct:]]` under `LC_ALL=C`, which is the 32 ASCII punctuation characters. Unicode punctuation and symbols stay in the slug.
  3. Running the extracted functions gives:
     - `### Config schema → wizard` (`docs/module-authoring.md:624`) → `config-schema-→-wizard`
     - `## API → Agent` (`docs/security.md:159`) → `api-→-agent`
     - `# Signing key rotation — Ed25519 → ECDSA P-256 (2026-07)` (`docs/key-rotation.md:1`) → `signing-key-rotation-—-ed25519-→-ecdsa-p-256-2026-07`
  4. Applying the documented rule instead gives `config-schema--wizard`, which is also GitHub's anchor.
  5. So a correct link `[x](module-authoring.md#config-schema--wizard)` would be reported "missing anchor", and a link to `#config-schema-→-wizard` would pass the checker but be dead on GitHub. `docs/install.md:602` and `docs/security.md:641` are affected the same way.
- **Expected**: The slug follows the documented rule 4 and drops non-letter Unicode characters such as `→` and `—`.
- **Actual**: They are kept. No audited link points at these five headings today (checked with grep), so the checker hasn't produced a wrong result in CI yet.

### C-hack-02: `check-links.sh` resolves `ftp://` and other non-HTTP URLs as local paths, though the contract excludes `ftp://`

- **Location**: `hack/check-links.sh:288-290`, compared with `specs/012-docs-refresh-and-outreach/contracts/docs-audit.md:173`
- **Category**: correctness
- **Suggested severity**: S4
- **Observation / repro**:
  1. The contract's exclusions are "External links (http://, https://, ftp://) — out of scope; hack/check-links.sh (OD-2) validates internal links and anchors only".
  2. The skip list is `http://* | https://* | mailto:*`.
  3. Put `[mirror](ftp://mirror.example.org/gameplane.tgz)` in `docs/install.md`. The target is joined to `docs/`, normalised to `docs/ftp:/mirror.example.org/gameplane.tgz`, and reported as `missing file`.
  4. The project's own chart reference style, `[chart](oci://ghcr.io/valgulnecron/charts/gameplane)`, fails the same way.
- **Expected**: Every URL with a scheme (`<scheme>://`), or at least `ftp://` as the contract lists, is skipped as external.
- **Actual**: Only http, https and mailto are skipped. There are no current occurrences, so this is a latent false failure.

### C-hack-03: `check-doc-versions.sh` builds `current_version_pattern` and never uses it

- **Location**: `hack/check-doc-versions.sh:51-53`
- **Category**: dead-code
- **Suggested severity**: S4
- **Observation / repro**:
  1. Line 53 assigns `current_version_pattern="(v)?${current_version//./\\.}"`, under the comment "We'll accept matches with or without leading 'v'".
  2. The variable is never read again (grep).
  3. The actual current-version test is a string comparison after stripping `v` (`check-doc-versions.sh:114-117`).
  4. The comment at line 51, "appVersion in chart is unquoted (0.2.0-beta.8)", is also wrong: `Chart.yaml:6` is `appVersion: "0.2.0-beta.8"`, which is quoted. The sed at line 44 handles both forms.
- **Expected**: No unused variables, and comments that describe the input.
- **Actual**: A dead assignment and a stale comment. This is separate from the tracked version-pattern finding (T011 (a), PR #420), but #420 edits the same lines and could remove both.

### C-hack-04: CLAUDE.md says `make dev-load` rebuilds images, but it doesn't, and it loads only 4 of the 12 images `make dev-up` builds

- **Location**: `CLAUDE.md:104`, compared with `Makefile:367-371` (`dev-load`), `Makefile:260-263` (`images`), `Makefile:353-365` (`dev-up`)
- **Category**: docs-drift / correctness
- **Suggested severity**: S3
- **Observation / repro**:
  1. `CLAUDE.md:104` says "`make dev-load` # Rebuild and reload local images into Kind".
  2. The recipe is four `kind load docker-image` lines, for operator, api, web and agent, with no build step. Its own help string is "Load local images into kind cluster (kind only)". Editing `api/` and running `make dev-load` reloads the old image.
  3. `make dev-up` (kind path) runs `make images TAG=dev`, which builds all 12 images, then `make dev-load`. So `sentinel`, `capture-sidecar`, `mcp-server`, `telemetry-receiver`, `audit-syslog-bridge` and the three `tunnel-*` images are built but never loaded. `deploy/kind/up.sh` has no `kind load` either (grep).
  4. `dev-install` sets `image.tag=dev` and `image.registry=ghcr.io/valgulnecron/gameplane` (`Makefile:386-387`), so the operator is told `--sentinel-image=ghcr.io/valgulnecron/gameplane/sentinel:dev` (`values.yaml:58`, `gameserver_sentinel.go:228`). No workflow publishes a `:dev` tag: publish-edge pushes `edge`/`sha-*`, and release pushes versions.
  5. On a `make dev-up` cluster, any feature that starts one of those images (wake sentinel, network capture, tunnels) should hit ErrImagePull or ImagePullBackOff. This was inferred from the recipes and not run.
- **Expected**: The description matches the target, and every image the dev chart references is loaded into kind.
- **Actual**: The target doesn't rebuild and loads 4 of the 12 images. The workaround is to `kind load` the rest by hand.

### C-hack-05: CLAUDE.md coverage table leaves out `gp-module`, whose 80% gate CI enforces

- **Location**: `CLAUDE.md:137-152`, compared with `gp-module/.testcoverage.yml` (`total: 80`), `Makefile:35` (`GO_MODULES` includes `gp-module`) and `.github/workflows/ci.yaml:467` (the `go` matrix includes `gp-module`)
- **Category**: docs-drift
- **Suggested severity**: S4
- **Observation / repro**:
  1. The table lists 13 Go modules plus web.
  2. `make cover-go-check`, run by every CI `go` leg, also gates `gp-module` at 80%.
  3. A contributor reading CLAUDE.md for the gate on a `gp-module` change finds none.
- **Expected**: A `gp-module | 80% | …` row.
- **Actual**: The row is missing. This is related to tracked finding T011 (e), where the CLAUDE.md repository map omits `gp-module`, but it is a different table and a different claim (a coverage gate, not a module count).

## Questions (not findings)

- **Fixed line number.** `check-doc-versions.sh:44` reads the appVersion by fixed line number (`sed -n '6p'`). The spec 012 contract names the location `Chart.yaml:6`, so it is stable today. But adding any line above `appVersion` (for example `kubeVersion:`) would make the checker compare every doc against the wrong string. Should it match on the `appVersion:` key instead? No input produces this today.
- **Directory links.** `check-links.sh:311` requires the target to be a regular file (`-f`), so a valid GitHub link to a directory (for example `[chart](../charts/gameplane/)`) would be reported "missing file". No audited file links to a directory today (checked with grep). Is that limitation intended?
- **`make lint` vs CI.** CLAUDE.md:135 describes `make lint`. `Makefile:246-252` lints only `GO_MODULES`, so it skips `test/e2e`, and it doesn't pass `--build-tags=envtest` for operator/api. CI does both (`ci.yaml:432-451`), so a clean `make lint` doesn't imply a clean CI lint. Agents are told not to run it locally, so the practical impact is low. Should the description say so?
- **Workflow items.** The path-filter gaps that stop these checkers from running when their inputs change are recorded as C-github-workflows-03.

## Held

held candidates: 0 (see OD-019)
