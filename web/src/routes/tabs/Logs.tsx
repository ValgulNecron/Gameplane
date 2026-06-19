import { useEffect, useRef, useState } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import { AlertTriangle, Download, Loader2 } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Logs } from "@/lib/endpoints";
import { openWS } from "@/lib/ws";
import { capitalize } from "@/lib/utils";
import type { GameServerPhase } from "@/types";

const MAX_LINES = 20_000;

// Log sources the tab streams from. "pod" follows the game container's
// stdout — visible while a server is still downloading/configuring during
// "Starting" (before it writes its own log file) and works without agent
// mTLS; "file" tails the configured game log file via the agent.
type LogSource = "pod" | "file";

export function LogsTab({
  name,
  logPath,
  phase,
  progressMessage,
}: {
  name: string;
  logPath?: string;
  phase?: GameServerPhase;
  progressMessage?: string;
}) {
  const [lines, setLines] = useState<string[]>([]);
  const [filter, setFilter] = useState("");
  // Default to container output so logs are never empty during startup.
  const [source, setSource] = useState<LogSource>("pod");
  const [connected, setConnected] = useState(false);
  const scrollerRef = useRef<HTMLDivElement | null>(null);

  // The game-log file is only an option when the template declares a
  // logPath; otherwise the only source is the container's stdout, which
  // is always available via the pod-log API.
  const effectiveSource: LogSource = logPath ? source : "pod";

  useEffect(() => {
    setLines([]);
    setConnected(false);
    const path =
      effectiveSource === "pod" ? Logs.podStreamPath(name) : Logs.fileStreamPath(name);
    const sock = openWS(path, {
      onOpen: () => setConnected(true),
      onClose: () => setConnected(false),
      onMessage: (data) =>
        setLines((prev) => {
          const next = prev.concat(typeof data === "string" ? data.split(/\r?\n/) : []);
          return next.length > MAX_LINES ? next.slice(-MAX_LINES) : next;
        }),
    });
    return () => sock.close();
  }, [name, effectiveSource]);

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

  // No output yet. While the server is still coming up (image pull /
  // first-boot install), the pod-log stream stays closed (the API replies
  // StatusTryAgainLater and openWS retries), so an empty black panel reads
  // as "broken". Show what's actually happening instead.
  const noOutput = lines.length === 0;
  const failed = phase === "Failed";
  const provisioning = phase !== undefined && phase !== "Running" && !failed;

  return (
    <div className="flex h-full flex-col">
      <div className="flex items-center gap-2 border-b border-border px-4 py-2">
        {logPath ? (
          <div className="flex rounded border border-border text-xs">
            <button
              type="button"
              onClick={() => setSource("pod")}
              aria-pressed={effectiveSource === "pod"}
              className={`h-8 rounded-l px-2 ${effectiveSource === "pod" ? "bg-surface font-medium" : "text-muted"}`}
              title="Follow the game container's stdout (download/config + startup)"
            >
              Container output
            </button>
            <button
              type="button"
              onClick={() => setSource("file")}
              aria-pressed={effectiveSource === "file"}
              className={`h-8 rounded-r border-l border-border px-2 ${effectiveSource === "file" ? "bg-surface font-medium" : "text-muted"}`}
              title="Tail the configured game log file via the agent"
            >
              Game log
            </button>
          </div>
        ) : (
          <span
            className="text-xs font-medium text-muted"
            title="Follow the game container's stdout (download/install + startup)"
          >
            Container output
          </span>
        )}
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
        {noOutput ? (
          <div className="flex h-full flex-col items-center justify-center gap-3 px-6 text-center">
            {failed ? (
              <>
                <AlertTriangle className="h-8 w-8 text-danger" />
                <div className="text-sm font-medium text-fg">
                  {progressMessage ? capitalize(progressMessage) : "The server failed to start."}
                </div>
                <div className="max-w-md text-xs text-muted">
                  No container output was captured. Check the Overview events for image-pull
                  or scheduling errors.
                </div>
              </>
            ) : (
              <>
                <Loader2 className="h-8 w-8 animate-spin text-primary" />
                {provisioning ? (
                  <>
                    <div className="text-sm font-medium text-fg">
                      {progressMessage ? capitalize(progressMessage) : "Starting the server…"}
                    </div>
                    <div className="max-w-md text-xs text-muted">
                      Downloading game files and starting up. The first start can take a few
                      minutes — install output appears here as it streams from the container.
                    </div>
                  </>
                ) : (
                  <div className="text-sm text-muted">
                    {connected ? "Connected — waiting for output…" : "Connecting to the log stream…"}
                  </div>
                )}
              </>
            )}
          </div>
        ) : (
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
        )}
      </div>
    </div>
  );
}
