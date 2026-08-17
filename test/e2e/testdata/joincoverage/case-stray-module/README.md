# Fixture: case-stray-module

**Check**: 4 (No Stray Modules in Coverage Table)

**Purpose**: Verify that the verifier rejects a module that is listed in the coverage table but does not exist in modules/.

**Principle**: One fixture per check — this fixture violates exactly one rule and is valid in all other respects. A gate never proven to fail is not a gate.

**Expected verifier behavior**: Exit 1 with error message containing "docs/game-coverage.md lists module 'factorio' which does not exist in modules/".

**Details**: The modules/minecraft-java/ directory exists but factorio is listed in the coverage table without a corresponding directory. This catches stray entries that were not cleaned up when a module was deleted or renamed.
