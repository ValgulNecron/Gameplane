# Feature Specification: Hardened GitHub Actions CI/CD, AI Automation & Multi-Module Dependabot

**Feature Branch**: `008-hardened-github-actions`

**Created**: 2026-08-27

**Status**: Draft

**Input**: User description: "Make a specs for hardened github action for both e2e, static test, etc.. you will also update the github ai action and dependabot"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Hardened Static Quality & Multi-Module Lint Gates (Priority: P1)

Developers submitting pull requests across any of the repository's modules (Go backends, TypeScript web frontend, Helm charts, Dockerfiles, or CI configuration) receive fast, robust, and tamper-resistant feedback from automated static analysis. All third-party actions run with pinned immutable commit SHAs, jobs operate under the principle of least privilege with explicit read-only token permissions where possible, and strict timeouts prevent hung jobs from consuming runner minutes.

**Why this priority**: Static testing is the first line of defense in CI. Supply chain security in GitHub Actions is critical; compromised third-party action tags or over-privileged tokens can lead to credential theft, secret leakage, or malicious code injection into builds and releases.

**Independent Test**: Can be independently tested by submitting a pull request modifying code in various submodules (e.g., `agent/`, `operator/`, `web/`) and verifying that:
1. Every static analysis job (linting, vetting, build verification, chart rendering, license check) runs with explicit minimal permissions (`contents: read`).
2. All third-party GitHub actions are pinned to full 40-character commit SHAs.
3. Every job and step has an explicit timeout configured.
4. Any deliberate lint, type, or formatting error causes the respective gate to fail cleanly without breaking downstream reporting.

**Acceptance Scenarios**:

1. **Given** a pull request touching Go code in any of the 14 Go modules, **When** CI runs the static analysis jobs, **Then** `golangci-lint` and `go vet` run against the affected modules without suppressing errors, completing within explicit job-level timeout limits.
2. **Given** a pull request touching the frontend dashboard (`web/`), **When** CI runs the web static check, **Then** Node.js dependencies are verified, TypeScript compiles in `strict` mode, ESLint validates rules without in-source suppression, and unit test coverage meets the defined threshold.
3. **Given** a pull request modifying Helm templates or CRD schemas, **When** CI executes the chart validation step, **Then** `helm lint` and `helm template` validate manifest syntax, CRD parity between `crds/` and `crd-manifests/` is asserted, and the committed `cosign.pub` key is checked against the chart template.
4. **Given** any workflow file in `.github/workflows/` or composite action in `.github/actions/`, **When** the workflow is parsed and executed, **Then** every external action reference (`uses:`) is pinned to an immutable commit SHA with an explanatory version comment (e.g., `uses: actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683 # v4.2.2`).

---

### User Story 2 - Resilient & Hardened Kind E2E Test Pipeline (Priority: P1)

Core maintainers and contributors run automated end-to-end integration tests in throwaway Kubernetes (Kind) clusters across multiple architectures (AMD64 and ARM64). The E2E pipeline builds images once, passes them immutably as artifacts, isolates tests into login-budgeted buckets, handles ephemeral capture containers, and automatically captures sanitized diagnostic logs on failure without leaking tokens or credentials.

**Why this priority**: Gameplane enforces a non-negotiable E2E-tested delivery principle. The E2E suites exercise real control-plane reconciliation, container capabilities (e.g., `CAP_NET_RAW`), game wire protocols, and web interactions. Hardening this pipeline ensures stability, prevents test flakiness, isolates network workloads, and guarantees that failure artifacts do not leak secrets.

**Independent Test**: Can be tested independently by running the E2E matrix across all test buckets (operator, API auth/roles/RBAC/agent/mods, multicluster, upgrade, live web, and game bot) on AMD64 and ARM64 runners, and confirming that:
1. Single-writer image build artifacts are loaded without rebuilds.
2. Cluster diagnostic dumping captures full pod, container, and ephemeral logs on failure while sanitizing sensitive environment variables or authorization tokens.
3. Superseded workflow runs are cancelled immediately to prevent runner queue congestion.
4. E2E buckets maintain 100% disjoint and exhaustive coverage verification.

**Acceptance Scenarios**:

