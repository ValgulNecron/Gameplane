# Fixture: case-covered-with-query-depth

**Check**: 5 (Status and Depth Must Be Consistent)

**Purpose**: Verify that the verifier rejects a covered module with insufficient depth (QUERY instead of JOINED).

**Principle**: One fixture per check — this fixture violates exactly one rule and is valid in all other respects. A gate never proven to fail is not a gate.

**Expected verifier behavior**: Exit 1 with error message containing "module 'garrys-mod' has Status 'covered-in-ci' but Depth 'QUERY'".

**Details**: The garrys-mod module is marked as covered-in-ci (runs in CI) but has Depth=QUERY (query-only, not a full join). Per spec FR-006, only JOINED depth counts as coverage. This catches accidental depth/status mismatches.
