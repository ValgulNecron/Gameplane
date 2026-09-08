import { useState } from "react";
import { Button, Input } from "@heroui/react";
import { Download, Eraser, Maximize2 } from "lucide-react";

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

// ConsoleShell is the M8 chrome from design.pen frame Xn5ns: a header
// toolbar (connection indicator + Clear/Download/Fullscreen) bracketing the
// xterm host, and a dedicated command-input bar. All behavior comes from the
// useConsoleTerminal handle; this component is presentation + local input.
export function ConsoleShell({ handle }: { handle: ConsoleHandle }) {
  const { hostRef, status, clear, download, toggleFullscreen, sendCommand } = handle;
  const [cmd, setCmd] = useState("");
  return (
    <div className="flex h-full flex-col">
      <div className="flex flex-wrap items-center gap-2 border-b border-border px-4 py-2">
        <span className="inline-flex items-center gap-1.5 font-mono text-xs">
          <span className={cn("h-2 w-2 rounded-full", STATUS_DOT[status])} aria-hidden />
          {STATUS_LABEL[status]}
        </span>
        <div className="ml-auto flex flex-wrap gap-2">
          <Button
            isIconOnly
            variant="ghost"
            size="sm"
            onPress={clear}
            aria-label="Clear terminal"
          >
            <Eraser className="h-4 w-4" />
          </Button>
          <Button
            isIconOnly
            variant="ghost"
            size="sm"
            onPress={download}
            aria-label="Download terminal buffer"
          >
            <Download className="h-4 w-4" />
          </Button>
          <Button
            isIconOnly
            variant="ghost"
            size="sm"
            onPress={toggleFullscreen}
            aria-label="Toggle fullscreen"
          >
            <Maximize2 className="h-4 w-4" />
          </Button>
        </div>
      </div>
      <div className="flex-1 p-4">
        <div ref={hostRef} className="h-full rounded-lg border border-border bg-[#0b0b0d] p-2" />
      </div>
      <form
        className="flex items-center gap-2 border-t border-border px-4 py-2"
        onSubmit={(e) => {
          e.preventDefault();
          const line = cmd.trim();
          if (!line) return;
          sendCommand(line);
          setCmd("");
        }}
      >
        <Input
          placeholder="Type a command…"
          value={cmd}
          onChange={(e) => setCmd(e.target.value)}
          className="flex-1 font-mono text-xs"
        />
        <Button
          type="submit"
          size="sm"
          variant="primary"
          isDisabled={!cmd.trim()}
        >
          Send
        </Button>
      </form>
    </div>
  );
}
