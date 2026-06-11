import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

// Each call to openWS pushes a new fake socket onto sockets[]; tests
// reach into the latest one to drive messages and assert cleanup.
type FakeSock = { close: ReturnType<typeof vi.fn>; send: ReturnType<typeof vi.fn>; sendMsg: (s: string) => void };
const sockets: FakeSock[] = [];

vi.mock("@/lib/ws", () => ({
  openWS: vi.fn((_path: string, opts: { onMessage: (d: string) => void }) => {
    const sock: FakeSock = {
      close: vi.fn(),
      send: vi.fn(),
      sendMsg: opts.onMessage,
    };
    sockets.push(sock);
    return { send: sock.send, close: sock.close };
  }),
}));

import { LogsTab } from "./Logs";

beforeEach(() => {
  sockets.length = 0;
});

afterEach(() => {
  vi.clearAllMocks();
});

describe("LogsTab", () => {
  it("counts incoming lines", async () => {
    // jsdom doesn't lay out the virtualized rows so we assert against
    // the line counter rather than the row text content. The rendering
    // path (via useVirtualizer) is exercised in real browsers and e2e.
    render(<LogsTab name="alpha" />);
    sockets[0].sendMsg("first line\nsecond line\n");
    // Two newlines produce three split entries (first, second, "").
    await waitFor(() => expect(screen.getByText(/3 lines/)).toBeInTheDocument());
  });

  it("filters lines by user-typed substring", async () => {
    render(<LogsTab name="alpha" />);
    sockets[0].sendMsg("error: bad config\ninfo: starting up");
    await waitFor(() => expect(screen.getByText(/2 lines/)).toBeInTheDocument());
    const filter = screen.getByPlaceholderText(/filter/i);
    await userEvent.type(filter, "error");
    await waitFor(() => expect(screen.getByText(/1 lines/)).toBeInTheDocument());
  });

  it("download button navigates to the log download URL", async () => {
    render(<LogsTab name="alpha" />);
    // jsdom can't navigate; swap in a plain object so the href
    // assignment is observable instead of throwing "not implemented".
    const original = window.location;
    Object.defineProperty(window, "location", {
      writable: true,
      value: { href: "" },
    });
    try {
      await userEvent.click(screen.getByRole("button", { name: /download/i }));
      expect(window.location.href).toBe("/servers/alpha/logs/download");
    } finally {
      Object.defineProperty(window, "location", {
        writable: true,
        value: original,
      });
    }
  });

  it("closes the socket on unmount", async () => {
    const { unmount } = render(<LogsTab name="alpha" />);
    unmount();
    expect(sockets[0].close).toHaveBeenCalled();
  });

  it("reconnects when the server name changes", async () => {
    const { rerender } = render(<LogsTab name="alpha" />);
    rerender(<LogsTab name="beta" />);
    expect(sockets[0].close).toHaveBeenCalled();
    expect(sockets).toHaveLength(2);
  });
});
