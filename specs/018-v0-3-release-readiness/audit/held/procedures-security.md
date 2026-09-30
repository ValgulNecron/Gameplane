# Procedures: SEC

> **DO NOT RUN: unreviewed draft with known defects (2026-09-24).** It must be rewritten and reviewed before T047/T048. Known problems:
> - `secret-redaction` creates a share on the pre-existing `mc-fabric` server, which FR-009 forbids.
> - `netguard-ssrf-protection` uses the wrong API group (`gameplane.io`, not `gameplane.local`), puts the cluster-scoped ModuleSource in a namespace, invents fields, and cites the wrong path (`api/internal/modsrc`).
> - `console-injection-guard` uses an invented GameServer manifest; it must create an `audit018-` server from the `minecraft-java` template via the API.
> - `audit-chain-tamper-detection` runs `sqlite3` on the devbox against a DB that lives on the API PVC. It also chains the three variants instead of doing snapshot → tamper → Verify → restore → Verify-ok for each one (OD-009).
> - Test passwords are in plain text, and evidence goes to `/tmp` instead of `audit/evidence/INV-SEC-NNN/`.

Shared conventions: [conventions.md](conventions.md).

### login-privacy

**Preconditions**

- Cluster is running and reachable at `$GP`.
- Two test accounts exist: `audit018-user-exists` (valid password: `testpass123`) and `audit018-wrong-password` (valid password: `differentpass`).
- Request-timing instrumentation available (stopwatch or curl's `-w` timing).

**Resources created**

None (uses pre-existing test accounts).

**Steps**

1. Create two test accounts via API or direct database insert if not yet present:
   - `audit018-user-exists` with password `testpass123`
   - `audit018-wrong-password` with password `differentpass`

2. **Request A**: Login with wrong password for existing user:
   ```bash
   curl -i -D /tmp/headers-a.txt -X POST $GP/auth/login \
     -H "Content-Type: application/json" \
     -d '{"username":"audit018-user-exists","password":"wrongpass"}' \
     -w "%{time_total}\n" > /tmp/resp-a.json 2>&1
   ```
   Record HTTP status, response body (should be `{"message":"invalid credentials"}`), and timing in `audit/evidence/INV-SEC-001/`.

3. **Request B**: Login with unknown username:
   ```bash
   curl -i -D /tmp/headers-b.txt -X POST $GP/auth/login \
     -H "Content-Type: application/json" \
     -d '{"username":"audit018-nosuchuser","password":"anypass"}' \
     -w "%{time_total}\n" > /tmp/resp-b.json 2>&1
   ```
   Record HTTP status, response body, and timing.

4. **Request C**: Login with wrong password for another user:
   ```bash
   curl -i -D /tmp/headers-c.txt -X POST $GP/auth/login \
     -H "Content-Type: application/json" \
     -d '{"username":"audit018-wrong-password","password":"wrongpass"}' \
     -w "%{time_total}\n" > /tmp/resp-c.json 2>&1
   ```
   Record HTTP status, response body, and timing.

5. Verify /login (GET) and public share pages (GET /shares/{token} for any valid share) return no internal data (version, host, server count, etc.).

6. (Login cost: 3 user attempts, under 5/min budget; also verifies timing-side-channel mitigation at api/internal/auth/local.go:140-147.)

**Expected**

- All three failure cases (A, B, C) return HTTP 401 with identical body `{"message":"invalid credentials"}`.
- Response times are similar across A, B, and C (within ±100ms; timing class verified by VerifyDummy hash cost at api/internal/auth/local.go:141).
- /login (GET) response contains no cluster version, hostname, or server count.
- Public share pages contain no internal data (authenticated user list, server names, etc.).

**Cleanup**

- Remove or disable test accounts `audit018-user-exists` and `audit018-wrong-password`.
- Delete `/tmp/headers-*.txt` and `/tmp/resp-*.json`.

**Automatable?**

Yes. Bucket: `api-auth`. Validate timing and response bodies; parameterize account creation/cleanup.

---

### rbac-permission-boundaries

**Preconditions**

- Four test accounts exist: `audit018-admin`, `audit018-operator`, `audit018-viewer`, `audit018-collab` (with appropriate role bindings in `gameplane-games` namespace).
- Test server exists: `audit018-rbac-test` (GameServer in `gameplane-games`, preferably a copy of `mc-fabric`).
- Collaborator binding: `audit018-collab` is a collaborator on `audit018-rbac-test` only (not namespace-level role).

**Resources created**

- GameServer: `audit018-rbac-test` (copy of `mc-fabric`, labeled `gameplane.io/audit: "018"`).
- Role bindings in `gameplane-games`: admin, operator, viewer, collaborator (to server).

**Steps**

1. **As audit018-viewer**: Attempt to write to a server (create a GameServerSchedule, update server config, or POST /servers/{name}:start):
   ```bash
   curl -i -X POST $GP/servers/audit018-rbac-test:start \
     -H "Cookie: gameplane_session=<viewer_session>; gameplane_csrf=<csrf>" \
     -H "X-Gameplane-CSRF: <csrf>" \
     -H "Content-Type: application/json" \
     -d '{}' > /tmp/viewer-write-attempt.json 2>&1
   ```
   Save response to evidence.

2. **As audit018-operator**: Attempt to access admin-only /admin/config path:
   ```bash
   curl -i -X GET $GP/admin/config \
     -H "Cookie: gameplane_session=<operator_session>; gameplane_csrf=<csrf>" \
     -H "X-Gameplane-CSRF: <csrf>" > /tmp/operator-admin-attempt.json 2>&1
   ```
   Save response to evidence.

3. **As audit018-collab (collaborator on audit018-rbac-test only)**:
   - a. Read the server: `curl -i -X GET $GP/servers/audit018-rbac-test … > /tmp/collab-read.json`
   - b. Write to the server: `curl -i -X POST $GP/servers/audit018-rbac-test:start …`
   - c. Attempt to read a different server (`mc-fabric`): `curl -i -X GET $GP/servers/mc-fabric …`
   - d. Attempt to transfer ownership (owner-only): `curl -i -X POST $GP/servers/audit018-rbac-test:transfer …`

4. **Cross-namespace/cluster access**: Create `audit018-rbac-test` in both a permitted and a forbidden namespace. As a user bound to one namespace, attempt GET /servers from the other (verify 400 or 403 on cross-namespace access).

5. (Login cost: 4 sessions × 1 request each = 4 logins, well under 6/min user budget.)

**Expected**

- Step 1: Viewer receives HTTP 403 (forbidden).
- Step 2: Operator receives HTTP 403 (forbidden).
- Step 3:
  - a. Collaborator can read: HTTP 200.
  - b. Collaborator can write: HTTP 200/202 (start request accepted).
  - c. Collaborator cannot read other servers: HTTP 403 (forbidden).
  - d. Collaborator cannot transfer: HTTP 403 (forbidden).
- Step 4: Cross-namespace access is rejected (HTTP 400 or 403).

**Cleanup**

- Delete GameServer `audit018-rbac-test`.
- Remove test role bindings.
- Remove test accounts.
- Delete `/tmp/*.json` evidence files.

**Automatable?**

Yes. Bucket: `api-rbac`. Parameterize account/server/binding setup; validate HTTP status codes.

---

### netguard-ssrf-protection

**Preconditions**

- Cluster is running; module sources can be created.
- Agent pod is running (or will run as part of a GameServer start).
- Network access from the cluster to `169.254.169.254` and `metadata.google.internal` is blocked or unreachable (cloud-neutral test environment).

**Resources created**

- ModuleSource CRDs: `audit018-metadata-aws` (URL: `http://169.254.169.254/latest/meta-data/`), `audit018-metadata-gce` (URL: `http://metadata.google.internal/`), `audit018-cgnat-test` (URL: `http://100.64.0.1/modules/`).
- GameServer (optional): for testing agent-side blocking via mod fetch.

**Steps**

1. **Create ModuleSource pointing at AWS metadata endpoint**:
   ```bash
   kubectl apply -f - << 'YAML'
   apiVersion: gameplane.io/v1alpha1
   kind: ModuleSource
   metadata:
     name: audit018-metadata-aws
     namespace: gameplane-system
   spec:
     type: oci
     ociRegistry:
       url: http://169.254.169.254/latest/meta-data/
   YAML
   sleep 2
   kubectl get ms audit018-metadata-aws -o jsonpath='{.status.phase}' > /tmp/ms-aws-phase.txt
   kubectl describe ms audit018-metadata-aws >> /tmp/ms-aws-status.txt 2>&1
   ```

2. **Create ModuleSource pointing at GCE metadata endpoint**:
   ```bash
   kubectl apply -f - << 'YAML'
   apiVersion: gameplane.io/v1alpha1
   kind: ModuleSource
   metadata:
     name: audit018-metadata-gce
     namespace: gameplane-system
   spec:
     type: oci
     ociRegistry:
       url: http://metadata.google.internal/
   YAML
   sleep 2
   kubectl describe ms audit018-metadata-gce >> /tmp/ms-gce-status.txt 2>&1
   ```

3. **Create ModuleSource pointing at CGNAT address** (operator policy allows this; agent will block):
   ```bash
   kubectl apply -f - << 'YAML'
   apiVersion: gameplane.io/v1alpha1
   kind: ModuleSource
   metadata:
     name: audit018-cgnat-test
     namespace: gameplane-system
   spec:
     type: oci
     ociRegistry:
       url: http://100.64.0.1/modules/
   YAML
   sleep 2
   kubectl describe ms audit018-cgnat-test >> /tmp/ms-cgnat-status.txt 2>&1
   ```

4. **Verify operator enforcement** (api/internal/modsrc/http.go:91 checks IsAllowed):
   - Operator allows RFC1918 (private registries on 192.168.x.x, 10.x.x.x, 172.16.x.x).
   - Operator blocks link-local (169.254.x.x) and metadata hostnames.

5. **Test agent-side blocking** (if GameServer starts and fetches a mod):
   - Start or trigger a GameServer that references `audit018-metadata-aws` or `audit018-cgnat-test`.
   - Watch agent logs: `kubectl logs -n gameplane-games <pod> -c gameplane-agent --tail=50 | grep -i netguard`
   - Verify agent blocks with "address is not an allowed destination" or similar.

6. (Login cost: 0 for kubectl operations; no API calls needed.)

**Expected**

- Step 1–3: ModuleSource CRs are created and their status shows an error (link-local blocked, metadata hostname rejected, or CGNAT blocked).
- Step 4: Operator logs show netguard rejection with source line reference to netguard.go:77-145 (IsAllowed checks).
- Step 5: Agent logs show netguard rejection; mod fetch fails with ErrBlockedAddr or similar.

**Cleanup**

- `kubectl delete ms audit018-metadata-aws audit018-metadata-gce audit018-cgnat-test -n gameplane-system`
- Delete `/tmp/ms-*.txt` evidence files.

**Automatable?**

No (requires cluster-external network blocking or DNS rebinding mitigation; deferred to multi-cluster E2E).
Alternative: unit test coverage in `netguard/netguard_test.go` (IsAllowed and IsPublic) and integration test in `operator/internal/modsrc/*_test.go` (HTTP source creation).

---

### console-injection-guard

**Preconditions**

- Test server exists: `audit018-console-test` (GameServer, preferably a copy of `mc-fabric`, running and ready).
- Server has console access via RCON or PTY.
- Test action exists with a string parameter: e.g., a "say" action that renders `say {{.Params.message}}`.

**Resources created**

- GameServer: `audit018-console-test` (copy of `mc-fabric`).
- Test action with string parameter (or use built-in if available).

**Steps**

1. **Create test GameServer** (if not already present):
   ```bash
   kubectl apply -f - << 'YAML'
   apiVersion: gameplane.io/v1alpha1
   kind: GameServer
   metadata:
     name: audit018-console-test
     namespace: gameplane-games
     labels:
       gameplane.io/audit: "018"
   spec:
     template:
       spec:
         containers:
         - name: game
           image: mcr.microsoft.com/windows/servercore:latest
           # ... rest of template from mc-fabric ...
   YAML
   kubectl wait --for=condition=Ready gs/audit018-console-test -n gameplane-games --timeout=180s
   ```

2. **Test CR/LF injection in console parameter**:
   ```bash
   curl -i -X POST $GP/servers/audit018-console-test:action \
     -H "Cookie: gameplane_session=<session>; gameplane_csrf=<csrf>" \
     -H "X-Gameplane-CSRF: <csrf>" \
     -H "Content-Type: application/json" \
     -d '{
       "action": "say",
       "params": {"message": "hello\r\nkill @s"}
     }' > /tmp/console-crlf-attempt.json 2>&1
   ```
   Save response and check error message.

3. **Test 512-character limit**:
   ```bash
   python3 << 'PYTHON'
   long_param = "x" * 513
   import json, subprocess
   payload = {
     "action": "say",
     "params": {"message": long_param}
   }
   result = subprocess.run([
     "curl", "-i", "-X", "POST", f"{os.environ.get('GP')}/servers/audit018-console-test:action",
     "-H", f"Cookie: gameplane_session={session}; gameplane_csrf={csrf}",
     "-H", f"X-Gameplane-CSRF: {csrf}",
     "-H", "Content-Type: application/json",
     "-d", json.dumps(payload)
   ], capture_output=True)
   with open("/tmp/console-long-attempt.json", "wb") as f:
     f.write(result.stdout)
   PYTHON
   ```
   Save response.

4. **Verify valid input is accepted**: Send a clean 512-char and sub-512-char string; confirm it processes without error.

5. (Login cost: 1 session + 2 test requests = 1 login, negligible budget impact.)

**Expected**

- Step 2: Request rejected with HTTP 400 and message "parameter \"message\" must not contain control characters" (gameaction/action.go:76).
- Step 3: Request rejected with HTTP 400 and message "parameter \"message\" is too long (max 512)" (gameaction/action.go:78).
- Step 4: Valid input accepted (HTTP 200/202).

**Cleanup**

- `kubectl delete gs audit018-console-test -n gameplane-games`
- Delete `/tmp/console-*.json` evidence files.

**Automatable?**

Yes. Bucket: `api-agent`. Mock the action endpoint; parameterize the CR/LF and long-string test cases.

---

### audit-chain-tamper-detection

**Preconditions**

- Database snapshot tool available: `sqlite3` or similar.
- API is scaled to 0 or 1 (to avoid concurrent audit inserts during tampering).
- Snapshots can be saved to `~/gameplane-audit-018/db-snapshots/` (outside git).
- Cluster has a quiet window (no other audit events being generated).
- API audit tables exist with at least 3 chained rows (hash + prev_hash populated per migration 005).

**Resources created**

- Database snapshots: `~/gameplane-audit-018/db-snapshots/tamper-update.db`, `tamper-delete.db`, `tamper-truncate.db` (off-git).

**Steps**

1. **Scale API to 0 replicas**:
   ```bash
   kubectl scale deployment gameplane-api -n gameplane-system --replicas=0
   sleep 5
   ```

2. **Backup the database** (fresh baseline):
   ```bash
   mkdir -p ~/gameplane-audit-018/db-snapshots
   DBPATH=/path/to/api/database.db  # Determine this from API deployment/PVC
   sqlite3 $DBPATH ".backup ~/gameplane-audit-018/db-snapshots/tamper-baseline.db"
   ```

3. **Generate a few audit rows** (scale back up, trigger API calls, then scale to 0 again):
   ```bash
   kubectl scale deployment gameplane-api -n gameplane-system --replicas=1
   kubectl wait --for=condition=ready pod -l app=gameplane-api -n gameplane-system --timeout=30s
   # Trigger 3+ API calls to generate audit rows (e.g., GET requests from audit018-viewer)
   sleep 10
   kubectl scale deployment gameplane-api -n gameplane-system --replicas=0
   sleep 5
   ```

4. **Tamper variant A: UPDATE a middle row**:
   ```bash
   sqlite3 $DBPATH << 'SQL'
   UPDATE audit_events SET path = '/servers/hacked' WHERE id = 2;
   SQL
   sqlite3 $DBPATH ".backup ~/gameplane-audit-018/db-snapshots/tamper-update.db"
   sqlite3 $DBPATH "SELECT id, path, hash, prev_hash FROM audit_events ORDER BY id;" > /tmp/tamper-update-state.txt
   ```

5. **Restore baseline and tamper variant B: DELETE a middle row**:
   ```bash
   sqlite3 $DBPATH ".restore ~/gameplane-audit-018/db-snapshots/tamper-baseline.db"
   sqlite3 $DBPATH << 'SQL'
   DELETE FROM audit_events WHERE id = 2;
   SQL
   sqlite3 $DBPATH ".backup ~/gameplane-audit-018/db-snapshots/tamper-delete.db"
   sqlite3 $DBPATH "SELECT id, path, hash, prev_hash FROM audit_events ORDER BY id;" > /tmp/tamper-delete-state.txt
   ```

6. **Restore baseline and tamper variant C: Truncate the tail** (delete newest rows):
   ```bash
   sqlite3 $DBPATH ".restore ~/gameplane-audit-018/db-snapshots/tamper-baseline.db"
   sqlite3 $DBPATH "DELETE FROM audit_events WHERE id > (SELECT MAX(id) - 1 FROM audit_events);"
   sqlite3 $DBPATH ".backup ~/gameplane-audit-018/db-snapshots/tamper-truncate.db"
   sqlite3 $DBPATH "SELECT id, path, hash, prev_hash FROM audit_events ORDER BY id;" > /tmp/tamper-truncate-state.txt
   ```

7. **Scale API back up**:
   ```bash
   kubectl scale deployment gameplane-api -n gameplane-system --replicas=1
   kubectl wait --for=condition=ready pod -l app=gameplane-api -n gameplane-system --timeout=30s
   ```

8. **Verify tampering is detected** (for each variant):
   ```bash
   for variant in update delete truncate; do
     curl -s -X POST $GP/admin/audit:verify \
       -H "Cookie: gameplane_session=<admin_session>; gameplane_csrf=<csrf>" \
       -H "X-Gameplane-CSRF: <csrf>" \
       -H "Content-Type: application/json" \
       > /tmp/verify-$variant.json
     cat /tmp/verify-$variant.json | grep -E '"ok"|"firstBadId"|"message"' >> /tmp/verify-results.txt
   done
   ```

9. **Check dashboard tamper banner**: Log in as admin and navigate to /admin/audit; verify a red banner appears indicating tampering (api/internal/audit/audit.go:456-461, VerifyResult marshaled to frontend).

10. (Login cost: 2 admin logins for verify + view, 1 login for checks; **deferred to T031** because requires quiet cluster window and production database access.)

**Expected**

- Variant A (UPDATE): Verify returns `{"ok": false, "firstBadId": 2, "message": "... stored hash does not match its recomputed content (the row was modified) ..."}`.
- Variant B (DELETE): Verify returns `{"ok": false, "firstBadId": 3, "message": "... prev_hash does not match the previous row's hash (a row was inserted, deleted, or reordered) ..."}`.
- Variant C (TRUNCATE tail): Verify returns `{"ok": false, "firstBadId": N, "message": "... (tail truncation detected via audit.head) ..."}` (api/internal/audit/audit.go:535-539, verifyHead).
- Dashboard displays an audit-tampering warning banner when logged in as admin.

**Cleanup**

- Restore baseline snapshot: `sqlite3 $DBPATH ".restore ~/gameplane-audit-018/db-snapshots/tamper-baseline.db"`
- Delete `/tmp/tamper-*.txt` and `/tmp/verify-*.json` evidence files.
- Snapshots remain in `~/gameplane-audit-018/db-snapshots/` for round record.

**Automatable?**

No (requires direct database access, quiet cluster, and production data tampering; deferred to T031).
Alternative: unit test in `api/internal/audit/audit_test.go` mocking row tampering scenarios.

---

### secret-redaction

**Preconditions**

- Test accounts: `audit018-admin` (full access), `audit018-viewer` (read-only).
- Test share link exists: create one via POST /servers/{name}:shares and capture the token.
- Auth provider with secret: create or configure an OIDC provider via /admin/auth/providers/{name}/secret.
- Mod registry API key: configure a provider (e.g., CurseForge) via /admin/registries/{provider}/secret.
- Audit logging is enabled (WithStdoutSink or database capture).

**Resources created**

- Share link: `audit018-share-test` (created via API, token captured).
- Auth provider secret: `audit018-provider-test` (clientSecret stored).
- Registry API key: `audit018-registry-nexus` (apiKey for Nexus Mods, for example).

**Steps**

1. **Create a share link and verify token is returned only on CREATE**:
   ```bash
   SHARE_RESP=$(curl -s -X POST $GP/servers/mc-fabric:shares \
     -H "Cookie: gameplane_session=<admin_session>; gameplane_csrf=<csrf>" \
     -H "X-Gameplane-CSRF: <csrf>" \
     -H "Content-Type: application/json" \
     -d '{"neverExpires": true, "canStart": false}')
   TOKEN=$(echo $SHARE_RESP | jq -r '.token')
   echo $TOKEN > /tmp/share-token.txt
   ```
   Verify token field is present in the response. Record response to evidence.

2. **List shares and verify token is ABSENT**:
   ```bash
   curl -s -X GET $GP/servers/mc-fabric:shares \
     -H "Cookie: gameplane_session=<admin_session>; gameplane_csrf=<csrf>" \
     > /tmp/shares-list.json
   grep -c '"token"' /tmp/shares-list.json > /tmp/token-count.txt  # Should be 0
   ```

3. **Check audit log for share-creation event and verify token is redacted**:
   ```bash
   curl -s -X GET $GP/admin/audit \
     -H "Cookie: gameplane_session=<admin_session>; gameplane_csrf=<csrf>" \
     | jq '.events[] | select(.path | contains(":shares")) | .path' > /tmp/audit-shares-paths.json
   # Verify path shows "<token>" not the actual token value
   grep -o '<token>' /tmp/audit-shares-paths.json
   ```

4. **Create an auth provider and verify clientSecret is NEVER echoed**:
   ```bash
   curl -i -X PUT $GP/admin/auth/providers/audit018-provider-test/secret \
     -H "Cookie: gameplane_session=<admin_session>; gameplane_csrf=<csrf>" \
     -H "X-Gameplane-CSRF: <csrf>" \
     -H "Content-Type: application/json" \
     -d '{"clientSecret": "super-secret-key-12345"}' \
     > /tmp/auth-provider-response.json 2>&1
   # Verify response does NOT contain "super-secret-key-12345"
   grep -i secret /tmp/auth-provider-response.json  # Should only show key names, not values
   ```

5. **GET the auth config and verify secret is ABSENT**:
   ```bash
   curl -s -X GET $GP/admin/config/auth \
     -H "Cookie: gameplane_session=<admin_session>; gameplane_csrf=<csrf>" \
     | jq '.providers[] | select(.name == "audit018-provider-test")' > /tmp/provider-config.json
   # Should contain reference name, not the secret
   ```

6. **Create a mod registry API key and verify it is NEVER echoed**:
   ```bash
   curl -i -X PUT $GP/admin/registries/nexus/secret \
     -H "Cookie: gameplane_session=<admin_session>; gameplane_csrf=<csrf>" \
     -H "X-Gameplane-CSRF: <csrf>" \
     -H "Content-Type: application/json" \
     -d '{"apiKey": "nexus-api-key-abcd1234"}' \
     > /tmp/registry-response.json 2>&1
   # Verify response does NOT contain "nexus-api-key-abcd1234"
   grep -i apikey /tmp/registry-response.json  # Should only show key names
   ```

7. **Verify audit log redacts both share tokens and secrets**:
   ```bash
   kubectl logs -n gameplane-system -l app=gameplane-api --tail=200 | grep -E 'audit|secret|token' > /tmp/api-logs-filtered.txt
   # Manually inspect: share tokens should appear as "<token>", secrets should not appear at all
   ```

8. (Login cost: 1 admin session + 3 read requests = 1 login, negligible budget.)

**Expected**

- Step 1: Response includes `"token": "<actual-token>"`.
- Step 2: List response has `"token"` field absent; all share objects have no token value.
- Step 3: Audit log shows `/shares/<token>` (redacted path, not the literal token).
- Step 4–5: PUT response and GET config contain no `clientSecret` value; only field references like `{"name": "gameplane-auth-audit018-provider-test", "keys": ["clientSecret"]}`.
- Step 6: PUT response contains no `apiKey` value.
- Step 7: API logs show share tokens redacted as `<token>` and secrets completely absent from the output.

**Cleanup**

- Delete share link via `DELETE /servers/{name}/shares/{id}`.
- Delete auth provider secret via `DELETE /admin/auth/providers/{name}/secret`.
- Delete registry API key via `DELETE /admin/registries/{provider}/secret`.
- Delete `/tmp/*-response.json`, `/tmp/*-token.txt`, `/tmp/*-paths.json`, `/tmp/*-logs-filtered.txt` evidence files.

**Automatable?**

Yes. Bucket: `api-auth`. Mock PUT/GET endpoints; parameterize token/secret payloads; validate response bodies do not contain plaintext secrets.
