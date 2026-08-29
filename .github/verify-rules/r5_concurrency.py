"""R5 -- push/PR workflows cancel superseded runs.

Without a concurrency group, three pushes in five minutes leave three full CI
runs in flight, two of them testing commits nobody will ever look at again. On
this repository a run boots several Kind clusters, so the waste is measured in
runner-hours rather than minutes.

Exempt: tag-only and workflow_dispatch-only workflows. Cancelling a release
mid-publish would leave a half-pushed registry, which is strictly worse than
letting a superseded run finish.
"""

from __future__ import annotations

from _common import Ctx, Violation

RULE_ID = "R5"
DESCRIPTION = "push/pull_request workflows declare a concurrency group"

# Triggers where a newer run genuinely supersedes an older one.
CANCELLABLE_TRIGGERS = {"push", "pull_request"}


def _pushes_tags_only(parsed) -> bool:
    """True when `on.push` is restricted to tags -- a release, not a branch build."""
    triggers = parsed.triggers
    if not isinstance(triggers, dict):
        return False
    push = triggers.get("push")
    if not isinstance(push, dict):
        return False
    return "tags" in push and "branches" not in push


def check(ctx: Ctx) -> list[Violation]:
    violations: list[Violation] = []

    for parsed in ctx.iter_workflows():
        names = set(parsed.trigger_names())
        if not names & CANCELLABLE_TRIGGERS:
            continue
        if names & CANCELLABLE_TRIGGERS == {"push"} and _pushes_tags_only(parsed):
            continue

        doc = parsed.doc
        if not isinstance(doc, dict) or "concurrency" not in doc:
            violations.append(
                Violation(
                    parsed.path,
                    1,
                    RULE_ID,
                    "workflow triggers on "
                    f"{', '.join(sorted(names & CANCELLABLE_TRIGGERS))} but declares "
                    "no `concurrency` group; superseded runs will not be cancelled",
                )
            )
            continue

        concurrency = doc["concurrency"]
        line = doc.key_line("concurrency", 1)

        if isinstance(concurrency, str):
            violations.append(
                Violation(
                    parsed.path,
                    line,
                    RULE_ID,
                    "`concurrency` is a bare string, so `cancel-in-progress` is off; "
                    "use the mapping form",
                )
            )
            continue

        if not isinstance(concurrency, dict):
            violations.append(
                Violation(parsed.path, line, RULE_ID, "`concurrency` is malformed")
            )
            continue

        if "group" not in concurrency:
            violations.append(
                Violation(
                    parsed.path, line, RULE_ID, "`concurrency` declares no `group`"
                )
            )
        if "cancel-in-progress" not in concurrency:
            violations.append(
                Violation(
                    parsed.path,
                    line,
                    RULE_ID,
                    "`concurrency` declares no `cancel-in-progress`; it defaults to "
                    "false, so superseded runs still consume runners",
                )
            )

    return violations


def summary(ctx: Ctx) -> str:
    gated = sum(
        1
        for parsed in ctx.iter_workflows()
        if set(parsed.trigger_names()) & CANCELLABLE_TRIGGERS
    )
    return f"{gated} of {len(ctx.workflows)} workflow(s) in scope"
