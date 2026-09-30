# Draft inventory rows: SEC (rc.0)

Links are relative to audit/inventory.md.

| ID | Capability | Component | Source | Procedure | CI test | Outcome | Reason | Evidence | Round | Findings |
|----|------------|-----------|--------|-----------|---------|---------|--------|----------|-------|----------|
| INV-SEC-001 | Login privacy: identical error response for unknown user, wrong password, and account-exists-wrong-password | api/ | api/internal/auth/local.go:105-173 | [p](procedures/security.md#login-privacy) | TBD | untested | | [e](INV-SEC-001/) | | |
| INV-SEC-002 | RBAC: viewer cannot write to servers | api/ | api/internal/rbac/rbac.go:54-254 | [p](procedures/security.md#rbac-permission-boundaries) | TBD | untested | | [e](INV-SEC-002/) | | |
| INV-SEC-003 | RBAC: operator cannot access admin-only paths | api/ | api/internal/rbac/rbac.go:54-254 | [p](procedures/security.md#rbac-permission-boundaries) | TBD | untested | | [e](INV-SEC-003/) | | |
| INV-SEC-004 | RBAC: collaborator can read and write their server, but cannot access other servers or transfer ownership | api/ | api/internal/rbac/rbac.go:54-254 | [p](procedures/security.md#rbac-permission-boundaries) | TBD | untested | | [e](INV-SEC-004/) | | |
| INV-SEC-005 | RBAC: cross-namespace/cluster access is prevented (user bound to one namespace cannot access servers in another) | api/ | api/internal/rbac/rbac.go:54-254 | [p](procedures/security.md#rbac-permission-boundaries) | TBD | untested | | [e](INV-SEC-005/) | | |
| INV-SEC-006 | NetGuard: operator policy blocks link-local (169.254.0.0/16) and metadata hostnames from ModuleSource | api/, operator/ | netguard/netguard.go:91-110 | [p](procedures/security.md#netguard-ssrf-protection) | TBD | untested | | [e](INV-SEC-006/) | | |
| INV-SEC-007 | NetGuard: agent policy blocks CGNAT (100.64.0.0/10) and link-local from mod fetch | agent/ | netguard/netguard.go:112-132 | [p](procedures/security.md#netguard-ssrf-protection) | TBD | untested | | [e](INV-SEC-007/) | | |
| INV-SEC-008 | Console guard: control characters (CR/LF) in action params are rejected | api/, agent/ | gameaction/action.go:75-76 | [p](procedures/security.md#console-injection-guard) | TBD | untested | | [e](INV-SEC-008/) | | |
| INV-SEC-009 | Console guard: action params longer than 512 characters are rejected | api/, agent/ | gameaction/action.go:78-79 | [p](procedures/security.md#console-injection-guard) | TBD | untested | | [e](INV-SEC-009/) | | |
| INV-SEC-010 | Audit-chain tamper: UPDATE to an audit row changes its hash, Verify detects the break | api/ | api/internal/audit/audit.go:240-520 | [p](procedures/security.md#audit-chain-tamper-detection) | TBD | untested | blocked candidate: requires quiet cluster window and direct database access; alternative: unit test in audit_test.go | [e](INV-SEC-010/) | | |
| INV-SEC-011 | Audit-chain tamper: DELETE of a middle audit row breaks the prev_hash chain, Verify detects the break | api/ | api/internal/audit/audit.go:240-520 | [p](procedures/security.md#audit-chain-tamper-detection) | TBD | untested | blocked candidate: requires quiet cluster window and direct database access; alternative: unit test in audit_test.go | [e](INV-SEC-011/) | | |
| INV-SEC-012 | Audit-chain tamper: DELETE of tail rows (truncation) is detected by audit.head anchor | api/ | api/internal/audit/audit.go:240-520, verifyHead | [p](procedures/security.md#audit-chain-tamper-detection) | TBD | untested | blocked candidate: requires quiet cluster window and direct database access; alternative: unit test in audit_test.go | [e](INV-SEC-012/) | | |
| INV-SEC-013 | Share token is returned only on create, omitted from list and dashboard | api/ | api/internal/handlers/shares.go:65-172 | [p](procedures/security.md#secret-redaction) | TBD | untested | | [e](INV-SEC-013/) | | |
| INV-SEC-014 | Share token is redacted as `<token>` in audit log paths | api/ | api/internal/audit/audit.go, audit redaction | [p](procedures/security.md#secret-redaction) | TBD | untested | | [e](INV-SEC-014/) | | |
| INV-SEC-015 | Auth-provider clientSecret is never echoed in response; only secret name and key list returned | api/ | api/internal/handlers/auth_provider_secret.go:74 | [p](procedures/security.md#secret-redaction) | TBD | untested | | [e](INV-SEC-015/) | | |
| INV-SEC-016 | Mod-registry API key is never echoed in response; only secret name and key list returned | api/ | api/internal/handlers/registry_secret.go:72 | [p](procedures/security.md#secret-redaction) | TBD | untested | | [e](INV-SEC-016/) | | |

Row count: 16
