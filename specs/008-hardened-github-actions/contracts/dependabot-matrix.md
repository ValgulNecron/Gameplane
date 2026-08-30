# Contract: Dependabot Ecosystem Matrix

**Feature**: 008-hardened-github-actions | **Date**: 2026-08-29

Target contents of `.github/dependabot.yml`. **Note: R7 (Dependabot<->tree parity check)
is a known coverage gap — no longer automatically enforced.** See OPEN-DECISIONS.md D-F.

> ✓ **Ratified values.** The directory parity requirement with `go.work` and the
> Dockerfile set (FR-017/FR-019), the presence of `groups`, and a declared
> `open-pull-requests-limit` (FR-021), the `chore(deps)` prefix (FR-021) are ratified.
> The specific limit numbers (5/10/5/5), the group batching strategy (one group per
> entry, minor+patch only), and the schedule (weekly Monday 03:00 UTC, no stagger)
> are **ratified per OPEN-DECISIONS.md D-B, D-C, D-D.**

---

## Baseline vs target

| Ecosystem | Today | Target | Effective coverage today |
|---|---|---|---|
| `gomod` | 1 entry (`/`) | **14** entries | **zero** — `/` has no `go.mod`, it is a `go.work` root and Dependabot does not traverse workspaces |
| `npm` | 1 entry (`/web`) | 1 entry | correct |
| `docker` | 1 entry (`/`) | **12** entries | **zero** — there is no root `Dockerfile` |
| `github-actions` | 1 entry (`/`) | 1 entry | correct |
| **Total** | 4 | **28** | |

Both dead entries fail silently. Dependabot does not warn about a directory with no
manifest, which is why this went unnoticed: the config looks plausible and does nothing.

The `dependabot/go_modules/...` branches currently on `origin` come from the **security**
updates path, which scans the dependency graph independently of `updates:`. Version updates
for Go have never run in this repository.

---

## gomod — 14 entries

Directories are `go.work` verbatim. No `/` entry.

```
/agent   /api   /audit-syslog-bridge   /capture-sidecar   /gameaction
/gameproto   /mcp-server   /netguard   /operator   /sentinel
/svcutil   /telemetry-receiver   /test/e2e   /tunnel
```

Per-entry shape:

```yaml
- package-ecosystem: "gomod"
  directory: "/operator"
  schedule:
    interval: "weekly"
    day: "monday"
    time: "03:00"
  commit-message:
    prefix: "chore(deps)"
    include: "scope"
  open-pull-requests-limit: 5
  groups:
    operator-minor-patch:
      update-types: ["minor", "patch"]
```

Each gomod entry has exactly one group batching all minor and patch updates. No k8s
carve-out — `k8s.io/api`, `k8s.io/apimachinery`, `k8s.io/client-go`, and
`sigs.k8s.io/controller-runtime` are version-locked to each other and may not compile
independently, but the simpler config is preferred. Dependabot-proposed PRs that fail
this constraint can be closed by hand.

**Limit arithmetic**: 14 × 5 = 70 worst case. In practice the minor/patch group
collapses to one PR per module, so a normal Monday is ≤ 14 Go PRs, most weeks far fewer.

---

## npm — 1 entry

```yaml
- package-ecosystem: "npm"
  directory: "/web"
  schedule:
    interval: "weekly"
    day: "monday"
    time: "03:00"
  commit-message:
    prefix: "chore(deps)"
    include: "scope"
  open-pull-requests-limit: 10
  groups:
    npm-minor-patch:
      update-types: ["minor", "patch"]
```

One group batching all minor and patch updates.

---

## docker — 12 entries

Derived from `find . -name Dockerfile -not -path './website/*'`:

```
/agent   /api   /audit-syslog-bridge   /capture-sidecar   /images/common/steamcmd
/images/games/nuclear-option   /mcp-server   /operator   /sentinel
/telemetry-receiver   /test/e2e   /web
```

**Corrections against FR-019** (see research.md D-08): drop `/` and `/tunnel` — neither has
a Dockerfile; add `/test/e2e` and `/web` — both do.

```yaml
- package-ecosystem: "docker"
  directory: "/operator"
  schedule:
    interval: "weekly"
    day: "monday"
    time: "03:00"
  commit-message:
    prefix: "chore(deps)"
    include: "scope"
  open-pull-requests-limit: 5
  groups:
    docker-base:
      patterns: ["*"]
```

---

## github-actions — 1 entry

```yaml
- package-ecosystem: "github-actions"
  directory: "/"
  schedule:
    interval: "weekly"
    day: "monday"
    time: "03:00"
  commit-message:
    prefix: "chore(deps)"
    include: "scope"
  open-pull-requests-limit: 5
  groups:
    actions:
      patterns: ["*"]
```

`directory: "/"` is correct here — the github-actions ecosystem is rooted at the repo and
discovers `.github/workflows/` itself. It also updates the composite actions under
`.github/actions/`.

**Interaction with SHA pinning**: this entry rewrites both the SHA and the `# vX.Y.Z`
comment when it proposes an upgrade — provided the comment is in the exact form
`# vX.Y.Z` (contracts/action-pins.md). This is what keeps pins current rather than frozen.
Pinning and Dependabot are complements, not alternatives: pinning gives integrity,
Dependabot gives freshness.

---

## Commit message prefix — a fix, not a preference

Current config uses `prefix: "chore: "`. With `include: "scope"`, Dependabot appends its own
scope, producing:

```
chore: (deps): bump k8s.io/api from 0.35.0 to 0.36.4
```

Malformed under conventional commits, and the repo's own convention (CLAUDE.md rule 11) is
`chore:` / `fix:` / `feat:` with a proper scope. Target `prefix: "chore(deps)"` yields:

```
chore(deps): bump k8s.io/api from 0.35.0 to 0.36.4
```

---

## Verification

**Coverage gap (R7): The Dependabot<->tree parity check is no longer automatically enforced.**
This requirement was deleted along with the verifier system (ruling D-F). Adding a 15th Go
module or a 13th Dockerfile will **not** redden CI until someone manually updates
`dependabot.yml`.

The intended checks were:

```sh
# gomod entries must equal go.work's module list, exactly
diff <(grep -E '^\s+\./' go.work | sed 's#^\s*\./#/#' | sort) \
     <(yq '.updates[] | select(.package-ecosystem=="gomod") | .directory' \
          .github/dependabot.yml | sort)

# docker entries must equal the Dockerfile locations, exactly
diff <(find . -name Dockerfile -not -path './website/*' \
         | sed 's#^\.##; s#/Dockerfile$##; s#^$#/#' | sort) \
     <(yq '.updates[] | select(.package-ecosystem=="docker") | .directory' \
          .github/dependabot.yml | sort)
```

Both diffs should be empty. Code review is the only current enforcement.

Additionally required (not automatically checked, but expected):

- Every entry declares `groups` with ≥ 1 group (prevents PR floods).
- Every entry declares `open-pull-requests-limit`.
- Every entry's `commit-message.prefix` is `chore(deps)`.
- No entry names a directory that does not exist.
