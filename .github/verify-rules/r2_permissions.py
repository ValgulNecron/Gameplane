"""R2 + R3 -- least-privilege tokens.

R2: every workflow declares a top-level `permissions` block.
R3: every job declares its own `permissions` block.

R3 has no allowlist and no exemption, deliberately. Writing the block even when
it is only `contents: read` is what makes the rule enforceable -- the moment
"jobs that obviously need nothing" become exempt, the rule needs a list of which
jobs those are, and that list is what rots.

The reason this matters concretely: an inherited scope is available to every
`run:` step, every dependency the job downloads, and every test binary it
executes. ci.yaml granted `statuses: write` at the top level and all 26 jobs
inherited it -- including the six Kind e2e jobs that build and run PR-authored
test code. Exactly one step ever used it.

Contract: specs/008-hardened-github-actions/contracts/permissions-matrix.md
"""

from __future__ import annotations

from _common import Ctx, Violation

RULE_ID = "R2"
DESCRIPTION = "workflow and job permissions declared explicitly (R2 + R3)"

# Scopes a workflow may declare at the top level. Anything beyond these has to
# be scoped to the single job that needs it, where it is reviewable.
ALLOWED_TOP_LEVEL = {"contents", "packages"}


def check(ctx: Ctx) -> list[Violation]:
    violations: list[Violation] = []

    for parsed in ctx.iter_workflows():
        doc = parsed.doc
        if not isinstance(doc, dict):
            continue

        # -- R2: top-level permissions present ---------------------------
        if "permissions" not in doc:
            violations.append(
                Violation(
                    parsed.path,
                    1,
                    "R2",
                    "workflow declares no top-level `permissions`; "
                    "add one (floor: `contents: read`)",
                )
            )
        else:
            perms = doc["permissions"]
            line = doc.key_line("permissions", 1)
            if isinstance(perms, dict):
                for scope, level in perms.items():
                    if scope not in ALLOWED_TOP_LEVEL and level != "none":
                        violations.append(
                            Violation(
                                parsed.path,
                                line,
                                "R2",
                                f"top-level grant `{scope}: {level}` is inherited by "
                                "every job; move it to the job that needs it",
                            )
                        )
                if perms.get("contents") == "write":
                    violations.append(
                        Violation(
                            parsed.path,
                            line,
                            "R2",
                            "`contents: write` at the top level is inherited by every "
                            "job; scope it to the job that writes to the repository",
                        )
                    )

        # -- R3: every job declares its own -------------------------------
        for job_id, job in parsed.jobs.items():
            if not isinstance(job, dict):
                continue
            # A job that only calls a reusable workflow inherits differently and
            # is out of scope for this repo; none exist today.
            if "uses" in job and "steps" not in job:
                continue
            if "permissions" not in job:
                violations.append(
                    Violation(
                        parsed.path,
                        parsed.jobs.key_line(job_id, 0),
                        "R3",
                        f"job `{job_id}` declares no `permissions`; it silently "
                        "inherits the workflow-level token",
                    )
                )

    return violations


def summary(ctx: Ctx) -> str:
    jobs = sum(1 for _ in ctx.iter_jobs())
    return f"{len(ctx.workflows)} workflow(s), {jobs} job(s)"
