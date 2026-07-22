import { Moon } from "lucide-react";

import type { GameServer } from "@/types";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { formatRelative, capitalize } from "@/lib/utils";

export function ServerSleepCard({ gs }: { gs?: GameServer }) {
  const spec = gs?.spec.idle;
  const status = gs?.status?.idle;

  // Render nothing unless idle is enabled or there's current status. A
  // disabled-but-materialized spec (enabled: false) must not render an empty
  // card; a status left over from before the disable took effect still
  // should, so that transient window isn't invisible either.
  if (!spec?.enabled && !status) return null;

  const asleep = status?.asleep === true;
  const emptySince = status?.emptySince;
  const asleepSince = status?.asleepSince;
  const lastWakeTime = status?.lastWakeTime;
  const reason = status?.reason;
  const afterMinutes = spec?.afterMinutes ?? 30;
  const wakeWindows = spec?.wakeWindows ?? [];
  // The operator (gameserver_idle.go's idleDecide/idleEligible) is
  // authoritative for every string status.idle.reason can hold; this is the
  // only one that means the sleep trigger can never fire. Every other reason
  // — busy, wrong phase, stale heartbeat, players online, counting down, or
  // just woken — is a normal, working state and must not read as broken.
  const neverSleeps = reason === "this game reports no player count";

  return (
    <Card>
      <CardHeader>
        <CardTitle>Sleep</CardTitle>
        <Moon className="h-4 w-4 text-muted" />
      </CardHeader>
      <CardContent>
        <dl className="space-y-3 text-sm">
          {/* Asleep state */}
          {asleep && (
            <>
              <div className="rounded-md border border-border bg-surface/60 px-3 py-2">
                <dt className="text-muted">State</dt>
                <dd className="font-mono text-fg">Asleep</dd>
              </div>
              {asleepSince && (
                <div className="rounded-md border border-border bg-surface/60 px-3 py-2">
                  <dt className="text-muted">Asleep since</dt>
                  <dd className="font-mono text-fg">{formatRelative(asleepSince)}</dd>
                </div>
              )}
              {lastWakeTime && (
                <div className="rounded-md border border-border bg-surface/60 px-3 py-2">
                  <dt className="text-muted">Last woken</dt>
                  <dd className="font-mono text-fg">{formatRelative(lastWakeTime)}</dd>
                </div>
              )}
              {/* A wake window that fails to parse keeps the server asleep
                  forever with no other signal — the operator deliberately
                  doesn't fail the reconcile for it, so this is the only place
                  that surfaces it. */}
              {reason && reason !== "asleep (no players)" && (
                <div className="rounded-md bg-surface/40 px-3 py-2 text-xs text-warning">
                  {capitalize(reason)}
                </div>
              )}
              <div className="rounded-md bg-surface/40 px-3 py-2 text-xs text-muted">
                Waking takes normal boot time before players can connect.
              </div>
            </>
          )}

          {/* Counting down state */}
          {!asleep && emptySince && (
            <>
              <div className="rounded-md border border-border bg-surface/60 px-3 py-2">
                <dt className="text-muted">State</dt>
                <dd className="font-mono text-fg">{reason ? capitalize(reason) : "Counting down"}</dd>
              </div>
              <div className="rounded-md border border-border bg-surface/60 px-3 py-2">
                <dt className="text-muted">Empty since</dt>
                <dd className="font-mono text-fg">{formatRelative(emptySince)}</dd>
              </div>
            </>
          )}

          {/* Active, not counting down: the operator's reason string IS the
              state (players online, stale heartbeat, wrong phase, busy, just
              woken, …) — only the one no-player-count reason means the
              trigger can never fire, so that's the only case worth a distinct
              label and an explanatory sub-line. */}
          {!asleep && !emptySince && reason && (
            <>
              <div className="rounded-md border border-border bg-surface/60 px-3 py-2">
                <dt className="text-muted">State</dt>
                <dd className="font-mono text-fg">
                  {neverSleeps ? "Will never sleep" : capitalize(reason)}
                </dd>
              </div>
              {neverSleeps && (
                <div className="rounded-md bg-surface/40 px-3 py-2 text-xs text-muted">
                  {capitalize(reason)}
                </div>
              )}
            </>
          )}

          {/* Just-enabled, not yet reconciled: none of the three blocks above
              apply, but staying silent here reads as the change not having
              taken effect. */}
          {spec?.enabled && !asleep && !emptySince && !reason && (
            <div className="rounded-md border border-border bg-surface/60 px-3 py-2">
              <dt className="text-muted">State</dt>
              <dd className="font-mono text-muted">Waiting for the operator</dd>
            </div>
          )}

          {/* Configuration */}
          {spec?.enabled && (
            <>
              <div className="rounded-md border border-border bg-surface/60 px-3 py-2">
                <dt className="text-muted">Sleep after</dt>
                <dd className="font-mono text-fg">
                  {afterMinutes} minute{afterMinutes === 1 ? "" : "s"}
                </dd>
              </div>

              <div className="space-y-1">
                <dt className="text-muted">Wake windows</dt>
                {wakeWindows.length > 0 ? (
                  <dd className="space-y-1">
                    {wakeWindows.map((cron, i) => (
                      <div
                        key={i}
                        className="rounded-md border border-border bg-surface/60 px-3 py-2 font-mono text-fg"
                      >
                        {cron}
                      </div>
                    ))}
                  </dd>
                ) : (
                  <dd className="text-xs text-muted">Not configured</dd>
                )}
              </div>
            </>
          )}
        </dl>
      </CardContent>
    </Card>
  );
}
