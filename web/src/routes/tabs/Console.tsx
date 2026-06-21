import { useQuery } from "@tanstack/react-query";
import "@xterm/xterm/css/xterm.css";

import { api } from "@/lib/api";
import { resolveConsoleMode } from "@/lib/capabilities";
import type { GameServer, GameTemplate } from "@/types";
import { ConsoleShell } from "./ConsoleShell";
import { useConsoleTerminal } from "./useConsoleTerminal";

export function ConsoleTab({ name }: { name: string }) {
  const { data: gs } = useQuery({
    queryKey: ["server", name],
    queryFn: () => api<GameServer>(`/servers/${name}`),
  });
  const templateName = gs?.spec.templateRef.name;
  const { data: tmpl } = useQuery({
    queryKey: ["template", templateName],
    queryFn: () => api<GameTemplate>(`/templates/${templateName}`),
    enabled: !!templateName,
  });
  const mode = resolveConsoleMode(tmpl);

  if (!gs || !tmpl) {
    return <div className="p-4 text-sm text-muted">Loading console…</div>;
  }
  if (mode === "none") {
    return (
      <div className="p-4 text-sm text-muted">
        This game template doesn&apos;t expose a console.
      </div>
    );
  }
  return mode === "pty" ? <PtyConsole name={name} /> : <RconConsole name={name} />;
}

// RconConsole speaks the existing line-based RCON protocol — text is
// buffered until Enter, then sent as {kind:"cmd"}. Server replies arrive
// as {kind:"out"|"err"} with a string body.
function RconConsole({ name }: { name: string }) {
  const handle = useConsoleTerminal({
    wsPath: `/ws/servers/${name}/console`,
    name,
    connectLabel: "— connected —",
    disconnectLabel: "— disconnected —",
    attach: (term, send) => {
      let buf = "";
      return {
        onMessage: (data) => {
          try {
            const env = JSON.parse(data) as { kind: string; body: string };
            if (env.kind === "err") {
              // Render real RCON errors in red so they read as failures,
              // not as ordinary command output (matches PtyConsole below).
              term.writeln(`\x1b[31m${env.body}\x1b[0m`);
            } else if (env.kind === "out") {
              term.writeln(env.body);
            }
          } catch {
            // Non-JSON frame — e.g. an HTML/redirect body from a WS upgrade
            // that didn't reach the agent. Don't echo it verbatim (that's
            // what surfaced a raw URL in the console); real failures arrive
            // as a {kind:"err"} envelope and the disconnect line reports it.
          }
        },
        onData: (d) => {
          for (const ch of d) {
            if (ch === "\r") {
              term.writeln("");
              send(JSON.stringify({ kind: "cmd", body: buf }));
              buf = "";
            } else if (ch === "\x7f") {
              if (buf.length > 0) {
                buf = buf.slice(0, -1);
                term.write("\b \b");
              }
            } else {
              buf += ch;
              term.write(ch);
            }
          }
        },
        sendCommand: (line) => {
          term.writeln(line);
          send(JSON.stringify({ kind: "cmd", body: line }));
        },
      };
    },
  });
  return <ConsoleShell handle={handle} />;
}

// PtyConsole speaks the streaming envelope used by api/internal/ws/attach.go.
// Every keystroke ships immediately as {kind:"stdin", body:base64} and
// terminal resizes ship as {kind:"resize", cols, rows}. Server frames
// arrive as {kind:"stdout", body:base64} (stderr merged into stdout under
// TTY) or {kind:"err", body:string} on a teardown.
function PtyConsole({ name }: { name: string }) {
  const handle = useConsoleTerminal({
    wsPath: `/ws/servers/${name}/console-pty`,
    name,
    connectLabel: "— attached —",
    disconnectLabel: "— detached —",
    attach: (term, send) => {
      const sendResize = () =>
        send(JSON.stringify({ kind: "resize", cols: term.cols, rows: term.rows }));
      return {
        onMessage: (data) => {
          try {
            const env = JSON.parse(data) as { kind: string; body?: string };
            if (env.kind === "stdout" && env.body) {
              term.write(b64decode(env.body));
            } else if (env.kind === "err" && env.body) {
              term.writeln(`\x1b[31m${env.body}\x1b[0m`);
            }
          } catch {
            // Malformed frame — drop.
          }
        },
        onData: (d) => send(JSON.stringify({ kind: "stdin", body: b64encode(d) })),
        onSocketOpen: sendResize,
        onTermResize: sendResize,
        sendCommand: (line) =>
          send(JSON.stringify({ kind: "stdin", body: b64encode(line + "\n") })),
      };
    },
  });
  return <ConsoleShell handle={handle} />;
}

// Browser-safe base64 helpers that round-trip arbitrary UTF-8 / binary
// payloads. xterm gives us strings (the runes the user pressed); the
// pty side wants raw bytes. Encoding via TextEncoder/TextDecoder
// guarantees the bytes on the wire are exactly the UTF-8 representation.
function b64encode(s: string): string {
  const bytes = new TextEncoder().encode(s);
  let bin = "";
  for (const b of bytes) bin += String.fromCharCode(b);
  return btoa(bin);
}

function b64decode(s: string): string {
  const bin = atob(s);
  const bytes = new Uint8Array(bin.length);
  for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i);
  return new TextDecoder().decode(bytes);
}
