import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { EventsTab } from "./Events";

const fetchMock = vi.fn();

beforeEach(() => {
  vi.stubGlobal("fetch", fetchMock);
});

afterEach(() => {
  fetchMock.mockReset();
  vi.unstubAllGlobals();
});

function withClient(ui: ReactNode) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={qc}>{ui}</QueryClientProvider>;
}

function jsonRes(body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  });
}

describe("EventsTab", () => {
  it("renders all events by default", async () => {
    fetchMock.mockImplementation((path: string) => {
      if (String(path).includes("/events")) {
        return Promise.resolve(
          jsonRes([
            {
              id: "e1",
              time: "2026-01-01T00:00:00Z",
              type: "Normal",
              reason: "Created",
              message: "created pod",
              source: "kubelet",
              object: "Pod/s1-0",
              count: 1,
            },
            {
              id: "e2",
              time: "2026-01-01T00:00:01Z",
              type: "Warning",
              reason: "Failed",
              message: "Back-off pulling image",
              source: "kubelet",
              object: "Pod/s1-0",
              count: 3,
            },
          ])
        );
      }
      return Promise.reject(new Error("unexpected fetch"));
    });

    render(withClient(<EventsTab name="s1" />));
    expect(await screen.findByText("Created: created pod")).toBeInTheDocument();
    expect(await screen.findByText("Failed: Back-off pulling image")).toBeInTheDocument();
  });

  it("filters to only info events when Info button is clicked", async () => {
    fetchMock.mockImplementation((path: string) => {
      if (String(path).includes("/events")) {
        return Promise.resolve(
          jsonRes([
            {
              id: "e1",
              time: "2026-01-01T00:00:00Z",
              type: "Normal",
              reason: "Created",
              message: "created pod",
              source: "kubelet",
              object: "Pod/s1-0",
              count: 1,
            },
            {
              id: "e2",
              time: "2026-01-01T00:00:01Z",
              type: "Warning",
              reason: "Failed",
              message: "Back-off pulling image",
              source: "kubelet",
              object: "Pod/s1-0",
              count: 3,
            },
          ])
        );
      }
      return Promise.reject(new Error("unexpected fetch"));
    });

    const user = userEvent.setup();
    render(withClient(<EventsTab name="s1" />));
    expect(await screen.findByText("Created: created pod")).toBeInTheDocument();

    const infoButton = screen.getByRole("button", { name: "Info" });
    await user.click(infoButton);

    // Info event should still be visible
    expect(screen.getByText("Created: created pod")).toBeInTheDocument();
    // Warning event should be hidden
    expect(screen.queryByText("Failed: Back-off pulling image")).not.toBeInTheDocument();
  });

  it("filters to only warnings when Warnings button is clicked", async () => {
    fetchMock.mockImplementation((path: string) => {
      if (String(path).includes("/events")) {
        return Promise.resolve(
          jsonRes([
            {
              id: "e1",
              time: "2026-01-01T00:00:00Z",
              type: "Normal",
              reason: "Created",
              message: "created pod",
              source: "kubelet",
              object: "Pod/s1-0",
              count: 1,
            },
            {
              id: "e2",
              time: "2026-01-01T00:00:01Z",
              type: "Warning",
              reason: "Failed",
              message: "Back-off pulling image",
              source: "kubelet",
              object: "Pod/s1-0",
              count: 3,
            },
          ])
        );
      }
      return Promise.reject(new Error("unexpected fetch"));
    });

    const user = userEvent.setup();
    render(withClient(<EventsTab name="s1" />));
    expect(await screen.findByText("Created: created pod")).toBeInTheDocument();

    const warningsButton = screen.getByRole("button", { name: "Warnings" });
    await user.click(warningsButton);

    // Warning event should be visible
    expect(screen.getByText("Failed: Back-off pulling image")).toBeInTheDocument();
    // Info event should be hidden
    expect(screen.queryByText("Created: created pod")).not.toBeInTheDocument();
  });

  it("shows empty message when no events match the filter", async () => {
    fetchMock.mockImplementation((path: string) => {
      if (String(path).includes("/events")) {
        return Promise.resolve(
          jsonRes([
            {
              id: "e1",
              time: "2026-01-01T00:00:00Z",
              type: "Normal",
              reason: "Created",
              message: "created pod",
              source: "kubelet",
              object: "Pod/s1-0",
              count: 1,
            },
          ])
        );
      }
      return Promise.reject(new Error("unexpected fetch"));
    });

    const user = userEvent.setup();
    render(withClient(<EventsTab name="s1" />));
    expect(await screen.findByText("Created: created pod")).toBeInTheDocument();

    const warningsButton = screen.getByRole("button", { name: "Warnings" });
    await user.click(warningsButton);

    expect(screen.getByText("No warnings or errors.")).toBeInTheDocument();
  });
});
