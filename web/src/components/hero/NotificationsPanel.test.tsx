import { describe, it, expect, vi, beforeEach } from "vitest";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderWithQuery } from "@/test/render";

// Store callbacks for triggering events in tests
let sseCallback: ((ev: unknown) => void) | null = null;

vi.mock("@/lib/sse", () => ({
  openEventStream: (opts: { onEvent: (ev: unknown) => void }) => {
    sseCallback = opts.onEvent;
    return () => {
      sseCallback = null;
    };
  },
  queryKeyForKind: (kind: string) => {
    const keyMap: Record<string, string[] | null> = {
      servers: ["servers"],
      templates: ["templates"],
      backups: ["backups"],
      schedules: ["schedules"],
      restores: ["restores"],
    };
    return keyMap[kind] ?? null;
  },
}));

import { NotificationsPanel } from "./NotificationsPanel";

describe("NotificationsPanel", () => {
  beforeEach(() => {
    sseCallback = null;
  });

  it("renders bell button with aria label", () => {
    renderWithQuery(<NotificationsPanel />);
    const bell = screen.getByRole("button", { name: /notifications/i });
    expect(bell).toBeInTheDocument();
  });

  it("shows no recent activity when there are no notices", async () => {
    renderWithQuery(<NotificationsPanel />);
    const bell = screen.getByRole("button", { name: /notifications/i });
    await userEvent.click(bell);
    expect(await screen.findByText("Recent activity")).toBeInTheDocument();
    expect(screen.getByText("No recent activity.")).toBeInTheDocument();
  });

  it("bell toggles the notifications panel open and closed", async () => {
    renderWithQuery(<NotificationsPanel />);
    const bell = screen.getByRole("button", { name: /notifications/i });

    // Initially closed — no "Recent activity" text
    expect(screen.queryByText("Recent activity")).not.toBeInTheDocument();

    // Click to open
    await userEvent.click(bell);
    expect(
      await screen.findByText("Recent activity")
    ).toBeInTheDocument();

    // Click to close
    await userEvent.click(bell);
    await waitFor(() => {
      expect(screen.queryByText("Recent activity")).not.toBeInTheDocument();
    });
  });

  it("displays notice events with text and time", async () => {
    renderWithQuery(<NotificationsPanel />);

    // Simulate an SSE event
    expect(sseCallback).not.toBeNull();
    sseCallback!({
      kind: "servers",
      eventType: "ADDED",
      object: { metadata: { name: "test-server" } },
    });

    const bell = screen.getByRole("button", { name: /notifications/i });
    await userEvent.click(bell);

    // Wait for the notice to appear
    expect(
      await screen.findByText(/added server test-server/)
    ).toBeInTheDocument();
  });

  it("badge shows unread count", async () => {
    renderWithQuery(<NotificationsPanel />);

    // Simulate multiple SSE events
    expect(sseCallback).not.toBeNull();
    sseCallback!({
      kind: "servers",
      eventType: "ADDED",
      object: { metadata: { name: "server-1" } },
    });
    sseCallback!({
      kind: "backups",
      eventType: "MODIFIED",
      object: { metadata: { name: "backup-1" } },
    });

    // Badge should show 2 unread
    expect(await screen.findByText("2")).toBeInTheDocument();
  });

  it("badge shows 9+ when unread count exceeds 9", async () => {
    renderWithQuery(<NotificationsPanel />);

    expect(sseCallback).not.toBeNull();
    // Simulate 11 events
    for (let i = 1; i <= 11; i++) {
      sseCallback!({
        kind: "servers",
        eventType: "ADDED",
        object: { metadata: { name: `server-${i}` } },
      });
    }

    expect(await screen.findByText("9+")).toBeInTheDocument();
  });

  it("resets unread count to 0 when panel is opened", async () => {
    renderWithQuery(<NotificationsPanel />);

    sseCallback!({
      kind: "servers",
      eventType: "ADDED",
      object: { metadata: { name: "server-1" } },
    });

    // Badge shows 1
    expect(await screen.findByText("1")).toBeInTheDocument();

    // Click bell to open panel
    const bell = screen.getByRole("button", { name: /notifications/i });
    await userEvent.click(bell);

    // Wait for panel to render
    await screen.findByText("Recent activity");

    // Badge should be gone (unread count reset to 0)
    await waitFor(() => {
      expect(screen.queryByText("1")).not.toBeInTheDocument();
    });
  });

  it("caps notices at 50 items", async () => {
    renderWithQuery(<NotificationsPanel />);

    // Simulate 60 events
    for (let i = 1; i <= 60; i++) {
      sseCallback!({
        kind: "servers",
        eventType: "ADDED",
        object: { metadata: { name: `server-${i}` } },
      });
    }

    const bell = screen.getByRole("button", { name: /notifications/i });
    await userEvent.click(bell);

    // Wait for panel to appear
    await screen.findByText("Recent activity");

    const noticeItems = screen.getAllByRole("listitem");
    // Should have exactly 50 items (capped)
    expect(noticeItems).toHaveLength(50);
  });

  it("removes trailing 's' from kind in notice text", async () => {
    renderWithQuery(<NotificationsPanel />);

    sseCallback!({
      kind: "servers",
      eventType: "MODIFIED",
      object: { metadata: { name: "my-server" } },
    });

    const bell = screen.getByRole("button", { name: /notifications/i });
    await userEvent.click(bell);

    // Should say "modified server" (not "modified servers")
    expect(
      await screen.findByText(/modified server my-server/)
    ).toBeInTheDocument();
  });
});
