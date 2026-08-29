"""Shared context and helpers for the workflow verification rules.

RULE MODULE CONTRACT
--------------------
Every sibling module named ``r<n>_*.py`` MUST expose exactly three names:

    RULE_ID     str   -- e.g. "R1". Used for ``workflows-verify.sh --rule R1``.
    DESCRIPTION str   -- one line, printed next to the pass/fail result.
    check(ctx)  fn    -- takes a Ctx, returns list[Violation]. Empty list = pass.

``check`` MUST NOT print, exit, or mutate ``ctx``; the dispatcher owns all output
and the context is shared across every rule in a single run.

The ``Ctx`` is built once per run and carries the parsed workflows, the parsed
composite actions, the parsed ``dependabot.yml``, the ``go.work`` module list and
the Dockerfile directory list. Rules read; they never re-parse.

WHY LINE NUMBERS ARE THREADED THROUGH
-------------------------------------
A violation the reader cannot locate is a violation they will not fix. PyYAML
discards position information, so ``LineLoader`` below re-attaches it: every
mapping remembers the source line each of its keys came from. Rules that need to
inspect syntax YAML throws away -- a trailing ``# v1.2.3`` comment on a ``uses:``
line, for instance -- combine the two: the parse proves the construct is real
(not a comment or a heredoc), and ``ParsedFile.raw_line()`` supplies the text.
"""

from __future__ import annotations

import os
import re
import subprocess
import sys
from dataclasses import dataclass, field
from typing import Any, Iterator

try:
    import yaml
except ImportError:  # pragma: no cover - environment problem, not a rule failure
    sys.stderr.write(
        "workflows-verify: PyYAML is required but not installed.\n"
        "  Debian/Ubuntu: sudo apt-get install -y python3-yaml\n"
        "  pip:           python3 -m pip install pyyaml\n"
        "GitHub-hosted runners ship it preinstalled.\n"
    )
    raise SystemExit(2)


# --------------------------------------------------------------------------
# Violations
# --------------------------------------------------------------------------


@dataclass(frozen=True)
class Violation:
    """One rule failure, anchored to a source location."""

    file: str  # repo-relative
    line: int  # 1-indexed; 0 when the finding is file-wide
    rule: str
    message: str

    def render(self) -> str:
        where = f"{self.file}:{self.line}" if self.line else self.file
        return f"  {where}: [{self.rule}] {self.message}"


# --------------------------------------------------------------------------
# YAML loading with line numbers
# --------------------------------------------------------------------------


class LineDict(dict):
    """A mapping that remembers the source line of each of its keys."""

    __slots__ = ("key_lines",)

    def key_line(self, key: Any, default: int = 0) -> int:
        return getattr(self, "key_lines", {}).get(key, default)


class LineLoader(yaml.SafeLoader):
    """SafeLoader that produces LineDict for every mapping.

    Deliberately derived from ``SafeLoader``, and the only constructor it
    overrides is the one for plain mappings. No tag can construct an arbitrary
    Python type, so ``yaml.load(..., Loader=LineLoader)`` carries exactly the
    guarantees of ``yaml.safe_load`` -- it just keeps the line numbers that
    ``safe_load`` throws away. Never widen this to ``yaml.Loader``.
    """


def _construct_mapping(loader: LineLoader, node: yaml.MappingNode) -> LineDict:
    loader.flatten_mapping(node)
    result = LineDict()
    result.key_lines = {}
    for key_node, value_node in node.value:
        key = loader.construct_object(key_node, deep=True)
        value = loader.construct_object(value_node, deep=True)
        result[key] = value
        try:
            result.key_lines[key] = key_node.start_mark.line + 1
        except TypeError:  # unhashable key; vanishingly rare in Actions YAML
            pass
    return result


LineLoader.add_constructor(
    yaml.resolver.BaseResolver.DEFAULT_MAPPING_TAG, _construct_mapping
)


def load_yaml(path: str) -> Any:
    with open(path, "r", encoding="utf-8") as handle:
        return yaml.load(handle, Loader=LineLoader)


# --------------------------------------------------------------------------
# Parsed files
# --------------------------------------------------------------------------


