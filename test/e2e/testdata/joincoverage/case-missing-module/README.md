# Fixture: case-missing-module

**Check**: 2 (Every Module in `modules/` Must Be Listed in docs/game-coverage.md)

**Purpose**: Verify that the verifier rejects a module directory in modules/ that is not listed in the coverage table.

**Principle**: One fixture per check — this fixture violates exactly one rule and is valid in all other respects. A gate never proven to fail is not a gate.

**Expected verifier behavior**: Exit 1 with error message containing "module 'factorio' found in modules/ but not listed in docs/game-coverage.md".

**Details**: The modules/factorio/ directory exists but factorio is NOT listed in the coverage table. This catches modules that were added to the submodule but the coverage status was not tracked.
