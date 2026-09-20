import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Button, Card } from "@heroui/react";
import type { GameServer } from "@/types";
import { Servers } from "@/lib/endpoints";
import { EventList } from "@/components/server/EventList";
import { mapServerEvent } from "@/lib/events";

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

  const filteredEvents = useMemo(() => {
    const events = (Array.isArray(rawEvents) ? rawEvents : []).map(mapServerEvent);
    return events.filter((e) => {
      if (filter === "all") return true;
      if (filter === "info") return e.kind === "info";
      if (filter === "warnings") return e.kind === "warn" || e.kind === "error";
      return true;
    });
  }, [rawEvents, filter]);

  return (
    <div className="space-y-6 p-6">
      <Card className="border border-border bg-surface">
        <Card.Header className="flex flex-col gap-4">
          <div className="flex items-center justify-between">
            <h2 className="text-base font-semibold text-foreground">Events</h2>
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
                  <Button
                    key={filterValue}
                    size="sm"
                    variant={filter === filterValue ? "primary" : "ghost"}
                    onClick={() => setFilter(filterValue)}
                    className="h-7 rounded-full px-3 text-xs font-medium"
                  >
                    {label}
                  </Button>
                );
              })}
            </div>
          </div>
        </Card.Header>
        <Card.Content className="px-0 py-0">
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
        </Card.Content>
      </Card>
    </div>
  );
}
