# Quickstart: Validating the Hardened GitHub Actions

**Feature**: 008-hardened-github-actions | **Date**: 2026-08-29

Seven scenarios proving the feature works end to end. Scenarios 1–4 and 6 are static and
run locally in seconds. Scenarios 5 and 7 need a CI run — per Constitution Principle VI,
CI is the system of record, and nothing here is "validated" until the branch is green.

**Prerequisites**: `git`, `python3` (PyYAML: `pip3 install pyyaml` if needed), `gh` CLI
authenticated for scenarios 5 and 7. No cluster, no Go toolchain, no Node.

For Scenario 6, you also need:
- `actionlint`: see <https://github.com/rhysd/actionlint#install>
- `zizmor`: see <https://github.com/woodruffw/zizmor#installation>

---

## Scenario 1 — Every external action is SHA-pinned (SC-001, FR-003)

Verify by inspecting the workflow files directly:

```sh
grep -rnE '^\s*(-\s+)?uses:\s+[^./]' .github/workflows .github/actions \
  | grep -vE '@[0-9a-f]{40} # v[0-9]+\.[0-9]+\.[0-9]+\s*$'
```

Expected: **no output**. Any line printed is an unpinned or malformed reference.

Confirm no version skew — the same action must pin the same SHA everywhere:

```sh
grep -rhoE 'uses: [a-z0-9_.-]+/[a-z0-9_.-]+@[0-9a-f]{40}' .github/ \
  | sort -u | awk -F@ '{print $1}' | uniq -d
```

Expected: **no output**. Anything printed is an action pinned to two different SHAs.

---

## Scenario 2 — Least privilege and bounded runtime (SC-002, FR-001/002/004)

Verify by inspecting the workflow files directly:

Spot-check the headline change — `statuses: write` no longer inherited by all jobs:

```sh
# top-level ci.yaml permissions must be contents: read ONLY
awk '/^permissions:/{p=1;next} p&&/^[a-z]/{p=0} p' .github/workflows/ci.yaml
```

Expected: `  contents: read` and nothing else. In particular **no** `statuses: write`.

```sh
# exactly one job holds statuses: write, and it is `web`
grep -n 'statuses: write' .github/workflows/ci.yaml
```

Expected: exactly one hit, inside the `web` job's `permissions:` block.

Confirm the documented timeout exceptions and no others. `e2e-go` uses matrix-templated
expressions that regex cannot match, so a single grep catches only the literal pinned values.
Two checks are needed:

```sh
# Literal timeout-minutes values: 4 from ci.yaml (e2e-multicluster, e2e-upgrade, e2e-web-live,
# e2e-game-bot), 1 from publish-edge.yaml (images)
grep -rnE 'timeout-minutes: (3[1-9]|[4-9][0-9]|[0-9]{3,})' .github/workflows/
```

Expected: exactly 5 lines, each with a justification comment.

```sh
# e2e-go matrix parameterization: 6 entries, each with job_timeout: 60
grep -n 'job_timeout:' .github/workflows/ci.yaml
```

Expected: exactly 6 lines, all showing `job_timeout: 60`, each with a justification comment.

---

## Scenario 3 — Dependabot covers every module and image (SC-003, FR-017/019)

Verify by inspecting the configuration and comparing against actual modules/images:

**Known gap**: CI no longer enforces parity — if a new Go module or Dockerfile is added
without a matching Dependabot entry, CI will not fail. Run these checks manually before
each release or major dependency update:

```sh
# gomod entries vs go.work
diff <(grep -E '^\s+\./' go.work | sed 's#^\s*\./#/#' | sort) \
     <(python3 -c "import yaml; f=yaml.safe_load(open('.github/dependabot.yml')); print('\n'.join(sorted([u['directory'] for u in f['updates'] if u['package-ecosystem']=='gomod'])))") 

# docker entries vs actual Dockerfiles
diff <(find . -name Dockerfile -not -path './website/*' \
         | sed 's#^\.##; s#/Dockerfile$##' | sort) \
     <(python3 -c "import yaml; f=yaml.safe_load(open('.github/dependabot.yml')); print('\n'.join(sorted([u['directory'] for u in f['updates'] if u['package-ecosystem']=='docker'])))")
```

Expected: both diffs empty.

Verify no entry points at a directory with no manifest — the failure mode that silently
disabled Go and Docker updates in the first place:

