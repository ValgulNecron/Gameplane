# Fixture: case-blocked-without-blocker

**Check**: 12 (Blocked Modules Must Have a Blocker)

**Purpose**: Verify that the verifier rejects a blocked module without a blocker description.

**Principle**: One fixture per check — this fixture violates exactly one rule and is valid in all other respects. A gate never proven to fail is not a gate.

**Expected verifier behavior**: Exit 1 with error message containing "module '7-days-to-die' has Status 'blocked-doc' but Blocker is empty".

**Details**: The 7-days-to-die module is marked as blocked-doc but the Blocker column is empty (—). A blocked module must describe what is blocking progress.
