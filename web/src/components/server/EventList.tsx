import type { NormalizedServerEvent } from "@/lib/events";
import { formatRelative } from "@/lib/utils";

function EventDot({ kind }: { kind: NormalizedServerEvent["kind"] }) {
  const color = {
    info: "bg-primary",
    warn: "bg-warning",
    error: "bg-danger",
  }[kind];
  return <span className={`mt-1.5 inline-block h-2 w-2 rounded-full ${color}`} />;
}

export function EventList({
  events,
  emptyMessage = "No events yet.",
}: {
  events: NormalizedServerEvent[];
  emptyMessage?: string;
}) {
  return (
    <>
      {events.length === 0 && (
        <div className="px-6 pb-6 text-sm text-muted">
          {emptyMessage}
        </div>
      )}
      <ul className="divide-y divide-border">
        {events.map((e) => (
          <li key={e.id} className="flex items-start gap-3 px-6 py-3">
            <EventDot kind={e.kind} />
            <div className="min-w-0 flex-1">
              <div className="text-sm text-fg">{e.message}</div>
              <div className="pt-0.5 text-xs text-muted">
                {e.source ?? "system"}
              </div>
            </div>
            <div className="text-xs text-muted">{formatRelative(e.ts)}</div>
          </li>
        ))}
      </ul>
    </>
  );
}
