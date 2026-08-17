# Fixture: case-invalid-test-name

**Check**: 7 (Test Name Must Be Valid)

**Purpose**: Verify that the verifier rejects a test name that does not exist in test/e2e/*_test.go.

**Principle**: One fixture per check — this fixture violates exactly one rule and is valid in all other respects. A gate never proven to fail is not a gate.

**Expected verifier behavior**: Exit 1 with error message containing "module 'minecraft-java' lists test 'TestGameServer_MinecraftJavaBot_BadName' which was not found in test/e2e/*_test.go".

**Details**: The minecraft-java module references a test name that does not exist. This catches typos and renamed tests that were not updated in the coverage table.
