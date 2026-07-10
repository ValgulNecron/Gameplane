import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import type { GameServer } from "@/types";
import { Servers } from "@/lib/endpoints";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { EventList } from "@/components/server/EventList";
import { mapServerEvent, type NormalizedServerEvent } from "@/lib/events";
import { cn } from "@/lib/utils";

type FilterType = "all" | "info" | "warnings";

export function EventsTab({
  name,
  ns,
  gs,
}: {
  name: string;
  ns?: string;
  gs?: GameServer;
}) {
  const [filter, setFilter] = useState<FilterType>("all");

  const { data: rawEvents } = useQuery({
    queryKey: ["events", name, ns],
    queryFn: () => Servers.events(name, ns),
    enabled: !!name,
    refetchInterval: gs?.status?.phase === "Running" ? 30_000 : 5_000,
    retry: false,
  });

  const events: NormalizedServerEvent[] = (
    Array.isArray(rawEvents) ? rawEvents : []
  ).map(mapServerEvent);

  const filteredEvents = events.filter((e) => {
    if (filter === "all") return true;
    if (filter === "info") return e.kind === "info";
    if (filter === "warnings") return e.kind === "warn" || e.kind === "error";
    return true;
  });

  return (
    <div className="space-y-6 p-6">
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <CardTitle>Events</CardTitle>
            <div className="flex gap-1">
              {["all", "info", "warnings"].map((f) => {
                const filterValue = f as FilterType;
                const label =
                  filterValue === "all"
                    ? "All"
                    : filterValue === "info"
                      ? "Info"
                      : "Warnings";
                return (
                  <button
                    key={filterValue}
                    onClick={() => setFilter(filterValue)}
                    className={cn(
                      "rounded px-3 py-1.5 text-xs font-medium transition-colors",
                      filter === filterValue
                        ? "bg-primary text-primary-fg"
                        : "bg-surface text-muted hover:text-fg",
                    )}
                  >
                    {label}
                  </button>
                );
              })}
            </div>
          </div>
        </CardHeader>
        <CardContent className="px-0">
          <EventList
            events={filteredEvents}
            emptyMessage={
              filter === "all"
                ? "No events yet."
                : filter === "info"
                  ? "No info events."
                  : "No warnings or errors."
            }
          />
        </CardContent>
      </Card>
    </div>
  );
}
