import { describe, it, expect, vi } from "vitest";
import { http, HttpResponse } from "msw";
import { screen } from "@testing-library/react";
import { server } from "@/test/server";
import { renderWithQuery } from "@/test/render";
import { makeServer, makeTemplate } from "@/test/factories";

// xterm + WebSocket integration is exercised by e2e; in jsdom we mock
// xterm so the constructor does nothing and openWS is a no-op handle.
vi.mock("@xterm/xterm", () => ({
  Terminal: vi.fn().mockImplementation(() => ({
    open: vi.fn(),
    write: vi.fn(),
    onData: vi.fn(),
    dispose: vi.fn(),
    loadAddon: vi.fn(),
  })),
}));
vi.mock("@xterm/addon-fit", () => ({
  FitAddon: vi.fn().mockImplementation(() => ({
    fit: vi.fn(),
  })),
}));
vi.mock("@xterm/xterm/css/xterm.css", () => ({}));
vi.mock("@/lib/ws", () => ({
  openWS: vi.fn(() => ({ send: vi.fn(), close: vi.fn() })),
}));

import { ConsoleTab } from "./Console";

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
});
