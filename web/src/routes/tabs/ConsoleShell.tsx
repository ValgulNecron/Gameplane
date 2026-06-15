import { useState } from "react";
import { Download, Eraser, Maximize2 } from "lucide-react";

import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import type { WSStatus } from "@/lib/ws";
import type { ConsoleHandle } from "./useConsoleTerminal";

// Connection indicator styling, keyed exhaustively on WSStatus so a new
// state can't render styleless. "connecting" and "reconnecting" share the
// pulsing amber dot; "open" is the steady green ● LIVE.
const STATUS_DOT: Record<WSStatus, string> = {
  open: "bg-success",
  reconnecting: "bg-warning animate-pulse",
  connecting: "bg-warning animate-pulse",
  closed: "bg-muted",
};
const STATUS_LABEL: Record<WSStatus, string> = {
  open: "LIVE",
  reconnecting: "reconnecting…",
  connecting: "connecting…",
  closed: "offline",
};

// ConsoleShell is the M8 chrome from design.pen frame IH0A9: a header
// toolbar (connection indicator + Clear/Download/Fullscreen) bracketing the
// xterm host, and a dedicated command-input bar. All behavior comes from the
// useConsoleTerminal handle; this component is presentation + local input.
export function ConsoleShell({ handle }: { handle: ConsoleHandle }) {
  const [cmd, setCmd] = useState("");
  return (
    <div className="flex h-full flex-col">
      <div className="flex items-center gap-2 border-b border-border px-4 py-2">
        <span className="inline-flex items-center gap-1.5 font-mono text-xs">
          <span className={cn("h-2 w-2 rounded-full", STATUS_DOT[handle.status])} aria-hidden />
          {STATUS_LABEL[handle.status]}
        </span>
        <div className="ml-auto flex gap-1">
          <Button variant="outline" size="sm" onClick={handle.clear}>
            <Eraser className="h-3 w-3" /> Clear
          </Button>
          <Button variant="outline" size="sm" onClick={handle.download}>
            <Download className="h-3 w-3" /> Download
          </Button>
          <Button variant="outline" size="sm" onClick={handle.toggleFullscreen}>
            <Maximize2 className="h-3 w-3" /> Fullscreen
          </Button>
        </div>
      </div>
      <div className="flex-1 p-4">
        <div ref={handle.hostRef} className="h-full rounded-lg border border-border bg-[#0b0b0d] p-2" />
      </div>
      <form
        className="flex items-center gap-2 border-t border-border px-4 py-2"
        onSubmit={(e) => {
          e.preventDefault();
          const line = cmd.trim();
          if (!line) return;
          handle.sendCommand(line);
          setCmd("");
        }}
      >
        <input
          value={cmd}
          onChange={(e) => setCmd(e.target.value)}
          placeholder="Type a command…"
          aria-label="Console command"
          className="h-8 flex-1 rounded border border-border bg-surface px-2 font-mono text-xs"
        />
        <Button variant="outline" size="sm" type="submit">
          Send
        </Button>
      </form>
    </div>
  );
}
