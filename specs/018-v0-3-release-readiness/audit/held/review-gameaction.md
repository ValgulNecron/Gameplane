# Held review notes: gameaction (OD-019, off-git)

- **Date**: 2026-09-24
- **Reviewer tier**: opus (verification pending)
- **Checked against**: `gameaction/specs.md` (invariant 2, security consideration 1)
- **Companion notes**: `audit/evidence/review-gameaction/notes.md`

This review found no held candidates: the console-injection guard behaves exactly as its spec defines it. What follows is one security-adjacent question. It is kept off-git because it concerns the guard's coverage.

## Candidate findings

None.

## Questions (not findings)

- **Scope of the control-character guard.** `hasControl` (`gameaction/action.go:88-95`) rejects runes `< 0x20` and `0x7f`, which is exactly the ASCII range the spec names (`specs.md:81,86-88,111`). Two kinds of character pass it: the Unicode C1 controls (U+0080–U+009F, for example NEL U+0085) and the line and paragraph separators U+2028/U+2029. I found no concrete case where a supported console treats them as a line break. Java `BufferedReader.readLine` splits only on `\n`, `\r` and `\r\n`. .NET `Console.ReadLine` splits only on `\n` and `\r\n`. A Linux TTY line discipline acts on single-byte special characters, all of them below 0x20, while UTF-8 encodes C1 as two bytes. Question for the maintainer: should the spec state that ASCII-only coverage is deliberate, or should the guard also reject `unicode.IsControl` runes and U+2028/U+2029 as defense in depth? `agent/internal/players/players.go:318-322` (`sanitizeReason`) mirrors the same ASCII-only rule, so the two should change together. To confirm the current behaviour: add `"hi\u0085stop"` and `"hi stop"` rows to `TestResolve_RejectsControlChars` and see which way CI goes.
