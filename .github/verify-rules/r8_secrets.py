"""R8 -- signing and API secrets stay in the workflows entitled to them.

Signing keys belong to the publish and release path. A test workflow that can
read `COSIGN_PRIVATE_KEY` lets any PR that can influence a test step sign an
artifact as the project.

The AI reviewer's key has a tighter rule still: it must be absent from the
`collect` job specifically. `collect` is the job that checks out the pull
request's code, so it is the one job in the repository that runs untrusted input
by design. It must have nothing worth stealing.

Contract: specs/008-hardened-github-actions/contracts/permissions-matrix.md
"""

from __future__ import annotations

import re

from _common import Ctx, Violation

RULE_ID = "R8"
DESCRIPTION = "signing and API secrets confined to entitled workflows"

PUBLISHING = {
    ".github/workflows/images.yaml",
    ".github/workflows/publish-edge.yaml",
    ".github/workflows/release.yaml",
    ".github/workflows/republish-modules.yaml",
}

# secret name -> set of workflow paths permitted to reference it
CONFINED = {
    "COSIGN_PRIVATE_KEY": PUBLISHING,
    "COSIGN_PASSWORD": PUBLISHING,
    "REGISTRY_PASSWORD": PUBLISHING,
    "REGISTRY_USERNAME": PUBLISHING,
    "GHCR_TOKEN": PUBLISHING,
    "DOCKERHUB_TOKEN": PUBLISHING,
    "ANTHROPIC_API_KEY": {".github/workflows/ai-review.yaml"},
}

SECRET_REF_RE = re.compile(r"secrets\.([A-Z0-9_]+)")

# The AI reviewer's untrusted half. It checks out PR code, so it holds nothing.
UNTRUSTED_JOBS = {(".github/workflows/ai-review.yaml", "collect")}


def check(ctx: Ctx) -> list[Violation]:
    violations: list[Violation] = []

    for parsed in ctx.iter_all():
        for lineno, raw in enumerate(parsed.lines, start=1):
            for name in SECRET_REF_RE.findall(raw):
                permitted = CONFINED.get(name)
                if permitted is not None and parsed.path not in permitted:
                    allowed = ", ".join(sorted(permitted))
                    violations.append(
                        Violation(
                            parsed.path,
                            lineno,
                            RULE_ID,
                            f"references secrets.{name}, which is confined to: {allowed}",
                        )
                    )

    # The per-job rule for the untrusted AI job.
    for parsed, job_id, job in ctx.iter_jobs():
        if (parsed.path, job_id) not in UNTRUSTED_JOBS:
            continue
        start = parsed.jobs.key_line(job_id, 0)
        end = _job_end(parsed, start)
        for lineno in range(start, end + 1):
            raw = parsed.raw_line(lineno)
            for name in SECRET_REF_RE.findall(raw):
                if name not in ("GITHUB_TOKEN",):
                    violations.append(
                        Violation(
                            parsed.path,
                            lineno,
                            RULE_ID,
                            f"job `{job_id}` runs checked-out pull-request code and "
                            f"must hold no secrets, but references secrets.{name}",
                        )
                    )

    return violations


def _job_end(parsed, start: int) -> int:
    """Last line of the job whose key sits on `start`.

    Jobs are keyed at a fixed indent under `jobs:`, so the next line at that same
    indent ends the block.
    """
    if not start:
        return 0
    indent = len(parsed.raw_line(start)) - len(parsed.raw_line(start).lstrip())
    for lineno in range(start + 1, len(parsed.lines) + 1):
        raw = parsed.raw_line(lineno)
        if not raw.strip() or raw.lstrip().startswith("#"):
            continue
        current = len(raw) - len(raw.lstrip())
        if current <= indent:
            return lineno - 1
    return len(parsed.lines)


def summary(ctx: Ctx) -> str:
    return f"{len(CONFINED)} confined secret name(s)"