1. **Given** a code change affecting backend services or E2E tests, **When** the CI workflow triggers, **Then** single-stage e2e container image archives are built on isolated runners and distributed to matrix jobs without recompilation.
2. **Given** a failing test inside a Kind E2E job, **When** the job fails, **Then** the diagnostic dumper exports pod descriptions, controller logs, and game server container logs to GitHub step summaries and artifacts, ensuring all secrets (passwords, tokens, keys) are redacted.
3. **Given** a rapid succession of commits pushed to an open pull request branch, **When** a new commit is detected, **Then** in-flight E2E runs for superseded commits are cancelled immediately.
4. **Given** game probe and bot tests requiring external base images, **When** executed in CI, **Then** they run in network-isolated environments with verified container tags and strict timeouts.

---

### User Story 3 - Comprehensive Multi-Module Dependabot Automation (Priority: P2)

Maintainers require automated, secure, and organized dependency update pull requests for all components in the monorepo. Dependabot is configured to monitor all 14 Go modules individually (addressing Go workspace multi-module boundaries), the web dashboard npm packages, Dockerfiles across core services and game images, and GitHub Actions workflows. Updates are grouped logically and scheduled to minimize notification noise.

**Why this priority**: Unpatched dependencies and stale actions expose the project to known CVEs and breaking ecosystem changes. The existing Dependabot configuration only monitors the root folder for Go and misses 13 critical Go modules. Expanding and grouping updates keeps all submodules secure and maintainable.

**Independent Test**: Can be independently tested by validating `.github/dependabot.yml` against GitHub's Dependabot schema, verifying that:
1. All 14 Go modules (`/agent`, `/api`, `/operator`, `/netguard`, etc.) have explicit `gomod` entries.
2. All Dockerfile directories across core services and game images have `docker` entries.
3. Update intervals, scheduled days, commit prefixes (e.g., `chore: `), and PR limits are configured.
4. Grouping rules bundle minor and patch updates together to prevent PR floods.

**Acceptance Scenarios**:

1. **Given** a new minor version release of a shared Go library, **When** Dependabot runs on its weekly schedule, **Then** it creates grouped pull requests per module or across submodules with conventional commit messages (e.g., `chore(deps): bump ...`).
2. **Given** a security patch for an npm dependency in `/web` or a base Docker image, **When** Dependabot detects the vulnerability, **Then** it opens a targeted update PR with high priority.
3. **Given** updates to GitHub Actions referenced in `.github/workflows/`, **When** Dependabot proposes an update, **Then** it provides commit SHA pinning with updated version tags.

---

### User Story 4 - Secure GitHub AI Workflow for PR Review & Spec Validation (Priority: P2)

Contributors and maintainers benefit from automated AI-assisted code review and specification compliance checks directly on pull requests. The AI action analyzes PR diffs against the Gameplane Constitution, per-module `specs.md`, and coding standards, posting constructive feedback as sticky comments or review annotations. The action runs with strict security isolation, protecting against prompt injection and preventing secret exposure on fork PRs.

**Why this priority**: Automated AI review catches specification drift, missing test coverage, design discrepancies, or forbidden suppression directives before human review, accelerating development while strictly adhering to security boundaries for untrusted code.

**Independent Test**: Can be tested independently by creating a pull request and triggering the AI review workflow, verifying that:
1. On fork PRs, the workflow executes with read-only permissions and cannot access repository secrets or write tokens.
2. The AI action reads the PR diff, compares changes against the Constitution and spec artifacts, and generates a structured summary.
3. The workflow maintains a single updated sticky comment per PR without spamming the discussion thread.
4. Malicious or adversarial prompts embedded in commit messages or file diffs are treated as untrusted input and ignored by the evaluation prompt.

**Acceptance Scenarios**:

1. **Given** a pull request containing changes to Go or TypeScript code, **When** the AI action executes, **Then** it evaluates whether the changes comply with Constitution principles (e.g., no `//nolint` directives, proper error wrapping, E2E test coverage presence) and posts an advisory report.
2. **Given** a pull request from an external fork repository, **When** the workflow is triggered, **Then** it runs in a secure, isolated context with read-only repository access and no access to write tokens or sensitive signing keys (`COSIGN_PRIVATE_KEY`).
3. **Given** multiple commits pushed to the same pull request, **When** the AI review re-runs, **Then** it updates its existing review comment in place with the latest commit SHA reference.

---

### Edge Cases

