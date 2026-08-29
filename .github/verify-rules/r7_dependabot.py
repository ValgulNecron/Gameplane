"""R7 -- Dependabot covers every module and image, and nothing dead.

The failure this rule exists to prevent has already happened here. The config
declared `gomod: /` and `docker: /`, and the repository root has neither a
`go.mod` (it is a `go.work` workspace, which Dependabot does not traverse) nor a
`Dockerfile`. Dependabot does not warn about a directory with no manifest -- it
silently skips it. So the config looked plausible and monitored nothing: all 14
Go modules and all 12 images went without version updates for the life of the
project.

Two checks follow from that, and the second is the important one:

  * parity   -- the declared directories equal the ones the tree actually has
  * liveness -- every declared directory really contains its ecosystem's manifest

Parity alone would have passed a config listing 14 plausible-looking but wrong
paths. Liveness is what catches a dead entry.

Contract: specs/008-hardened-github-actions/contracts/dependabot-matrix.md
"""

from __future__ import annotations

import os

from _common import Ctx, Violation

RULE_ID = "R7"
DESCRIPTION = "dependabot covers every Go module, Dockerfile, npm and actions dir"

MANIFEST = {
    "gomod": "go.mod",
    "npm": "package.json",
    "docker": "Dockerfile",
}

# FR-021 names this prefix explicitly ("chore(deps): "), so it is enforced.
EXPECTED_PREFIX = "chore(deps)"

# No per-ecosystem limit ceiling is enforced. FR-021 requires that a limit be
# declared; it does not say what it should be, and the Edge Cases section says
# "e.g., max 5-10". An earlier revision enforced gomod=3 -- a number nobody
# approved, and one below the spec's own floor. See OPEN-DECISIONS.md D-B.


def _entries(ctx: Ctx, ecosystem: str) -> list[dict]:
    if not isinstance(ctx.dependabot, dict):
        return []
    updates = ctx.dependabot.get("updates")
    if not isinstance(updates, list):
        return []
    return [
        entry
        for entry in updates
        if isinstance(entry, dict) and entry.get("package-ecosystem") == ecosystem
    ]


def _dirs(entries: list[dict]) -> list[str]:
    dirs: list[str] = []
    for entry in entries:
        directory = entry.get("directory")
        if isinstance(directory, str):
            dirs.append(directory)
        for directory in entry.get("directories") or []:
            if isinstance(directory, str):
                dirs.append(directory)
    return dirs


def check(ctx: Ctx) -> list[Violation]:
    path = ctx.dependabot_path
    violations: list[Violation] = []

    if not isinstance(ctx.dependabot, dict):
        return [Violation(path, 0, RULE_ID, "dependabot.yml is missing or unparseable")]

    updates = ctx.dependabot.get("updates")
    if not isinstance(updates, list) or not updates:
        return [Violation(path, 0, RULE_ID, "dependabot.yml declares no `updates`")]

    # -- parity ----------------------------------------------------------
    for ecosystem, expected in (
        ("gomod", ctx.go_work_modules),
        ("docker", ctx.dockerfile_dirs),
    ):
        declared = sorted(set(_dirs(_entries(ctx, ecosystem))))
        wanted = sorted(set(expected))

        for missing in sorted(set(wanted) - set(declared)):
            violations.append(
                Violation(
                    path,
                    0,
                    RULE_ID,
                    f"{ecosystem}: {missing} exists in the tree but has no dependabot "
                    "entry, so its dependencies are unmonitored",
                )
            )
        for extra in sorted(set(declared) - set(wanted)):
            violations.append(
                Violation(
                    path,
                    0,
                    RULE_ID,
                    f"{ecosystem}: {extra} is declared but the tree has no such "
                    f"{'module' if ecosystem == 'gomod' else 'image'} there",
                )
            )

    npm_entries = _entries(ctx, "npm")
    if not npm_entries:
        violations.append(
            Violation(path, 0, RULE_ID, "no npm entry; /web is unmonitored")
        )
    actions_entries = _entries(ctx, "github-actions")
    if not actions_entries:
        violations.append(
            Violation(
                path,
                0,
                RULE_ID,
                "no github-actions entry; SHA pins will never be upgraded",
            )
        )

    # -- liveness and per-entry hygiene -----------------------------------
    for entry in updates:
        if not isinstance(entry, dict):
            continue
        ecosystem = entry.get("package-ecosystem")
        line = entry.key_line("package-ecosystem", 0) if hasattr(entry, "key_line") else 0

        for directory in _dirs([entry]):
            manifest = MANIFEST.get(ecosystem)
            if manifest:
                full = os.path.join(ctx.repo_root, directory.lstrip("/"), manifest)
                if not os.path.exists(full):
                    violations.append(
                        Violation(
                            path,
                            line,
                            RULE_ID,
                            f"{ecosystem}: {directory} contains no {manifest}; "
                            "dependabot silently ignores this entry",
                        )
                    )
            elif ecosystem == "github-actions" and directory != "/":
                violations.append(
                    Violation(
                        path,
                        line,
                        RULE_ID,
                        f"github-actions entry must use directory '/', not {directory}",
                    )
                )

        groups = entry.get("groups")
        if not isinstance(groups, dict) or not groups:
            violations.append(
                Violation(
                    path,
                    line,
                    RULE_ID,
                    f"{ecosystem} {entry.get('directory')}: no `groups`; "
                    "ungrouped updates flood CI with one PR per dependency",
                )
            )

        if "open-pull-requests-limit" not in entry:
            violations.append(
                Violation(
                    path,
                    line,
                    RULE_ID,
                    f"{ecosystem} {entry.get('directory')}: no "
                    "`open-pull-requests-limit`",
                )
            )
        commit = entry.get("commit-message")
        prefix = commit.get("prefix") if isinstance(commit, dict) else None
        if prefix != EXPECTED_PREFIX:
            violations.append(
                Violation(
                    path,
                    line,
                    RULE_ID,
                    f"{ecosystem} {entry.get('directory')}: commit-message.prefix is "
                    f"{prefix!r}, expected {EXPECTED_PREFIX!r} "
                    "(a trailing ': ' yields a malformed `chore: (deps):` subject)",
                )
            )

    return violations


def summary(ctx: Ctx) -> str:
    updates = ctx.dependabot.get("updates") if isinstance(ctx.dependabot, dict) else []
    counts: dict[str, int] = {}
    for entry in updates or []:
        if isinstance(entry, dict):
            eco = str(entry.get("package-ecosystem"))
            counts[eco] = counts.get(eco, 0) + 1
    rendered = ", ".join(f"{k} {v}" for k, v in sorted(counts.items()))
    return f"{len(updates or [])} entries ({rendered})"
