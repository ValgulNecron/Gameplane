---
description: "Task list for Feature 015: Dedicated Server Modules for Top Steam Games"
---

# Tasks: Dedicated Server Modules for Top Steam Games

**Input**: Design documents from `specs/015-top-steam-game-modules/`: `spec.md`, `plan.md`, `data-model.md`, `contracts/module-spec-contract.md`, `contracts/gametemplate-contract.md`, `contracts/engine-matrix-contract.md`.

**Prerequisites**: plan.md (required), spec.md (required for user stories), data-model.md, contracts/.

**Branch note**: `spec.md` and `plan.md` header the feature as `014-top-steam-game-modules`; the folder is `specs/015-top-steam-game-modules/`; the actual git branch checked out for this work is `claude/top-steam-game-modules-x7oyy1`. None of the three agree. This task list uses the real branch name and the real folder path, and T099 records the numbering mismatch as a documentation debt rather than silently picking one.

**Ground truth**: every task below is written against the shipped CRD schema (`modules/.schema/gametemplate.schema.json`) and the real `modules/` submodule contents, not against `contracts/gametemplate-contract.md` or `data-model.md` where those disagree with the schema — see T005/OPEN-DECISIONS.md for the enumerated mismatches. In particular: `spec.capabilities.lifecycle.stop` (not `spec.lifecycle.stop`) is an array of 1-16 command strings with no `action`/`command`/`timeoutSeconds` keys; `spec.rcon.protocol` is one of `source|telnet|websocket|battleye|satisfactory|palworld|none` (the schema's enum; `modules/validate.py:129` additionally accepts `nuclearoption`, used by `modules/nuclear-option/` — a schema/validator drift worth noting to the maintainer, no effect on this feature's 26 games); `spec.storage` has no `defaultSize`/`subPaths`; `spec.security` has no `readOnlyRootFilesystem`; the sample manifest is `samples/gameserver.yaml`, not `samples/server.yaml`. The schema's field description states `capabilities.lifecycle.stop` requires `rcon.protocol != none`, but the apiserver does not enforce that as a CEL rule and `modules/dont-starve-together/template.yaml` ships a `stop` sequence under `rcon.protocol: none` + `consoleMode: pty` — treat the requirement as documented-but-unenforced and see OPEN-DECISIONS.md for the ruling this feature needs from the maintainer before touching any `none`-protocol module's `capabilities.lifecycle.stop`.

**Tests**: Included — mandatory per spec.md's User Story acceptance scenarios, FR-011, and Constitution Principle I (E2E-Tested Delivery). Every module gets a real wire-protocol e2e probe in `test/e2e/`; resource-heavy games (>8 GB RAM) get their test committed with an explicit CI-exclusion comment in `test/e2e/buckets.sh`, per the existing `bucket_bot_heavy` pattern — never a silent skip. Every new game additionally gets a `test/e2e/internal/<game>/` probe package (Phase 2) proven to fail against a dead address and succeed against a real listener before any test task is allowed to depend on it, per Constitution Principle I.

**Local execution**: Per CLAUDE.md rule 8, `make test`, `make lint`, `make cover`, `go test`, and `npm test` MUST NOT be run locally — GitHub Actions is the source of truth. The one sanctioned local check is `python3 modules/validate.py` in its default (non-`--pin`) mode (plan.md: "local execution limited to standard static preflight checks"); T098 uses it. `--pin` mode rewrites templates and is used only by T097.

**Organization**: Tasks are grouped by user story (US1-US5, priorities from spec.md: US1 P1, US2 P1, US3 P2, US4 P2, US5 P3) so each story is independently deliverable. Because every story after US1 edits the same per-module `template.yaml` files US1 authors, US2-US5 run *after* US1 completes rather than concurrently with it (see Dependencies below) — they remain independently testable, just not independently schedulable against a not-yet-authored file.

## Format: `T### [P?] [US#?] Description`

