# Fixture: case-covered-without-bucket

**Check**: 8 (Covered Modules Must Have a Bucket)

**Purpose**: Verify that the verifier rejects a covered module without a bucket assignment.

**Principle**: One fixture per check — this fixture violates exactly one rule and is valid in all other respects. A gate never proven to fail is not a gate.

**Expected verifier behavior**: Exit 1 with error message containing "module 'minecraft-java' has Status 'covered-deferred' but Bucket is empty".

**Details**: The minecraft-java module is marked as covered-deferred but the Bucket column is empty (—). A covered module must be assigned to a bucket so CI can track which test suite runs it.
