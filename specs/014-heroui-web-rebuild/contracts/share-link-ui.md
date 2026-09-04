# Contract: share-link surfaces (slice 5, new build)

The API is complete (`api/internal/handlers/shares.go`, mounted in `api/cmd/main.go`); the dashboard has nothing. These surfaces are built directly on HeroUI, following the seven designed frames and three designed dialogs. Field names and response shapes are read from the handler and `api/specs.md` at implementation time; this contract fixes the surfaces, the states and the privacy rules, not JSON keys.

## API consumed

| Purpose | Method and path | Auth |
|---|---|---|
| Create link for a server | `POST /servers/{name}:shares` | session, server write |
| List links | `GET /servers/{name}:shares` | session |
| Revoke link | `DELETE /servers/{name}/shares/{id}` | session |
| Resolve a link (public page load) | `GET /shares/{token}` | none, rate-limited (`auth.ShareLimiter`) |
| Start the server from a link | `POST /shares/{token}/start` | none, rate-limited, only if the link allows start |

Multi-cluster: the authenticated calls thread `?cluster=` like every other endpoint namespace; the public calls do not.

## Surfaces

### Settings · Share links (`xCJlu`, empty state `dQV9N`)

- New section in `web/src/routes/tabs/Settings.tsx`'s `SECTIONS`, file `tabs/settings/ShareLinks.tsx`, between RBAC & access and Danger zone as drawn.
- Table of links: label, created, expires, capabilities (view / can start), status; per-row Revoke opening `S7SCDc` (AlertDialog danger).
- Create button opens `atqRh` (Modal: label, expiry, allow-start switch); success opens `VM7ro` showing the full URL once with a copy action and the warning that it will not be shown again.
- Empty state per `dQV9N`.
- Visible only with the same permission the API enforces for creation; viewers see the list read-only if the API allows the list call, otherwise the section is hidden. Determined from the handler, not assumed.

### Public share page (`C2LQE4 q31B6w qFLfB EcoGD epZO2`)

- Route path `/share/$token` per OD-1 (Settled 2026-09-03), registered in `web/src/router/tree.tsx` outside the authenticated layout, no sidebar, no top bar.
- States from the resolve response: **Up** (address, port, players if exposed), **Asleep and can start** (Start button → `POST …/start` → **Starting** with polling), **Asleep view only**, **Invalid or expired** (neutral copy, no hint whether the token ever existed).
- Honours the stored appearance preference; no toggle.

## Privacy rules (FR-005, rule 3)

- The public page renders only what the resolve response returns for that token. It never shows cluster name, namespace, version, other servers, user names or counts.
- Invalid, expired and revoked tokens produce the same page and the same copy.
- No telemetry, no error detail beyond the neutral copy; the API's rate-limit response maps to the same invalid state.
- The public route is excluded from the authenticated `AppLayout` and from the 401 redirect logic.

## Tests

- Vitest for the section, both dialogs, each public state, and the neutral-error rule (invalid vs expired vs rate-limited render identically).
- Playwright mock spec for create → copy → revoke, and for the public page states.
- Playwright live spec: create a link for the seeded server, open it signed out, revoke it, reopen and get the invalid page.
- `web/specs.md` gains a Share links entry under Routing and under Settings sub-sections.
