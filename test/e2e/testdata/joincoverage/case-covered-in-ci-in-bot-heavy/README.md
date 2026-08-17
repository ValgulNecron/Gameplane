# Fixture: case-covered-in-ci-in-bot-heavy

**Check**: 10 (Covered-in-CI Cannot Use Bot-Heavy Bucket)

**Purpose**: Verify that the verifier rejects a covered-in-ci module assigned to the bot-heavy bucket.

**Principle**: One fixture per check — this fixture violates exactly one rule and is valid in all other respects. A gate never proven to fail is not a gate.

**Expected verifier behavior**: Exit 1 with error message containing "module 'minecraft-java' has Status 'covered-in-ci' but Bucket 'bot-heavy'".

**Details**: The minecraft-java module is marked as covered-in-ci (runs in default CI) but assigned to bot-heavy (which does not run in default CI). This catches misclassifications that would silently exclude tests from the default CI run.
