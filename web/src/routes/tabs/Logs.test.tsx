import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

// Each call to openWS pushes a new fake socket onto sockets[]; tests
// reach into the latest one to drive messages/lifecycle and assert cleanup.
type FakeSock = {
  path: string;
  close: ReturnType<typeof vi.fn>;
  send: ReturnType<typeof vi.fn>;
  sendMsg: (s: string) => void;
  open: () => void;
  closeEv: () => void;
};
const sockets: FakeSock[] = [];

vi.mock("@/lib/ws", () => ({
  openWS: vi.fn(
    (
      path: string,
      opts: { onMessage: (d: string) => void; onOpen?: () => void; onClose?: () => void },
    ) => {
      const sock: FakeSock = {
        path,
        close: vi.fn(),
        send: vi.fn(),
        sendMsg: opts.onMessage,
        open: () => opts.onOpen?.(),
        closeEv: () => opts.onClose?.(),
      };
      sockets.push(sock);
      return { send: sock.send, close: sock.close };
    },
  ),
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

  it("defaults to the container-output (pod) stream so startup logs show", () => {
    render(<LogsTab name="alpha" />);
    expect(sockets[0].path).toBe("/ws/servers/alpha/logs/pod?from=start");
  });

  it("switches to the game-log file stream when toggled (template has a logPath)", async () => {
    render(<LogsTab name="alpha" logPath="/data/logs/latest.log" />);
    await userEvent.click(screen.getByRole("button", { name: /game log/i }));
    expect(sockets[0].close).toHaveBeenCalled();
    expect(sockets[sockets.length - 1].path).toBe("/ws/servers/alpha/logs");
  });

  it("offers only container output (no toggle) when the template has no logPath", () => {
    render(<LogsTab name="alpha" />);
    // The "Game log" toggle is absent and only the pod stream is opened.
    expect(screen.queryByRole("button", { name: /game log/i })).not.toBeInTheDocument();
    expect(sockets).toHaveLength(1);
    expect(sockets[0].path).toBe("/ws/servers/alpha/logs/pod?from=start");
  });

  it("shows the provisioning placeholder with the operator message while starting", () => {
    render(<LogsTab name="alpha" phase="Starting" progressMessage="pulling the game image" />);
    // Lowercase operator message is capitalized for display.
    expect(screen.getByText("Pulling the game image")).toBeInTheDocument();
    expect(screen.getByText(/install output appears here/i)).toBeInTheDocument();
  });

  it("shows a failure placeholder (no spinner) when the server has Failed", () => {
    render(
      <LogsTab name="alpha" phase="Failed" progressMessage="cannot pull the image — check the image reference" />,
    );
    expect(screen.getByText("Cannot pull the image — check the image reference")).toBeInTheDocument();
    expect(screen.getByText(/check the overview events/i)).toBeInTheDocument();
    // Not the "starting" install copy.
    expect(screen.queryByText(/install output appears here/i)).not.toBeInTheDocument();
  });

  it("hides the placeholder once log lines arrive", async () => {
    render(<LogsTab name="alpha" phase="Starting" progressMessage="installing server files" />);
    expect(screen.getByText("Installing server files")).toBeInTheDocument();
    sockets[0].sendMsg("Downloading server-1.21.jar…\n");
    await waitFor(() =>
      expect(screen.queryByText("Installing server files")).not.toBeInTheDocument(),
    );
  });

  it("shows a connecting message when running with no output yet", () => {
    render(<LogsTab name="alpha" phase="Running" />);
    expect(screen.getByText(/connecting to the log stream/i)).toBeInTheDocument();
  });

  it("switches to waiting-for-output once the socket opens while running", async () => {
    render(<LogsTab name="alpha" phase="Running" />);
    sockets[0].open();
    await waitFor(() => expect(screen.getByText(/waiting for output/i)).toBeInTheDocument());
  });
});
