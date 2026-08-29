"""R9 -- `pull_request_target` is banned repository-wide.

`pull_request_target` runs the workflow definition from the base branch, with
full access to repository secrets *and* a read-write `GITHUB_TOKEN`, on an event
raised by an arbitrary outside contributor. On its own that is merely dangerous.
It becomes remote code execution the moment anyone adds

    - uses: actions/checkout
      with:
        ref: ${{ github.event.pull_request.head.sha }}

because the attacker's code is now executing in a job holding every secret the
repository has. That checkout is also the most natural thing in the world to
add -- the workflow appears not to see the PR's changes without it -- which is
what makes this trigger a trap rather than just a sharp edge.

The safe construction for the same goal is the `workflow_run` split used by
ai-review.yaml: one unprivileged job holds the untrusted code, a second
privileged job holds the capability, and an artifact passes between them.

Rationale: specs/008-hardened-github-actions/research.md D-05
"""

from __future__ import annotations

from _common import Ctx, Violation

RULE_ID = "R9"
DESCRIPTION = "no workflow uses the pull_request_target trigger"

FORBIDDEN = "pull_request_target"


def check(ctx: Ctx) -> list[Violation]:
    violations: list[Violation] = []

    for parsed in ctx.iter_workflows():
        if FORBIDDEN not in parsed.trigger_names():
            continue

        triggers = parsed.triggers
        line = 1
        if isinstance(triggers, dict) and hasattr(triggers, "key_line"):
            line = triggers.key_line(FORBIDDEN, 1)

        violations.append(
            Violation(
                parsed.path,
                line,
                RULE_ID,
                "uses `pull_request_target`, which grants secrets and a write token "
                "on an event any outside contributor can raise; use the workflow_run "
                "split instead (see research.md D-05)",
            )
        )

    return violations


def summary(ctx: Ctx) -> str:
    return f"{len(ctx.workflows)} workflow(s) checked"
