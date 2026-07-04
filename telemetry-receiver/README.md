# telemetry-receiver

The collection endpoint for Gameplane's anonymous usage telemetry. The
API's reporter (`api/internal/telemetry`) POSTs a tiny JSON payload once
a day when the admin has enabled **Admin Settings → Telemetry → Send
anonymous usage metrics**:

```json
{ "version": "0.2.0-beta.5", "servers": 3, "templates": 7 }
```

No names, namespaces, hostnames, or identifiers of any kind — the
payload is the whole message. The receiver validates it, logs it as a
structured line, and aggregates it into Prometheus metrics. Raw reports
are never stored.

## Behavior

| Route | Method | Response |
|---|---|---|
| `/ingest` | POST | `204` accepted · `400` malformed/unknown fields/negative counts · `401` bad or missing token (when `AUTH_TOKEN` is set) · `413` body over 16 KiB |
| `/metrics` | GET | Prometheus text format |
| `/healthz` | GET | `200 ok` |

Metrics:

```
gameplane_telemetry_reports_total{version="0.2.0-beta.5"}  # reports by reported version
gameplane_telemetry_servers                                 # histogram of GameServer counts
gameplane_telemetry_templates                               # histogram of GameTemplate counts
```

Version strings that don't look like a version (bad charset, > 32 chars)
are counted under `version="invalid"` so hostile input can't explode
label cardinality or leak into the metrics page.

## Configuration (environment variables)

| Variable | Default | Meaning |
|---|---|---|
| `LISTEN_ADDR` | `:8080` | HTTP listen address |
| `AUTH_TOKEN` | *(empty)* | When set, `/ingest` requires an exactly-matching `Authorization` header (compared constant-time). Use the same Secret on the API side (`GAMEPLANE_TELEMETRY_AUTH`). |

## Run via the Helm chart

Set `api.telemetry.receiver.enabled=true` and the chart deploys the
receiver next to the API and points the API's `--telemetry-endpoint` at
it automatically (unless `api.telemetry.endpoint` is set to an external
URL). See `docs/install.md`.

## Run standalone

```sh
docker run --rm -p 8080:8080 \
  -e AUTH_TOKEN='Bearer …' \
  ghcr.io/valgulnecron/gameplane/telemetry-receiver:edge
```

Point any Gameplane install at it with
`--telemetry-endpoint=https://telemetry.example.com/ingest` (or
`GAMEPLANE_TELEMETRY_ENDPOINT`); the admin toggle still gates whether
anything is sent.
