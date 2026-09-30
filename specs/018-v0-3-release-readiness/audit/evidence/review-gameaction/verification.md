# T045 guards chunk: independent verification (opus)

**Method.** I tried to refute each candidate in `notes.md` (written by another opus reviewer) against master `13a859ff`. `gameaction/` is the same on this branch and on master.

What I read:
- `gameaction/action.go`, `gameaction/action_test.go`, `gameaction/specs.md` and `gameaction/go.mod`, in full.
- Both importers' call sites: `api/internal/ws/actions.go:213-245` and `agent/internal/actions/actions.go:120-140`.
- The CRD's `ActionParamSpec` (`operator/api/v1alpha1/gametemplate_types.go:663-694`). It declares no `MaxLength` on parameter values.
- The feature spec folders, which I grepped for a decision on the length cap or on defaults. There is none.
- `audit/findings.md`, which tracks none of these candidates.

To confirm the runtime behaviour, I built a throwaway program in the session scratchpad, outside the repo. It imports the real module through a `replace` directive. This is not a test or lint suite, and no repo file changed. The gameaction review has no held candidates; its one question stays off-git (OD-019).

| Candidate | Verdict (kept/rejected) | Severity | Reason |
|---|---|---|---|
| C-gameaction-01 | kept | S4 | Confirmed. `action.go:78` checks `len(val) > 512`, which counts UTF-8 bytes. `action.go:29` and `specs.md:20,81,113` promise a 512-*character* cap. The scratch run rejected 300 × `é` (600 bytes) and 171 × `中` (513 bytes), and accepted 170 × `中` (510 bytes). `TestResolve_TooLong` uses only ASCII, so it can't tell bytes from characters. A byte cap suits the cap's stated purpose (resource exhaustion, `specs.md:141`), so the likely fix is wording: the spec, the doc comment and the error text should say "bytes". The user can work around it by shortening the text. |
| C-gameaction-02 | kept | S4 | Confirmed. With `{Required: true, Default: "hello"}` and the value `""`, `Resolve` returns `{m: hello}` and a nil error (`action.go:35-40`). `specs.md:117` says a required parameter "rejects empty or whitespace-only values, even with a default". A whitespace-only value is rejected, as the spec says. The rest of the same paragraph matches the code ("a missing value is filled with the default"), so the spec contradicts itself. The code's behaviour is the reasonable one, so only the spec text is wrong. |
| C-gameaction-03 | kept | S4 | Confirmed. `Compile("x", "say {{rand}}")` fails with `function "rand" not defined`. `action.go:105` registers no `Funcs`, and `text/template` has no random builtin. So `specs.md:121` names a function that doesn't exist and implies templates can be non-deterministic, when rendering is in fact fully deterministic. Wording only. |
| C-gameaction-04 | kept | S4 | Confirmed. `specs.md:5` says "Go 1.25+", while `go.mod:3` declares `go 1.26.0`. Every Go module in the workspace declares `go 1.26.0`. The same stale "1.25" is in `gameproto/specs.md:5` (C-gameproto-07), `test/e2e/internal/specs.md:5,262` and `audit-syslog-bridge/specs.md:55`, so one sweep could fix all of them. |

### C-gameaction-01

**Location:** `gameaction/action.go:78-80` (the check). The claims are at `gameaction/action.go:29` ("the 512-char cap") and `gameaction/specs.md:20,81,113`.

**Repro / observation:**
1. Read `gameaction/action.go:74-81` on master. For a `string` or untyped parameter, after the control-character check, `if len(val) > 512` returns `parameter %q is too long (max 512)`. Go's `len` on a string counts bytes, not runes.
2. Write a scratch `main` that imports `github.com/ValgulNecron/gameplane/gameaction` through a `replace` directive pointing at the repo. Call `gameaction.Resolve([]gameaction.Param{{Name: "message", Type: "string"}}, map[string]string{"message": strings.Repeat("é", 300)})`. It returns `parameter "message" is too long (max 512)`.
3. With `strings.Repeat("中", 171)` (171 characters, 513 bytes), it returns the same error. With `strings.Repeat("中", 170)` (510 bytes), the error is nil.
4. On a cluster, any module action with a free-text `string` parameter behaves the same way, whether it runs through the API's stdin path (`api/internal/ws/actions.go:221`) or the agent's RCON path (`agent/internal/actions/actions.go:132`). A 200-character CJK message is rejected with "too long (max 512)".

**Expected:** Code and docs agree on the unit. Either the check counts characters (`utf8.RuneCountInString(val) > 512`), or `action.go:29`, `specs.md:20,81,113` and the error message say "512 bytes".

**Actual:** The docs say 512 characters, but the code enforces 512 bytes. So a non-ASCII value of 257 to 512 two-byte characters, or 171 to 512 three-byte characters, is rejected, and the error message ("max 512") doesn't explain why.

### C-gameaction-02

**Location:** `gameaction/action.go:34-40`; the claim is at `gameaction/specs.md:117`.

**Repro / observation:**
1. Read `gameaction/action.go:34-40`. `if !ok || val == "" { val = p.Default }` runs before the required check at `:38`, so an empty value is replaced by the default before `Required` is checked.
2. In the scratch program, `Resolve([]Param{{Name: "m", Type: "string", Required: true, Default: "hello"}}, map[string]string{"m": ""})` returns `map[m:hello]` and a nil error.
3. The same call with `"  "` returns `parameter "m" is required`, because the default is substituted only for `""`.

**Expected** (`specs.md:117`): "A param with `Required: true` rejects empty or whitespace-only values, even with a default."

**Actual:** An empty value is silently replaced by the default and accepted. Only a whitespace-only value is rejected. The last sentence of the same paragraph describes the code's real behaviour, so the paragraph contradicts itself. The fix is to reword `specs.md:117`: an empty value falls back to the default, and a whitespace-only value is rejected.

### C-gameaction-03

**Location:** `gameaction/action.go:105` (`template.New(name).Option("missingkey=error").Parse(command)`, with no `.Funcs`); the claim is at `gameaction/specs.md:121`.

**Repro / observation:**
1. Read `gameaction/specs.md:121`: "given the same template and params, rendering always produces the same output (modulo non-determinism in the template itself, e.g., `rand`)."
2. Read `gameaction/action.go:104-110`. No function map is registered. The Go `text/template` builtins (`and`, `call`, `html`, `index`, `slice`, `js`, `len`, `not`, `or`, `print`, `printf`, `println`, `urlquery`, `eq`, `ne`, `lt`, `le`, `gt`, `ge`) include nothing random.
3. In the scratch program, `gameaction.Compile("x", "say {{rand}}")` returns `parse action command "x": template: x:1: function "rand" not defined`.

**Expected:** Invariant 7 says rendering is fully deterministic, because no non-deterministic function is available to a template.

**Actual:** The spec names a function that doesn't exist and suggests that module authors can use one.

### C-gameaction-04

**Location:** `gameaction/specs.md:5`; `gameaction/go.mod:3`.

**Repro / observation:**
1. Read `gameaction/specs.md:5`: "**Dependencies:** stdlib only (Go 1.25+)".
2. Read `gameaction/go.mod:3`: `go 1.26.0`. A Go 1.25 toolchain can't build the module unless it downloads a newer toolchain.
3. `grep -h '^go ' */go.mod test/e2e/go.mod | sort | uniq -c` shows 15 modules, all at `go 1.26.0`.

**Expected:** The spec header states the minimum Go version from `go.mod`, 1.26.

**Actual:** The header says 1.25+.