- **Fork Pull Requests with Read-Only GITHUB_TOKEN**: Fork PRs cannot write commit statuses, modify check runs, or post PR comments using default tokens. Workflows must gracefully degrade (e.g., outputting reports to `$GITHUB_STEP_SUMMARY` without failing the build).
- **Transient Network Failures During Tool / Image Downloads**: Sigstore TUF initialization, Kind binary downloads, Playwright browser downloads, or Go module downloads may experience transient network blips. All remote fetch operations must implement bounded retries and exponential backoff.
- **Third-Party Action Supply Chain Compromise**: Upstream action repositories can have tags reassigned to malicious commits. Hardening requires pinning to full 40-character SHAs across all workflow files.
- **Prompt Injection via PR Content**: An untrusted PR might include prompt-injection instructions inside file comments or commit messages designed to manipulate the AI reviewer. The AI action must sanitize inputs, treat diffs as untrusted data, and enforce system prompt boundaries.
- **Dependabot PR Floods on Monorepos**: With 14 Go modules and multiple Dockerfiles, Dependabot could open dozens of simultaneous PRs. Grouping rules and strict open PR limits (e.g., max 5–10) must be configured to prevent overwhelming CI runners.
- **Secret Leakage in Diagnostic Log Dumps**: When a Kind cluster fails, dumping all container logs and pod descriptions could accidentally print tokens, certificates, or environment secrets. The dump-cluster-state action must avoid dumping secret resources and sanitize logged environment variables.

---

## Requirements *(mandatory)*

### Functional Requirements

#### Workflow Permissions & Security Hardening
- **FR-001**: Every workflow in `.github/workflows/` MUST define top-level default permissions set to `contents: read` or `{}` (least privilege).
- **FR-002**: Any job requiring elevated permissions (e.g., `statuses: write`, `pull-requests: write`, `packages: write`) MUST explicitly declare those specific permissions at the job level.
- **FR-003**: All external GitHub Actions (`uses: <owner>/<repo>@...`) in workflows and composite actions MUST be pinned to immutable 40-character commit SHAs, accompanied by an inline comment denoting the release tag or version.
- **FR-004**: Every job MUST have an explicit `timeout-minutes` configuration (defaulting to <= 30 minutes unless specifically justified, e.g., heavy game bot suites).
- **FR-005**: All push and pull request workflows MUST implement concurrency groups (`concurrency: { group: ..., cancel-in-progress: true }`) to abort obsolete in-flight runs on new pushes.
- **FR-006**: Workflows MUST NOT execute untrusted code or evaluate expressions directly in bash run scripts where user-controlled variables (e.g., PR title, branch name, commit message) could lead to script injection. All user inputs must be passed via environment variables.

#### Multi-Module Static Testing & Quality Gates
- **FR-007**: The CI workflow MUST execute static checks (`go vet`, `golangci-lint`) across all 14 Go modules in the repository (`agent`, `api`, `audit-syslog-bridge`, `capture-sidecar`, `gameaction`, `gameproto`, `mcp-server`, `netguard`, `operator`, `sentinel`, `svcutil`, `telemetry-receiver`, `test/e2e`, `tunnel`).
- **FR-008**: The CI workflow MUST execute frontend static checks for `web/`, including strict TypeScript type checking (`tsc --noEmit`), ESLint without in-source suppression directives, and Vitest coverage reporting.
- **FR-009**: Helm chart validation MUST run `helm lint` and `helm template`, verify parity between `charts/gameplane/crds` and `charts/gameplane/crd-manifests`, and assert that the embedded cosign verification key matches the repository's root `cosign.pub`.
- **FR-010**: The static test pipeline MUST assert that per-module code coverage gates defined in `.testcoverage.yml` and `web/vitest.config.ts` are strictly enforced.
- **FR-011**: The CI workflow MUST execute lint-gate configuration verification (`./test/e2e/lint-gate-verify.sh verify`) and game protocol coverage verification (`./test/e2e/joincoverage.sh verify`).

#### E2E Pipeline Hardening & Diagnostics
- **FR-012**: E2E test container images MUST be built once via single-stage builder jobs (`build-images` on AMD64 and `build-images-arm64` on ARM64) and shared across all downstream E2E matrix jobs via compressed artifacts.
- **FR-013**: E2E jobs MUST execute against clean, throwaway Kind clusters with dedicated network bridges, verifying both AMD64 and ARM64 runner platforms.
- **FR-014**: The cluster diagnostic dumper (`dump-cluster-state`) MUST trigger automatically on job failure (`if: failure()`), exporting pod descriptions, controller logs, and container logs while strictly redacting sensitive tokens and passwords.
- **FR-015**: E2E buckets MUST be disjoint and exhaustive, verified by `./test/e2e/buckets.sh verify` on every run.
- **FR-016**: The CI summary reporter job (`report`) MUST run with `if: always()`, consolidating status across all matrix legs and publishing both a GitHub Actions step summary and an idempotent, sticky PR comment.

