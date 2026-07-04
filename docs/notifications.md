# Notifications

Gameplane can push cluster events — a server going unhealthy, a backup or
restore finishing — to external sinks: **Discord**, **Slack**, **SMTP**
(email), or a **generic webhook**. Sinks are configured in the dashboard
under **Admin Settings → Notifications** (persisted via
`PUT /admin/config/notifications`); their credentials live in Kubernetes
Secrets, never in the database.

Like the [audit sinks](install.md#audit-log), notifications **mirror, they
never gate**: delivery is best-effort from a bounded in-memory queue, and a
slow or down sink never blocks reconciliation or an API request.

## Events

| Event | Fires when | On by default |
|---|---|---|
| `server.unhealthy` | a GameServer's phase escalates to `Failed` (bad image, persistent crash-loop, non-zero exit), or a previously-healthy server loses its agent heartbeat | yes |
| `server.recovered` | a server with an outstanding `server.unhealthy` alert becomes healthy again — ordinary starts never fire it | yes |
| `backup.failed` | a Backup enters phase `Failed` | yes |
| `backup.succeeded` | a Backup enters phase `Succeeded` | no |
| `restore.failed` | a Restore enters phase `Failed` | yes |
| `restore.succeeded` | a Restore enters phase `Succeeded` | no |

A sink with no explicit event filter receives the defaults above (failures
plus the paired recovery). User-intended transitions — stopping, suspending —
never notify.

**Restart semantics:** only transitions observed while the API pod is running
are notified. A backup that starts *and* fails entirely while the API is down
is missed by the sinks (it is still visible in the dashboard, `kubectl`, and
the `GameplaneBackupFailed` Prometheus alert). There is deliberately no
cross-restart watermark.

## Sink credentials (Secrets)

Each sink references a Secret by name (`configRef`). The Secret must live in
the **control-plane namespace** (where the API runs) and must carry the label
`gameplane.local/notification-sink: "true"` — the API refuses to read
Secrets without it, so granting someone `config:manage` does not let them
exfiltrate arbitrary control-plane Secrets by pointing a sink at one.

Expected keys per kind:

| Kind | Keys |
|---|---|
| `discord` | `url` — the Discord webhook URL |
| `slack` | `url` — the Slack incoming-webhook URL |
| `webhook` | `url`; optional `authorization` (sent verbatim as the `Authorization` header) |
| `smtp` | `host`; `from`; `to` (comma-separated); optional `port` (default `587`), `username` + `password` (AUTH PLAIN), `tls` = `starttls` (default) \| `implicit` \| `none` |

```sh
# A Discord sink named "team-alerts":
kubectl -n gameplane-system create secret generic team-alerts \
  --from-literal=url='https://discord.com/api/webhooks/…'
kubectl -n gameplane-system label secret team-alerts \
  gameplane.local/notification-sink=true

# An SMTP sink:
kubectl -n gameplane-system create secret generic ops-mail \
  --from-literal=host=smtp.example.com \
  --from-literal=from=gameplane@example.com \
  --from-literal=to=ops@example.com \
  --from-literal=username=gameplane \
  --from-literal=password='…'
kubectl -n gameplane-system label secret ops-mail \
  gameplane.local/notification-sink=true
```

Note: `net/smtp` refuses to send AUTH PLAIN credentials over an unencrypted
connection, so `tls: none` only works for unauthenticated relays.

## Payloads

- **discord** — one embed, red for failures / green for recoveries and
  successes, timestamped.
- **slack** — a plain `{"text": …}` message (works with every
  incoming-webhook variant and most compatibles, e.g. Mattermost).
- **webhook** — the full structured event:

  ```json
  {
    "type": "backup.failed",
    "ts": "2026-07-04T10:12:00Z",
    "kind": "Backup",
    "name": "nightly",
    "namespace": "gameplane-games",
    "message": "restic exited with code 1",
    "instance": "prod",
    "test": false
  }
  ```

- **smtp** — a plain-text mail with the same fields.

`instance` is **Admin Settings → General → Instance name**, so alerts from
different Gameplane installs are tellable apart.

## Testing a sink

`POST /admin/notifications/sinks/{name}/test` (permission `config:manage`)
delivers a synthetic event to the *persisted* sink synchronously and returns
the real outcome: `200 {"delivered":true}`, `404` (unknown sink), `422`
(no `configRef` yet), or `502` with the delivery error. The endpoint is
rate-limited (~12/min per IP) since every hit dials out.

## Delivery semantics and observability

Failed deliveries are retried twice (2 s, then 8 s backoff) on network
errors and 5xx; a 4xx response (revoked webhook, bad token) is not retried.
Outcomes are counted at `/metrics`:

```
gameplane_notify_deliveries_total{kind="discord|slack|smtp|webhook|queue",
                                  result="sent|failed|dropped|skipped_no_secret"}
```

Watch `failed`/`dropped`/`skipped_no_secret` — a growing delta means alerts
admins are counting on aren't arriving. Sink dials are guarded by the same
SSRF dial-guard the operator uses for module sources; see
[security](security.md#notifications).
