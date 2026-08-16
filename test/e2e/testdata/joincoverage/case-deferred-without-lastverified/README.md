# Fixture: case-deferred-without-lastverified

**Check**: 11 (Covered Modules Must Have a Last Verified Date)

**Purpose**: Verify that the verifier rejects a covered module without a Last Verified date.

**Principle**: One fixture per check — this fixture violates exactly one rule and is valid in all other respects. A gate never proven to fail is not a gate.

**Expected verifier behavior**: Exit 1 with error message containing "module 'minecraft-java' has Status 'covered-deferred' but Last Verified is empty".

**Details**: The minecraft-java module is marked as covered-deferred but the Last Verified column is empty (—). A covered module must have a date to track when it was last verified.
