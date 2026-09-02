# External Outreach Tracking

**Last updated**: 2026-09-02  
**Feature**: 012 — Documentation Refresh, Comparison Table, and External Outreach

This file tracks submissions to external directories to grow Gameplane's 
visibility and discoverability. Status updates are committed to git 
(see [Feature Specification](./spec.md) FR-022 for details).

## Submission Status

| Target | Submission URL | Status | Submitted Reference | Notes / History |
|--------|---|---|---|---|
| AlternativeTo | https://alternativeto.net/add-app/ | pending | pending | No blockers identified. Ready for maintainer account creation and submission. See contract for submission template. |
| Awesome-Selfhosted | https://github.com/awesome-selfhosted/awesome-selfhosted-data/pulls | deferred [2026-09-02, first release 2026-06-22 is under the 4-month minimum; eligible from 2026-10-22] | N/A | First release 2026-06-22 (CHANGELOG.md:656); 4-month minimum requires wait until 2026-10-22. See contract for submission template. Re-evaluate on or after 2026-10-22. |
| Awesome-Kubernetes | https://github.com/ramitsurana/awesome-kubernetes | deferred [2026-09-02, 25-star / 3-contributor eligibility rule not verified; revisit in a later release] | N/A | Eligibility metrics (25+ GitHub stars and 3+ contributors) were not verified at deferral. Per OD-6b, no pre-check task or submission attempt in this feature. See contract for eligibility criteria. |

## Draft Submissions (OD-5: agents draft, maintainer submits)

### DRAFT SUBMISSION — AlternativeTo

**Submission Portal**: https://alternativeto.net/add-app/

Use the form at https://alternativeto.net/add-app/ and populate fields with:

```
Platforms:         Kubernetes (1.28+)
License Type:      Open Source (AGPL-3.0-or-later)
Description:       Kubernetes-native game server control panel. 
                   Features: idle auto-sleep, multi-cluster, RBAC, 
                   restic backups, OIDC auth, OCI game templates.
Tags:              [Kubernetes, Game Servers, DevOps, Self-Hosted, 
                   Control Panel, Container Orchestration]
Website:           https://github.com/ValgulNecron/Gameplane
Repository:        https://github.com/ValgulNecron/Gameplane
```

### DRAFT SUBMISSION — Awesome-Selfhosted

**Submission Portal**: https://github.com/awesome-selfhosted/awesome-selfhosted-data (PR to data repo)

**Status**: Deferred until 2026-10-22 (4-month eligibility date). Submit as a PR to https://github.com/awesome-selfhosted/awesome-selfhosted-data/pulls once eligible.

File: `software/gameplane.yml`

```yaml
Title:       Gameplane
URL:         https://github.com/ValgulNecron/Gameplane
License:     AGPL-3.0-or-later
Description: |
  Kubernetes-native game server control panel. Features:
  - Idle auto-sleep with configurable wake windows
  - Multi-cluster GameServer registration
  - RBAC with three built-in roles (admin/operator/viewer)
  - Restic backups to S3-compatible storage
  - OIDC authentication (Keycloak, Google, GitHub)
  - OCI bundle game templates via ModuleSource
  - Supports Minecraft, Terraria, and custom games
  - Helm chart deployment on k3s homelab to production
  
  Beta status: feature-complete for v1 scope, stabilized for external testing.

Category:    Games - Administrative Utilities & Control Panels
Tags:        [Kubernetes, Game Servers, Self-Hosted, DevOps, AGPL]
```

### DRAFT SUBMISSION — Awesome-Kubernetes

**Submission Portal**: https://github.com/ramitsurana/awesome-kubernetes (PR to main README)

**Status**: Deferred (eligibility metrics not verified). No submission attempt at this time. Revisit in a later release if metrics improve.

Add under appropriate category (Infrastructure & Container Orchestration, Kubernetes Tools, or similar):

```markdown
- [Gameplane](https://github.com/ValgulNecron/Gameplane) - Kubernetes-native 
  game server control panel with idle auto-sleep, RBAC, restic backups, OIDC auth, 
  and OCI bundle game templates. Self-hosted, runs on k3s homelab to production. 
  AGPL-3.0 open source. [BETA: v0.2.0-beta.8, feature-complete for v1 scope]
```

## Status Vocabulary

The status values used in the submission table follow this vocabulary (D-D):

- **`pending`** — Submission has not been made. Target is eligible and authorized.
- **`in-progress [YYYY-MM-DD]`** — Submission is in flight (draft prepared, account created, form submitted to third party). Date is when work started.
- **`submitted [YYYY-MM-DD]`** — Submission has been completed (PR opened, form submitted, email sent). Date is the submission date. Terminal state.
- **`rejected [YYYY-MM-DD, reason]`** — Third party rejected the submission. Date and reason must be present. Terminal state.
- **`deferred [YYYY-MM-DD, reason]`** — Submission is not currently possible due to blockers (age requirement, eligibility unknown, etc.). Reason explains blocker and may include future-eligibility date. Terminal state.

**Note on third-party acceptance (FR-023)**: Acceptance or rejection by a third party (e.g., a PR merged by Awesome-Selfhosted) is recorded in the "Notes / History" column only. It does not change the submission status in this table.

## Next Steps

1. **AlternativeTo**: Maintainer creates free account (requires email verification), prepares submission form with template content above, submits.
2. **Awesome-Selfhosted**: Deferred until 2026-10-22 (4-month eligibility date). No action required during this feature; re-evaluate after the date if desired.
3. **Awesome-Kubernetes**: Deferred per OD-6b; no pre-check task or submission attempt. Revisit in a later release if metrics improve or maintainer explicitly authorizes.
