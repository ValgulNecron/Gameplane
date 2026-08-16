# Fixture: case-covered-without-test

**Check**: 6 (Covered Modules Must Have a Test)

**Purpose**: Verify that the verifier rejects a covered module without a test name.

**Principle**: One fixture per check — this fixture violates exactly one rule and is valid in all other respects. A gate never proven to fail is not a gate.

**Expected verifier behavior**: Exit 1 with error message containing "module 'minecraft-java' has Status 'covered-in-ci' but Test is empty".

**Details**: The minecraft-java module is marked as covered-in-ci but the Test column is empty (—). A covered module must have a test function name listed so CI can verify it exists and runs.
