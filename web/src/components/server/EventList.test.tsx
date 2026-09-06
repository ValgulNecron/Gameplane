import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { EventList } from "./EventList";
import type { NormalizedServerEvent } from "@/lib/events";

// Mock formatRelative to return consistent test values
vi.mock("@/lib/utils", () => ({
  formatRelative: (ts: string) => {
    const date = new Date(ts);
    if (date.toISOString().includes("2024-01-01")) return "1h ago";
    if (date.toISOString().includes("2024-01-02")) return "2h ago";
    return "3m ago";
  },
}));

describe("EventList", () => {
  const mockEvents: NormalizedServerEvent[] = [
    {
      id: "event-1",
      ts: "2024-01-01T12:00:00Z",
      kind: "info",
      message: "Container started successfully",
      source: "kubelet",
    },
    {
      id: "event-2",
      ts: "2024-01-02T12:00:00Z",
      kind: "warn",
      message: "High memory usage detected",
      source: "metrics-server",
    },
    {
      id: "event-3",
      ts: "2024-01-03T12:00:00Z",
      kind: "error",
      message: "Failed to pull image",
      source: "kubelet",
    },
    {
      id: "event-4",
      ts: "2024-01-04T12:00:00Z",
      kind: "info",
      message: "Pod scheduled on node",
    },
  ];

  it("renders a list of events with messages", () => {
    render(<EventList events={mockEvents} />);
    expect(screen.getByText("Container started successfully")).toBeInTheDocument();
    expect(screen.getByText("High memory usage detected")).toBeInTheDocument();
    expect(screen.getByText("Failed to pull image")).toBeInTheDocument();
    expect(screen.getByText("Pod scheduled on node")).toBeInTheDocument();
  });

  it("renders event sources correctly", () => {
    render(<EventList events={mockEvents} />);
    expect(screen.getByText("kubelet")).toBeInTheDocument();
    expect(screen.getByText("metrics-server")).toBeInTheDocument();
  });

  it("renders 'system' as default source when source is undefined", () => {
    render(<EventList events={mockEvents} />);
    const systemSources = screen.getAllByText("system");
    expect(systemSources.length).toBeGreaterThan(0);
  });

  it("renders relative timestamps", () => {
    render(<EventList events={mockEvents} />);
    expect(screen.getByText("1h ago")).toBeInTheDocument();
    expect(screen.getByText("2h ago")).toBeInTheDocument();
  });

  it("renders empty state when no events", () => {
    render(<EventList events={[]} emptyMessage="No events yet." />);
    expect(screen.getByText("No events yet.")).toBeInTheDocument();
  });

  it("renders custom empty message", () => {
    render(<EventList events={[]} emptyMessage="No warnings or errors." />);
    expect(screen.getByText("No warnings or errors.")).toBeInTheDocument();
  });

  it("does not render list when events are empty", () => {
    const { container } = render(<EventList events={[]} />);
    const listItems = container.querySelectorAll("li");
    expect(listItems.length).toBe(0);
  });

  it("renders event list with correct number of items", () => {
    const { container } = render(<EventList events={mockEvents} />);
    const listItems = container.querySelectorAll("li");
    expect(listItems.length).toBe(mockEvents.length);
  });

  it("renders info events with accent colored dots", () => {
    const { container } = render(<EventList events={[mockEvents[0]] } />);
    const dot = container.querySelector(".bg-accent");
    expect(dot).toBeInTheDocument();
  });

  it("renders warning events with warning colored dots", () => {
    const { container } = render(<EventList events={[mockEvents[1]]} />);
    const dot = container.querySelector(".bg-warning");
    expect(dot).toBeInTheDocument();
  });

  it("renders error events with danger colored dots", () => {
    const { container } = render(<EventList events={[mockEvents[2]]} />);
    const dot = container.querySelector(".bg-danger");
    expect(dot).toBeInTheDocument();
  });

  it("renders each event in its own list item", () => {
    const { container } = render(<EventList events={mockEvents} />);
    const listItems = container.querySelectorAll("li");
    expect(listItems.length).toBe(4);
    expect(listItems[0]).toHaveTextContent("Container started successfully");
    expect(listItems[1]).toHaveTextContent("High memory usage detected");
    expect(listItems[2]).toHaveTextContent("Failed to pull image");
    expect(listItems[3]).toHaveTextContent("Pod scheduled on node");
  });

  it("applies correct styling classes to list structure", () => {
    const { container } = render(<EventList events={mockEvents} />);
    const list = container.querySelector("ul");
    expect(list).toHaveClass("divide-y", "divide-border");
  });

  it("applies flex layout to each event row", () => {
    const { container } = render(<EventList events={[mockEvents[0]]} />);
    const listItem = container.querySelector("li");
    expect(listItem).toHaveClass("flex", "items-start", "gap-3", "px-6", "py-3");
  });

  it("renders message text with foreground color", () => {
    const { container } = render(<EventList events={[mockEvents[0]]} />);
    const messageDiv = container.querySelector(".text-foreground");
    expect(messageDiv).toBeInTheDocument();
    expect(messageDiv?.textContent).toContain("Container started successfully");
  });

  it("renders source and timestamp in muted text color", () => {
    const { container } = render(<EventList events={[mockEvents[0]]} />);
    const mutedTexts = container.querySelectorAll(".text-muted");
    expect(mutedTexts.length).toBeGreaterThanOrEqual(2);
  });

  it("handles multiple events with different kinds", () => {
    const mixedEvents: NormalizedServerEvent[] = [
      { id: "1", ts: "2024-01-01T00:00:00Z", kind: "info", message: "Info event" },
      { id: "2", ts: "2024-01-01T00:00:00Z", kind: "warn", message: "Warn event" },
      { id: "3", ts: "2024-01-01T00:00:00Z", kind: "error", message: "Error event" },
    ];
    const { container } = render(<EventList events={mixedEvents} />);
    expect(container.querySelector(".bg-accent")).toBeInTheDocument();
    expect(container.querySelector(".bg-warning")).toBeInTheDocument();
    expect(container.querySelector(".bg-danger")).toBeInTheDocument();
  });

  it("uses event id as key for list items", () => {
    const { container, rerender } = render(<EventList events={mockEvents} />);
    const initialItems = container.querySelectorAll("li");
    expect(initialItems.length).toBe(4);

    const reorderedEvents = [mockEvents[3], mockEvents[2], mockEvents[1], mockEvents[0]];
    rerender(<EventList events={reorderedEvents} />);
    const rerenderItems = container.querySelectorAll("li");
    expect(rerenderItems.length).toBe(4);
    expect(rerenderItems[0]).toHaveTextContent("Pod scheduled on node");
  });
});
