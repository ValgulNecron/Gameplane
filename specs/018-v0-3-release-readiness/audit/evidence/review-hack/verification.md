# T045 ci chunk: independent verification (opus)

**Method.** I tried to refute each candidate in `notes.md` (written by another opus reviewer) against master `13a859ff`. This branch has no diff from master under `hack/`, `Makefile`, `CLAUDE.md` or `docs/`.

What I read:
- `hack/check-links.sh` and `hack/check-doc-versions.sh`, in full.
- The `Makefile` dev, image, lint and coverage targets (`:8-12`, `:35`, `:115-140`, `:232-280`, `:353-391`).
- The `CLAUDE.md` Canonical Commands section, and the spec 012 link contract (`specs/012-docs-refresh-and-outreach/contracts/docs-audit.md:160-200`).
- `audit/findings.md`, and the deploy chunk's `evidence/review-deploy/notes.md` for overlap.
- The diffs of the two open fix PRs that touch these files: #420 (F-026) and #421 (F-030), read with `gh pr diff`.

To confirm the slug behaviour, I copied `github_slug` and `strip_heading_marker` (`check-links.sh:151-183`) into a scratch file outside the repo and ran them on the cited headings. I also queried GHCR anonymously for the `:dev` tags that `make dev-install` references. I ran no test, lint or workflow. The hack review has no held candidates (`notes.md` "held candidates: 0"), and there is no `audit/held/review-hack.md`.

| Candidate | Verdict (kept/rejected) | Severity | Reason |
|---|---|---|---|
| C-hack-01 | kept | S4 | Confirmed by running the extracted functions. `### Config schema → wizard` gives `config-schema-→-wizard`, `## API → Agent` gives `api-→-agent`, and the `docs/key-rotation.md:1` title keeps both `—` and `→`. The control heading `## Beta Status & Limitations` gives `beta-status--limitations`, as the header's worked example says. The header's rule 4 (`:48-49`) and the contract (`docs-audit.md:185`, "special chars removed") both say non-letter characters are dropped. No audited link points at the five affected headings today (grep), so CI is green. But the anchor set the checker builds for those files is wrong now: a correct GitHub anchor would fail the check, and a dead one would pass. |
| C-hack-02 | rejected | n/a | The contract does exclude `ftp://` (`docs-audit.md:173`), and the skip list (`check-links.sh:288-290`) doesn't. But none of the 18 audited files has an `](ftp://` or `](oci://` link (grep), the project publishes nothing over FTP, and `oci://` is not in the contract's list. If such a link were ever added, the checker would fail closed with a `missing file <path>` message naming the target, so the author would see it at once. That is a speculative input with a loud, harmless outcome. |
| C-hack-03 | rejected | n/a | Confirmed that `current_version_pattern` (`check-doc-versions.sh:53`) is never read, and that the `:51` comment says "unquoted" while `Chart.yaml:6` is quoted. Neither changes behaviour: the sed at `:44` handles both forms, and the header comment at `:42-43` already says "quoted or unquoted". An unused variable and a stale internal comment are style. Open PR #420 rewrites this block and keeps both lines, so the maintainer can drop them there if wanted. |
| C-hack-04 | kept (narrowed) | S4 | Kept for the `CLAUDE.md:104` wording only. `dev-load` (`Makefile:367-371`) is four `kind load docker-image` lines with no build step or build prerequisite, so "Rebuild" is wrong. The other half of the candidate says `dev-load` loads 4 of the 12 built images, so sentinel, tunnel and capture pods on a `make dev-up` cluster can't pull their images. That is the same defect as the deploy chunk's C-deploy-02 (`evidence/review-deploy/notes.md`, `Makefile:367-371`), so it is left there and counted once. A supporting observation for that candidate is below. |
| C-hack-05 | rejected | n/a | Confirmed that the table (`CLAUDE.md:137-152`) has no `gp-module` row and that `gp-module/.testcoverage.yml:13` is `total: 80`. This is already being fixed under tracked finding F-030: open PR #421 (T053) adds `\| gp-module \| 80% \| Module authoring CLI \|` to this exact table, as well as the repository-map entry (from `gh pr diff 421`). It duplicates tracked work. Reopen it only if #421 is dropped without that row. |

### C-hack-01