@dataclass
class ParsedFile:
    """A workflow or composite action: its parse tree plus its raw text."""

    path: str  # repo-relative, e.g. ".github/workflows/ci.yaml"
    doc: Any  # parsed document (LineDict at the top level)
    text: str
    lines: list[str] = field(default_factory=list)

    def raw_line(self, lineno: int) -> str:
        """1-indexed source line, or '' when out of range."""
        if 1 <= lineno <= len(self.lines):
            return self.lines[lineno - 1]
        return ""

    # -- convenience accessors -------------------------------------------

    @property
    def jobs(self) -> LineDict:
        jobs = self.doc.get("jobs") if isinstance(self.doc, dict) else None
        return jobs if isinstance(jobs, dict) else LineDict()

    @property
    def triggers(self) -> Any:
        """The `on:` block.

        YAML 1.1 resolves a bare ``on`` key to the boolean True, so a plain
        ``doc["on"]`` silently misses every workflow trigger in the repository.
        Both spellings are checked here so no rule has to remember this.
        """
        if not isinstance(self.doc, dict):
            return None
        if "on" in self.doc:
            return self.doc["on"]
        return self.doc.get(True)

    def trigger_names(self) -> list[str]:
        """Trigger names as a flat list, whatever form `on:` takes."""
        raw = self.triggers
        if raw is None:
            return []
        if isinstance(raw, str):
            return [raw]
        if isinstance(raw, list):
            return [str(item) for item in raw]
        if isinstance(raw, dict):
            return [("on" if key is True else str(key)) for key in raw.keys()]
        return []


# --------------------------------------------------------------------------
# Context
# --------------------------------------------------------------------------


@dataclass
class Ctx:
    repo_root: str
    workflows: list[ParsedFile]
    actions: list[ParsedFile]
    dependabot: Any
    dependabot_path: str
    go_work_modules: list[str]  # e.g. ["/agent", "/api", ...]
    dockerfile_dirs: list[str]  # e.g. ["/agent", "/api", ...]

    # -- iterators consumed by rules --------------------------------------

    def iter_workflows(self) -> Iterator[ParsedFile]:
        yield from self.workflows

    def iter_all(self) -> Iterator[ParsedFile]:
        """Workflows and composite actions together."""
        yield from self.workflows
        yield from self.actions

    def iter_jobs(self) -> Iterator[tuple[ParsedFile, str, Any]]:
        for parsed in self.workflows:
            jobs = parsed.jobs
            for job_id, job in jobs.items():
                if isinstance(job, dict):
                    yield parsed, str(job_id), job

    def iter_steps(self) -> Iterator[tuple[ParsedFile, str, Any]]:
        """Every step in every job, plus composite-action steps.

        The job id is ``"<composite>"`` for composite-action steps, which have
        no enclosing job.
        """
        for parsed, job_id, job in self.iter_jobs():
            for step in job.get("steps") or []:
                if isinstance(step, dict):
                    yield parsed, job_id, step
        for parsed in self.actions:
            runs = parsed.doc.get("runs") if isinstance(parsed.doc, dict) else None
            if isinstance(runs, dict):
                for step in runs.get("steps") or []:
                    if isinstance(step, dict):
                        yield parsed, "<composite>", step

    def iter_uses(self) -> Iterator[tuple[ParsedFile, int, str]]:
        """Every `uses:` value, with the line it sits on."""
        for parsed, _job_id, step in self.iter_steps():
            if "uses" in step:
                value = step["uses"]
                if isinstance(value, str):
                    yield parsed, step.key_line("uses") if isinstance(
                        step, LineDict
                    ) else 0, value

    def iter_run_blocks(self) -> Iterator[tuple[ParsedFile, int, str, str, Any]]:
        """Every `run:` script body.

        Yields (parsed_file, line, script, job_id, step). The step is included so
        a rule can inspect the sibling ``env:`` block -- which is what separates
        a safe interpolation from an injectable one.
        """
        for parsed, job_id, step in self.iter_steps():
            if "run" in step:
                script = step["run"]
                if isinstance(script, str):
                    line = step.key_line("run") if isinstance(step, LineDict) else 0
                    yield parsed, line, script, job_id, step


# --------------------------------------------------------------------------
# Context construction
# --------------------------------------------------------------------------


_GO_WORK_MODULE_RE = re.compile(r"^\s*\./(\S+)")


