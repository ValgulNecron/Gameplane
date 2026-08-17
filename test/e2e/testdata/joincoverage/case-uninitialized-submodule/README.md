# Fixture: case-uninitialized-submodule

**Check**: 1 (Modules Submodule Must Be Initialized)

**Purpose**: Verify that the verifier rejects an uninitialized or empty modules/ directory.

**Principle**: One fixture per check — this fixture violates exactly one rule and is valid in all other respects. A gate never proven to fail is not a gate.

**Expected verifier behavior**: Exit 1 with error message containing "modules/ is not initialized".

**Details**: The modules/ directory is deliberately absent or empty. This fixture has no modules/ directory, simulating an uninitialized git submodule state.
