# Review H25: MCP server read-only layers, docs precision

Verdict: **needs-fix** (one missing CHANGELOG line; the content is otherwise correct)

Branch `fix/018-harden-mcp-readonly-docs`, commit `15a521c6` on `origin/master` (`61f50265`). Worktree clean.

## Scope
- The diff touches exactly `mcp-server/main.go`, `mcp-server/internal/kube/client.go`, `mcp-server/specs.md`, `mcp-server/README.md` and `docs/dependencies.md` (+54/-46). Nothing is under `specs/*/audit/held/`, and `SECURITY_AUDIT.md` is untouched.
- All five edits match the brief's AFTER blocks word for word, and no other lines in those files changed. The "7 CRDs" wording, the dependency table (with its doc-versions markers) and the README "Run standalone" example are all left as they were.

## Accuracy against the finding and the code
- The finding asks for two things: layer 1 should say only what the package boundary enforces, and RBAC should be named as the only layer that stops mutation by any code in the process. All five places now say this, in wording that matches `docs/architecture.md:306-316`.
- Claims checked against the code:
  - `runServe` calls `ctrl.GetConfig()` (`main.go:111`) and `kube.New(cfg)` (`main.go:115`).
  - `New` is at `client.go:124` and `NewFrom` at `client.go:144`, both exported.
  - The specs.md "Source" line now points at named symbols instead of line numbers that go stale.
- The standalone-kubeconfig note is accurate: RBAC covers only the in-cluster ServiceAccount. It is a plain statement of fact, not a description of how to use a gap.
- No other text in the repo still says "even by mistake", "structurally real" or "three independent layers".
- Small leftovers (not blocking, optional):
  - `mcp-server/specs.md` keeps the heading `### 2. RBAC backstop (authoritative)`, while the body now calls it the "authoritative layer". The heading is not wrong; renaming it to `### 2. RBAC (authoritative)` would make it consistent.
  - The `TestClientHasNoMutatingMethods` doc comment in `mcp-server/main_test.go:27-39` still calls the boundary a "structural guarantee". It is scoped to "through a *kube.Client", so it is close to accurate. It sits outside the brief's scope and is in a test file, so leave it unless the maintainer wants it aligned.

## Wording leaks
- The added lines and the commit message contain no finding IDs, group IDs, repro steps or description of how to use a gap. The only "held" match is the English phrase "held by two layers", which is fine.

## Commit
- The commit is signed off, the message is neutral, and it carries the Claude-Session trailer.
- The Co-Authored-By trailer names `Claude Haiku 4.5`, while the brief's template says Opus 5.5. That fits CLAUDE.md 11 ("current-running-model"), so it is informational only.

## Compile checks (run by reviewer)
- In `mcp-server/`: `gofmt -l .` printed nothing, and `go build ./...` and `go vet ./...` passed.
- The change is comments only, so no tests are needed. There is no E2E, so `buckets.sh` and the login budget do not apply.

## Issue (needs-fix)
1. **The CHANGELOG line is missing.** Plan rule 4 ("Each held PR adds one line under ... Security hardening") and the wave's hard rules both require it. The sibling docs-only group H26 includes one. The brief's "no CHANGELOG line (docs-only group)" contradicts rule 4, and the implementer followed the brief.
   - **Fix:** in `CHANGELOG.md`, go to the end of `## [Unreleased]`, just before `## [0.3.0-rc.1] — 2026-09-23` (currently line 150).
     - If `### Security hardening` is absent (as on this branch's base), add this, with a blank line before `## [0.3.0-rc.1]`:
       ```
       ### Security hardening

       - **mcp-server:** hardened the read-only guarantee documentation to state each enforcing layer precisely.
       ```
     - If H26 has already merged and the heading exists, add only the bullet under it. Expect a trivial rebase conflict with H26 on that heading.
   - Commit it as a separate signed commit, for example `docs(changelog): note mcp-server read-only docs hardening`, with the same trailers. Add `CHANGELOG.md` to the PR body's Changes list: "`CHANGELOG.md`: one line under "Security hardening"."
