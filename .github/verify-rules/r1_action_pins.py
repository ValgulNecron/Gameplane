"""R1 -- every external action is pinned to an immutable commit SHA.

A tag is a mutable pointer. Anyone with push access to an action's repository
can re-point `v4` at a malicious commit and every consumer picks it up on the
next run, silently. A SHA cannot be re-pointed.

The trailing `# vX.Y.Z` comment is not decoration: it is the exact form
Dependabot's github-actions ecosystem parses and rewrites when it proposes an
upgrade. Without it the pin is frozen rather than maintained, so the comment is
checked as strictly as the SHA itself.

Contract: specs/008-hardened-github-actions/contracts/action-pins.md
"""

from __future__ import annotations

from _common import (
    EXTERNAL_USES_RE,
    PIN_COMMENT_RE,
    SHA_RE,
    Ctx,
    Violation,
    is_local_uses,
)

RULE_ID = "R1"
DESCRIPTION = "external actions pinned to 40-char SHAs with a version comment"


def check(ctx: Ctx) -> list[Violation]:
    violations: list[Violation] = []
    # repo -> {sha: first "file:line" that pinned it}, for the skew check below.
    seen: dict[str, dict[str, str]] = {}

    for parsed, line, uses in ctx.iter_uses():
        if is_local_uses(uses):
            continue

        match = EXTERNAL_USES_RE.match(uses)
        if not match:
            violations.append(
                Violation(
                    parsed.path,
                    line,
                    RULE_ID,
                    f"unrecognized action reference: {uses!r}",
                )
            )
            continue

        repo = match.group("repo")
        ref = match.group("ref")

        if not SHA_RE.match(ref):
            violations.append(
                Violation(
                    parsed.path,
                    line,
                    RULE_ID,
                    f"{repo} is pinned to the mutable ref {ref!r}; "
                    "use a 40-character commit SHA with a `# vX.Y.Z` comment",
                )
            )
            continue

        # The SHA is real, so the parse has done its job. The version comment
        # lives in syntax YAML discards, so read it back off the raw line.
        raw = parsed.raw_line(line)
        if not PIN_COMMENT_RE.search(raw):
            violations.append(
                Violation(
                    parsed.path,
                    line,
                    RULE_ID,
                    f"{repo} is pinned but carries no `# vX.Y.Z` comment; "
                    "Dependabot needs it to propose upgrades",
                )
            )

        where = f"{parsed.path}:{line}"
        pins = seen.setdefault(repo, {})
        pins[ref] = pins.get(ref, where)

    for repo, pins in sorted(seen.items()):
        if len(pins) > 1:
            detail = ", ".join(
                f"{sha[:12]}… at {where}" for sha, where in sorted(pins.items())
            )
            first_where = sorted(pins.values())[0]
            path, _, lineno = first_where.rpartition(":")
            violations.append(
                Violation(
                    path,
                    int(lineno) if lineno.isdigit() else 0,
                    RULE_ID,
                    f"{repo} is pinned to {len(pins)} different SHAs ({detail}); "
                    "every call site must agree",
                )
            )

    return violations


def summary(ctx: Ctx) -> str:
    total = sum(1 for _p, _l, u in ctx.iter_uses() if not is_local_uses(u))
    repos = {
        EXTERNAL_USES_RE.match(u).group("repo")
        for _p, _l, u in ctx.iter_uses()
        if not is_local_uses(u) and EXTERNAL_USES_RE.match(u)
    }
    return f"{total} call site(s) across {len(repos)} distinct action(s)"
