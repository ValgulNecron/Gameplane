# H23 review — fix/018-harden-telemetry-ingest-validation

Verdict: **pass**

Commit `95b3d856` on base `61f5026529afb4e81e613d80c02049cdef6a0b0a` (merge-base with origin/master equals the brief's scouted base). Working tree is clean.

## Brief conformance
- `telemetry-receiver/main.go`, `main_test.go` and `README.md` are byte-identical to `briefs/files/H23/` (diffed).
- CHANGELOG hunk matches the brief: new `### Security hardening` heading at the end of `## [Unreleased]`, directly above `## [0.3.0-rc.1]`, one line `- **telemetry-receiver:** hardened ingest payload validation.` Correct format.
- Commit message matches section 5 exactly; signed-off; trailers present.
- Only the four listed files changed. No `specs/*/audit/held/` or `SECURITY_AUDIT.md` in the diff.

## Findings coverage
- F-202 (missing fields / top-level null accepted): `wirePayload` uses pointer fields, and `decodePayload` rejects a nil pointer. That covers an absent field, a `null` field, and a top-level `null`, which leaves every pointer nil. Arrays and numbers fail to decode into a struct. Closed.
- F-203 (trailing content, size cap bypass): `io.ReadAll(http.MaxBytesReader(...))` reads the whole body under the cap, so any body over 16 KiB gives a `MaxBytesError`, which maps to 413. A second `Decode` must return `io.EOF`, so non-whitespace trailing bytes, a second object and a trailing value are all rejected with 400, while trailing whitespace is still accepted. Closed. This matches the finding's "Expected" text.
- Negative-count and version-sanitising logic is unchanged.

## Legitimate callers
- The only sender is `api/internal/telemetry/telemetry.go:59-63,115`: `json.Marshal` of a struct with all three fields and no `omitempty`. Its output is a single object with no trailing bytes, so it is unaffected. The chart files (`charts/gameplane/templates/api.yaml`, `values.yaml`) reference the URL only.

## Tests
- `TestIngestRequiresCompleteSingleReport`: 12 subcases. Before the fix, the missing-field, empty-object, null-field, null-body, second-object and trailing-garbage cases got 204, so the test would fail. The final metrics check confirms nothing was counted (a CounterVec with no series emits no `{`-labelled sample line).
- `TestIngestAcceptsZeroCountsAndTrailingWhitespace`: guards against over-rejecting. It checks that a zero-count report is still accepted, since a pointer to 0 is non-nil.
- `TestIngestBodyLimitCoversWholeBody`: before the fix, the padding cases got 204, so the test would fail. The at-limit body (exactly `maxBody` bytes) gets 204 and is the only one counted. The boundary arithmetic is correct: one byte over gives 413.
- No existing test was changed or weakened. No E2E was added, so buckets.sh and the login budget don't apply.
- Test names describe the control they check.

## Docs/specs
- The README row is updated accurately.
- `specs.md` is unchanged, per the plan (lines 14 and 55-57 already state the required-fields contract). Optional follow-up for the other `specs.md` work, not required here: the failure column at `specs.md:45` could also say "missing fields / trailing content".

## Wording leaks
- I grepped the diff for `F-NNN`, `Hxx`, exploit, bypass, attack and skew: no hits. Comments, test names, the commit message and the CHANGELOG line are all neutral.

## Compile checks (run by reviewer)
- In `telemetry-receiver`, `gofmt -l .` printed nothing, and `go build ./...` and `go vet ./...` both passed.

## Issues
None.