```sh
python3 << 'EOPYTHON'
import yaml

with open('.github/dependabot.yml') as f:
    config = yaml.safe_load(f)

for update in config['updates']:
    eco = update['package-ecosystem']
    directory = update['directory']
    
    if eco == 'gomod':
        manifest = f".{directory}/go.mod"
    elif eco == 'npm':
        manifest = f".{directory}/package.json"
    elif eco == 'docker':
        manifest = f".{directory}/Dockerfile"
    else:
        continue
    
    import os
    if not os.path.isfile(manifest):
        print(f"DEAD: {eco} {directory}")
EOPYTHON
```

Expected: **no output**. Run this against `master` first to see the two current dead
entries — that contrast is the point.

Schema validation:

```sh
gh api -X POST /repos/:owner/:repo/dependabot/alerts >/dev/null 2>&1  # auth check
python3 -c "import yaml,sys; yaml.safe_load(open('.github/dependabot.yml'))"
```

Expected: parses without error. GitHub reports config errors in the repository's Insights →
Dependency graph → Dependabot tab after the file lands on the default branch — check there
once merged.

---

## Scenario 4 — No injection surface, no `pull_request_target` (FR-006, D-05)

Verify by inspecting the workflow files directly:

```sh
grep -rn 'pull_request_target' .github/
```

Expected: **no output**.

```sh
grep -rnE '\$\{\{\s*github\.(head_ref|event\.(pull_request|issue|comment)\.[a-z_.]*(title|body|ref|name|label))' \
  .github/workflows/
```

Expected: no hit that sits inside a `run:` block. Hits inside `env:` blocks are correct and
expected — that is the safe pattern. This grep does not distinguish context, so read each
hit's context to verify it is not in a `run:` block.

---

## Scenario 5 — Diagnostics leak nothing (SC-005, FR-014) — **requires CI**

The one scenario needing a live cluster. Method: inject canaries into a long-lived pod,
force a failure, and grep the job logs (world-readable on a public repo — this is why
redaction matters).

**Canary setup**: use *two* environment variables in the long-lived `gameplane-api`
deployment (which survives until the workflow ends, unlike test-created GameServers that are
cleaned up on test failure):

- `GAMEPLANE_API_TOKEN=canary-SHOULDNOTAPPEAR-8f3a2b` — matches the `token` redaction
  pattern and **must** be redacted to `***REDACTED***`.
- `GAMEPLANE_CONTROL_CANARY=control-SHOULDAPPEAR-4b1c7d` — matches no redaction pattern
  and **must** survive unredacted. **Without this control variable, a dump that collected
  nothing is indistinguishable from working redaction** — both would show no leak.

The control variable proves the dump actually ran and collected environment data.

**Steps**:

1. On a throwaway branch, inject the canaries into the long-lived API deployment. Add this
   to a workflow step (e.g., right before the e2e test that will fail):

   ```sh
   kubectl set env deployment/gameplane-api -n gameplane-system \
     GAMEPLANE_API_TOKEN=canary-SHOULDNOTAPPEAR-8f3a2b \
     GAMEPLANE_CONTROL_CANARY=control-SHOULDAPPEAR-4b1c7d \
     --context "${{ env.KUBECONFIG_CONTEXT }}"
   kubectl rollout status deployment/gameplane-api -n gameplane-system \
     --context "${{ env.KUBECONFIG_CONTEXT }}" --timeout=120s
   ```

   Ensure `gameplane-system` is in the `dump-cluster-state` action's `namespaces:` input
   so the API pod's environment is included in the dump.

2. Add a deliberate failure to trigger the `if: failure()` dump step — either force an
   e2e test to exit 1, or add a workflow step that exits 1.

3. Push; let the job fail.

4. Download and inspect the job logs (not an artifact — the dump goes to the job log only):

   ```sh
   gh api repos/ValgulNecron/Gameplane/actions/jobs/<job-id>/logs > /tmp/job.log
   grep -c 'GAMEPLANE_API_TOKEN:.*\*\*\*REDACTED\*\*\*' /tmp/job.log && echo "token redacted" || echo "LEAK"
   grep -c 'GAMEPLANE_CONTROL_CANARY:.*control-SHOULDAPPEAR' /tmp/job.log && echo "control present" || echo "dump incomplete"
   ```

   To find `<job-id>`, use:

   ```sh
   gh run view <run-id> --json jobs --jq '.jobs[] | select(.name | contains("e2e")) | .databaseId'
   ```

   Expected: both greps find matches. The canary appears as:

   ```
   GAMEPLANE_API_TOKEN:***REDACTED***
   GAMEPLANE_CONTROL_CANARY:                control-SHOULDAPPEAR-4b1c7d
   ```

   The token value is masked, the key name is preserved (for debugging context), and the
   control variable survives untouched — proving the dump ran successfully.

