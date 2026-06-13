import { useQuery } from "@tanstack/react-query";
import { Activity } from "lucide-react";

import type { GameTemplate } from "@/types";
import { Servers } from "@/lib/endpoints";
import { rconAvailable } from "@/lib/capabilities";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";

// ServerStatusCard renders the module-declared live metrics
// (spec.capabilities.status.metrics) for the Overview tab. It reads
// GET /servers/{name}/status while the server is running and lines the
// values up against the declared metrics so the layout stays stable even
// before the first reading arrives. Renders nothing when the game
// declares no metrics or has no RCON.
export function ServerStatusCard({
  name,
  tmpl,
  running,
}: {
  name: string;
  tmpl?: GameTemplate;
  running: boolean;
}) {
  const metrics = tmpl?.spec.capabilities?.status?.metrics ?? [];
  const show = metrics.length > 0 && rconAvailable(tmpl);

  const { data: readings } = useQuery({
    queryKey: ["server-status", name],
    queryFn: () => Servers.status(name),
    enabled: show && running,
    refetchInterval: 10_000,
    retry: false,
  });

  if (!show) return null;

  const byId = new Map((readings ?? []).map((r) => [r.id, r]));

  return (
    <Card>
      <CardHeader>
        <CardTitle>Game status</CardTitle>
        <Activity className="h-4 w-4 text-muted" />
      </CardHeader>
      <CardContent>
        <dl className="space-y-2 text-sm">
          {metrics.map((m) => {
            const reading = byId.get(m.id);
            const value = reading?.value?.trim();
            return (
              <div
                key={m.id}
                className="flex items-center justify-between gap-2 rounded-md border border-border bg-surface/60 px-3 py-2"
              >
                <dt className="text-muted">{m.displayName}</dt>
                <dd className="font-mono text-fg">
                  {value ? (
                    <>
                      {value}
                      {m.unit ? <span className="text-muted"> {m.unit}</span> : null}
                    </>
                  ) : (
                    <span className="text-muted">{running ? "—" : "offline"}</span>
                  )}
                </dd>
              </div>
            );
          })}
        </dl>
      </CardContent>
    </Card>
  );
}