#### Dependabot Monorepo Configuration
- **FR-017**: `.github/dependabot.yml` MUST configure separate `package-ecosystem: "gomod"` entries for all 14 Go modules across the repository.
- **FR-018**: `.github/dependabot.yml` MUST configure `package-ecosystem: "npm"` for `/web`.
- **FR-019**: `.github/dependabot.yml` MUST configure `package-ecosystem: "docker"` for all directories containing Dockerfiles (`/`, `/agent`, `/api`, `/audit-syslog-bridge`, `/capture-sidecar`, `/mcp-server`, `/operator`, `/sentinel`, `/telemetry-receiver`, `/tunnel`, `/images/common/steamcmd`, `/images/games/nuclear-option`).
- **FR-020**: `.github/dependabot.yml` MUST configure `package-ecosystem: "github-actions"` for `/` to automate action SHA and tag updates.
- **FR-021**: Dependabot configuration MUST define update schedules (e.g., weekly on Mondays), commit message prefixes (`chore(deps): `), open pull request limits, and dependency groups to batch minor and patch version bumps together.

#### GitHub AI Review Action Hardening & Integration
- **FR-022**: An AI review workflow MUST be defined to evaluate incoming pull requests against repository standards, Constitution rules, and specification requirements.
- **FR-023**: The AI review action MUST execute in a secure security posture: using read-only permissions on fork pull requests, preventing execution of arbitrary untrusted workflows, and withholding production signing or deployment secrets.
- **FR-024**: The AI review action MUST maintain an idempotent sticky comment on the pull request, updating the evaluation summary when new commits are pushed.
- **FR-025**: The AI action MUST sanitize PR metadata and diffs prior to prompt evaluation, ensuring prompt injection payloads within user code diffs cannot hijack or alter the AI reviewer's instructions.

---

### Key Entities

- **Workflow Security Policy**: The declared permission matrix, SHA pinning standard, and timeout boundaries enforced across all `.github/workflows/*.yaml` files.
- **E2E Image Bundle**: An immutable tarball artifact (`e2e-images-<arch>`) containing prebuilt component images (`operator`, `api`, `agent`, `sentinel`, `capture-sidecar`, `gameprobe`) distributed to test runners.
- **Cluster Diagnostics Bundle**: The structured log and event capture generated by `dump-cluster-state` upon E2E job failure.
- **Dependabot Ecosystem Matrix**: The multi-module configuration set inside `.github/dependabot.yml` defining update schedules, groups, and directory mappings across Go, npm, Docker, and GitHub Actions.
- **AI Reviewer Agent / Action**: The automated workflow step that evaluates PR diffs against project rules and posts structured, non-blocking advisory feedback.

---

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of external GitHub Actions in `.github/workflows/` and `.github/actions/` are pinned to immutable 40-character commit SHAs.
- **SC-002**: 100% of workflow jobs explicitly define minimal required permissions and explicit execution timeouts.
- **SC-003**: Dependabot monitors all 14 Go submodules, frontend npm packages, all Dockerfile locations, and GitHub Actions without omitting any repository component.
- **SC-004**: CI static and E2E feedback is delivered reliably, with zero unbudgeted job hangs (no job running indefinitely beyond its configured timeout).
- **SC-005**: 100% of E2E failure runs produce comprehensive, sanitized diagnostic summaries without leaking credentials or secrets.
- **SC-006**: AI review workflows run safely on both internal branches and external fork PRs without permission errors or secret exposure.

---

## Assumptions

- Standard GitHub-hosted runners (`ubuntu-latest` and `ubuntu-24.04-arm`) are used for CI jobs, with public repository allowances for ARM runners.
- Go version 1.25/1.26 and Node.js 24 remain the target toolchains for backend and frontend workflows respectively.
- For AI actions, a secure API key or GitHub token is provided via repository secrets (`ANTHROPIC_API_KEY`, `GEMINI_API_KEY`, or standard GitHub App token), with fork PR execution handled safely (e.g. read-only analysis without leaking keys or using GitHub Actions OIDC / workflow_run where needed).
- Dependabot grouped updates reduce pull request volume while keeping dependencies current.
- Production signing secrets (`COSIGN_PRIVATE_KEY`) remain strictly restricted to master branch and release tag workflows (`publish-edge.yaml`, `release.yaml`, `images.yaml`, `republish-modules.yaml`) and are never exposed to test or review workflows.
