# Fixture: case-test-bucket-mismatch

**Check**: 15 (Test and Bucket Cross-References Must Match)

**Purpose**: Verify that the verifier rejects a module where the test is listed in coverage but not in the bucket's definition.

**Principle**: One fixture per check — this fixture violates exactly one rule and is valid in all other respects. A gate never proven to fail is not a gate.

**Expected verifier behavior**: Exit 1 with error message containing "module 'minecraft-java' lists test 'TestGameServer_MinecraftJavaBot_Joined' in Bucket 'bot-fast' but this test is not found in buckets.sh's 'bot-fast' definition".

**Details**: The minecraft-java module lists a test in the coverage table and assigns it to the bot-fast bucket, but the test does not appear in buckets.sh's bot-fast bucket definition. This catches test/bucket mismatches that could result in tests not being executed.