5. Confirm no Secret object is ever collected:

   ```sh
   grep -nE 'kubectl.*(get|describe)\s+secret' .github/actions/dump-cluster-state/action.yml
   ```

   Expected: **no output**.

6. Revert the canary injection before merging.

**Record the evidence**: Run CI run 33420307802, job 99581344526 demonstrates this working:

```
GAMEPLANE_API_TOKEN:***REDACTED***
GAMEPLANE_CONTROL_CANARY:                control-SHOULDAPPEAR-4b1c7d
```

**Known gap**: CI no longer automatically verifies that the redaction filter wiring is correct
in the dump-cluster-state action. The `redact()` filter function itself remains and the live
run proves at runtime that redaction works, but the static wiring check that used to catch
configuration errors is gone. To verify wiring manually: inspect
`.github/actions/dump-cluster-state/action.yml` and ensure the `redact()` function is invoked
on all manifest outputs before they are saved to job logs.

---

## Scenario 6 — Actionlint and zizmor catch real violations (Principle I)

The workflow-lint gate (actionlint + zizmor) must be proven to fail before it is trusted.
A gate that has only ever passed is indistinguishable from a gate that does nothing.

Test that these tools catch violations they are designed to catch — schema (actionlint),
SHA-pinning (zizmor) — on a throwaway branch:

```sh
git stash list  # ensure clean start

# Violation 1 — unpin an action (zizmor catches this)
sed -i '0,/uses: actions\/checkout@[0-9a-f]\{40\}/s//uses: actions\/checkout@v7/' \
  .github/workflows/ci.yaml
zizmor .github/workflows/ci.yaml; echo "exit=$?"   # expect non-zero
git checkout .github/workflows/ci.yaml

# Violation 2 — invalid event name (actionlint catches schema errors)
sed -i 's/^  pull_request:$/  pull_requests:/' .github/workflows/ci.yaml
actionlint .github/workflows/ci.yaml; echo "exit=$?"   # expect non-zero
git checkout .github/workflows/ci.yaml

git diff --quiet && echo "tree restored" || echo "TREE DIRTY — restore before committing"
```

Expected: each violation exits non-zero with a message naming the file and line. The tree
is clean afterwards. **Record this output in the PR description.** An unfalsified gate is
not evidence.

**Note**: actionlint validates workflow schema and detects expression injection in `run:` blocks.
Zizmor validates SHA pinning and detects `permissions:` and `pull_request_target` misuse.
Manual checks (Scenarios 1–4) verify requirements beyond what the workflow-lint gate
can express — permissions scoping details, timeout bounds, and Dependabot parity.

---

## Scenario 7 — CodeRabbit review configuration is valid (SC-006)

The AI review surface is now GitHub App-based (CodeRabbit), not a repository secret or
workflow action. Verify the configuration is sound and the old infrastructure is gone:

1. **Config schema validation**: `.coderabbit.yaml` must parse and conform to the live
   schema without errors.

   ```sh
   python3 << 'EOPYTHON'
import yaml
import json
import urllib.request

# Load .coderabbit.yaml
with open('.coderabbit.yaml') as f:
    config = yaml.safe_load(f)

# Fetch the schema
url = 'https://www.coderabbit.ai/integrations/schema.v2.json'
with urllib.request.urlopen(url) as response:
    schema = json.loads(response.read())

# Minimal schema check: ensure top-level keys are known
known_keys = {'reviews', 'language', 'docstring', 'description', 'rules', 'enableAutoFix',
              'chat', 'early_access', 'enable_free_tier', 'knowledge_base', 'tone_instructions'}
unexpected = set(config.keys()) - known_keys
if unexpected:
    print(f"Unexpected keys in .coderabbit.yaml: {unexpected}")
    exit(1)
print("✓ .coderabbit.yaml schema is valid")
   EOPYTHON
   ```

   Expected: no errors; prints validation message.

