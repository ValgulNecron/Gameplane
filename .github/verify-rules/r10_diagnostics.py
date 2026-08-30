"""R10 -- failure diagnostics are collected but never leak credentials.

When a Kind e2e job fails, `dump-cluster-state` writes pod descriptions and
container logs into a step summary and an uploaded artifact. On a public
repository both are readable by anyone who can see the run.

GitHub's own secret masking does not help here. It masks values registered
through `secrets.*` or `add-mask`; the e2e suites bootstrap admin users and mint
session tokens *inside the cluster at test time*, so the runner has never seen
those values and passes them through untouched. That gap is the whole reason
this rule exists.

Two requirements:

  * Secret objects are never collected at all. There is no redaction good enough
    to make `kubectl get secret -o yaml` safe, so it simply must not run.
  * Every stream that is collected passes through the redaction filter before it
    reaches either sink. Redaction happens at emit, not at read: an artifact is
    already published by the time a consumer could filter it.

Contract: specs/008-hardened-github-actions/data-model.md, entity E5
"""

from __future__ import annotations

import re

from _common import Ctx, Violation

RULE_ID = "R10"
DESCRIPTION = "cluster diagnostics are redacted and never dump Secret objects"

ACTION_PATH = ".github/actions/dump-cluster-state/action.yml"

# The shell function every collection step must pipe through.
REDACT_FN = "redact"

SECRET_DUMP_RE = re.compile(
    r"kubectl[^\n|]*\b(get|describe)\b[^\n|]*\bsecrets?\b",
    re.IGNORECASE,
)

# Commands whose output reaches a sink and therefore must be filtered.
COLLECTING_RE = re.compile(
    r"kubectl[^\n|]*\b(logs|describe|get\s+events|events)\b|helm[^\n|]*\bhistory\b",
    re.IGNORECASE,
)


def _find_action(ctx: Ctx):
    for parsed in ctx.actions:
        if parsed.path == ACTION_PATH:
            return parsed
    return None


def _line_of(run_line: int, offset: int) -> int:
    """Calculate absolute line number for a hit inside a run block.

    The run block starts at run_line; the script content begins at
    run_line + 1, so an offset within the script maps to run_line + 1 + offset.
    """
    return run_line + 1 + offset if run_line else 0


def check(ctx: Ctx) -> list[Violation]:
    parsed = _find_action(ctx)
    if parsed is None:
        return [
            Violation(
                ACTION_PATH, 0, RULE_ID, "composite action is missing from the tree"
            )
        ]

    violations: list[Violation] = []

    # -- the redaction helper must exist ----------------------------------
    if not re.search(rf"^\s*{REDACT_FN}\s*\(\)\s*\{{", parsed.text, re.MULTILINE):
        violations.append(
            Violation(
                parsed.path,
                0,
                RULE_ID,
                f"no `{REDACT_FN}()` shell function defined; collected logs would reach "
                "the step summary and artifact unfiltered",
            )
        )

    # -- examine only run: block bodies, never raw YAML lines ----------------
    for parsed_file, run_line, script, job_id, _step in ctx.iter_run_blocks():
        # Only check the dump-cluster-state action
        if parsed_file.path != ACTION_PATH:
            continue

        for offset, source_line in enumerate(script.splitlines()):
            stripped = source_line.strip()
            if stripped.startswith("#"):
                continue

            # -- Secret objects are never collected -------------------------
            if SECRET_DUMP_RE.search(stripped):
                violations.append(
                    Violation(
                        parsed_file.path,
                        _line_of(run_line, offset),
                        RULE_ID,
                        "collects Kubernetes Secret objects; no redaction makes this safe, "
                        "so it must not run at all",
                    )
                )

            # -- every collecting command is filtered ---------------------
            if COLLECTING_RE.search(stripped):
                if f"| {REDACT_FN}" not in stripped and f"|{REDACT_FN}" not in stripped:
                    violations.append(
                        Violation(
                            parsed_file.path,
                            _line_of(run_line, offset),
                            RULE_ID,
                            f"collects cluster output without piping through `{REDACT_FN}`; "
                            "a single unfiltered stream defeats the control",
                        )
                    )

    return violations


def summary(ctx: Ctx) -> str:
    parsed = _find_action(ctx)
    if parsed is None:
        return ""
    collecting = sum(1 for line in parsed.lines if COLLECTING_RE.search(line.strip()))
    return f"{collecting} collection step(s) filtered"