**Location:** `hack/check-links.sh:161-166` (the punctuation strip in `github_slug`). The rule it should implement is documented at `hack/check-links.sh:48-52` and at `specs/012-docs-refresh-and-outreach/contracts/docs-audit.md:185`. Affected headings in audited files: `docs/module-authoring.md:624`, `docs/security.md:159`, `docs/security.md:641`, `docs/key-rotation.md:1` and `docs/install.md:602`.

**Repro / observation:**
1. Read `hack/check-links.sh:166`. The strip is `LC_ALL=C sed … -e 's/[[:punct:]]//g'`. In the C locale `[[:punct:]]` matches only the 32 ASCII punctuation bytes, so it never matches the UTF-8 bytes of `→` (E2 86 92) or `—` (E2 80 94).
2. Copy lines 151-183 of the script into a scratch file, `source` it, and run `github_slug "$(strip_heading_marker '### Config schema → wizard')"`. It prints `config-schema-→-wizard`. `## API → Agent` gives `api-→-agent`. `# Signing key rotation — Ed25519 → ECDSA P-256 (2026-07)` gives `signing-key-rotation-—-ed25519-→-ecdsa-p-256-2026-07`.
3. For comparison, `## Beta Status & Limitations` gives `beta-status--limitations`, which matches the worked example at `:60-67`. So the ASCII path is correct, and only non-ASCII punctuation and symbols are affected.
4. GitHub builds the anchor by removing every character that is not a letter, digit, space, hyphen or underscore, which is the rule the header states. The first heading's GitHub anchor is therefore `#config-schema--wizard`.
5. Consequence: a link `[wizard](module-authoring.md#config-schema--wizard)` in any audited doc works on GitHub but makes `make check-links` report `missing anchor`. A link to `#config-schema-→-wizard` passes the checker but is dead on GitHub.
6. `grep -rnoE '\]\([^)]*#(config-schema|api-|signing-key-rotation|sqlite-database-adoption|trust-chain)[^)]*\)'` over the audited files finds nothing, so no current link hits this.

**Expected:** Rule 4 as documented at `check-links.sh:48-49`: remove "any character that is not a letter (including Unicode letters), digit, space, hyphen, or underscore". Non-letter Unicode characters such as `→` and `—` are dropped, and Unicode letters are kept.

**Actual:** Only ASCII punctuation is dropped, and Unicode punctuation and symbols stay in the slug. The defect is latent until someone links to one of the five headings, and then it gives a false failure or a false pass.

### C-hack-04

**Location:** `CLAUDE.md:104`, compared with `Makefile:367-371` (`dev-load`) and `Makefile:353-365` (`dev-up`, whose kind path runs `make images` at `:361` before `make dev-load` at `:362`).

**Repro / observation:**
1. Read `CLAUDE.md:104`: "`make dev-load` # Rebuild and reload local images into Kind".
2. Read `Makefile:367-371`. `dev-load` has no prerequisites. Its recipe is `kind load docker-image $(REGISTRY)/{operator,api,web,agent}:$(TAG)`, and its own help string is "Load local images into kind cluster (kind only)".
3. On a running `make dev-up` cluster, edit a file under `api/` and run `make dev-load`. kind loads the `ghcr.io/valgulnecron/gameplane/api:dev` image that is already in the local Docker daemon, which was built before the edit. The edit does not reach the cluster until `make images TAG=dev` (or `make image-api`) runs first.

**Expected:** `CLAUDE.md` says what `dev-load` does, "load the already-built local images into kind", and names the build step, matching the target's help string.

**Actual:** `CLAUDE.md` says the target rebuilds. An agent following it after a code change deploys a stale image.

**Supporting observation for C-deploy-02 (not counted here):** On 2026-09-24, anonymous GHCR manifest requests for `valgulnecron/gameplane/sentinel:dev` and `valgulnecron/gameplane/agent:dev` both returned 404, and the same requests for `:edge` returned 200. `dev-install` points a `make dev-up` kind cluster at `ghcr.io/valgulnecron/gameplane/<component>:dev` (`Makefile:386-387`). For the 8 images `dev-load` skips (sentinel, capture-sidecar, the three tunnels, mcp-server, telemetry-receiver and audit-syslog-bridge), neither kind nor GHCR can supply that ref, because `deploy/kind/up.sh` loads no images and GHCR has no `:dev` tag.
