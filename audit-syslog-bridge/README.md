# audit-syslog-bridge

A tiny, generic **HTTP-JSON → syslog** relay. It accepts an HTTP `POST` whose
body is a single JSON document and re-emits that document as an
[RFC 5424](https://www.rfc-editor.org/rfc/rfc5424) syslog message to a
configured collector (TCP, TCP over TLS, or UDP).

It exists so Gameplane's [audit webhook sink](../docs/install.md#audit-log) can
reach a syslog/SIEM endpoint, but it is deliberately **schema-agnostic**: it
forwards the received body verbatim as the syslog `MSG`, so it works for any
JSON-webhook source, not just audit events. Nothing in it is Gameplane-specific.

## Behavior

- `POST /` with a JSON body → forwarded as one syslog record. Returns `204`.
  - TCP uses RFC 6587 octet-counting framing (`<len> <msg>`); UDP sends the bare
    message as one datagram.
  - The receive time becomes the syslog `TIMESTAMP`; the body becomes `MSG`
    (collapsed to a single line).
- `GET /healthz` → `200 ok` (for Kubernetes probes).
- Other methods → `405`; empty body → `400`; a configured-but-mismatched auth
  header → `401`; a failed forward to the collector → `502`.

The forwarder reuses a lazily-dialed connection and reconnects once on a write
error.

## Configuration (environment variables)

| Var | Default | Meaning |
|---|---|---|
| `LISTEN_ADDR` | `:8514` | HTTP listen address |
| `SYSLOG_ADDR` | — (**required**) | collector `host:port` |
| `SYSLOG_NETWORK` | `tcp` | `tcp` or `udp` |
| `SYSLOG_TLS` | `false` | wrap a `tcp` connection in (verified) TLS |
| `APP_NAME` | `gameplane-audit` | RFC 5424 `APP-NAME` |
| `FACILITY` | `local0` | `kern`/`user`/`daemon`/`auth`/…/`local0`–`local7` |
| `SEVERITY` | `info` | `emerg`/`alert`/`crit`/`err`/`warning`/`notice`/`info`/`debug` |
| `SYSLOG_HOSTNAME` | OS hostname | RFC 5424 `HOSTNAME` field |
| `AUTH_HEADER` | — | if set, inbound requests must carry this exact `Authorization` header |

## Run via the Helm chart

The chart deploys this automatically when you set
`api.audit.webhook.syslogBridge.enabled=true` and a collector address — see the
[install docs](../docs/install.md#audit-log). Standalone:

```sh
docker run -e SYSLOG_ADDR=syslog.example:514 \
  ghcr.io/valgulnecron/gameplane/audit-syslog-bridge:edge
```
