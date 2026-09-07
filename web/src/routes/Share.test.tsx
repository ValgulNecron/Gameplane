import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { SharePage } from "./Share";
import { Shares, APIError } from "@/lib/api";

vi.mock("@/lib/api", () => ({
  Shares: {
    resolve: vi.fn(),
    start: vi.fn(),
  },
  APIError: class APIError extends Error {
    status: number;
    body: string;
    constructor(status: number, body: string) {
      super(`${status}: ${body}`);
      this.status = status;
      this.body = body;
    }
  },
}));

let mockUseParams = vi.fn();

vi.mock("@tanstack/react-router", async () => {
  const actual = await vi.importActual("@tanstack/react-router");
  return {
    ...actual,
    useParams: mockUseParams,
  };
});

// Helper to render component with a mocked token parameter
function renderWithRouter(token: string) {
  mockUseParams.mockReturnValue({ token });
  return render(<SharePage />);
}

describe("SharePage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Clear localStorage
    localStorage.clear();
  });

  afterEach(() => {
    vi.clearAllTimers();
  });

  describe("Loading state", () => {
    it("T181: renders loading spinner initially", async () => {
      vi.mocked(Shares.resolve).mockImplementation(
        () => new Promise(() => {}) // Never resolves
      );

      renderWithRouter("test-token");
      expect(screen.getByRole("progressbar")).toBeInTheDocument();
    });
  });

  describe("Up state (T182)", () => {
    it("renders server name and Online status", async () => {
      vi.mocked(Shares.resolve).mockResolvedValue({
        serverName: "mc-survival",
        status: "Running",
        address: { host: "play.gameplane.example", port: 25565 },
        playersOnline: 3,
      });

      renderWithRouter("test-token");

      await waitFor(() => {
        expect(screen.getByText("mc-survival")).toBeInTheDocument();
        expect(screen.getByText("Online")).toBeInTheDocument();
      });
    });

    it("displays address and port if exposed", async () => {
      vi.mocked(Shares.resolve).mockResolvedValue({
        serverName: "test-server",
        status: "Running",
        address: { host: "example.com", port: 8080 },
      });

      renderWithRouter("test-token");

      await waitFor(() => {
        expect(screen.getByText("example.com:8080")).toBeInTheDocument();
      });
    });

    it("shows player count if available", async () => {
      vi.mocked(Shares.resolve).mockResolvedValue({
        serverName: "test-server",
        status: "Running",
        address: { host: "example.com", port: 8080 },
        playersOnline: 5,
      });

      renderWithRouter("test-token");

      await waitFor(() => {
        expect(screen.getByText("5 players online")).toBeInTheDocument();
      });
    });

    it("hides address section if not exposed", async () => {
      vi.mocked(Shares.resolve).mockResolvedValue({
        serverName: "test-server",
        status: "Running",
        address: undefined,
      });

      renderWithRouter("test-token");

      await waitFor(() => {
        expect(screen.getByText("Not exposed")).toBeInTheDocument();
      });
    });

    it("T182: respects FR-005 privacy (no cluster/namespace/version in response)", async () => {
      vi.mocked(Shares.resolve).mockResolvedValue({
        serverName: "mc-survival",
        status: "Running",
        address: { host: "play.gameplane.example", port: 25565 },
        playersOnline: 3,
      });

      renderWithRouter("test-token");

      await waitFor(() => {
        expect(screen.queryByText(/cluster/i)).not.toBeInTheDocument();
        expect(screen.queryByText(/namespace/i)).not.toBeInTheDocument();
        expect(screen.queryByText(/v0\.|v1\.|v2\.|beta|alpha/i)).not.toBeInTheDocument();
      });
    });

    it("T182: copy address button copies address to clipboard", async () => {
      const mockClipboard = vi.fn();
      Object.defineProperty(navigator, "clipboard", {
        value: { writeText: mockClipboard },
        configurable: true,
      });

      vi.mocked(Shares.resolve).mockResolvedValue({
        serverName: "test-server",
        status: "Running",
        address: { host: "example.com", port: 8080 },
      });

      renderWithRouter("test-token");

      await waitFor(() => {
        expect(screen.getByText("example.com:8080")).toBeInTheDocument();
      });

      const copyBtn = screen.getByRole("button", { name: /copy/i });
      fireEvent.click(copyBtn);

      expect(mockClipboard).toHaveBeenCalledWith("example.com:8080");
    });
  });

  describe("Asleep states (T183)", () => {
    it("renders Asleep with Start button by default", async () => {
      vi.mocked(Shares.resolve).mockResolvedValue({
        serverName: "mc-survival",
        status: "Suspended",
      });

      renderWithRouter("test-token");

      await waitFor(() => {
        expect(screen.getByText("mc-survival")).toBeInTheDocument();
        expect(screen.getByText("Asleep")).toBeInTheDocument();
        expect(
          screen.getByText("This server is asleep to save resources")
        ).toBeInTheDocument();
      });

      expect(screen.getByRole("button", { name: /start server/i })).toBeInTheDocument();
    });

    it("shows view-only when user tries to start but polling shows still asleep", async () => {
      vi.useFakeTimers();

      vi.mocked(Shares.resolve)
        .mockResolvedValueOnce({
          serverName: "mc-survival",
          status: "Suspended",
        })
        .mockResolvedValueOnce({
          serverName: "mc-survival",
          status: "Suspended",
        });

      vi.mocked(Shares.start).mockResolvedValue();

      renderWithRouter("test-token");

      await waitFor(() => {
        expect(screen.getByRole("button", { name: /start server/i })).toBeInTheDocument();
      });

      const startBtn = screen.getByRole("button", { name: /start server/i });
      fireEvent.click(startBtn);

      expect(vi.mocked(Shares.start)).toHaveBeenCalledWith("test-token");

      // Should show Starting state
      await waitFor(() => {
        expect(screen.getByText(/The server is waking up/)).toBeInTheDocument();
      });

      // Advance time to trigger polling
      vi.advanceTimersByTime(2000);

      // Polling should show it's still asleep, so transition to view-only
      await waitFor(() => {
        expect(screen.getByText("This server is asleep right now")).toBeInTheDocument();
        expect(
          screen.queryByRole("button", { name: /start server/i })
        ).not.toBeInTheDocument();
      });

      vi.useRealTimers();
    });

    it("handles Stopped status the same as Suspended", async () => {
      vi.mocked(Shares.resolve).mockResolvedValue({
        serverName: "test-server",
        status: "Stopped",
      });

      renderWithRouter("test-token");

      await waitFor(() => {
        expect(screen.getByRole("button", { name: /start server/i })).toBeInTheDocument();
      });
    });

    it("T183: Start button calls Shares.start and transitions to Starting", async () => {
      vi.mocked(Shares.resolve)
        .mockResolvedValueOnce({
          serverName: "mc-survival",
          status: "Suspended",
        })
        .mockResolvedValueOnce({
          serverName: "mc-survival",
          status: "Starting",
        });

      vi.mocked(Shares.start).mockResolvedValue();

      renderWithRouter("test-token");

      await waitFor(() => {
        expect(screen.getByRole("button", { name: /start server/i })).toBeInTheDocument();
      });

      const startBtn = screen.getByRole("button", { name: /start server/i });
      fireEvent.click(startBtn);

      expect(vi.mocked(Shares.start)).toHaveBeenCalledWith("test-token");

      await waitFor(() => {
        expect(screen.getByText(/The server is waking up/)).toBeInTheDocument();
      });
    });
  });

  describe("Starting state with polling (T184)", () => {
    it("T184: renders Starting state with spinner and message", async () => {
      vi.mocked(Shares.resolve).mockResolvedValue({
        serverName: "mc-survival",
        status: "Starting",
      });

      renderWithRouter("test-token");

      await waitFor(() => {
        expect(screen.getByText("mc-survival")).toBeInTheDocument();
        expect(screen.getByText(/The server is waking up/)).toBeInTheDocument();
      });
    });

    it("T184: polls resolve endpoint every 2 seconds", async () => {
      vi.useFakeTimers();

      vi.mocked(Shares.resolve).mockResolvedValue({
        serverName: "mc-survival",
        status: "Starting",
      });

      renderWithRouter("test-token");

      await waitFor(() => {
        expect(screen.getByText(/The server is waking up/)).toBeInTheDocument();
      });

      // First call on mount
      expect(vi.mocked(Shares.resolve)).toHaveBeenCalledTimes(1);

      // Advance time
      vi.advanceTimersByTime(2000);

      await waitFor(() => {
        expect(vi.mocked(Shares.resolve)).toHaveBeenCalledTimes(2);
      });

      vi.useRealTimers();
    });

    it("T184: transitions to Up when server is Running", async () => {
      vi.useFakeTimers();

      vi.mocked(Shares.resolve)
        .mockResolvedValueOnce({
          serverName: "mc-survival",
          status: "Starting",
        })
        .mockResolvedValueOnce({
          serverName: "mc-survival",
          status: "Running",
          address: { host: "play.gameplane.example", port: 25565 },
        });

      renderWithRouter("test-token");

      await waitFor(() => {
        expect(screen.getByText(/The server is waking up/)).toBeInTheDocument();
      });

      vi.advanceTimersByTime(2000);

      await waitFor(() => {
        expect(screen.getByText("Online")).toBeInTheDocument();
        expect(screen.getByText("play.gameplane.example:25565")).toBeInTheDocument();
      });

      vi.useRealTimers();
    });

    it("T184: cancels polling on unmount", async () => {
      vi.useFakeTimers();
      const clearIntervalSpy = vi.spyOn(globalThis, "clearInterval");

      vi.mocked(Shares.resolve).mockResolvedValue({
        serverName: "mc-survival",
        status: "Starting",
      });

      const { unmount } = render(
        <div>
          <SharePage />
        </div>
      );

      await waitFor(() => {
        expect(vi.mocked(Shares.resolve)).toHaveBeenCalled();
      });

      unmount();

      // clearInterval should have been called
      expect(clearIntervalSpy).toHaveBeenCalled();

      vi.useRealTimers();
    });
  });

  describe("Invalid/expired state (T185)", () => {
    it("T185: maps 404 error to invalid state with neutral copy", async () => {
      vi.mocked(Shares.resolve).mockRejectedValue(
        new APIError(404, "Not found")
      );

      renderWithRouter("test-token");

      await waitFor(() => {
        expect(screen.getByText("Link not available")).toBeInTheDocument();
        expect(
          screen.getByText(/This link may be invalid, expired, or revoked/)
        ).toBeInTheDocument();
      });
    });

    it("T185: maps 429 rate-limit to invalid state with same message", async () => {
      vi.mocked(Shares.resolve).mockRejectedValue(
        new APIError(429, "Too many requests")
      );

      renderWithRouter("test-token");

      await waitFor(() => {
        expect(screen.getByText("Link not available")).toBeInTheDocument();
        expect(
          screen.getByText(/This link may be invalid, expired, or revoked/)
        ).toBeInTheDocument();
      });
    });

    it("T185: maps auth errors to invalid state with neutral copy", async () => {
      vi.mocked(Shares.resolve).mockRejectedValue(
        new APIError(401, "Unauthorized")
      );

      renderWithRouter("test-token");

      await waitFor(() => {
        expect(screen.getByText("Link not available")).toBeInTheDocument();
      });
    });

    it("T185: does not reveal whether link was valid, revoked, or expired", async () => {
      render(
        <div>
          <SharePage />
        </div>
      );

      // Test scenario 1: invalid token (404)
      vi.mocked(Shares.resolve).mockRejectedValueOnce(
        new APIError(404, "Not found")
      );

      await waitFor(() => {
        expect(screen.getByText("Link not available")).toBeInTheDocument();
      });

      const invalidMsg = screen.getByText(/This link may be invalid/);
      expect(invalidMsg).toBeInTheDocument();

      // Verify no specific error detail is shown
      expect(screen.queryByText(/404|expired|revoked|valid/i)).not.toBeInTheDocument();
    });
  });

  describe("Appearance preference (T186)", () => {
    it("T186: reads appearance preference from localStorage", async () => {
      localStorage.setItem("gameplane-theme", "dark");

      vi.mocked(Shares.resolve).mockResolvedValue({
        serverName: "test-server",
        status: "Running",
      });

      renderWithRouter("test-token");

      await waitFor(() => {
        expect(document.documentElement.getAttribute("data-theme")).toBe("dark");
      });
    });

    it("T186: applies light theme when stored", async () => {
      localStorage.setItem("gameplane-theme", "light");

      vi.mocked(Shares.resolve).mockResolvedValue({
        serverName: "test-server",
        status: "Running",
      });

      renderWithRouter("test-token");

      await waitFor(() => {
        expect(document.documentElement.getAttribute("data-theme")).toBe("light");
      });
    });

    it("T186: defaults to system preference when not stored", async () => {
      localStorage.removeItem("gameplane-theme");

      const mockMatchMedia = vi.fn((query) => ({
        matches: query === "(prefers-color-scheme: dark)",
      }));
      window.matchMedia = mockMatchMedia as unknown as typeof window.matchMedia;

      vi.mocked(Shares.resolve).mockResolvedValue({
        serverName: "test-server",
        status: "Running",
      });

      renderWithRouter("test-token");

      await waitFor(() => {
        expect(document.documentElement.getAttribute("data-theme")).toBe("dark");
      });
    });

    it("T186: does not expose an appearance toggle", async () => {
      vi.mocked(Shares.resolve).mockResolvedValue({
        serverName: "test-server",
        status: "Running",
      });

      renderWithRouter("test-token");

      await waitFor(() => {
        expect(screen.getByText("test-server")).toBeInTheDocument();
      });

      // Verify no toggle or theme controls exist
      expect(
        screen.queryByRole("button", { name: /light|dark|appearance/i })
      ).not.toBeInTheDocument();
      expect(screen.queryByRole("group", { name: /appearance/i })).not.toBeInTheDocument();
    });
  });

  describe("Privacy compliance (FR-005)", () => {
    it("T186: never renders cluster name", async () => {
      vi.mocked(Shares.resolve).mockResolvedValue({
        serverName: "mc-survival",
        status: "Running",
        address: { host: "play.gameplane.example", port: 25565 },
      });

      renderWithRouter("test-token");

      await waitFor(() => {
        expect(screen.getByText("mc-survival")).toBeInTheDocument();
      });

      expect(screen.queryByText(/prod-east|cluster-1|my-cluster/i)).not.toBeInTheDocument();
    });

    it("T186: never renders namespace", async () => {
      vi.mocked(Shares.resolve).mockResolvedValue({
        serverName: "mc-survival",
        status: "Running",
      });

      renderWithRouter("test-token");

      await waitFor(() => {
        expect(screen.getByText("mc-survival")).toBeInTheDocument();
      });

      expect(screen.queryByText(/namespace|default|prod/i)).not.toBeInTheDocument();
    });

    it("T186: never renders version string", async () => {
      vi.mocked(Shares.resolve).mockResolvedValue({
        serverName: "mc-survival",
        status: "Running",
      });

      renderWithRouter("test-token");

      await waitFor(() => {
        expect(screen.getByText("mc-survival")).toBeInTheDocument();
      });

      expect(screen.queryByText(/v\d+\.\d+|beta|alpha|rc\d+/i)).not.toBeInTheDocument();
    });

    it("T186: never renders user names", async () => {
      vi.mocked(Shares.resolve).mockResolvedValue({
        serverName: "mc-survival",
        status: "Running",
      });

      renderWithRouter("test-token");

      await waitFor(() => {
        expect(screen.getByText("mc-survival")).toBeInTheDocument();
      });

      expect(screen.queryByText(/alice|bob|admin|owner|user/i)).not.toBeInTheDocument();
    });

    it("T186: never renders server counts", async () => {
      vi.mocked(Shares.resolve).mockResolvedValue({
        serverName: "mc-survival",
        status: "Running",
      });

      renderWithRouter("test-token");

      await waitFor(() => {
        expect(screen.getByText("mc-survival")).toBeInTheDocument();
      });

      // Make sure "3 players online" is shown only if from the response
      // but not server counts like "5 servers" or "10 running"
      expect(screen.queryByText(/\d+ servers|servers? running/i)).not.toBeInTheDocument();
    });

    it("T186: handles empty/neutral response as invalid state", async () => {
      vi.mocked(Shares.resolve).mockResolvedValue({
        serverName: "", // Empty serverName indicates error/neutral response
        status: "Unknown",
      });

      renderWithRouter("test-token");

      await waitFor(() => {
        expect(screen.getByText("Link not available")).toBeInTheDocument();
      });
    });
  });

  describe("Error handling", () => {
    it("handles network errors gracefully", async () => {
      vi.mocked(Shares.resolve).mockRejectedValue(new Error("Network error"));

      renderWithRouter("test-token");

      await waitFor(() => {
        expect(screen.getByText("Link not available")).toBeInTheDocument();
      });
    });

    it("handles Start call failures by showing invalid state", async () => {
      vi.mocked(Shares.resolve).mockResolvedValueOnce({
        serverName: "mc-survival",
        status: "Suspended",
      });

      vi.mocked(Shares.start).mockRejectedValue(
        new APIError(429, "Rate limited")
      );

      renderWithRouter("test-token");

      await waitFor(() => {
        expect(screen.getByRole("button", { name: /start server/i })).toBeInTheDocument();
      });

      const startBtn = screen.getByRole("button", { name: /start server/i });
      fireEvent.click(startBtn);

      await waitFor(() => {
        expect(screen.getByText("Link not available")).toBeInTheDocument();
      });
    });
  });
});
