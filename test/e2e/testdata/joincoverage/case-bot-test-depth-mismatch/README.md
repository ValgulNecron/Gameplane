# Fixture: case-bot-test-depth-mismatch

**Check**: 16 (Bot Test Names Must Agree With Depth)

**Purpose**: Verify that the verifier rejects a bot test whose name suffix does not match its Depth.

**Principle**: One fixture per check — this fixture violates exactly one rule and is valid in all other respects. A gate never proven to fail is not a gate.

**Expected verifier behavior**: Exit 1 with error message containing "module 'minecraft-java' lists test 'TestGameServer_MinecraftJavaBot_Joined' but Depth is 'QUERY'".

**Details**: The minecraft-java module lists a test ending in "_Joined" (indicating a full join test) but assigns it Depth=QUERY (indicating a query-only test). This naming convention prevents depth assertions from drifting silently.
