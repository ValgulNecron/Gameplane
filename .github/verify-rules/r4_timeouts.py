"""R4 -- every job is bounded by an explicit timeout.

The GitHub default is 360 minutes. An unbounded job that hangs on a network
stall does not fail fast; it idles for six hours and then fails. On a public
repository that is both a cost and a queue-starvation problem, and the worst
place to be unbounded is the publish/release workflows, where a stuck
`docker push` sits on a live registry credential.

Ceiling is 30 minutes. Four jobs are permitted above it, each of which must
carry an inline comment saying why -- an exception nobody can explain is an
exception that should not exist.

The matrix case is the one that matters for enforceability: `timeout-minutes:
${{ matrix.job_timeout }}` is an expression, not a literal, so a naive rule sees
"a value is present" and passes. Every job could then evade the ceiling by
routing its timeout through a matrix. This rule resolves the expression back to
the matrix's literal values and checks each one.
"""

from __future__ import annotations

import re

from _common import Ctx, Violation

RULE_ID = "R4"
DESCRIPTION = "every job declares timeout-minutes within the documented ceiling"

CEILING = 30

# (workflow, job) -> maximum permitted. Each entry must also carry an inline
# justification comment in the YAML; see JUSTIFY_RE below.
EXCEPTIONS = {
    (".github/workflows/ci.yaml", "e2e-game-bot"): 50,
    # e2e-go's `operator` matrix leg runs 8-way parallel behind a 20m test
    # timeout; 35 is that plus headroom for cluster boot and image load. The
    # other five legs are 25-30. Keyed per job, so this allows the highest leg.
    (".github/workflows/ci.yaml", "e2e-go"): 35,
    (".github/workflows/images.yaml", "game-images"): 60,
    (".github/workflows/release.yaml", "images"): 45,
    (".github/workflows/publish-edge.yaml", "images"): 45,
}

MATRIX_EXPR_RE = re.compile(r"^\$\{\{\s*matrix\.([\w-]+)\s*\}\}$")
JUSTIFY_RE = re.compile(r"#\s*\S")


def _matrix_values(job: dict, key: str) -> list:
    """Every literal value `matrix.<key>` can take for this job."""
    strategy = job.get("strategy")
    if not isinstance(strategy, dict):
        return []
    matrix = strategy.get("matrix")
    if not isinstance(matrix, dict):
        return []

    values = []
    direct = matrix.get(key)
    if isinstance(direct, list):
        values.extend(direct)
    # `include:` entries can introduce the key for combinations the axes omit.
    include = matrix.get("include")
    if isinstance(include, list):
        for entry in include:
            if isinstance(entry, dict) and key in entry:
                values.append(entry[key])
    return values


def _justified(parsed, line: int) -> bool:
    """True when the timeout line, or the line above it, carries a comment."""
    for candidate in (line, line - 1):
        if JUSTIFY_RE.search(parsed.raw_line(candidate).split("timeout-minutes")[-1]):
            return True
        stripped = parsed.raw_line(candidate).strip()
        if stripped.startswith("#") and len(stripped) > 3:
            return True
    return False


def check(ctx: Ctx) -> list[Violation]:
    violations: list[Violation] = []

    for parsed, job_id, job in ctx.iter_jobs():
        job_line = parsed.jobs.key_line(job_id, 0)

        if "timeout-minutes" not in job:
            violations.append(
                Violation(
                    parsed.path,
                    job_line,
                    RULE_ID,
                    f"job `{job_id}` declares no `timeout-minutes`; it defaults to "
                    "360 minutes",
                )
            )
            continue

        raw_value = job["timeout-minutes"]
        line = job.key_line("timeout-minutes", job_line)
        limit = EXCEPTIONS.get((parsed.path, job_id), CEILING)

        candidates: list = []
        if isinstance(raw_value, str):
            expr = MATRIX_EXPR_RE.match(raw_value.strip())
            if expr:
                candidates = _matrix_values(job, expr.group(1))
                if not candidates:
                    violations.append(
                        Violation(
                            parsed.path,
                            line,
                            RULE_ID,
                            f"job `{job_id}` sets timeout-minutes from "
                            f"`matrix.{expr.group(1)}`, which the matrix never "
                            "defines; the ceiling cannot be checked",
                        )
                    )
                    continue
            else:
                violations.append(
                    Violation(
                        parsed.path,
                        line,
                        RULE_ID,
                        f"job `{job_id}` sets timeout-minutes to the unresolvable "
                        f"expression {raw_value!r}; use a literal or a matrix key",
                    )
                )
                continue
        else:
            candidates = [raw_value]

        for value in candidates:
            try:
                minutes = int(value)
            except (TypeError, ValueError):
                violations.append(
                    Violation(
                        parsed.path,
                        line,
                        RULE_ID,
                        f"job `{job_id}` has a non-numeric timeout-minutes: {value!r}",
                    )
                )
                continue

            if minutes <= 0:
                violations.append(
                    Violation(
                        parsed.path, line, RULE_ID,
                        f"job `{job_id}` has a non-positive timeout-minutes: {minutes}",
                    )
                )
            elif minutes > limit:
                if (parsed.path, job_id) in EXCEPTIONS:
                    violations.append(
                        Violation(
                            parsed.path,
                            line,
                            RULE_ID,
                            f"job `{job_id}` exceeds even its documented allowance "
                            f"({minutes} > {limit})",
                        )
                    )
                else:
                    violations.append(
                        Violation(
                            parsed.path,
                            line,
                            RULE_ID,
                            f"job `{job_id}` has timeout-minutes {minutes}, above the "
                            f"{CEILING}-minute ceiling; add it to EXCEPTIONS in "
                            "r4_timeouts.py with a justification, or lower it",
                        )
                    )

        if (parsed.path, job_id) in EXCEPTIONS and not _justified(parsed, line):
            violations.append(
                Violation(
                    parsed.path,
                    line,
                    RULE_ID,
                    f"job `{job_id}` uses an above-ceiling timeout without an inline "
                    "comment explaining why",
                )
            )

    return violations


def summary(ctx: Ctx) -> str:
    jobs = sum(1 for _ in ctx.iter_jobs())
    return f"{jobs} job(s) bounded, {len(EXCEPTIONS)} documented exception(s)"
