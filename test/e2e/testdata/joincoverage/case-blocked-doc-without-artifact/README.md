# Fixture: case-blocked-doc-without-artifact

**Check**: 13 (Blocked-Doc Modules Must Name a Concrete Unblocking Artifact)

**Purpose**: Verify that the verifier rejects a blocked-doc module whose blocker description does not name a concrete artifact.

**Principle**: One fixture per check — this fixture violates exactly one rule and is valid in all other respects. A gate never proven to fail is not a gate.

**Expected verifier behavior**: Exit 1 with error message containing "module 'factorio' has Status 'blocked-doc' but Blocker does not name a concrete artifact".

**Details**: The factorio module is marked as blocked-doc but the Blocker text is "unknown" which does not contain any recognized artifact keywords (packet capture, field map, reverse-engineer, vendor documentation, protocol spec, documentation, specification). This catches incomplete research documentation.