def _read_go_work(repo_root: str) -> list[str]:
    """Module directories from go.work, as '/'-prefixed repo-relative paths."""
    path = os.path.join(repo_root, "go.work")
    if not os.path.exists(path):
        return []
    modules: list[str] = []
    in_use_block = False
    with open(path, "r", encoding="utf-8") as handle:
        for raw in handle:
            line = raw.split("//", 1)[0].rstrip()
            if not line.strip():
                continue
            if re.match(r"^use\s*\($", line.strip()):
                in_use_block = True
                continue
            if in_use_block and line.strip() == ")":
                in_use_block = False
                continue
            single = re.match(r"^use\s+\./(\S+)", line.strip())
            if single:
                modules.append("/" + single.group(1))
                continue
            if in_use_block:
                match = _GO_WORK_MODULE_RE.match(line)
                if match:
                    modules.append("/" + match.group(1))
    return sorted(set(modules))


def _find_dockerfile_dirs(repo_root: str) -> list[str]:
    """Directories containing a Dockerfile, excluding the website submodule.

    Uses ``git ls-files`` so untracked scratch files and submodule contents
    cannot alter the result -- the same set a reviewer sees in the diff.
    """
    try:
        out = subprocess.run(
            ["git", "-C", repo_root, "ls-files", "*Dockerfile", "Dockerfile"],
            capture_output=True,
            text=True,
            check=True,
        ).stdout
        paths = [p for p in out.splitlines() if os.path.basename(p) == "Dockerfile"]
    except (subprocess.CalledProcessError, FileNotFoundError):
        paths = []
        for dirpath, dirnames, filenames in os.walk(repo_root):
            dirnames[:] = [
                d for d in dirnames if d not in (".git", "node_modules", "website")
            ]
            if "Dockerfile" in filenames:
                paths.append(os.path.relpath(os.path.join(dirpath, "Dockerfile"), repo_root))

    dirs = set()
    for path in paths:
        if path.startswith("website/"):
            continue
        parent = os.path.dirname(path)
        dirs.add("/" + parent if parent else "/")
    return sorted(dirs)


def _parse_files(repo_root: str, rel_paths: list[str]) -> list[ParsedFile]:
    parsed_files: list[ParsedFile] = []
    for rel in sorted(rel_paths):
        full = os.path.join(repo_root, rel)
        with open(full, "r", encoding="utf-8") as handle:
            text = handle.read()
        parsed_files.append(
            ParsedFile(
                path=rel,
                doc=yaml.load(text, Loader=LineLoader),
                text=text,
                lines=text.splitlines(),
            )
        )
    return parsed_files


def find_repo_root() -> str:
    here = os.path.dirname(os.path.abspath(__file__))
    return os.path.abspath(os.path.join(here, "..", ".."))


def build_ctx(repo_root: str | None = None) -> Ctx:
    root = repo_root or find_repo_root()

    workflow_dir = os.path.join(root, ".github", "workflows")
    workflow_paths = [
        os.path.join(".github", "workflows", name)
        for name in os.listdir(workflow_dir)
        if name.endswith((".yaml", ".yml"))
    ] if os.path.isdir(workflow_dir) else []

    action_dir = os.path.join(root, ".github", "actions")
    action_paths: list[str] = []
    if os.path.isdir(action_dir):
        for name in os.listdir(action_dir):
            for candidate in ("action.yml", "action.yaml"):
                rel = os.path.join(".github", "actions", name, candidate)
                if os.path.exists(os.path.join(root, rel)):
                    action_paths.append(rel)

    dependabot_rel = os.path.join(".github", "dependabot.yml")
    dependabot_full = os.path.join(root, dependabot_rel)
    dependabot = load_yaml(dependabot_full) if os.path.exists(dependabot_full) else None

    return Ctx(
        repo_root=root,
        workflows=_parse_files(root, workflow_paths),
        actions=_parse_files(root, action_paths),
        dependabot=dependabot,
        dependabot_path=dependabot_rel,
        go_work_modules=_read_go_work(root),
        dockerfile_dirs=_find_dockerfile_dirs(root),
    )


# --------------------------------------------------------------------------
# Shared matchers
# --------------------------------------------------------------------------

# A `uses:` value that names a published action (owner/repo[/path]@ref) rather
# than a local composite action (./.github/...) or a container image (docker://).
EXTERNAL_USES_RE = re.compile(r"^(?P<repo>[\w.-]+/[\w.-]+)(?P<path>(?:/[\w./-]+)?)@(?P<ref>.+)$")

# The required pin form: 40 hex chars followed by a `# vX.Y.Z` comment.
SHA_RE = re.compile(r"^[0-9a-f]{40}$")
PIN_COMMENT_RE = re.compile(r"#\s*(v\d+\.\d+\.\d+(?:-[\w.]+)?)\s*$")


def is_local_uses(value: str) -> bool:
    return value.startswith("./") or value.startswith("docker://")
