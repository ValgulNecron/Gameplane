import { Moon } from "lucide-react";

import type { GameServer } from "@/types";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { formatRelative, capitalize } from "@/lib/utils";

export function ServerSleepCard({ gs }: { gs?: GameServer }) {
  const spec = gs?.spec.idle;
  const status = gs?.status?.idle;

  // Render nothing unless idle is configured or has current status.
  if (!spec && !status) return null;

  const asleep = status?.asleep === true;
  const emptySince = status?.emptySince;
  const asleepSince = status?.asleepSince;
  const lastWakeTime = status?.lastWakeTime;
  const reason = status?.reason;
  const afterMinutes = spec?.afterMinutes ?? 30;
  const wakeWindows = spec?.wakeWindows ?? [];

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
                <dd className="font-mono text-fg">{reason ?? "Counting down"}</dd>
              </div>
              <div className="rounded-md border border-border bg-surface/60 px-3 py-2">
                <dt className="text-muted">Empty since</dt>
                <dd className="font-mono text-fg">{formatRelative(emptySince)}</dd>
              </div>
            </>
          )}

          {/* Will never sleep state */}
          {!asleep && !emptySince && reason && (
            <>
              <div className="rounded-md border border-border bg-surface/60 px-3 py-2">
                <dt className="text-muted">State</dt>
                <dd className="font-mono text-fg">Will never sleep</dd>
              </div>
              <div className="rounded-md bg-surface/40 px-3 py-2 text-xs text-muted">
                {capitalize(reason)}
              </div>
            </>
          )}

          {/* Configuration */}
          {spec?.enabled && (
            <>
              <div className="rounded-md border border-border bg-surface/60 px-3 py-2">
                <dt className="text-muted">Sleep after</dt>
                <dd className="font-mono text-fg">{afterMinutes} minutes</dd>
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
