# Review: gameaction

- **Date**: 2026-09-24
- **Reviewer tier**: opus (verification pending)
- **Checked against**: `gameaction/specs.md`; package doc `gameaction/action.go:1-5`; `docs/architecture.md:277-283`; `docs/module-authoring.md:1135-1195` (actions section); `CLAUDE.md` coverage table

## Scope reviewed

In scope, read in full:
- `gameaction/action.go` (120 lines)
- `gameaction/action_test.go` (118 lines), read to see which behaviour is pinned
- `gameaction/specs.md`
- `gameaction/go.mod`, `gameaction/.testcoverage.yml`

Cross-referenced, read only in part:
- The two importers: `api/internal/ws/actions.go:150-250` (`actionParams`, `runStdinAction`) and the `gameaction.` call sites in `agent/internal/actions/actions.go:46-132`, found by grep
- `operator/api/v1alpha1/gametemplate_types.go:650-694` (`ActionParamSpec`, including the `Type` kubebuilder enum)
- `agent/internal/players/players.go:305-325` (`sanitizeReason`, which mirrors `hasControl`)
- `web/src/components/server/ServerActionsCard.tsx`, grepped for a client-side length limit (none found)

Not done: I ran no tests (Rule 8). The only command run was `go build ./...` in `gameaction/`, which passed.

## Method

1. Compared each contract bullet and invariant in `specs.md` (lines 70-121) with `Resolve`, `validateParam`, `hasControl`, `Compile` and `Render`, and built concrete inputs for each branch.
2. Confirmed that both importers call `Resolve` before `Compile`/`Render`, which is the "independent validation" invariant.
3. Checked error wrapping and the `text/template` configuration.

## Observations (no finding)

- Both importers validate independently before rendering, as `specs.md:109` says. The API calls `gameaction.Resolve` at `api/internal/ws/actions.go:221` before `Compile` at `:234`. The agent calls it at `agent/internal/actions/actions.go:132`.
- The CRD restricts `ActionParamSpec.Type` to `string;int;bool;enum`, defaulting to `string` (`gametemplate_types.go:680-681`). So the `default:` branch of `validateParam` (`action.go:74-81`) only ever sees `string` or `""` from a validated CRD.
- Declared defaults go through the same `validateParam` path as user values (`action.go:35-49`). An enum default outside `Enum` is therefore rejected when the action is resolved.
- `Compile` and `Render` wrap errors with `%w` (`action.go:107,117`). `Resolve`'s error messages name the parameter but never echo its value.
- `missingkey=error` applies to the `map[string]string` index, so a template reference to an undeclared key fails at render. `TestRender_MissingKey` pins this.
- The test names listed in `specs.md:151-159` all exist in `action_test.go`. The coverage gate is 91 in both `.testcoverage.yml` and `CLAUDE.md`.
- `docs/module-authoring.md:1187-1192` and `docs/architecture.md:277-283` describe the guard accurately.

held candidates: 0 (see OD-019). One security-adjacent question is recorded off-git in `audit/held/review-gameaction.md`.

## Candidate findings

### C-gameaction-01: The "512-character" cap counts UTF-8 bytes, not characters

- **Location**: `gameaction/action.go:78-80`; claims at `gameaction/action.go:29`, `gameaction/specs.md:20`, `gameaction/specs.md:81`, `gameaction/specs.md:113`
- **Category**: correctness / docs-drift
- **Suggested severity**: S4. Non-ASCII users get a lower effective limit; they can work around it by shortening the value.
- **Observation / repro**:
  1. Declare `Param{Name: "message", Type: "string"}` and call `Resolve` with `message` set to 300 copies of `é` (U+00E9, 2 bytes each, 600 bytes in total).
  2. `len(val)` is 600, so `action.go:78` returns `parameter "message" is too long (max 512)`.
  3. The same happens for 171 or more CJK characters (3 bytes each), for example a Chinese `say` broadcast.
- **Expected**: `specs.md:20` says "Enforce a 512-character length cap on string parameters", and `specs.md:113` says "String parameters are capped at 512 characters". A 300-character value would then be accepted.
- **Actual**: The cap is 512 bytes. Values of 257 to 512 two-byte characters, or 171 to 512 three-byte characters, are rejected. Either the check should count runes (`utf8.RuneCountInString`), or the spec, the doc comment and the error message should say "bytes".

### C-gameaction-02: The spec says a required parameter rejects an empty value "even with a default"; the code fills an empty value with the default

- **Location**: `gameaction/action.go:34-40`; claim at `gameaction/specs.md:117`
- **Category**: docs-drift
- **Suggested severity**: S4
- **Observation / repro**:
  1. Declare `Param{Name: "m", Type: "string", Required: true, Default: "hello"}`.
  2. Call `Resolve(decls, map[string]string{"m": ""})`.
  3. `action.go:35-36` treats `""` the same as a missing key and sets `val = "hello"`. The required check at `:38` passes, and the result is `{"m": "hello"}` with a nil error.
- **Expected** (spec text at `specs.md:117`): "A param with `Required: true` rejects empty or whitespace-only values, even with a default."
- **Actual**: An empty value with a non-empty default is accepted and replaced by the default. Only a whitespace-only value such as `"  "` is rejected, because the default isn't substituted for it. The same paragraph goes on to say "a missing value is filled with the default, then checked for emptiness", which matches the code, so the paragraph contradicts itself.

### C-gameaction-03: The spec gives `rand` as an example template function; `Compile` registers no functions and `text/template` has no `rand`

- **Location**: `gameaction/action.go:105` (`template.New(name).Option("missingkey=error").Parse(command)`, with no `.Funcs`); claim at `gameaction/specs.md:121`
- **Category**: docs-drift
- **Suggested severity**: S4
- **Observation / repro**:
  1. `specs.md:121`: "given the same template and params, rendering always produces the same output (modulo non-determinism in the template itself, e.g., `rand`)."
  2. `Compile("x", "say {{rand}}")` fails at parse time with `template: x:1: function "rand" not defined`. The `text/template` builtins are `and`, `call`, `html`, `index`, `slice`, `js`, `len`, `not`, `or`, `print`, `printf`, `println`, `urlquery` and the comparison functions. None of them is random.
- **Expected**: The invariant says rendering is fully deterministic, because no non-deterministic function is available.
- **Actual**: The spec names a function that doesn't exist, which suggests template authors can use one.

### C-gameaction-04: The spec header says "Go 1.25+", but the module declares `go 1.26.0`

- **Location**: `gameaction/specs.md:5`; `gameaction/go.mod:3`
- **Category**: docs-drift
- **Suggested severity**: S4
- **Observation / repro**:
  1. `specs.md:5`: "**Dependencies:** stdlib only (Go 1.25+)".
  2. `go.mod:3`: `go 1.26.0`. The module can't be built with a Go 1.25 toolchain unless it downloads a newer one.
- **Expected**: The header's minimum Go version matches `go.mod` (1.26).
- **Actual**: The header says 1.25+. `gameproto/specs.md:5` has the same drift; see C-gameproto-07.

## Questions (not findings)

- When an optional parameter has a default, `Resolve` substitutes the default only for `""`, not for a whitespace-only value (`action.go:35-36`). So an `int` parameter with `Default: "60"` given `"  "` fails with `must be an integer` instead of falling back to `60`. Is that intended? The web client pre-fills defaults (`ServerActionsCard.tsx:374`), so users rarely hit it.
