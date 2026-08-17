# Joincoverage Fixtures

This directory contains 16 test fixtures for the game protocol E2E coverage verifier (`test/e2e/joincoverage.sh`). Each fixture is a self-contained test case that violates exactly one verifier rule.

## Principle: One Fixture Per Check

A verifier that has never failed is not a verifier; it is a rubber stamp. **A gate never proven to fail is not a gate.**

Each of the 16 hard checks (1–16) in the verifier contract has exactly one corresponding fixture in this directory. The fixture:

1. **Violates exactly one rule** — it is valid in all other respects.
2. **Causes the verifier to exit 1** — the failure is hard, not a warning.
3. **Is minimal** — it includes only the minimum state needed to trigger its check.
4. **Tests a real failure**, not just the happy path — the verifier cannot be trusted to work until it has been proven to fail correctly.

If a verifier change removes a check or breaks it silently, the corresponding fixture stops failing, and an integration test catches the regression.

## Fixture Structure

Each fixture directory is a **miniature repository root** containing:

- **`README.md`**: Documents the check number, expected error message, and why this fixture triggers that specific check.
- **`docs/game-coverage.md`**: A malformed or inconsistent coverage table (usually broken in only one way).
- **`modules/`**: A minimal directory structure with empty module directories (one per line in the coverage table, with special exceptions noted below).
- **`buckets.sh`** (optional): A minimal buckets.sh implementation for fixtures that need it (e.g., check 15).

The verifier is pointed at each fixture directory root using the `--root <dir>` override and reads `docs/game-coverage.md` relative to that root.

## The Fixtures

| Fixture | Check | Violation |
|---------|-------|-----------|
| `case-uninitialized-submodule/` | 1 | modules/ is empty or uninitialized |
| `case-missing-module/` | 2 | module in modules/ but not listed in coverage |
| `case-duplicate-module/` | 3 | module listed twice in coverage |
| `case-stray-module/` | 4 | module listed in coverage but not in modules/ |
| `case-covered-with-query-depth/` | 5 | covered-in-ci with QUERY depth (should be JOINED) |
| `case-covered-without-test/` | 6 | covered-in-ci with empty Test column |
| `case-invalid-test-name/` | 7 | test name not found in test/e2e/*_test.go |
| `case-covered-without-bucket/` | 8 | covered module with empty Bucket column |
| `case-bad-bucket-name/` | 9 | bucket name not recognized |
| `case-covered-in-ci-in-bot-heavy/` | 10 | covered-in-ci in bot-heavy bucket (should use default-executed bucket) |
| `case-deferred-without-lastverified/` | 11 | covered module with empty Last Verified date |
| `case-blocked-without-blocker/` | 12 | blocked module with empty Blocker column |
| `case-blocked-doc-without-artifact/` | 13 | blocked-doc with Blocker not naming a concrete artifact |
| `case-architectural-not-out-of-scope/` | 14 | out-of-scope-by-design with Blocker Class != 'architectural' |
| `case-test-bucket-mismatch/` | 15 | test not found in its bucket's definition |
| `case-bot-test-depth-mismatch/` | 16 | bot test name ends in _Joined but Depth is QUERY |

## Testing the Verifier

An integration test (`test/e2e/joincoverage_test.sh`) runs the verifier against each fixture and asserts:

1. Exit code is 1 (hard failure).
2. Error message matches the expected pattern for that check.
3. The verifier also exits 0 against the real repository root (verifying success conditions).

This ensures the verifier itself is correct and catches regressions when checks are modified.

## Using Fixtures for Manual Testing

To manually test the verifier against a fixture (after another agent adds the `--root` override):

```bash
# From repo root, test against a fixture:
test/e2e/joincoverage.sh verify --root test/e2e/testdata/joincoverage/case-missing-module

# Expect: exit 1, error about missing module
echo $?  # Should be 1
```

To verify the verifier passes against the real repository:

```bash
# From repo root:
test/e2e/joincoverage.sh verify

# Expect: exit 0 (all checks pass)
echo $?  # Should be 0
```

## Fixture Design Notes

### Special Cases

- **`case-uninitialized-submodule/`**: Has NO `modules/` directory at all, to trigger the "modules not initialized" check.
- **`case-missing-module/`**: Has `modules/factorio/` in the filesystem but `factorio` is NOT listed in coverage.md (only minecraft-java is). This violates the rule that every module in modules/ must be listed.
- **`case-stray-module/`**: Lists `factorio` in coverage.md but only has `modules/minecraft-java/` (not `modules/factorio/`). This violates the rule that every listed module must exist in modules/.
- **`case-test-bucket-mismatch/`**: Includes a minimal `buckets.sh` that defines `bot-fast` but does NOT list the test that coverage.md claims is in that bucket.

