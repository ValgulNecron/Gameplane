# audit-syslog-bridge — Specification

**Status:** beta (v0.2.0-beta.7)  
**Module / package:** `github.com/ValgulNecron/gameplane/audit-syslog-bridge`

## Purpose

Optional, schema-agnostic HTTP-JSON → RFC 5424 syslog relay. Sits behind the API's audit webhook sink (`api.audit.webhook.syslogBridge.enabled`) to forward audit events to a syslog/SIEM collector, but works for *any* JSON webhook source — nothing is Gameplane-specific.

## Responsibilities

- Listen on HTTP, accept `POST /` with a JSON body, forward it as one RFC 5424 syslog record
- Encode the received timestamp, hostname, app-name, and collapse the JSON body to a single-line MSG field
- Frame the message for the wire: RFC 6587 octet-counting (`<len> <msg>`) for TCP (reliable stream framing), bare message for UDP (one datagram)
- Maintain a lazily-dialed, reused syslog connection; reconnect once on write error
- Bound each write with a deadline (default 5s) to prevent a hung collector from blocking the HTTP handler indefinitely
- Provide `/healthz` for Kubernetes probes
- Enforce optional bearer-token auth on POST /

## Non-goals / boundaries

NOT Gameplane-specific — forwards the received JSON body verbatim as the syslog MSG, so it does not parse or understand the Gameplane audit schema. Works equally well as a generic webhook-to-syslog relay for any source. See README.md for config/run instructions.

## Directory & package layout

Single flat package (`main`): `main.go` (relay + config + server logic), `bridge_test.go` (unit tests covering HTTP handler, syslog framing, config validation, TCP/UDP forwarding, auth, reconnection), `go.mod` (workspace-linked, stdlib-only), `Dockerfile`, `README.md`.

## External interface / contracts

**HTTP endpoints:**
- `POST /` — forward JSON body as syslog record
  - Request: any Content-Type, JSON body up to 64 KiB
  - Response: `204 No Content` on success; `400 Bad Request` if body empty/unreadable; `401 Unauthorized` if `AUTH_HEADER` is set and does not match; `405 Method Not Allowed` for GET/other; `502 Bad Gateway` if forward to collector fails
- `GET /healthz` — Kubernetes probe
  - Response: `200 OK` with body "ok"

**Transports:**
- TCP (RFC 6587 octet-counting framing): `<decimal-length> <message>`, reliable for audit trails
- UDP: bare message, one datagram per record
- TLS: wraps TCP in verified TLS 1.2+; `SYSLOG_TLS=true` (requires `SYSLOG_NETWORK=tcp`)

**Auth:**
- Optional bearer token: if `AUTH_HEADER` environment variable is set, the HTTP request `Authorization` header must match exactly (constant-time comparison); absence of the env var = any POST accepted

## Key invariants

- Forwards the received JSON body verbatim (schema-agnostic; does not parse, validate, or understand audit events)
- Prefers TCP over UDP for audit/compliance trails (UDP has no delivery confirmation; TCP surfaces a dead collector as a 502)
- Connection reuse: lazily dials once, then reuses; on write error, closes and reconnects once before surfacing the error
- Write deadline per frame (5s default) prevents a collector that accepts but does not drain from blocking indefinitely and wedging the handler behind the connection mutex
- RFC 5424 compliance: formats message as `<PRI>1 TIMESTAMP HOSTNAME APP-NAME PROCID MSGID STRUCTURED-DATA MSG`; collapses embedded newlines/CRs to spaces so each syslog record is one line; PROCID, MSGID, STRUCTURED-DATA are "-"

## Dependencies

**Go 1.25** (workspace-linked to `go.work` alongside `operator/`, `api/`, `agent/`, etc.)

**Imports:** stdlib only (`crypto/tls`, `net`, `net/http`, `log/slog`, `sync`, `time`, `context`, etc.). No external dependencies.

## Security considerations

- **Auth boundary:** optional bearer token (`AUTH_HEADER` env var) gates who may inject records (constant-time comparison; absence = open)
- **TLS transport:** `SYSLOG_TLS=true` wraps the TCP connection in verified TLS 1.2+, protecting audit events in transit
- **Trust boundary:** API ← audit-syslog-bridge ← SIEM/syslog collector. The API rate-limits webhook submissions; the bridge forwards verbatim and is stateless, so it does not amplify or replay events
- **DoS mitigation:** inbound body capped at 64 KiB; read and write deadlines bound goroutines so a misbehaving client/collector cannot wedge the process

## Testing & coverage

**Coverage gate:** 70% (`.testcoverage.yml`). Tests cover HTTP handler (methods, auth, empty body), RFC 5424 framing, facility/severity enum validation, TCP/UDP forward, connection reuse, write deadline enforcement, forward-failure 502, graceful shutdown on context cancel, and env-var defaults. Uncovered: `main()`/`run()` process signal handling (ListenAndServe + SIGTERM), which is not unit-testable; ~30% gap is acceptable and noted in the gate comment.

## References

- `audit-syslog-bridge/README.md` — behavior table, config env-vars, transport tradeoffs, run instructions
- `docs/security.md` — audit integrity, threat model, pre-auth privacy (login page anonymity is separate; the bridge sits behind auth)
- `docs/install.md#audit-log` — Helm values to enable syslog-bridge, auth-header Secret wiring
- API audit webhook sink (`api/internal/handlers/audit.go`, `api/internal/notify/`) — calls `POST http://syslog-bridge-svc:8514/` with audit events
