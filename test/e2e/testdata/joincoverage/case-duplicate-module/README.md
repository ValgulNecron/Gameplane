# Fixture: case-duplicate-module

**Check**: 3 (No Duplicate Modules in Coverage Table)

**Purpose**: Verify that the verifier rejects a module that appears twice in the coverage table.

**Principle**: One fixture per check — this fixture violates exactly one rule and is valid in all other respects. A gate never proven to fail is not a gate.

**Expected verifier behavior**: Exit 1 with error message containing "module 'minecraft-java' listed twice in docs/game-coverage.md".

**Details**: The modules/minecraft-java/ directory exists and minecraft-java appears in two rows of the coverage table. This catches accidental duplicates that could cause confusing behavior.
