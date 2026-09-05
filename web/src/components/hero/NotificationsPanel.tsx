import { useState, useEffect } from "react";
import type { JSX } from "react";
import { useQueryClient } from "@tanstack/react-query";
import {
  Popover,
  Button,
  Card,
} from "@heroui/react";
import { Bell } from "lucide-react";
import { openEventStream, queryKeyForKind, type GameplaneEvent } from "@/lib/sse";

interface Notice {
  id: number;
  text: string;
  at: string;
}

// No props today — the component owns its own SSE subscription and local
// state (see the doc comment below). Kept as a named type so callers and
// future props have a stable place to land.
export type NotificationsPanelProps = Record<string, never>;

/**
 * NotificationsPanel opens the /events SSE stream: each watch event
 * invalidates the matching TanStack Query cache (so views refresh without
 * waiting for the next poll) and is buffered into a dropdown panel. The
 * badge shows the unread count.
 */
export function NotificationsPanel(): JSX.Element {
  const qc = useQueryClient();
  const [notices, setNotices] = useState<Notice[]>([]);
  const [open, setOpen] = useState(false);
  const [unread, setUnread] = useState(0);

  useEffect(() => {
    let seq = 0;
    const dispose = openEventStream({
      onEvent: (ev: GameplaneEvent) => {
        const key = queryKeyForKind(ev.kind);
        if (key) void qc.invalidateQueries({ queryKey: key });
        const name = ev.object?.metadata?.name ?? "";
        const verb = ev.eventType.toLowerCase();
        seq += 1;
        const notice: Notice = {
          id: seq,
          text: `${verb} ${ev.kind.replace(/s$/, "")} ${name}`.trim(),
          at: new Date().toLocaleTimeString(),
        };
        setNotices((prev) => [notice, ...prev].slice(0, 50));
        setUnread((u) => u + 1);
      },
    });
    return dispose;
  }, [qc]);

  return (
    <Popover
      isOpen={open}
      onOpenChange={(newOpen) => {
        setOpen(newOpen);
        if (newOpen) {
          setUnread(0);
        }
      }}
    >
      <Popover.Trigger>
        <Button
          isIconOnly
          variant="ghost"
          aria-label="Notifications"
          className="relative"
        >
          <Bell className="h-[18px] w-[18px]" />
          {unread > 0 && (
            <span className="absolute right-1 top-1 flex h-3.5 min-w-3.5 items-center justify-center rounded-full bg-primary px-1 text-[9px] font-medium text-primary-fg">
              {unread > 9 ? "9+" : unread}
            </span>
          )}
        </Button>
      </Popover.Trigger>
      <Popover.Content className="w-72 p-0">
        <Card className="border-none shadow-lg">
          <div className="border-b border-divider px-3 py-2 text-xs font-medium text-default-500">
            Recent activity
          </div>
          {notices.length === 0 ? (
            <div className="px-3 py-4 text-sm text-default-500">
              No recent activity.
            </div>
          ) : (
            <ul className="max-h-80 overflow-auto">
              {notices.map((n) => (
                <li
                  key={n.id}
                  className="flex items-center justify-between gap-2 px-3 py-2 text-sm border-b border-divider last:border-b-0"
                >
                  <span className="truncate font-mono text-xs text-default-700">
                    {n.text}
                  </span>
                  <span className="shrink-0 text-[10px] text-default-500">
                    {n.at}
                  </span>
                </li>
              ))}
            </ul>
          )}
        </Card>
      </Popover.Content>
    </Popover>
  );
}
