# Fixture: case-bad-bucket-name

**Check**: 9 (Bucket Name Must Be Valid When Present)

**Purpose**: Verify that the verifier rejects a bucket name that is not recognized.

**Principle**: One fixture per check — this fixture violates exactly one rule and is valid in all other respects. A gate never proven to fail is not a gate.

**Expected verifier behavior**: Exit 1 with error message containing "module 'minecraft-java' has Bucket 'bot-invalid' which is not recognized".

**Details**: The minecraft-java module is assigned to a bucket named 'bot-invalid' which does not exist in the list of known buckets. This catches typos and outdated bucket names.
