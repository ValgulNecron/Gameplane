import { useEffect, useRef } from "react";
import { useQuery } from "@tanstack/react-query";
import { Terminal } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import "@xterm/xterm/css/xterm.css";

import { api } from "@/lib/api";
import { openWS } from "@/lib/ws";
import { resolveConsoleMode } from "@/lib/capabilities";
import type { GameServer, GameTemplate } from "@/types";

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
  const hostRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    if (!hostRef.current) return;
    const term = new Terminal({
      theme: { background: "#0b0b0d" },
      fontFamily: "JetBrains Mono, monospace",
      fontSize: 13,
      cursorBlink: true,
    });
    const fit = new FitAddon();
    term.loadAddon(fit);
    term.open(hostRef.current);
    fit.fit();

    const sock = openWS(`/ws/servers/${name}/console`, {
      onMessage: (data) => {
        if (typeof data !== "string") return;
        try {
          const env = JSON.parse(data) as { kind: string; body: string };
          if (env.kind === "out" || env.kind === "err") {
            term.writeln(env.body);
          }
        } catch {
          // Non-JSON frame — e.g. an HTML/redirect body from a WS upgrade
          // that didn't reach the agent. Don't echo it verbatim (that's
          // what surfaced a raw URL in the console); real failures arrive
          // as a {kind:"err"} envelope and onClose reports the disconnect.
        }
      },
      onOpen: () => term.writeln("\x1b[90m— connected —\x1b[0m"),
      onClose: () => term.writeln("\x1b[31m— disconnected —\x1b[0m"),
    });

    let buf = "";
    const disp = term.onData((d) => {
      for (const ch of d) {
        if (ch === "\r") {
          term.writeln("");
          sock.send(JSON.stringify({ kind: "cmd", body: buf }));
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
    });

    const onResize = () => fit.fit();
    window.addEventListener("resize", onResize);
    return () => {
      window.removeEventListener("resize", onResize);
      disp.dispose();
      sock.close();
      term.dispose();
    };
  }, [name]);

  return (
    <div className="h-full p-4">
      <div ref={hostRef} className="h-full rounded-lg border border-border bg-[#0b0b0d] p-2" />
    </div>
  );
}

// PtyConsole speaks the streaming envelope used by api/internal/ws/attach.go.
// Every keystroke ships immediately as {kind:"stdin", body:base64} and
// terminal resizes ship as {kind:"resize", cols, rows}. Server frames
// arrive as {kind:"stdout", body:base64} (stderr merged into stdout under
// TTY) or {kind:"err", body:string} on a teardown.
function PtyConsole({ name }: { name: string }) {
  const hostRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    if (!hostRef.current) return;
    const term = new Terminal({
      theme: { background: "#0b0b0d" },
      fontFamily: "JetBrains Mono, monospace",
      fontSize: 13,
      cursorBlink: true,
      // Matches the kubelet's default; the resize messages we send will
      // override this immediately, but the initial render still benefits
      // from a sane default.
      cols: 100,
      rows: 30,
    });
    const fit = new FitAddon();
    term.loadAddon(fit);
    term.open(hostRef.current);
    fit.fit();

    const sock = openWS(`/ws/servers/${name}/console-pty`, {
      onMessage: (data) => {
        if (typeof data !== "string") return;
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
      onOpen: () => {
        term.writeln("\x1b[90m— attached —\x1b[0m");
        sendResize();
      },
      onClose: () => term.writeln("\x1b[31m— detached —\x1b[0m"),
    });

    function sendResize() {
      sock.send(JSON.stringify({ kind: "resize", cols: term.cols, rows: term.rows }));
    }

    const dataDisp = term.onData((d) => {
      sock.send(JSON.stringify({ kind: "stdin", body: b64encode(d) }));
    });
    const resizeDisp = term.onResize(sendResize);

    const onWindowResize = () => fit.fit();
    window.addEventListener("resize", onWindowResize);
    return () => {
      window.removeEventListener("resize", onWindowResize);
      dataDisp.dispose();
      resizeDisp.dispose();
      sock.close();
      term.dispose();
    };
  }, [name]);

  return (
    <div className="h-full p-4">
      <div ref={hostRef} className="h-full rounded-lg border border-border bg-[#0b0b0d] p-2" />
    </div>
  );
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
