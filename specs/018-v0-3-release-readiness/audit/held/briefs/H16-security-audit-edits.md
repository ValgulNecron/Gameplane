# H16: proposed `SECURITY_AUDIT.md` edits (maintainer working tree only, never commit from a held branch)

Read from the current working-tree copy `/home/valgul/project/Gameplane/SECURITY_AUDIT.md` on 2026-09-24. Only one line needs to change. Apply it after the H16 PR merges (the new wording describes the merged behaviour). The file keeps its uncommitted OD-019 rewording; this is an edit on top of it.

## Edit 1: line 155

The current line says both policies refuse the CGNAT ranges. Only `IsPublic` does; `IsAllowed` keeps CGNAT reachable apart from the one metadata address. It also names only the operator and the agent as callers.

BEFORE (line 155, verbatim):
```
- **Outbound network guard (`netguard`):** `IsAllowed` (operator) and `IsPublic` (agent) check the resolved address at dial time. They refuse link-local and cloud metadata ranges (169.254.0.0/16), IPv6 translation prefixes, and the Kubernetes pod/node CGNAT ranges, including addresses reached through DNS.
```

AFTER:
```
- **Outbound network guard (`netguard`):** `IsAllowed` (operator module sources, API notification sinks, agent WebSocket RCON) and `IsPublic` (agent mod downloads, API Steam resolver) check the resolved address at dial time, including addresses reached through DNS. Both refuse link-local addresses (169.254.0.0/16, where the cloud metadata endpoint lives), the other known cloud metadata addresses, and the IPv6 translation prefixes. Only `IsPublic` also refuses private and loopback addresses, the Kubernetes pod/node CGNAT range (100.64.0.0/10), and the other special-purpose ranges that aren't globally reachable.
```

No other line of the file mentions netguard, `IsAllowed`, `IsPublic` or CGNAT (checked with grep on the working-tree copy).