- **[P]**: Different files, no dependency on another incomplete task in this list — safe to run concurrently with every other `[P]` task in the file, not just its neighbors.
- **[US#]**: User story tag (US1-US5); present on every task inside a user-story phase (3-7) and on no task outside one.
- Every description carries a repo-relative path (never `/home/user/Gameplane/...`).

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Get the `modules/` submodule and its validator ready to receive work.

- [ ] T001 Initialize the `modules/` git submodule (`git submodule update --init modules`) and confirm the checkout is non-empty — `modules/.schema/gametemplate.schema.json`, `modules/.schema/module.schema.json`, and `modules/validate.py` must all be present and readable before any later task runs.
- [ ] T002 [P] Inside the `modules/` submodule checkout (the separate `ValgulNecron/gameplane-module` repo), create a working branch (e.g. `top-steam-game-modules`) off its default branch — `modules/` — since every module content change in this feature is committed in that repo, never in this one; this repo only ever gets the bumped submodule pointer (T103).
- [ ] T003 [P] Confirm `modules/.schema/module.schema.json` and `modules/.schema/gametemplate.schema.json` are the sole binding authority for `module.yaml`/`template.yaml` shape for this feature, superseding any conflicting prose in `specs/015-top-steam-game-modules/data-model.md` and `specs/015-top-steam-game-modules/contracts/gametemplate-contract.md` — anchor file `modules/.schema/gametemplate.schema.json`.
- [ ] T004 [P] Install PyYAML so `python3 modules/validate.py` can run locally (`pip install --user pyyaml`, or a repo-local venv) — `modules/validate.py`.

**Checkpoint**: submodule present, schema authority confirmed, validator runnable locally.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Produce the shared artifacts every Phase 3-7 task consumes, including a real wire-protocol probe package for every new game. **Nothing in Phase 3 onward may start until this phase is done**, and no Phase 4-7 test task may depend on a probe package until its proof task (bundled into the same task below) passes.

- [ ] T005 [P] Reconcile `specs/015-top-steam-game-modules/contracts/gametemplate-contract.md`, `specs/015-top-steam-game-modules/contracts/module-spec-contract.md`, `specs/015-top-steam-game-modules/contracts/engine-matrix-contract.md`, and `specs/015-top-steam-game-modules/data-model.md` against the shipped CRD schema `modules/.schema/gametemplate.schema.json` and the real `modules/` contents, and record every mismatch in a new `specs/015-top-steam-game-modules/OPEN-DECISIONS.md`: (1) `gametemplate-contract.md` §2 lists `rest`/`cli` as `rcon.protocol` values — neither exists; the real enum is `source|telnet|websocket|battleye|satisfactory|palworld|none`; (2) `gametemplate-contract.md` §3 and `data-model.md` §2.2/§3.2 describe `spec.lifecycle.stop` as an object with `action`/`command`/`timeoutSeconds` — the real field is `spec.capabilities.lifecycle.stop`, a 1-16-item array of plain command strings; the schema's prose says it requires `rcon.protocol != none` but this is not CEL-enforced, and `modules/dont-starve-together/template.yaml` ships a `stop` array under `rcon.protocol: none` — record this as an open ambiguity needing a maintainer ruling, not something this task resolves; (3) `data-model.md` §2.2 lists `storage.defaultSize`, `storage.subPaths`, and `security.readOnlyRootFilesystem` — none exist on the CRD; (4) `data-model.md` §2.2 marks `storage`, `security`, `lifecycle`, `versions`, `ports`, `categories`, `accentColor`, `description` Required — the CRD requires only `displayName`, `game`, `image`, `version`; (5) `module-spec-contract.md` §1 specifies `samples/server.yaml` — every real module in `modules/` uses `samples/gameserver.yaml`; (6) `engine-matrix-contract.md`'s per-game `rcon.protocol` cells contradict four shipped templates — `garrys-mod` is `none` with no `capabilities` block at all (not `source`); `7-days-to-die` is `none` (not `telnet`; its TelnetPassword lives in serverconfig.xml under serverfiles/, unreachable from the world-saves mount); `factorio` is `source` **with** `consoleMode: pty` (not `none`+pty alone); `project-zomboid` is `source` (not `none`+pty). State explicitly that editing the contract/data-model files themselves is **out of scope for this task** and requires maintainer sign-off before any such edit is applied.
- [ ] T006 [P] Produce `specs/015-top-steam-game-modules/engine-matrix-resolved.md`: a per-module table for all 26 modules in FR-001, giving each one's real `rcon.protocol` (drawn only from `source|telnet|websocket|battleye|satisfactory|palworld|none`, corrected per T005 item 6 for garrys-mod/7-days-to-die/factorio/project-zomboid), `consoleMode` (`rcon|pty|none`), `spec.ports` entries, `spec.storage.mountPath`, and `spec.capabilities.lifecycle.stop` command array — correcting every `rest`/`cli` cell inherited from `specs/015-top-steam-game-modules/contracts/engine-matrix-contract.md` to a real enum value, or to `none` + `consoleMode: pty` for stdin/PTY-only games (Terraria, BeamMP, Arma Reforger, ETS2, Don't Starve Together, Bannerlord — per FR-013's diagnostic-idle requirement where a token applies; Factorio and Project Zomboid are excluded from this list per T005 item 6). Also derive, from each new module's declared (or to-be-declared) `spec.storage.size`/`storage.extra` and `spec.resources` against the same disk/memory criteria `test/e2e/buckets.sh`'s `bucket_bot_heavy` comment block already uses, a fast/heavy CI-bucket classification column for the 13 new modules; this column is the single source T054/T061/T074/T091 cite instead of asserting a bucket inline. This file is the single input every Phase 3 module task and every Phase 4-7 audit task cites for its rcon/lifecycle/storage/probe/bucket values.
- [ ] T007 [P] Add a directory-layout rule to `modules/validate.py` that errors when a module directory is missing `README.md`, `specs.md`, or a non-empty `samples/` directory — closing the gap where FR-002/SC-001/SC-003 are currently unenforced by the validator — and add a matching pass/fail-style self-check harness at `modules/test-validate-py.sh` (new file, following `modules/test-build-sh.sh`'s harness style — not added to `test-build-sh.sh` itself, whose own header scopes it to build.sh's signing preconditions) exercising the new rule against a fixture module missing each file in turn, and register it in the `gameplane-module` repo's own CI under `modules/.github/`.
- [ ] T008 [P] Add a shared `specs.md` section skeleton at `specs/015-top-steam-game-modules/contracts/specs-skeleton.md`, reproducing the 9 numbered headings from `specs/015-top-steam-game-modules/contracts/module-spec-contract.md` §2 verbatim (`# Gameplane Module Specification: <Display Name>` through `## 9. References & Upstream Documentation`), for the 26 per-module `specs.md` files authored in Phase 3 to be written against.
- [ ] T009 Edit `test/e2e/gamebot_helpers_e2e_test.go`, adding the 13 new game slugs to `fastGameSet` / `heavyGameSet` per the classification in `specs/015-top-steam-game-modules/engine-matrix-resolved.md` (T006) — `skipUnlessGameInScope` (gamebot_helpers_e2e_test.go:73) skips any game absent from both sets via `parseGameScope`, so every one of the 19 new e2e test tasks in Phases 4-7 depends on this task, not the reverse. Do not create a second helpers file — a duplicate copy would not change `skipUnlessGameInScope`'s behavior and risks package-level redeclaration. Bucket-registration in `test/e2e/buckets.sh` is a separate concern, handled by the dedicated tasks T054, T061, T074 and T091, which are never `[P]` and always depend on the test tasks they register.
- [ ] T010 For `fivem`, `farming-simulator-25`, `euro-truck-simulator-2`, and `beammp`: identify a concrete upstream image (name + `@sha256:` digest) that already idles-with-diagnostics on a missing token and supervises its auxiliary processes (FR-012/FR-013), or record in `specs/015-top-steam-game-modules/OPEN-DECISIONS.md` that FR-012/FR-013 cannot be met with an off-the-shelf image for that game and needs a maintainer ruling — a `GameTemplate` can only set `spec.command`/`spec.args`/`spec.env` over an upstream image, and building custom images is out of scope (research.md Decision 1). Depends on T005 (writes to the same OPEN-DECISIONS.md).
- [ ] T011 [P] Author the FiveM e2e probe package: `test/e2e/internal/fivem/app.go` (package main, standard `-addr`/`-deadline`/`-expect-depth`/`-expect-fail` flags, own retry loop) and `test/e2e/internal/fivem/spec.md`, reusing `test/e2e/internal/protocol/{a2sproto,sourceproto,joindepth}` where the wire format matches; prove the probe fails against a dead address and succeeds against a real listener before any task depends on it. Model on `test/e2e/internal/cs2/app.go`.
- [ ] T012 [P] Author the Team Fortress 2 e2e probe package: `test/e2e/internal/team-fortress-2/app.go` and `test/e2e/internal/team-fortress-2/spec.md` (Source/A2S_INFO family via `test/e2e/internal/protocol/{a2sproto,sourceproto}`); prove fail-dead/succeed-live per Constitution Principle I.
- [ ] T013 [P] Author the Farming Simulator 25 e2e probe package: `test/e2e/internal/farming-simulator-25/app.go` and `test/e2e/internal/farming-simulator-25/spec.md` (GIANTS/HTTP query); prove fail-dead/succeed-live.
- [ ] T014 [P] Author the Euro Truck Simulator 2 e2e probe package: `test/e2e/internal/euro-truck-simulator-2/app.go` and `test/e2e/internal/euro-truck-simulator-2/spec.md` (A2S_INFO via `test/e2e/internal/protocol/a2sproto`); prove fail-dead/succeed-live.
- [ ] T015 [P] Author the Mount & Blade II: Bannerlord e2e probe package: `test/e2e/internal/mount-and-blade-2-bannerlord/app.go` and `.../spec.md` (A2S_INFO via `a2sproto`); prove fail-dead/succeed-live.
- [ ] T016 [P] Author the tModLoader e2e probe package: `test/e2e/internal/tmodloader/app.go` and `.../spec.md` (Terraria-family custom TCP via `test/e2e/internal/protocol/joindepth`); prove fail-dead/succeed-live.
- [ ] T017 [P] Author the BeamMP e2e probe package: `test/e2e/internal/beammp/app.go` and `.../spec.md` (custom TCP/UDP handshake); prove fail-dead/succeed-live.
- [ ] T018 [P] Author the Left 4 Dead 2 e2e probe package: `test/e2e/internal/left-4-dead-2/app.go` and `.../spec.md` (A2S_INFO via `a2sproto`); prove fail-dead/succeed-live.
- [ ] T019 [P] Author The Isle e2e probe package: `test/e2e/internal/the-isle/app.go` and `.../spec.md` (A2S_INFO via `a2sproto`); prove fail-dead/succeed-live.
- [ ] T020 [P] Author the ARK: Survival Evolved e2e probe package: `test/e2e/internal/ark-survival-evolved/app.go` and `.../spec.md` (A2S_INFO via `a2sproto`, cluster-aware per `joindepth` where applicable); prove fail-dead/succeed-live.
- [ ] T021 [P] Author the Arma Reforger e2e probe package: `test/e2e/internal/arma-reforger/app.go` and `.../spec.md` (A2S_INFO via `a2sproto`); prove fail-dead/succeed-live.
- [ ] T022 [P] Author the Hell Let Loose e2e probe package: `test/e2e/internal/hell-let-loose/app.go` and `.../spec.md` (A2S_INFO via `a2sproto`); prove fail-dead/succeed-live.
- [ ] T023 [P] Author the Squad e2e probe package: `test/e2e/internal/squad/app.go` and `.../spec.md` (Source-family A2S_INFO via `a2sproto`/`sourceproto`); prove fail-dead/succeed-live.

**Checkpoint**: `OPEN-DECISIONS.md` and `engine-matrix-resolved.md` exist and are internally consistent with the shipped schema; the validator enforces layout; the specs skeleton is in place; `gamebot_helpers_e2e_test.go` recognizes all 13 new games; every new game has a proven wire-protocol probe package. User story work can now begin.

---

## Phase 3: User Story 1 - One-Click Deployment for Top Steam Multiplayer Dedicated Servers (Priority: P1) 🎯 MVP

**Goal**: Every one of the 26 top-Steam-multiplayer games has a complete, schema-valid module package that boots into a healthy, ready-to-accept-players state with correct ports and storage.

**Independent Test**: Instantiate any of the 26 modules with default parameters and verify the server boots to healthy with correct port exposure and storage persistence (spec.md US1 Independent Test).

**Scope note**: this phase authors each module's `template.yaml` **once, completely** — storage, rcon, lifecycle, mods, and probes stanzas all included per `specs/015-top-steam-game-modules/engine-matrix-resolved.md` and the house style of `modules/rust/template.yaml` — because a module is not deployable without all of them together. User Stories 2-5 do not re-author these files; they verify, extend narrowly, and write tests against what is authored here. Treat every "audit" task in Phases 4-7 as reading and, where needed, incrementally adjusting these files — not rewriting them. All 13 existing modules already ship `module.yaml`, `template.yaml` and `README.md` — those are **verified and updated**, never authored fresh; only `specs.md` (all 26) and `samples/` (7 of the 13 existing modules) are genuinely missing. T050/T051 close the FR-006 security-context gap for all 26 modules once every `template.yaml` in this phase is settled.

Of the 13 target *existing* modules, 7 have no `samples/` directory today and get one authored here for the first time: `project-zomboid`, `dayz`, `garrys-mod`, `7-days-to-die`, `ark-survival-ascended`, `dont-starve-together`, `satisfactory`. The other 6 existing modules (`cs2`, `palworld`, `rust`, `terraria`, `factorio`, `valheim`) already have `samples/gameserver.yaml` and get it verified/updated instead. **None** of the 26 modules has a `specs.md` today — every task below authors one.

- [ ] T024 [P] [US1] Standardize Counter-Strike 2 (`cs2`, existing module): verify/update the existing `modules/cs2/module.yaml` against `modules/.schema/module.schema.json`, and author `modules/cs2/specs.md` (per `specs/015-top-steam-game-modules/contracts/specs-skeleton.md`); bring `modules/cs2/template.yaml`'s storage/rcon(`source`)/lifecycle/probes stanzas in line with `specs/015-top-steam-game-modules/engine-matrix-resolved.md`; update `modules/cs2/README.md`; verify `modules/cs2/samples/gameserver.yaml` against the standardized template.
- [ ] T025 [P] [US1] Standardize Palworld (`palworld`, existing module): verify/update the existing `modules/palworld/module.yaml` and author `modules/palworld/specs.md`; align `modules/palworld/template.yaml`'s storage/rcon(`palworld`)/lifecycle/probes stanzas with `specs/015-top-steam-game-modules/engine-matrix-resolved.md`; update `modules/palworld/README.md`; verify `modules/palworld/samples/gameserver.yaml`.
- [ ] T026 [P] [US1] Create FiveM (`fivem`, new module): author `modules/fivem/module.yaml`, `modules/fivem/template.yaml` (full storage/rcon(`none`)+`consoleMode: pty`/lifecycle/probes per `specs/015-top-steam-game-modules/engine-matrix-resolved.md`, house style from `modules/rust/template.yaml`, plus the FR-013 graceful-idle diagnostic entrypoint for a missing CFX license key and the FR-012 single-pod txAdmin/database bundling using the image identified in T010), `modules/fivem/README.md`, `modules/fivem/specs.md`, `modules/fivem/samples/gameserver.yaml`. Depends on T010.
- [ ] T027 [P] [US1] Standardize Rust (`rust`, existing module, house-style reference): verify/update the existing `modules/rust/module.yaml` and author `modules/rust/specs.md`; `modules/rust/template.yaml` is already the feature's style exemplar — **verify only** against `specs/015-top-steam-game-modules/engine-matrix-resolved.md`, do not rewrite it; any change to `modules/rust/template.yaml` must be raised as a separate, non-`[P]` task since 24 other Phase 3 tasks cite it as their house-style reference; update `modules/rust/README.md`; verify `modules/rust/samples/gameserver.yaml`.
- [ ] T028 [P] [US1] Standardize Project Zomboid (`project-zomboid`, existing module): verify/update the existing `modules/project-zomboid/module.yaml` and author `modules/project-zomboid/specs.md`; align `modules/project-zomboid/template.yaml`'s storage/rcon(`source`)/lifecycle/probes stanzas with `specs/015-top-steam-game-modules/engine-matrix-resolved.md` (per T005 item 6, this module is `rcon.protocol: source`, not `none`+pty); update `modules/project-zomboid/README.md`; author `modules/project-zomboid/samples/gameserver.yaml` (currently missing).
- [ ] T029 [P] [US1] Create Team Fortress 2 (`team-fortress-2`, new module): author `modules/team-fortress-2/module.yaml`, `modules/team-fortress-2/template.yaml` (storage/rcon(`source`)/lifecycle/probes per `specs/015-top-steam-game-modules/engine-matrix-resolved.md`, house style from `modules/rust/template.yaml`), `modules/team-fortress-2/README.md`, `modules/team-fortress-2/specs.md`, `modules/team-fortress-2/samples/gameserver.yaml`.
- [ ] T030 [P] [US1] Standardize DayZ (`dayz`, existing module): verify/update the existing `modules/dayz/module.yaml` and author `modules/dayz/specs.md`; align `modules/dayz/template.yaml`'s storage/rcon(`battleye`)/lifecycle/probes stanzas and Steam Workshop mod paths with `specs/015-top-steam-game-modules/engine-matrix-resolved.md`; update `modules/dayz/README.md`; author `modules/dayz/samples/gameserver.yaml` (currently missing).
- [ ] T031 [P] [US1] Create Farming Simulator 25 (`farming-simulator-25`, new module): author `modules/farming-simulator-25/module.yaml`, `modules/farming-simulator-25/template.yaml` (Wine/Proton headless layer using the image identified in T010, `rcon: none` + `consoleMode: pty`, the web admin port declared under `spec.ports` rather than an unsupported rcon protocol, single-pod bundling of the web management portal per FR-012), `modules/farming-simulator-25/README.md`, `modules/farming-simulator-25/specs.md`, `modules/farming-simulator-25/samples/gameserver.yaml`. Depends on T010.
- [ ] T032 [P] [US1] Create Euro Truck Simulator 2 (`euro-truck-simulator-2`, new module): author `modules/euro-truck-simulator-2/module.yaml`, `modules/euro-truck-simulator-2/template.yaml` (`rcon: none` + `consoleMode: pty`, ETS2 logon token via `spec.env` with the FR-013 graceful-idle diagnostic when it's missing, using the image identified in T010), `modules/euro-truck-simulator-2/README.md`, `modules/euro-truck-simulator-2/specs.md`, `modules/euro-truck-simulator-2/samples/gameserver.yaml`. Depends on T010.
- [ ] T033 [P] [US1] Standardize Garry's Mod (`garrys-mod`, existing module): verify/update the existing `modules/garrys-mod/module.yaml` and author `modules/garrys-mod/specs.md`; confirm `modules/garrys-mod/template.yaml`'s `rcon.protocol: none` with no `capabilities` block against `specs/015-top-steam-game-modules/engine-matrix-resolved.md` — this is a deliberate, documented omission (no RCON reachable; see the template's header comment), not a gap to fill on this task; align only the storage/probes stanzas; update `modules/garrys-mod/README.md`; author `modules/garrys-mod/samples/gameserver.yaml` (currently missing). Steam Workshop collection support is out of scope here — see T075.
- [ ] T034 [P] [US1] Create Mount & Blade II: Bannerlord (`mount-and-blade-2-bannerlord`, new module): author `modules/mount-and-blade-2-bannerlord/module.yaml`, `modules/mount-and-blade-2-bannerlord/template.yaml` (`rcon: none` + `consoleMode: pty`, match-based server with no persistent world save), `modules/mount-and-blade-2-bannerlord/README.md`, `modules/mount-and-blade-2-bannerlord/specs.md`, `modules/mount-and-blade-2-bannerlord/samples/gameserver.yaml`.
- [ ] T035 [P] [US1] Standardize Terraria (`terraria`, existing module): verify/update the existing `modules/terraria/module.yaml` and author `modules/terraria/specs.md`; align `modules/terraria/template.yaml`'s storage/rcon(`none`)+`consoleMode: pty`/lifecycle/probes stanzas with `specs/015-top-steam-game-modules/engine-matrix-resolved.md`; update `modules/terraria/README.md`; verify `modules/terraria/samples/gameserver.yaml`.
- [ ] T036 [P] [US1] Standardize 7 Days to Die (`7-days-to-die`, existing module): verify/update the existing `modules/7-days-to-die/module.yaml` and author `modules/7-days-to-die/specs.md`; confirm `modules/7-days-to-die/template.yaml`'s `rcon.protocol: none` with no `capabilities` block against `specs/015-top-steam-game-modules/engine-matrix-resolved.md` (per T005 item 6 — TelnetPassword lives in serverconfig.xml under serverfiles/, unreachable from the world-saves mount; note the module's declared telnet port is 8081, not the engine-matrix-contract's 8082 — record that divergence in OPEN-DECISIONS, do not silently renumber it); align only the storage/probes stanzas; update `modules/7-days-to-die/README.md`; author `modules/7-days-to-die/samples/gameserver.yaml` (currently missing).
- [ ] T037 [P] [US1] Create tModLoader (`tmodloader`, new module): author `modules/tmodloader/module.yaml`, `modules/tmodloader/template.yaml` (`rcon: none` + `consoleMode: pty`, Terraria modding framework layout, `capabilities.mods` workshop/mod-install path per FR-010), `modules/tmodloader/README.md`, `modules/tmodloader/specs.md`, `modules/tmodloader/samples/gameserver.yaml`.
- [ ] T038 [P] [US1] Create BeamMP (`beammp`, new module): author `modules/beammp/module.yaml`, `modules/beammp/template.yaml` (`rcon: none` + `consoleMode: pty`, BeamMP auth-key FR-013 graceful-idle diagnostic using the image identified in T010, custom vehicles/maps mount under `capabilities.mods`), `modules/beammp/README.md`, `modules/beammp/specs.md`, `modules/beammp/samples/gameserver.yaml`. Depends on T010.
- [ ] T039 [P] [US1] Standardize ARK: Survival Ascended (`ark-survival-ascended`, existing module): verify/update the existing `modules/ark-survival-ascended/module.yaml` and author `modules/ark-survival-ascended/specs.md`; align `modules/ark-survival-ascended/template.yaml`'s storage/rcon(`source`)/lifecycle/probes stanzas and cluster-travel storage layout with `specs/015-top-steam-game-modules/engine-matrix-resolved.md`; update `modules/ark-survival-ascended/README.md`; author `modules/ark-survival-ascended/samples/gameserver.yaml` (currently missing).
- [ ] T040 [P] [US1] Create Left 4 Dead 2 (`left-4-dead-2`, new module): author `modules/left-4-dead-2/module.yaml`, `modules/left-4-dead-2/template.yaml` (storage/rcon(`source`)/lifecycle/probes per `specs/015-top-steam-game-modules/engine-matrix-resolved.md`), `modules/left-4-dead-2/README.md`, `modules/left-4-dead-2/specs.md`, `modules/left-4-dead-2/samples/gameserver.yaml`.
- [ ] T041 [P] [US1] Standardize Factorio (`factorio`, existing module): verify/update the existing `modules/factorio/module.yaml` and author `modules/factorio/specs.md`; confirm `modules/factorio/template.yaml`'s `rcon.protocol: source` **with** `consoleMode: pty` against `specs/015-top-steam-game-modules/engine-matrix-resolved.md` (per T005 item 6 — this is not a `none`+pty module); align the storage/lifecycle/probes stanzas; update `modules/factorio/README.md`; verify `modules/factorio/samples/gameserver.yaml`.
- [ ] T042 [P] [US1] Create The Isle (`the-isle`, new module): author `modules/the-isle/module.yaml`, `modules/the-isle/template.yaml` (storage/rcon(`source`)/lifecycle/probes per `specs/015-top-steam-game-modules/engine-matrix-resolved.md`), `modules/the-isle/README.md`, `modules/the-isle/specs.md`, `modules/the-isle/samples/gameserver.yaml`.
- [ ] T043 [P] [US1] Standardize Don't Starve Together (`dont-starve-together`, existing module): verify/update the existing `modules/dont-starve-together/module.yaml` and author `modules/dont-starve-together/specs.md`; align `modules/dont-starve-together/template.yaml`'s storage/rcon(`none`)+`consoleMode: pty` Lua console/lifecycle/probes stanzas and master/caves multi-shard storage layout with `specs/015-top-steam-game-modules/engine-matrix-resolved.md` — note this module ships a `capabilities.lifecycle.stop` array under `rcon.protocol: none` (the OPEN-DECISIONS.md ambiguity from T005 item 2); do not remove it; update `modules/dont-starve-together/README.md`; author `modules/dont-starve-together/samples/gameserver.yaml` (currently missing).
- [ ] T044 [P] [US1] Standardize Valheim (`valheim`, existing module): verify/update the existing `modules/valheim/module.yaml` and author `modules/valheim/specs.md`; align `modules/valheim/template.yaml`'s storage/rcon/lifecycle/probes and BepInEx mod-loader stanzas with `specs/015-top-steam-game-modules/engine-matrix-resolved.md`; update `modules/valheim/README.md`; verify `modules/valheim/samples/gameserver.yaml`.
- [ ] T045 [P] [US1] Standardize Satisfactory (`satisfactory`, existing module): verify/update the existing `modules/satisfactory/module.yaml` and author `modules/satisfactory/specs.md`; align `modules/satisfactory/template.yaml`'s storage/rcon(`satisfactory`)/lifecycle/probes stanzas with `specs/015-top-steam-game-modules/engine-matrix-resolved.md`; update `modules/satisfactory/README.md`; author `modules/satisfactory/samples/gameserver.yaml` (currently missing).
- [ ] T046 [US1] Create ARK: Survival Evolved (`ark-survival-evolved`, new module): author `modules/ark-survival-evolved/module.yaml`, `modules/ark-survival-evolved/template.yaml` (storage/rcon(`source`)/lifecycle/probes per `specs/015-top-steam-game-modules/engine-matrix-resolved.md`, cluster-travel storage layout consistent with `ark-survival-ascended`), `modules/ark-survival-evolved/README.md`, `modules/ark-survival-evolved/specs.md`, `modules/ark-survival-evolved/samples/gameserver.yaml`. Not `[P]`: requires `modules/ark-survival-ascended/template.yaml`'s finished cluster-travel layout. Depends on T039.
- [ ] T047 [P] [US1] Create Arma Reforger (`arma-reforger`, new module): author `modules/arma-reforger/module.yaml`, `modules/arma-reforger/template.yaml` (`rcon: none` + `consoleMode: pty`, authenticated-SteamCMD-login note per spec.md Edge Cases, Steam Workshop mod support), `modules/arma-reforger/README.md`, `modules/arma-reforger/specs.md`, `modules/arma-reforger/samples/gameserver.yaml`.
- [ ] T048 [P] [US1] Create Hell Let Loose (`hell-let-loose`, new module): author `modules/hell-let-loose/module.yaml`, `modules/hell-let-loose/template.yaml` (storage/rcon(`source`)/lifecycle/probes per `specs/015-top-steam-game-modules/engine-matrix-resolved.md`; `spec.versions` includes the default Hell Let Loose build plus an optional Vietnam community-preset version, per spec.md Clarifications — see also T097's digest-pinning pass), `modules/hell-let-loose/README.md`, `modules/hell-let-loose/specs.md`, `modules/hell-let-loose/samples/gameserver.yaml`.
- [ ] T049 [P] [US1] Create Squad (`squad`, new module): author `modules/squad/module.yaml`, `modules/squad/template.yaml` (storage/rcon(`source`)/lifecycle/probes per `specs/015-top-steam-game-modules/engine-matrix-resolved.md`), `modules/squad/README.md`, `modules/squad/specs.md`, `modules/squad/samples/gameserver.yaml`.
- [ ] T050 [US1] Audit `spec.security.runAsUser`/`runAsGroup`/`fsGroup` and the `HOME` entry in `spec.env` in the 13 **new** modules' `template.yaml` against `modules/validate.py` rule 2 (image `User` must match `runAsUser`), rule 3 (`runAsUser` + SteamCMD-shaped image requires `HOME`) and rule 8 (PUID/PGID images need `fsGroup`), satisfying FR-006. Modules: `fivem`, `team-fortress-2`, `farming-simulator-25`, `euro-truck-simulator-2`, `mount-and-blade-2-bannerlord`, `tmodloader`, `beammp`, `left-4-dead-2`, `the-isle`, `ark-survival-evolved`, `arma-reforger`, `hell-let-loose`, `squad`. Depends on T026, T029, T031, T032, T034, T037, T038, T040, T042, T046, T047, T048, T049.
- [ ] T051 [US1] Audit `spec.security.runAsUser`/`runAsGroup`/`fsGroup` and the `HOME` entry in `spec.env` in the 13 **existing** modules' `template.yaml` against the same `modules/validate.py` rules 2/3/8, satisfying FR-006. Modules: `cs2`, `palworld`, `rust`, `project-zomboid`, `dayz`, `garrys-mod`, `terraria`, `7-days-to-die`, `ark-survival-ascended`, `factorio`, `dont-starve-together`, `valheim`, `satisfactory`. Depends on T024, T025, T027, T028, T030, T033, T035, T036, T039, T041, T043, T044, T045.

**Checkpoint**: all 26 modules have a complete, schema-conformant package, including verified security-context wiring. User Story 1 is independently deployable and testable.

---

## Phase 4: User Story 2 - Persistent Game State, World Saves, and Multi-Shard Clustering (Priority: P1)

**Goal**: Savegames, world files, and configs survive container restarts and image upgrades without corruption; multi-instance games (DST master/caves, ARK cluster travel) keep cross-shard state consistent. SC-005's "100% of game servers restart without data loss" is verified here by **sampling**: T052/T053 cover one representative of the two storage topologies this feature introduces (a single-volume game and a cluster-travel multi-shard game); the remaining 24 modules are covered by the storage/mountPath audits T055-T058, not by a dedicated restart test each — this is a sampling choice, not full coverage, and is recorded as such rather than claimed as 100% exercised.

**Independent Test**: Start a server, generate in-game state, restart the container, and verify all world data/configs/saves are preserved without corruption (spec.md US2 Independent Test).

- [ ] T052 [P] [US2] Author `test/e2e/fivem_persistence_e2e_test.go`: write a marker via the FiveM data volume, restart the `fivem` GameServer, and assert the marker and world/config state under `modules/fivem`'s `storage.mountPath` survive the restart. Depends on T011 (fivem probe package proven).
- [ ] T053 [P] [US2] Author `test/e2e/arksurvival_cluster_persistence_e2e_test.go`: verify ARK: Survival Ascended and ARK: Survival Evolved cluster-travel shard storage stays consistent across a restart of either shard. Depends on T020 (ark-survival-evolved probe package proven).
- [ ] T054 [US2] Register the two new test names from T052/T053 into `test/e2e/buckets.sh`, citing `specs/015-top-steam-game-modules/engine-matrix-resolved.md`'s (T006) bucket classification column: the FiveM test into `bucket_bot_fast`; the ARK cluster test into `bucket_bot_heavy` with a per-test CI-exclusion comment (storage/memory footprint) matching the existing entries' style. Depends on T052, T053.
- [ ] T055 [US2] Audit `storage.mountPath` safety and sizing against `specs/015-top-steam-game-modules/engine-matrix-resolved.md` and `modules/validate.py` Rule 4 (no shadowing) for the 9 lightweight new modules: `modules/fivem/template.yaml`, `modules/team-fortress-2/template.yaml`, `modules/farming-simulator-25/template.yaml`, `modules/euro-truck-simulator-2/template.yaml`, `modules/mount-and-blade-2-bannerlord/template.yaml`, `modules/tmodloader/template.yaml`, `modules/beammp/template.yaml`, `modules/left-4-dead-2/template.yaml`, `modules/the-isle/template.yaml`.
- [ ] T056 [US2] Audit `storage.mountPath` safety, sizing, and cluster/multi-shard wiring for the three remaining heavy new modules: `modules/squad/template.yaml`, `modules/arma-reforger/template.yaml`, `modules/hell-let-loose/template.yaml`.
- [ ] T057 [US2] Audit ARK cluster-travel storage topology consistency between `modules/ark-survival-ascended/template.yaml` (existing) and `modules/ark-survival-evolved/template.yaml` (new) so shared cluster identifiers and volume layouts match.
- [ ] T058 [US2] Audit the Don't Starve Together master/caves multi-shard storage layout in `modules/dont-starve-together/template.yaml` and `modules/dont-starve-together/README.md`.

**Checkpoint**: persistence is verified for every heavy/clustered module and exercised by a real restart e2e test; User Stories 1 and 2 both work independently.

---

## Phase 5: User Story 3 - Interactive Administration, Remote Console (RCON/API), and Operational Actions (Priority: P2)

**Goal**: Moderators can issue live admin commands and trigger a graceful, save-then-stop shutdown appropriate to each engine. SC-006's "100% of modules with remote console interfaces successfully execute administrative commands and graceful save-on-shutdown" is verified here by **sampling** one Source-RCON-family module (`team-fortress-2`) and one Source-family cluster module (`squad`) with dedicated e2e tests (T059/T060); the remaining RCON-capable modules are covered by the protocol-family audits T062-T071, not each by its own RCON e2e test. `capabilities.lifecycle.stop` is documented-but-not-CEL-enforced to require `rcon.protocol != none` (see T005 item 2/OPEN-DECISIONS.md) — `modules/dont-starve-together/template.yaml` ships a working `stop` sequence under `rcon.protocol: none`, so no audit task in this phase may delete a `none`-protocol module's `capabilities.lifecycle.stop` on the strength of the schema's prose alone.

**Independent Test**: Issue an administrative command via the supported control protocol to a running server and observe the response; trigger a stop action and observe the pre-shutdown save sequence (spec.md US3 Independent Test).

- [ ] T059 [P] [US3] Author `test/e2e/teamfortress2_rcon_e2e_test.go`: issue a Source RCON admin command against a running `team-fortress-2` server and verify the response, then trigger stop and verify `modules/team-fortress-2/template.yaml`'s `capabilities.lifecycle.stop` sequence runs before the pod terminates. Depends on T012 (team-fortress-2 probe package proven).
- [ ] T060 [P] [US3] Author `test/e2e/squad_rcon_e2e_test.go`: issue a Source-family RCON admin command against a running `squad` server and verify the response, then trigger stop and verify `modules/squad/template.yaml`'s `capabilities.lifecycle.stop` sequence. Depends on T023 (squad probe package proven).
- [ ] T061 [US3] Register the two new test names from T059/T060 into `test/e2e/buckets.sh`, citing `specs/015-top-steam-game-modules/engine-matrix-resolved.md`'s (T006) bucket classification column: the Team Fortress 2 test into `bucket_bot_fast`; the Squad test into `bucket_bot_heavy` with a CI-exclusion comment. Depends on T059, T060.
- [ ] T062 [US3] Audit `source`-protocol `rcon`/`capabilities.lifecycle.stop`/`capabilities.actions` stanzas against `specs/015-top-steam-game-modules/engine-matrix-resolved.md` for `modules/cs2/template.yaml`, `modules/team-fortress-2/template.yaml`, `modules/left-4-dead-2/template.yaml`, `modules/factorio/template.yaml` (source protocol **with** `consoleMode: pty` — confirm both stanzas coexist correctly, per T005 item 6), `modules/project-zomboid/template.yaml` (source protocol, per T005 item 6).
- [ ] T063 [US3] Audit `source`-protocol `rcon`/`capabilities.lifecycle.stop`/`capabilities.actions` stanzas for `modules/squad/template.yaml`, `modules/the-isle/template.yaml`, `modules/hell-let-loose/template.yaml`.
- [ ] T064 [US3] Audit `source`-protocol `rcon`/`capabilities.lifecycle.stop` cluster consistency (shared `SaveWorld` stop command) across `modules/ark-survival-ascended/template.yaml` and `modules/ark-survival-evolved/template.yaml`.
- [ ] T065 [US3] Audit the `battleye`-protocol `rcon`/`capabilities.lifecycle.stop` stanza in `modules/dayz/template.yaml`. Confirm the documented deliberate absence of a full `capabilities.lifecycle.stop` still holds (BattlEye RCon has a `#shutdown` command, but DayZ's save semantics around it are the caveat the template's header records), or record a change request in `specs/015-top-steam-game-modules/OPEN-DECISIONS.md` for maintainer sign-off — do not add a stop stanza on the strength of the engine matrix alone.
- [ ] T066 [US3] Verify the `websocket`-protocol `rcon`/`capabilities.lifecycle.stop` stanza in `modules/rust/template.yaml` against `specs/015-top-steam-game-modules/engine-matrix-resolved.md` (this is the house-style reference — confirm no drift, do not restructure it).
- [ ] T067 [US3] Audit the `satisfactory`- and `palworld`-protocol `rcon`/`capabilities.lifecycle.stop` stanzas in `modules/satisfactory/template.yaml` and `modules/palworld/template.yaml`.
- [ ] T068 [US3] Audit the `rcon.protocol: none`, no-`capabilities`-block modules `modules/garrys-mod/template.yaml` and `modules/7-days-to-die/template.yaml` (per T005 item 6): confirm the documented deliberate absence of RCON and `capabilities.lifecycle.stop` still holds — garrys-mod has no console reachable at all; 7-days-to-die's LinuxGSM entrypoint (user.sh) already traps SIGINT/SIGTERM and runs `sdtdserver stop` on its own, so the omission is not a gap — or record a change request in OPEN-DECISIONS.md for maintainer sign-off; do not add a stop stanza on the strength of the engine matrix alone. Also record 7-days-to-die's declared telnet port (8081) vs. the engine-matrix-contract's port (8082) as an open item, not something to silently renumber.
- [ ] T069 [US3] Audit `none` + `consoleMode: pty` stdin-console `capabilities.lifecycle.stop` stanzas for the existing modules `modules/terraria/template.yaml` and `modules/dont-starve-together/template.yaml` — both are genuinely `rcon.protocol: none` (unlike factorio/project-zomboid, see T005 item 6); dont-starve-together's ships a working `stop` array despite `rcon.protocol: none` (the OPEN-DECISIONS.md ambiguity) — do not remove it here.
- [ ] T070 [US3] Audit `none` + `consoleMode: pty` stdin-console `capabilities.lifecycle.stop` stanzas for `modules/euro-truck-simulator-2/template.yaml`, `modules/mount-and-blade-2-bannerlord/template.yaml`, `modules/arma-reforger/template.yaml`.
- [ ] T071 [US3] Audit `none` + `consoleMode: pty` `capabilities.lifecycle.stop` stanzas and the FR-013 graceful-idle-on-missing-token diagnostics for `modules/fivem/template.yaml`, `modules/beammp/template.yaml`, `modules/tmodloader/template.yaml`, `modules/farming-simulator-25/template.yaml`. Depends on T010 (upstream-image ruling for fivem/beammp/farming-simulator-25).

**Checkpoint**: every protocol family's admin/lifecycle stanza is verified and two real RCON e2e tests exist; User Stories 1-3 work independently.

---

## Phase 6: User Story 4 - Automated Modding, Frameworks, and Workshop Synchronization (Priority: P2)

**Goal**: Mods, workshop collections, and modding frameworks install and load through declared `capabilities.mods` paths, without hand-edited entrypoints.

**Independent Test**: Deploy a modded framework module with a workshop collection ID or mod list, boot it, and verify the mods are downloaded, mounted, and loaded (spec.md US4 Independent Test).

- [ ] T072 [P] [US4] Author `test/e2e/tmodloader_mods_e2e_test.go`: verify a configured mod is downloaded/mounted/loaded under `modules/tmodloader/template.yaml`'s `capabilities.mods` layout. Depends on T016 (tmodloader probe package proven).
- [ ] T073 [P] [US4] Author `test/e2e/garrysmod_workshop_e2e_test.go`: verify a Steam Workshop collection ID supplied to `modules/garrys-mod` mounts and activates during startup.
- [ ] T074 [US4] Register the two new test names from T072/T073 into `test/e2e/buckets.sh` `bucket_bot_fast`. Depends on T072, T073.
- [ ] T075 [US4] Author `capabilities.mods` Steam Workshop wiring (FR-010) — currently absent — for `modules/dayz/template.yaml`, `modules/squad/template.yaml`, `modules/arma-reforger/template.yaml`. For `modules/garrys-mod/template.yaml`, `modules/7-days-to-die/template.yaml`, and `modules/project-zomboid/template.yaml`: none of the three ships a `capabilities.mods` block today, and garrys-mod's template header states the omission is deliberate ("No capabilities block either: with no console reachable at all…") — flag that adding mods to these three reverses a documented deliberate omission, and get maintainer sign-off in `specs/015-top-steam-game-modules/OPEN-DECISIONS.md` before making the edit.
- [ ] T076 [US4] Audit `capabilities.mods` dedicated resource-directory/plugin-path layouts for the framework modules `modules/tmodloader/template.yaml`, `modules/fivem/template.yaml`, `modules/beammp/template.yaml`.
- [ ] T077 [US4] Audit `capabilities.mods.path` (not `.loaders` — CS2 has no loader, its addons resolve via a fixed `game/csgo/addons` path) in `modules/cs2/template.yaml`, and `capabilities.mods.loaders` (bepinex → bepinex/plugins) in `modules/valheim/template.yaml`, confirming addon paths never shadow `storage.mountPath` per `modules/validate.py` Rule 4, and that valheim's non-empty `spec.versions` satisfies the CEL rule gating `loaders`.

**Checkpoint**: workshop and framework mod paths are verified with real e2e coverage; User Stories 1-4 work independently.

---

## Phase 7: User Story 5 - Automated Health Probing & Protocol Verification (Priority: P3)

**Goal**: Every server's status accurately reflects wire-level readiness (Starting/Ready/Unhealthy), never just "container running." Constitution Principle I's probe-trust rule (a probe must be proven to fail against a dead address and succeed against a real listener before it is trusted) is satisfied by the fail/succeed proof bundled into each Phase 2 probe-package task (T011-T023), not re-proven here.

**Independent Test**: Query a running server via its native protocol and observe correct status transitions between Initializing, Ready, Unhealthy, and Stopped (spec.md US5 Independent Test).

- [ ] T078 [P] [US5] Author `test/e2e/fivem_bot_e2e_test.go`: query the `fivem` server via its probe package and assert the Ready transition and returned player-count metrics. Depends on T011.
- [ ] T079 [P] [US5] Author `test/e2e/teamfortress2_bot_e2e_test.go`: A2S_INFO health probe against `team-fortress-2`. Depends on T012.
- [ ] T080 [P] [US5] Author `test/e2e/farmingsimulator25_bot_e2e_test.go`: GIANTS/HTTP health probe against `farming-simulator-25`. Depends on T013.
- [ ] T081 [P] [US5] Author `test/e2e/eurotrucksimulator2_bot_e2e_test.go`: A2S_INFO health probe against `euro-truck-simulator-2`. Depends on T014.
- [ ] T082 [P] [US5] Author `test/e2e/mountandblade2bannerlord_bot_e2e_test.go`: A2S_INFO health probe against `mount-and-blade-2-bannerlord`. Depends on T015.
- [ ] T083 [P] [US5] Author `test/e2e/tmodloader_bot_e2e_test.go`: custom-TCP (Terraria-family) health probe against `tmodloader`. Depends on T016.
- [ ] T084 [P] [US5] Author `test/e2e/beammp_bot_e2e_test.go`: custom TCP/UDP health probe against `beammp`. Depends on T017.
- [ ] T085 [P] [US5] Author `test/e2e/left4dead2_bot_e2e_test.go`: A2S_INFO health probe against `left-4-dead-2`. Depends on T018.
- [ ] T086 [P] [US5] Author `test/e2e/theisle_bot_e2e_test.go`: A2S_INFO health probe against `the-isle`. Depends on T019.
- [ ] T087 [P] [US5] Author `test/e2e/arksurvivalevolved_bot_e2e_test.go`: A2S_INFO health probe against `ark-survival-evolved`, heavy game. Depends on T020.
- [ ] T088 [P] [US5] Author `test/e2e/armareforger_bot_e2e_test.go`: A2S_INFO health probe against `arma-reforger`, heavy game. Depends on T021.
- [ ] T089 [P] [US5] Author `test/e2e/hellletloose_bot_e2e_test.go`: A2S_INFO health probe against `hell-let-loose`, heavy game. Depends on T022.
- [ ] T090 [P] [US5] Author `test/e2e/squad_bot_e2e_test.go`: A2S_INFO health probe against `squad`, heavy game (distinct file/test from the RCON coverage in T060). Depends on T023.
- [ ] T091 [US5] Register the 13 new test names from T078-T090 into `test/e2e/buckets.sh`, citing `specs/015-top-steam-game-modules/engine-matrix-resolved.md`'s (T006) bucket classification column: the 9 lightweight games (FiveM, Team Fortress 2, Farming Simulator 25, Euro Truck Simulator 2, Bannerlord, tModLoader, BeamMP, Left 4 Dead 2, The Isle) into `bucket_bot_fast`; the 4 heavy games (ARK: Survival Evolved, Arma Reforger, Hell Let Loose, Squad) into `bucket_bot_heavy`, each with a per-test CI-exclusion comment matching the existing entries' style (storage/memory footprint) — note `bucket_bot_heavy` is deliberately never executed by any CI job, matching the existing 9-entry precedent. Depends on T078, T079, T080, T081, T082, T083, T084, T085, T086, T087, T088, T089, T090.
- [ ] T092 [US5] Audit `spec.probes.readiness` as a plain Kubernetes Probe (`tcpSocket`/`httpGet` target, `initialDelaySeconds`, `periodSeconds`, `failureThreshold` — the CRD's `spec.probes` has no A2S/UDP-ping/protocol-aware probe type) across the existing modules `modules/cs2/template.yaml`, `modules/palworld/template.yaml`, `modules/rust/template.yaml`, `modules/dayz/template.yaml`, `modules/garrys-mod/template.yaml`, `modules/ark-survival-ascended/template.yaml`, `modules/valheim/template.yaml` against `specs/015-top-steam-game-modules/engine-matrix-resolved.md`. Wire-protocol correctness (A2S_INFO, query responses) for these games is verified by their existing `test/e2e/internal/<game>/` probe packages, not by this task.
- [ ] T093 [US5] Audit `spec.probes.readiness` as a plain Kubernetes Probe (same scope as T092) across `modules/terraria/template.yaml`, `modules/factorio/template.yaml`, `modules/7-days-to-die/template.yaml`. Wire-protocol correctness (Terraria custom TCP, Factorio custom UDP ping, 7-days-to-die's query protocol) is verified by their existing `test/e2e/internal/<game>/` probe packages, not by this task.
- [ ] T094 [US5] Audit `spec.probes.readiness` as a plain Kubernetes Probe (same scope as T092) for `modules/project-zomboid/template.yaml`, `modules/dont-starve-together/template.yaml`, and `modules/satisfactory/template.yaml`. Wire-protocol correctness (including satisfactory's HTTPS API) is verified by the existing `test/e2e/internal/<game>/` probe packages, not by this task.

**Checkpoint**: all 26 modules have a real protocol-aware health probe (existing 13 already had one; the 13 new modules get one in Phase 2 plus a bot test here), and the CRD's `spec.probes` Kubernetes-Probe configuration is verified separately from wire-protocol correctness. All five user stories are independently functional, and Constitution Principle I's probe-trust rule is satisfied for every new game.

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Documentation (including the CI-gating game-coverage table), version-catalog pinning, validation, and the cross-repo submodule/PR mechanics that land the feature.

- [ ] T095 [P] Add one row per new module (`fivem`, `team-fortress-2`, `farming-simulator-25`, `euro-truck-simulator-2`, `mount-and-blade-2-bannerlord`, `tmodloader`, `beammp`, `left-4-dead-2`, `the-isle`, `ark-survival-evolved`, `arma-reforger`, `hell-let-loose`, `squad`) to `docs/game-coverage.md`'s first table (Module | Game | Status | Depth | Test | Bucket | Last Verified | Blocker | Blocker Class), satisfying `test/e2e/joincoverage.sh` checks 2 (module-in-`modules/`-must-be-listed — the CI job `e2e bucket coverage`, `.github/workflows/ci.yaml:775`, hard-fails without this), 5 (status/depth consistency), 8 (covered-has-bucket), 11 (covered-has-last-verified), 12/13 (blocked-doc must name a concrete artifact — "undocumented", "wire format", "reverse-engineer", …), 14 (out-of-scope-by-design needs an architectural blocker) and 15 (Test+Bucket must match `test/e2e/buckets.sh`). Depends on T054, T061, T074, T091 (the bucket-registration tasks whose Test/Bucket values this table must match).
- [ ] T096 [P] Cross-link the new catalog from `docs/module-authoring.md`, adding the 13 new module names to whatever module inventory list already exists there.
- [ ] T097 Author `spec.versions` (id / displayName / image, exactly one `default: true`) for the 13 new modules (`fivem`, `team-fortress-2`, `farming-simulator-25`, `euro-truck-simulator-2`, `mount-and-blade-2-bannerlord`, `tmodloader`, `beammp`, `left-4-dead-2`, `the-isle`, `ark-survival-evolved`, `arma-reforger`, `hell-let-loose`, `squad`) with `@sha256:` digest-pinned images, satisfying FR-003 and the data-model `VersionCatalog` entity; run `python3 modules/validate.py --pin` (Makefile:311) to re-pin — this is the only task in the file permitted to use `--pin` — and confirm rule 10 (images pinned) and rule 5 (image resolves) are clean. Not `[P]`: rewrites the same `template.yaml` files Phase 3's `[P]` module tasks own. Depends on T026, T029, T031, T032, T034, T037, T038, T040, T042, T046, T047, T048, T049.
- [ ] T098 Run `python3 modules/validate.py` locally in its default, read-only mode (do NOT pass `--pin` — see T097, the only task permitted to). It validates every directory under `modules/` (17 today, 30 after this feature, including `enshrouded`, `minecraft-java`, `nuclear-option`, `v-rising` — outside this feature's 26); fix every reported error and unacknowledged warning in the 26 modules this feature owns until they are 0/0 per SC-002, and leave the other modules unchanged unless a regression is introduced here. Depends on T097.
- [ ] T099 Record, in `specs/015-top-steam-game-modules/OPEN-DECISIONS.md` (from T005), the unresolved numbering mismatch noted at the top of this file — `spec.md`/`plan.md` say `014-top-steam-game-modules`, the folder is `015-top-steam-game-modules`, the real branch is `claude/top-steam-game-modules-x7oyy1` — as a documentation debt for the maintainer to settle, not something this task list resolves unilaterally. Depends on T005 (same file).
- [ ] T100 Commit and push the `modules/` submodule branch created in T002 (including T097's `--pin` re-pin and T098's clean validator run, and T007's `test-validate-py.sh` check now registered in that repo's own CI) so the `gameplane-module` repo's own CI runs its suites. Depends on T002, T007, T097, T098.
- [ ] T101 Push this repo's branch (`claude/top-steam-game-modules-x7oyy1`, containing T005/T006/T099's docs, T095/T096's doc updates, and every Phase 2-7 change) so GitHub Actions runs `make test`/`make lint`/`make cover`/the e2e buckets; do not run any of these locally (CLAUDE.md rule 8) — anchor file `.github/workflows/ci.yaml`. Depends on T095, T099.
- [ ] T102 Open the PR in `gameplane-module` for the branch from T002/T100, referencing this feature's spec folder in the description. Depends on T100. — `modules/`.
- [ ] T103 Once the maintainer merges the `gameplane-module` PR, bump the `modules` submodule pointer in this repo (`git add modules && git commit`) and push it to the branch from T101. Depends on T102. — `modules` (submodule gitlink).
- [ ] T104 Verify the CI run on this repo's branch is green (`gh run watch` / `gh pr checks`, checked against the branch directly — before any PR is opened), including the `e2e bucket coverage` job (which now also validates against `docs/game-coverage.md` per T095) — fix any failures with follow-up commits on the branch rather than force-pushing over history. Depends on T101, T095, T103.
- [ ] T105 Once T104 confirms the branch is green, open this repo's PR and apply labels `type: feature`, `area: modules`, `area: specs`, `area: e2e` via `gh api -X POST repos/ValgulNecron/Gameplane/issues/<n>/labels -f "labels[]=type: feature" -f "labels[]=area: modules" -f "labels[]=area: specs" -f "labels[]=area: e2e"` — `gh pr edit` is broken on this repo per CLAUDE.md rule 14, so this REST call is mandatory, not optional. Open the PR only once CI is green: `dismiss_stale_reviews_on_push` (CLAUDE.md rule 12) drops any approval if a fixup commit lands afterward. Depends on T104. — anchor `specs/015-top-steam-game-modules/`.
- [ ] T106 Once the maintainer approves and merges this repo's PR (T105), create a fresh branch off `master` for the spec-folder rename of `specs/015-top-steam-game-modules/` — the merged feature branch is deleted in T109 and cannot be reused. Depends on T105.
- [ ] T107 On the branch from T106, rename `specs/015-top-steam-game-modules/` to `specs/done_015-top-steam-game-modules/` (`git mv`) and fix every in-repo reference to the old path in the same commit (`grep -rIn "specs/015-top-steam-game-modules" --exclude-dir=.git .`), as a single signed `docs: mark feature 015 complete` commit, per CLAUDE.md rule 16. Depends on T106.
- [ ] T108 Open a PR for the T107 commit and apply labels `type: docs`, `area: specs` via the `gh api` REST route (`gh pr edit` is broken per CLAUDE.md rule 14). Depends on T107.
- [ ] T109 After the maintainer approves and merges the T108 rename PR, delete both branches, remote and local: the original feature branch (`claude/top-steam-game-modules-x7oyy1`) and the rename branch created in T106, per CLAUDE.md rule 12 — anchor `specs/done_015-top-steam-game-modules/tasks.md`. This is the final task in the file. Depends on T108.

**Checkpoint**: game-coverage table and module-authoring catalog documented, version catalogs pinned, validator clean, both repos' CI green, PR merged and labeled, spec folder renamed `done_`, both branches cleaned up.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: no dependencies.
- **Foundational (Phase 2)**: depends on Setup; BLOCKS every user story (Phase 3-7). Includes 13 `[P]` new-game probe packages (T011-T023), each individually depended on by the Phase 4-7 test tasks for that game — see each test task's own "Depends on" line.
- **User Story 1 (Phase 3)**: depends on Foundational only (specifically T006's resolved matrix and T008's specs skeleton; T010's upstream-image ruling for fivem/farming-simulator-25/euro-truck-simulator-2/beammp). No dependency on any other story.
- **User Stories 2-5 (Phases 4-7)**: each depends on Foundational **and** on User Story 1 (Phase 3) having authored the `template.yaml` files they audit and test — this is a file-ownership dependency, not a feature dependency, and it is the reason these phases are numbered after Phase 3 even though US2 shares US1's P1 priority. Each new-game test task additionally depends on that game's Phase 2 probe package. Stories 2-5 remain independently testable from each other (spec.md's per-story Independent Test criteria hold once Phase 3 is done, with SC-005/SC-006 covered by the stated sampling); they run in priority order here (P1 → P2 → P2 → P3) purely to give the earliest checkpoint the most value, not because US3 needs US2 or similar.
- **Polish (Phase 8)**: depends on all desired user stories being complete.

### Within Each User-Story Phase

- E2E test-authoring tasks (new files) are `[P]` — genuinely independent of each other and of the audit tasks, but each new-game test task depends on that game's Phase 2 probe package (T011-T023).
- Bucket-registration tasks are never `[P]` (they all write the single shared `test/e2e/buckets.sh`) and always depend on the test tasks they register.
- Audit/fix tasks that touch a `template.yaml` already authored in Phase 3 are never `[P]` — they are safe by phase ordering (Phase 3 always precedes), but marking them `[P]` against each other would risk two tasks editing the same file if the plan were ever parallelized across phases, so they are sequenced instead.
- Every new e2e test task, and every existing one modelled on, MUST call `t.Parallel()`, use per-test unique resource names, and respect the shared-state guards `ociPushMu` / `ensureResticRepo` (CLAUDE.md e2e conventions; Constitution Principle I). Model on `test/e2e/cs2_bot_e2e_test.go` and `test/e2e/gamebot_helpers_e2e_test.go`; load the `e2e-test-authoring` skill before starting any test-authoring task.

### Parallel Opportunities

- All `[P]` Setup tasks (T002-T004) together.
- All `[P]` Foundational tasks (T005-T009, T011-T023) together, once T001 (submodule init) is done — T010 is not `[P]` (writes the same OPEN-DECISIONS.md as T005; depends on it).
- 25 of the 26 `[P] [US1]` module tasks in Phase 3 (T024-T045, T047-T049) together — each owns its own `modules/<name>/` directory. T046 (ARK: Survival Evolved) is not `[P]`: it depends on T039 (ARK: Survival Ascended) for cluster-travel layout consistency.
- Within each of Phases 4-7, all `[P]`-tagged e2e test tasks together (respecting each one's probe-package dependency from Phase 2).

---

## Parallel Example: User Story 1

```bash
# Any subset of the 25 [P] module tasks can run together — each is confined to
# its own modules/<name>/ directory. T046 (ark-survival-evolved) is excluded:
# it depends on T039 and is not [P].
Task: "T024 [P] [US1] Standardize modules/cs2/"
Task: "T026 [P] [US1] Create modules/fivem/"
Task: "T029 [P] [US1] Create modules/team-fortress-2/"
Task: "T049 [P] [US1] Create modules/squad/"
```

## Parallel Example: User Story 4

```bash
Task: "T072 [P] [US4] test/e2e/tmodloader_mods_e2e_test.go"
Task: "T073 [P] [US4] test/e2e/garrysmod_workshop_e2e_test.go"
```

## Parallel Example: User Story 5

```bash
Task: "T078 [P] [US5] test/e2e/fivem_bot_e2e_test.go"
Task: "T079 [P] [US5] test/e2e/teamfortress2_bot_e2e_test.go"
Task: "T087 [P] [US5] test/e2e/arksurvivalevolved_bot_e2e_test.go"
Task: "T090 [P] [US5] test/e2e/squad_bot_e2e_test.go"
```

---

## Implementation Strategy

### MVP First (Setup + Foundational + User Story 1)

1. Complete Phase 1 (Setup).
2. Complete Phase 2 (Foundational) — CRITICAL, blocks everything else; includes proving all 13 new-game probe packages against a dead address and a real listener.
3. Complete Phase 3 (User Story 1) — all 26 modules deployable.
4. **STOP and VALIDATE**: run `python3 modules/validate.py` locally in its default mode (T098's check, pulled forward), confirm 0 errors; push the branch and let CI's e2e buckets confirm the 13 already-existing bot tests still pass against the standardized templates.
5. This is the deployable MVP — every module in FR-001 exists and boots.

### Incremental Delivery

1. Setup + Foundational → foundation ready, all new-game probes proven.
2. + User Story 1 → all 26 modules deployable (MVP).
3. + User Story 2 → persistence and multi-shard clustering verified (sampled, SC-005).
4. + User Story 3 → RCON/admin and graceful shutdown verified (sampled, SC-006).
5. + User Story 4 → modding/workshop paths verified.
6. + User Story 5 → health probing verified for all 26 (Constitution Principle I fully satisfied).
7. Polish → game-coverage table published, version catalogs pinned, both repos' PRs merged, labeled, branches cleaned, spec folder renamed `done_`.

---

## Notes

- `[P]` tasks touch different files and carry no dependency on another incomplete task — safe to hand to concurrent subagents per CLAUDE.md rule 13's smallest-model-first, fan-out-in-one-Workflow guidance; start every module-authoring, probe-package-authoring, and e2e-test-authoring task at `haiku`, escalating only on demonstrated failure, and review each wave one tier up before merging its fixes.
- `modules/` is a git submodule pointing at `ValgulNecron/gameplane-module` — every `modules/<name>/` edit in Phases 3-7 is committed in that repo, not this one; this repo only ever records the bumped submodule pointer (T103).
- Never run `make test`, `make lint`, `make cover`, `go test`, or `npm test` locally — GitHub Actions is authoritative (CLAUDE.md rule 8). `python3 modules/validate.py` in its default mode (T098) is the sole sanctioned local check; `--pin` mode (T097) rewrites templates and is not a read-only preflight.
- A new e2e test that is not registered in `test/e2e/buckets.sh` fails the `e2e bucket coverage` CI job — every test-authoring task above has a paired registration task; do not skip it. That same CI job also fails on any `modules/` directory missing from `docs/game-coverage.md` (T095) — a module can be schema-valid and still fail CI if this table is stale.
- Heavy games get their e2e test **committed and bucketed**, with an explicit CI-exclusion comment in `test/e2e/buckets.sh` (matching the existing `bucket_bot_heavy` block's per-test justification comments) — never a silent skip, per spec.md Edge Cases and Constitution Principle I. `bucket_bot_heavy` itself is never executed by any CI job today.
- Every new e2e test MUST call `t.Parallel()`, use per-test unique resource names, and respect `ociPushMu` / `ensureResticRepo` (CLAUDE.md e2e conventions; Constitution Principle I). Model on `test/e2e/cs2_bot_e2e_test.go` and `test/e2e/gamebot_helpers_e2e_test.go`; load the `e2e-test-authoring` skill before starting any test-authoring task in this file.
- CRD Go types are not touched by this feature; if any task ever needs to, it must be paired with `make generate && make manifests` in the same task per CLAUDE.md rule 7 — no such task currently exists in this list.

---

## Summary Statistics

- **Total tasks**: 109 (T001-T109)
- **Phase 1 (Setup)**: 4 tasks (T001-T004)
- **Phase 2 (Foundational)**: 19 tasks (T005-T023) — 17 `[P]` (all except T009, T010)
- **Phase 3 (US1)**: 28 tasks (T024-T051) — 25 `[P] [US1]` module tasks (T046 is not `[P]`) plus 2 non-`[P]` `[US1]` security audits (T050, T051)
- **Phase 4 (US2)**: 7 tasks (T052-T058) — 2 `[P]`
- **Phase 5 (US3)**: 13 tasks (T059-T071) — 2 `[P]`
- **Phase 6 (US4)**: 6 tasks (T072-T077) — 2 `[P]`
- **Phase 7 (US5)**: 17 tasks (T078-T094) — 13 `[P]`
- **Phase 8 (Polish)**: 15 tasks (T095-T109) — 2 `[P]`
- **New modules authored**: 13 (`fivem`, `team-fortress-2`, `farming-simulator-25`, `euro-truck-simulator-2`, `mount-and-blade-2-bannerlord`, `tmodloader`, `beammp`, `left-4-dead-2`, `the-isle`, `ark-survival-evolved`, `arma-reforger`, `hell-let-loose`, `squad`)
- **Existing modules verified/updated**: 13 (`cs2`, `palworld`, `rust`, `project-zomboid`, `dayz`, `garrys-mod`, `terraria`, `7-days-to-die`, `ark-survival-ascended`, `factorio`, `dont-starve-together`, `valheim`, `satisfactory`)
- **New `samples/gameserver.yaml` authored**: 7 (`project-zomboid`, `dayz`, `garrys-mod`, `7-days-to-die`, `ark-survival-ascended`, `dont-starve-together`, `satisfactory`)
- **New e2e wire-protocol probe packages authored**: 13 (`test/e2e/internal/<game>/app.go` + `spec.md`, one per new module — T011-T023)
- **New e2e test files authored**: 19 (T052, T053, T059, T060, T072, T073, T078-T090)
