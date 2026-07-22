import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { ServerSleepCard } from "./ServerSleepCard";
import { makeServer } from "@/test/factories";

describe("ServerSleepCard", () => {
  it("renders nothing when idle is not configured or active", () => {
    const gs = makeServer();
    const { container } = render(<ServerSleepCard gs={gs} />);
    expect(container.firstChild).toBeNull();
  });

  it("renders nothing when gs is undefined", () => {
    const { container } = render(<ServerSleepCard gs={undefined} />);
    expect(container.firstChild).toBeNull();
  });

  it("renders the card when idle is configured", () => {
    const gs = makeServer({
      spec: {
        templateRef: { name: "minecraft-vanilla" },
        idle: { enabled: true, afterMinutes: 30, wakeWindows: ["0 9 * * *"] },
      },
    });
    render(<ServerSleepCard gs={gs} />);
    expect(screen.getByText("Sleep")).toBeInTheDocument();
    expect(screen.getByText("30 minutes")).toBeInTheDocument();
    expect(screen.getByText("0 9 * * *")).toBeInTheDocument();
  });

  it("displays asleep state with timestamps", () => {
    const gs = makeServer({
      status: {
        idle: {
          asleep: true,
          asleepSince: "2026-05-07T12:00:00Z",
          lastWakeTime: "2026-05-07T09:00:00Z",
        },
      },
    });
    render(<ServerSleepCard gs={gs} />);
    expect(screen.getByText("Asleep")).toBeInTheDocument();
    expect(screen.getByText("Asleep since")).toBeInTheDocument();
    expect(screen.getByText("Last woken")).toBeInTheDocument();
    expect(screen.getByText("Waking takes normal boot time before players can connect.")).toBeInTheDocument();
  });

  it("displays counting-down state", () => {
    const gs = makeServer({
      status: {
        idle: {
          asleep: false,
          emptySince: "2026-05-07T11:00:00Z",
          reason: "empty, sleeping in 5m30s",
        },
      },
    });
    render(<ServerSleepCard gs={gs} />);
    expect(screen.getByText("Empty since")).toBeInTheDocument();
  });

  it("displays will-never-sleep state", () => {
    const gs = makeServer({
      status: {
        idle: {
          asleep: false,
          reason: "no player count reported",
        },
      },
    });
    render(<ServerSleepCard gs={gs} />);
    expect(screen.getByText("Will never sleep")).toBeInTheDocument();
    expect(screen.getByText("No player count reported")).toBeInTheDocument();
  });

  it("lists multiple wake windows", () => {
    const gs = makeServer({
      spec: {
        templateRef: { name: "minecraft-vanilla" },
        idle: {
          enabled: true,
          afterMinutes: 60,
          wakeWindows: ["0 9 * * 1-5", "0 12 * * 6-0"],
        },
      },
    });
    render(<ServerSleepCard gs={gs} />);
    expect(screen.getByText("0 9 * * 1-5")).toBeInTheDocument();
    expect(screen.getByText("0 12 * * 6-0")).toBeInTheDocument();
  });

  it("shows unconfigured when no wake windows", () => {
    const gs = makeServer({
      spec: {
        templateRef: { name: "minecraft-vanilla" },
        idle: { enabled: true, afterMinutes: 30, wakeWindows: [] },
      },
    });
    render(<ServerSleepCard gs={gs} />);
    expect(screen.getByText("Not configured")).toBeInTheDocument();
  });

  it("displays configured caveats", () => {
    const gs = makeServer({
      spec: {
        templateRef: { name: "minecraft-vanilla" },
        idle: { enabled: true, afterMinutes: 30, wakeWindows: [] },
      },
    });
    render(<ServerSleepCard gs={gs} />);
    expect(screen.getByText("A game that reports no player count will never sleep.")).toBeInTheDocument();
    expect(screen.getByText("A wake window never restarts a server you stopped by hand.")).toBeInTheDocument();
  });
});
