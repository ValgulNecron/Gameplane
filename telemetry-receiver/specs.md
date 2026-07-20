# telemetry-receiver — Specification

**Status:** beta (v0.2.0-beta.7)  
**Module / package:** github.com/ValgulNecron/gameplane/telemetry-receiver  
**Go version:** 1.25

## Purpose

Optional collector for the anonymous usage telemetry the API reports daily. Provides a public or in-cluster HTTP endpoint that accepts tiny, identifying-information-free JSON reports, validates them, and aggregates the metrics without retaining raw data.

## Responsibilities

- **HTTP server**: three routes — `/ingest` (POST), `/metrics` (GET), `/healthz` (GET)
- **Payload validation**: strict JSON schema (`{version, servers, templates}` only); rejects unknown fields, malformed JSON, oversized bodies (>16 KiB), and negative counts
- **Version sanitization**: version strings matching `^[A-Za-z0-9][A-Za-z0-9._+-]{0,31}$` are passed through; anything else is bucketed as `"invalid"` to prevent label cardinality explosion
- **Authentication**: optional constant-time token validation when `AUTH_TOKEN` is set
- **Metrics aggregation**: three in-memory Prometheus metrics — `gameplane_telemetry_reports_total` (counter by version), `gameplane_telemetry_servers` (histogram), `gameplane_telemetry_templates` (histogram)
- **Structured logging**: logs each accepted report via `log/slog` with version/servers/templates fields

## Non-goals / boundaries

Stores **nothing** persistent — raw reports are never written to disk or database. The API's opt-in (`Admin Settings → Telemetry`) is the privacy gate; this receiver assumes incoming data is already anonymized at the source. Refer to `telemetry-receiver/README.md` for configuration, environment variables, and run instructions.

## Directory & package layout

Single package `main`:

```
telemetry-receiver/
├── main.go             # HTTP server, config, ingest handler, metrics setup
├── main_test.go        # Unit tests (validation, auth, shutdown, config loading)
├── go.mod / go.sum     # Dependencies: prometheus/client_golang only (+ transitives)
├── .testcoverage.yml   # Coverage gate: 70% total
├── Dockerfile          # Multi-stage: build→distroless:nonroot
├── README.md           # User-facing: behavior table, config, run examples
└── .gitignore          # Standard Go ignores
```

## External interface / contracts

### HTTP routes

| Route | Method | Success response | Failures |
|-------|--------|------------------|----------|
| `/ingest` | POST | `204 No Content` | `400` malformed JSON / unknown fields / negative counts; `401` missing/wrong Authorization (when `AUTH_TOKEN` set); `413` body >16 KiB |
| `/metrics` | GET | Prometheus text format | Never errors |
| `/healthz` | GET | `200 OK`, body `"ok"` | Never errors |

### Ingest payload

```json
{ "version": "0.2.0-beta.7", "servers": 3, "templates": 7 }
```

- **version** (string, required): free-form version string; sanitized to `"invalid"` if it doesn't match `^[A-Za-z0-9][A-Za-z0-9._+-]{0,31}$` (printable ASCII, at most 32 chars)
- **servers** (integer, required): GameServer count; must be ≥ 0
- **templates** (integer, required): GameTemplate count; must be ≥ 0
- Unknown fields cause rejection

### Prometheus metrics

```
gameplane_telemetry_reports_total{version="0.2.0-beta.7"}  # counter, by version label
gameplane_telemetry_servers_bucket{le="…"}                 # histogram (buckets: 0,1,2,5,10,25,50,100,250,+Inf)
gameplane_telemetry_servers_sum                            # histogram sum
gameplane_telemetry_servers_count                          # histogram count
gameplane_telemetry_templates_bucket{le="…"}               # histogram (same buckets)
gameplane_telemetry_templates_sum                          # histogram sum
gameplane_telemetry_templates_count                        # histogram count
```

## Key invariants

- **No persistent storage**: all metrics are in-memory, ephemeral across restart
- **Version label bounded**: hostile input (e.g., `<script>…</script>`, 200+ chars) becomes label `version="invalid"` — impossible for unvalidated input to explode label cardinality or leak into the metrics page
- **Strict validation**: JSON decoder enforces exact field list (no extras, no renames); negative counts immediately rejected; body size capped at 16 KiB
- **Authentication is optional**: if `AUTH_TOKEN` is empty, `/ingest` is public; otherwise, the exact `Authorization` header must match (constant-time comparison defeats timing attacks)
- **Anonymity by contract**: payload contains no names, namespaces, hostnames, cluster IDs, or pod IPs — privacy is enforced upstream (by the API's telemetry reporter), and this receiver has no way to add PII
- **Graceful shutdown**: catches SIGINT/SIGTERM, shuts down HTTP server with 5s context timeout

## Data & persistence

None. In-memory Prometheus metrics only. On restart, all counters and histograms reset.

## Security considerations

- **Input boundary**: JSON decoder with `DisallowUnknownFields` + explicit length validation prevents injection or DoS via malformed payloads
- **Label safety**: version string sanitization with regex prevents arbitrary label values and protects the metrics endpoint (a common Prometheus DoS vector)
- **Authentication**: constant-time token comparison (via `crypto/subtle.ConstantTimeCompare`) is timing-attack-resistant; token transmitted in plaintext (use TLS in production)
- **Minimal dependencies**: only `prometheus/client_golang` — no database, no serialization frameworks, no dynamic code loading
- **Container hardening**: distroless image, nonroot user (UID 65532), no shell or package manager
- **Privacy by design**: no storage means no breach risk; opt-in flag at the API side (the receiver is point-blank trustworthy — verify the API's admin toggle gating instead)

## Testing & coverage

Coverage gate: **70%** (`.testcoverage.yml`).

Test suite covers:
- **Validation**: malformed JSON, unknown fields, negative counts, oversized bodies all rejected with correct HTTP status
- **Authentication**: missing auth returns 401; wrong token returns 401; correct token passes
- **Label sanitization**: hostile version strings bucketed as `"invalid"`, not leaked raw into metrics
- **Metrics side effects**: accepted reports increment counters and populate histograms
- **Config loading**: environment variables override defaults
- **Graceful shutdown**: context cancellation shuts down the server cleanly
- **HTTP method guards**: non-POST to `/ingest`, non-GET to `/metrics`/`/healthz` return 405

Untested (main process wiring): `main()` signal handling and server startup errors — not unit-testable without spawning goroutines, deferred to integration tests.

## References

- **`telemetry-receiver/README.md`** — configuration table, environment variables, behavior examples, run via Helm or Docker
- **`api/internal/telemetry`** — the API's reporter that POSTs to this endpoint
- **`charts/gameplane/`** — Helm chart integration (`api.telemetry.receiver.enabled`)
- **`CLAUDE.md`** — project architecture summary
