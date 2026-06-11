import { useEffect, useRef, useState } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import { Download } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Logs } from "@/lib/endpoints";
import { openWS } from "@/lib/ws";

const MAX_LINES = 20_000;

export function LogsTab({ name }: { name: string }) {
  const [lines, setLines] = useState<string[]>([]);
  const [filter, setFilter] = useState("");
  const scrollerRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    setLines([]);
    const sock = openWS(`/ws/servers/${name}/logs`, {
      onMessage: (data) =>
        setLines((prev) => {
          const next = prev.concat(typeof data === "string" ? data.split(/\r?\n/) : []);
          return next.length > MAX_LINES ? next.slice(-MAX_LINES) : next;
        }),
    });
    return () => sock.close();
  }, [name]);

  const filtered = filter
    ? lines.filter((l) => l.toLowerCase().includes(filter.toLowerCase()))
    : lines;

  const rowVirtualizer = useVirtualizer({
    count: filtered.length,
    getScrollElement: () => scrollerRef.current,
    estimateSize: () => 18,
    overscan: 20,
  });

  useEffect(() => {
    // Autoscroll to bottom on new lines.
    const el = scrollerRef.current;
    if (el) el.scrollTop = el.scrollHeight;
  }, [lines.length]);

  return (
    <div className="flex h-full flex-col">
      <div className="flex items-center gap-2 border-b border-border px-4 py-2">
        <input
          placeholder="filter…"
          className="h-8 w-64 rounded border border-border bg-surface px-2 font-mono text-xs"
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
        />
        <span className="text-xs text-muted">{filtered.length.toLocaleString()} lines</span>
        <div className="ml-auto">
          <Button
            variant="outline"
            size="sm"
            onClick={() => {
              window.location.href = Logs.downloadURL(name);
            }}
          >
            <Download className="h-3 w-3" /> Download
          </Button>
        </div>
      </div>
      <div ref={scrollerRef} className="flex-1 overflow-auto bg-[#0b0b0d] font-mono text-xs">
        <div style={{ height: rowVirtualizer.getTotalSize(), position: "relative", width: "100%" }}>
          {rowVirtualizer.getVirtualItems().map((v) => (
            <div
              key={v.key}
              style={{
                position: "absolute", top: 0, left: 0, right: 0,
                transform: `translateY(${v.start}px)`, height: v.size,
              }}
              className="whitespace-pre px-4 leading-[18px]"
            >
              {filtered[v.index]}
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
