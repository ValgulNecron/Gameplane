import { describe, it, expect, vi, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { screen, waitFor } from "@testing-library/react";
import { server } from "@/test/server";
import { renderWithQuery } from "@/test/render";
import { makeServer, makeTemplate } from "@/test/factories";

// xterm + WebSocket integration is exercised by e2e; in jsdom we mock
// xterm with instance capture (so tests can drive onData/onMessage) and
// openWS with a recording stub.
type WSOpts = {
  onMessage?: (data: unknown) => void;
  onOpen?: () => void;
  onClose?: () => void;
};

const mocks = vi.hoisted(() => {
  const terms: Array<{
    open: ReturnType<typeof vi.fn>;
    write: ReturnType<typeof vi.fn>;
    writeln: ReturnType<typeof vi.fn>;
    onData: ReturnType<typeof vi.fn>;
    onResize: ReturnType<typeof vi.fn>;
    dispose: ReturnType<typeof vi.fn>;
    loadAddon: ReturnType<typeof vi.fn>;
    cols: number;
    rows: number;
  }> = [];
  const wsCalls: Array<{ path: string; opts: unknown }> = [];
  const wsHandle = { send: vi.fn(), close: vi.fn() };
  return { terms, wsCalls, wsHandle };
});

vi.mock("@xterm/xterm", () => ({
  Terminal: vi.fn().mockImplementation(() => {
    const inst = {
      open: vi.fn(),
      write: vi.fn(),
      writeln: vi.fn(),
      onData: vi.fn(() => ({ dispose: vi.fn() })),
      onResize: vi.fn(() => ({ dispose: vi.fn() })),
      dispose: vi.fn(),
      loadAddon: vi.fn(),
      cols: 100,
      rows: 30,
    };
    mocks.terms.push(inst);
    return inst;
  }),
}));
vi.mock("@xterm/addon-fit", () => ({
  FitAddon: vi.fn().mockImplementation(() => ({
    fit: vi.fn(),
  })),
}));
vi.mock("@xterm/xterm/css/xterm.css", () => ({}));
vi.mock("@/lib/ws", () => ({
  openWS: vi.fn((path: string, opts: unknown) => {
    mocks.wsCalls.push({ path, opts });
    return mocks.wsHandle;
  }),
}));

import { ConsoleTab } from "./Console";

beforeEach(() => {
  mocks.terms.length = 0;
  mocks.wsCalls.length = 0;
  mocks.wsHandle.send.mockClear();
  mocks.wsHandle.close.mockClear();
});

describe("ConsoleTab", () => {
  it("renders an empty-state when the template has no console", async () => {
    server.use(
      http.get("/servers/alpha", () =>
        HttpResponse.json(
          makeServer({ spec: { templateRef: { name: "noop" } } }),
        ),
      ),
      http.get("/templates/noop", () =>
        HttpResponse.json(makeTemplate({ spec: { ...makeTemplate().spec, consoleMode: "none", rcon: undefined } })),
      ),
    );
    renderWithQuery(<ConsoleTab name="alpha" />);
    expect(await screen.findByText(/doesn't expose a console/i)).toBeInTheDocument();
  });

  it("shows loading until both server + template arrive", async () => {
    server.use(
      http.get("/servers/alpha", () =>
        HttpResponse.json(makeServer({ spec: { templateRef: { name: "minecraft-vanilla" } } })),
      ),
      http.get("/templates/minecraft-vanilla", async () => {
        await new Promise((r) => setTimeout(r, 50));
        return HttpResponse.json(makeTemplate({ spec: { ...makeTemplate().spec, consoleMode: "rcon" } }));
      }),
    );
    renderWithQuery(<ConsoleTab name="alpha" />);
    expect(screen.getByText(/Loading console/i)).toBeInTheDocument();
  });

  it("opens the PTY bridge when consoleMode is pty and frames stdin/stdout as base64", async () => {
    server.use(
      http.get("/servers/alpha", () =>
        HttpResponse.json(makeServer({ spec: { templateRef: { name: "terraria" } } })),
      ),
      http.get("/templates/terraria", () =>
        HttpResponse.json(
          makeTemplate({ spec: { ...makeTemplate().spec, consoleMode: "pty", rcon: undefined } }),
        ),
      ),
    );
    renderWithQuery(<ConsoleTab name="alpha" />);

    await waitFor(() =>
      expect(mocks.wsCalls.map((c) => c.path)).toContain("/ws/servers/alpha/console-pty"),
    );
    const call = mocks.wsCalls.find((c) => c.path === "/ws/servers/alpha/console-pty");
    const opts = call?.opts as WSOpts;
    const term = mocks.terms[mocks.terms.length - 1];

    // Attaching sends an initial resize with the terminal's geometry.
    opts.onOpen?.();
    expect(mocks.wsHandle.send).toHaveBeenCalledWith(
      JSON.stringify({ kind: "resize", cols: 100, rows: 30 }),
    );

    // Keystrokes ship immediately as base64 stdin frames.
    const onData = term.onData.mock.calls[0][0] as (d: string) => void;
    onData("ls\n");
    expect(mocks.wsHandle.send).toHaveBeenCalledWith(
      JSON.stringify({ kind: "stdin", body: "bHMK" }), // base64("ls\n")
    );

    // stdout frames are base64-decoded into the terminal.
    opts.onMessage?.(JSON.stringify({ kind: "stdout", body: btoa("hello") }));
    expect(term.write).toHaveBeenCalledWith("hello");

    // err frames render as red text lines; malformed frames are dropped.
    opts.onMessage?.(JSON.stringify({ kind: "err", body: "attach torn down" }));
    expect(term.writeln).toHaveBeenCalledWith(expect.stringContaining("attach torn down"));
    opts.onMessage?.("not json");
    expect(term.write).toHaveBeenCalledTimes(1);
  });

  it("opens the RCON socket when consoleMode is rcon", async () => {
    server.use(
      http.get("/servers/alpha", () =>
        HttpResponse.json(makeServer({ spec: { templateRef: { name: "minecraft-vanilla" } } })),
      ),
      http.get("/templates/minecraft-vanilla", () =>
        HttpResponse.json(makeTemplate({ spec: { ...makeTemplate().spec, consoleMode: "rcon" } })),
      ),
    );
    renderWithQuery(<ConsoleTab name="alpha" />);

    await waitFor(() =>
      expect(mocks.wsCalls.map((c) => c.path)).toContain("/ws/servers/alpha/console"),
    );
  });
});
