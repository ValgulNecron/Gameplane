# Fixture: case-architectural-not-out-of-scope

**Check**: 14 (Out-of-Scope-by-Design Must Have Architectural Blocker)

**Purpose**: Verify that the verifier rejects an out-of-scope-by-design module with a non-architectural blocker class.

**Principle**: One fixture per check — this fixture violates exactly one rule and is valid in all other respects. A gate never proven to fail is not a gate.

**Expected verifier behavior**: Exit 1 with error message containing "module 'valheim' has Status 'out-of-scope-by-design' but Blocker Class 'documentation'".

**Details**: The valheim module is marked as out-of-scope-by-design but the Blocker Class is 'documentation' instead of 'architectural'. Out-of-scope decisions must be permanent (architectural), not reversible (documentation). This catches misclassified blockers.