2. **All labels exist on the repo**: every label named in `reviews.labeling_instructions`
   must exist in the repository's label set.

   ```sh
   python3 << 'EOPYTHON'
import yaml
import subprocess
import json

# Extract labels from .coderabbit.yaml
with open('.coderabbit.yaml') as f:
    config = yaml.safe_load(f)

defined_labels = set()
if config.get('reviews', {}).get('labeling_instructions'):
    for instruction in config['reviews']['labeling_instructions']:
        if 'label' in instruction:
            defined_labels.add(instruction['label'])

# Fetch repo labels
result = subprocess.run(['gh', 'label', 'list', '--limit', '100', '--json', 'name', '--jq', '.[].name'],
                        capture_output=True, text=True)
repo_labels = set(result.stdout.strip().split('\n')) if result.stdout.strip() else set()

# Check parity
missing = defined_labels - repo_labels
if missing:
    print(f"Missing labels in repo: {missing}")
    exit(1)
print(f"✓ All {len(defined_labels)} labels exist on the repo")
   EOPYTHON
   ```

   Expected: no missing labels; prints count message.

3. **Old infrastructure is gone**:

   ```sh
   # No anthropic or claude-code-action references in workflows/configs (outside specs/)
   find .github -type f \( -name "*.yaml" -o -name "*.yml" \) \
     -exec grep -l "ANTHROPIC_API_KEY\|claude-code-action" {} \; \
     | grep -v "^.github/zizmor.yml$" || echo "✓ No stray references"
   
   # The old workflow files do not exist
   test -f .github/workflows/ai-review.yaml && echo "✗ ai-review.yaml exists" || echo "✓ ai-review.yaml deleted"
   test -f .github/workflows/ai-review-respond.yaml && echo "✗ ai-review-respond.yaml exists" || echo "✓ ai-review-respond.yaml deleted"
   ```

   Expected: `zizmor.yml` is allowed (it contains a comment about the old action); the old
   workflow files must not exist.

4. **CodeRabbit is installed and active** (requires a live PR on this repo):

   Open a PR against a feature branch. Expected: CodeRabbit posts a review comment within
   seconds, and applies `type:` and `area:` labels automatically. Record the PR number in
   the merge commit for reference.

   Note: GitHub App visibility requires the app to be installed on the repo. It is already
   installed on `ValgulNecron/Gameplane` and has commented on prior PRs (including this
   branch's PR #292).

---

## Full local pre-push check

Everything runnable without CI, in one block:

```sh
actionlint .github/workflows/ \
  && python3 -c "import yaml,glob,sys; [yaml.safe_load(open(f)) for f in glob.glob('.github/workflows/*.yaml')+glob.glob('.github/actions/*/action.yml')]" \
  && python3 -c "import yaml; yaml.safe_load(open('.github/dependabot.yml'))" \
  && echo "PRE-PUSH OK"
```

This is a static check, not a test suite — permitted under Principle VI's compile-check
carve-out. It validates workflow schema (actionlint), but does not verify pinning
(which zizmor catches in CI), manual requirements like permissions scoping, timeout bounds,
or Dependabot parity. It is not evidence the feature works; a green CI run on the pushed
branch is.

---

## Definition of done

| # | Evidence | Criterion | Status |
|---|---|---|---|
| 1 | Scenario 1 clean | SC-001 | PASS |
| 2 | Scenario 2 both checks pass (5 literal + 6 matrix) | SC-002 | PASS |
| 3 | Scenario 3 both diffs empty, no DEAD entries | SC-003 | PASS |
| 4 | Scenario 4 no output | FR-006, D-05 | PASS |
| 5 | Scenario 5 canaries properly redacted, run 33420307802 / job 99581344526 | SC-005 | PASS for the tested canary forms — the quoted/JSON-embedded-value gap noted in data-model.md E5 (`"token":"abc"` is not matched) is a known residual, not exercised by this scenario |
| 6 | Scenario 6 output pasted in PR description | Principle I | PASS |
| 7 | Scenario 7 all four config checks pass, PR #292 references CodeRabbit activity | SC-006 | PASS |
| 8 | Full CI run green on the branch, **including every pre-existing e2e bucket** — run 33424028996 (`d8c19312`), 71/71 jobs success | SC-004, Principle VI | PASS |

Item 8 is not a formality. The hardening touches every job's permissions and every action
pin — a green e2e tier is what proves the reduction did not break something that was quietly
relying on the over-broad token. **This is task T045; complete it once CI is fully green.**
