import { useEffect, useMemo, useRef, useState } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import { AlertTriangle, Download, Loader2 } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Logs } from "@/lib/endpoints";
import { openWS } from "@/lib/ws";
import { capitalize, cn } from "@/lib/utils";
import type { GameServerPhase } from "@/types";

const MAX_LINES = 20_000;

// Best-effort client-side log-level extraction. Game logs vary, but most
// emit a recognizable level token (e.g. Minecraft "[Server thread/INFO]");
// this drives filtering + row coloring only, never parsing semantics, and
// returns null when no level is found.
export type LogLevel = "ERROR" | "WARN" | "INFO" | "DEBUG";
const LEVEL_RE = /\b(ERROR|ERR|SEVERE|FATAL|WARNING|WARN|INFO|DEBUG|TRACE|FINE)\b/i;
export function parseLogLevel(line: string): LogLevel | null {
  const m = LEVEL_RE.exec(line);
  if (!m) return null;
  const t = m[1].toUpperCase();
  if (t === "ERROR" || t === "ERR" || t === "SEVERE" || t === "FATAL") return "ERROR";
  if (t === "WARN" || t === "WARNING") return "WARN";
  if (t === "DEBUG" || t === "TRACE" || t === "FINE") return "DEBUG";
  return "INFO";
}
const LEVELS: LogLevel[] = ["INFO", "WARN", "ERROR", "DEBUG"];
const LEVEL_CLASS: Record<LogLevel, string> = {
  ERROR: "text-red-400",
  WARN: "text-amber-400",
  INFO: "",
  DEBUG: "text-muted",
};

// Log sources the tab streams from. "pod" follows the whole pod timeline —
// each setup/init container's output, then the game container's stdout —
// so the install/setup step is visible while a server is still "Starting"
// (before it writes its own log file), and it works without agent mTLS;
// "file" tails the configured game log file via the agent.
type LogSource = "pod" | "file";

export function LogsTab({
  name,
  ns,
  logPath,
  phase,
  progressMessage,
}: {
  name: string;
  ns?: string;
  logPath?: string;
  phase?: GameServerPhase;
  progressMessage?: string;
}) {
  const [lines, setLines] = useState<string[]>([]);
  const [filter, setFilter] = useState("");
  const [level, setLevel] = useState<"all" | LogLevel>("all");
  // Default to container output so logs are never empty during startup.
  const [source, setSource] = useState<LogSource>("pod");
  const [connected, setConnected] = useState(false);
  // The agent-backed game-log stream can fail to connect (e.g. the agent
  // is unreachable); track that so we can show an actionable notice instead
  // of an endless "Connecting…" spinner.
  const [streamFailed, setStreamFailed] = useState(false);
  const scrollerRef = useRef<HTMLDivElement | null>(null);

  // The game-log file is only an option when the template declares a
  // logPath; otherwise the only source is the container's stdout, which
  // is always available via the pod-log API.
  const effectiveSource: LogSource = logPath ? source : "pod";

  useEffect(() => {
    setLines([]);
    setConnected(false);
    setStreamFailed(false);
    const path =
      effectiveSource === "pod" ? Logs.podStreamPath(name, ns) : Logs.fileStreamPath(name, ns);
    const sock = openWS(path, {
      onOpen: () => setConnected(true),
      onClose: () => setConnected(false),
      onStatus: (status, info) => {
        if (status === "open") {
          setStreamFailed(false);
          // Only the agent-backed file stream surfaces an actionable
          // failure: the user can fall back to container output. The pod
          // stream's retries are expected during provisioning, so leave
          // those to the provisioning placeholder.
        } else if (effectiveSource === "file" && status === "reconnecting" && info.attempt >= 2) {
          setStreamFailed(true);
        }
      },
      onMessage: (data) =>
        setLines((prev) => {
          const next = prev.concat(typeof data === "string" ? data.split(/\r?\n/) : []);
          return next.length > MAX_LINES ? next.slice(-MAX_LINES) : next;
        }),
    });
    return () => sock.close();
  }, [name, ns, effectiveSource]);

  // Parse each line's level once; derive counts + the filtered view from it.
  const parsed = useMemo(
    () => lines.map((text) => ({ text, level: parseLogLevel(text) })),
    [lines],
  );
  const counts = useMemo(() => {
    const c: Record<LogLevel, number> = { ERROR: 0, WARN: 0, INFO: 0, DEBUG: 0 };
    for (const p of parsed) if (p.level) c[p.level]++;
    return c;
  }, [parsed]);
  const needle = filter.toLowerCase();
  const filtered = useMemo(
    () =>
      parsed.filter((p) => {
        if (needle && !p.text.toLowerCase().includes(needle)) return false;
        if (level !== "all" && p.level !== level) return false;
        return true;
      }),
    [parsed, needle, level],
  );

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
  // The game-log file stream couldn't reach the agent and produced nothing —
  // offer container output (always available) instead of spinning forever.
  const fileUnavailable = effectiveSource === "file" && streamFailed && noOutput;

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
              title="Follow the pod's setup + game container output (install/startup)"
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
            title="Follow the pod's setup + game container output (install/startup)"
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
        <div className="flex gap-1">
          {(["all", ...LEVELS] as const).map((lv) => (
            <button
              key={lv}
              type="button"
              onClick={() => setLevel(lv)}
              aria-pressed={level === lv}
              className={cn(
                "rounded px-2 py-1 text-[11px] font-medium",
                level === lv ? "bg-primary/15 text-primary" : "text-muted hover:text-fg",
              )}
            >
              {lv === "all" ? "All" : `${lv[0]}${lv.slice(1).toLowerCase()}`}
              {lv !== "all" && ` ${counts[lv]}`}
            </button>
          ))}
        </div>
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
            ) : fileUnavailable ? (
              <>
                <AlertTriangle className="h-8 w-8 text-warning" />
                <div className="text-sm font-medium text-fg">Game log unavailable</div>
                <div className="max-w-md text-xs text-muted">
                  Couldn&apos;t reach the agent to tail the game log file. Container output is
                  still available.
                </div>
                <Button variant="outline" size="sm" onClick={() => setSource("pod")}>
                  Use container output
                </Button>
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
            {rowVirtualizer.getVirtualItems().map((v) => {
              const row = filtered[v.index];
              return (
                <div
                  key={v.key}
                  style={{
                    position: "absolute", top: 0, left: 0, right: 0,
                    transform: `translateY(${v.start}px)`, height: v.size,
                  }}
                  className={cn("whitespace-pre px-4 leading-[18px]", row.level && LEVEL_CLASS[row.level])}
                >
                  {row.text}
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}
