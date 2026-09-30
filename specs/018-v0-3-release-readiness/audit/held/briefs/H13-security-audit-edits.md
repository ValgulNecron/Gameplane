# H13: proposed SECURITY_AUDIT.md edits (maintainer's working-tree copy only)

Off-git (OD-019). No branch edits `SECURITY_AUDIT.md`. The maintainer applies these to the uncommitted working-tree copy at `/home/valgul/project/Gameplane/SECURITY_AUDIT.md`, section "Items reviewed but not retained as findings".

**The working-tree file changed while this brief was being written.** At the first read (2026-09-24, before 23:27) the section was titled "Controls reviewed with no gap found" and the three lines were at 150, 151 and 155 (the reworded OD-019 text the maintainer copied over). At 23:27 the file was rewritten (now 152 lines) with different wording, and the lines are now at 136, 137 and 141. The scout did not change it. Cause, as far as read-only checks show: another session switched the main checkout's branch (reflog: `018-v0-3-release-readiness` -> `design/g7-tunnel-wizard-flow` -> `fix/018-web-ui-polish-f134-design`, and `stash@{0}` "WIP on fix/018-web-ui-polish-f134-design" was created), and `git status` no longer shows `SECURITY_AUDIT.md` as modified. So the maintainer's uncommitted OD-019 rewording is most likely in `stash@{0}` now, not lost; the scout did not open the stash (no git show on this file). The "earlier text" BEFORE blocks match that rewording. The maintainer should restore it before applying these edits, then use the "earlier text" BEFOREs. Each edit below gives the BEFORE for the current text and, as "earlier text", the line that was read first; the AFTER works for either.

Why: F-114 item 6 (the files package does not use `ConfinePath`/`ConfineRelPath`; it has its own resolver) and item 1 (`IsPublic` is not the agent's only dial policy: the WebRcon dial uses `IsAllowed`). Edit 3 records the F-112 control once H13 merges. The edit 1 wording "including each uploaded file's full destination" is true only once H30 merges: apply edit 1 after H30 (or drop that clause if H13's edits are applied first).

Apply with exact string replacement. Each BEFORE is one whole line.

## Edit 1: agent filesystem (current line 136)

BEFORE (current text, line 136):

~~~~
- **Agent filesystem & mod confinement (`agent/internal/files`, `agent/internal/mods`):** Verified strict path confinement (`ConfinePath`, `ConfineRelPath`), symlink traversal prevention, zip-slip defense, and file size/count bounds on multipart uploads and archive extractions.
~~~~

Earlier text (line 150 at the first read):

~~~~
- **Agent filesystem and mod handling (`agent/internal/files`, `agent/internal/mods`):** file access is confined to the server's data directory (`ConfinePath`, `ConfineRelPath`). Symlinks can't lead outside it, archive entries can't be written outside the target directory, and uploads and archive extraction are bounded in size and file count.
~~~~

AFTER:

~~~~
- **Agent filesystem and mod handling (`agent/internal/files`, `agent/internal/mods`):** file access is confined to the server's data directory. The file API resolves every path, including each uploaded file's full destination, inside the data root with its own resolver, and the mod code uses `ConfinePath` for single names and `ConfineRelPath` for archive entry paths. Symlinks can't lead outside it, archive entries can't be written outside the target directory, and uploads and archive extraction are bounded in size and file count.
~~~~

## Edit 2: outbound network guard (current line 141)

BEFORE (current text, line 141):

~~~~
- **Outbound network guard (`netguard`):** Audited `IsAllowed` (operator) and `IsPublic` (agent) dial-time address controls; confirmed all cloud metadata ranges (including 169.254.0.0/16, IPv6 translation prefixes, and Kubernetes pod/node CGNAT blocks) are blocked against SSRF and DNS rebinding.
~~~~

Earlier text (line 155 at the first read):

~~~~
- **Outbound network guard (`netguard`):** `IsAllowed` (operator) and `IsPublic` (agent) check the resolved address at dial time. They refuse link-local and cloud metadata ranges (169.254.0.0/16), IPv6 translation prefixes, and the Kubernetes pod/node CGNAT ranges, including addresses reached through DNS.
~~~~

AFTER (H13's minimal change: only the callers):

~~~~
- **Outbound network guard (`netguard`):** `IsAllowed` (the operator's fetches, and the agent's WebRcon dial to the game in its own pod) and `IsPublic` (the agent's mod downloads) check the resolved address at dial time. They refuse link-local and cloud metadata ranges (169.254.0.0/16), IPv6 translation prefixes, and the Kubernetes pod/node CGNAT ranges, including addresses reached through DNS.
~~~~

Coordination with H16: `held/briefs/H16-security-audit-edits.md` also rewrites this line (its BEFORE is the earlier line-155 text). Its AFTER already lists "agent WebSocket RCON" under `IsAllowed` and corrects the CGNAT sentence, which is H16's to fix (not H13's). **Prefer H16's line.** Use the H13 AFTER above only if H13 merges well before H16 and the maintainer wants the callers corrected in the meantime; H16's AFTER then replaces the whole line.

## Edit 3 (optional): agent console (current line 137)

BEFORE (current text, line 137):

~~~~
- **Agent console & WebSocket multiplexing (`agent/internal/console`, `api/internal/ws`):** Evaluated RCON and PTY console routes; access is gated behind `servers:console` RBAC, and stdin/RCON command parameters are protected against control-character injection via `gameaction.Resolve`.
~~~~

Earlier text (line 151 at the first read):

~~~~
- **Agent console and WebSocket multiplexing (`agent/internal/console`, `api/internal/ws`):** RCON and PTY console routes require the `servers:console` permission, and `gameaction.Resolve` rejects control characters in command parameters.
~~~~

AFTER:

~~~~
- **Agent console and WebSocket multiplexing (`agent/internal/console`, `api/internal/ws`):** RCON and PTY console routes require the `servers:console` permission, and `gameaction.Resolve` rejects control characters in command parameters. RCON error text stays inside the agent: the console answers with a fixed error message and logs the detail, and WebRcon dial errors are redacted before anything logs them.
~~~~
