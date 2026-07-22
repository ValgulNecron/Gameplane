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

  // A2: a disabled-but-materialized spec.idle (the apiserver defaults
  // enabled to false rather than omitting the field) must not render an
  // empty card once the operator has cleared status.idle to match.
  it("renders nothing when idle is enabled:false and there's no status", () => {
    const gs = makeServer({
      spec: {
        templateRef: { name: "minecraft-vanilla" },
        idle: { enabled: false, afterMinutes: 60 },
      },
      status: {},
    });
    const { container } = render(<ServerSleepCard gs={gs} />);
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

  // A6: singular/plural.
  it("pluralizes a one-minute sleep-after threshold", () => {
    const gs = makeServer({
      spec: {
        templateRef: { name: "minecraft-vanilla" },
        idle: { enabled: true, afterMinutes: 1, wakeWindows: [] },
      },
    });
    render(<ServerSleepCard gs={gs} />);
    expect(screen.getByText("1 minute")).toBeInTheDocument();
  });

  it("displays asleep state with timestamps", () => {
    const gs = makeServer({
      status: {
        idle: {
          asleep: true,
          asleepSince: "2026-05-07T12:00:00Z",
          lastWakeTime: "2026-05-07T09:00:00Z",
          reason: "asleep (no players)",
        },
      },
    });
    render(<ServerSleepCard gs={gs} />);
    expect(screen.getByText("Asleep")).toBeInTheDocument();
    expect(screen.getByText("Asleep since")).toBeInTheDocument();
    expect(screen.getByText("Last woken")).toBeInTheDocument();
    expect(screen.getByText("Waking takes normal boot time before players can connect.")).toBeInTheDocument();
    // The happy-path reason is redundant with the "Asleep" state row itself
    // and must not be echoed a second time.
    expect(screen.queryByText("Asleep (no players)")).not.toBeInTheDocument();
  });

  // A3: an unparseable wake window keeps the server asleep forever with no
  // other signal — the operator deliberately doesn't fail the reconcile for
  // it, so the Overview is the only place this surfaces.
  it("surfaces the reason while asleep when a wake window failed to parse", () => {
    const gs = makeServer({
      status: {
        idle: {
          asleep: true,
          asleepSince: "2026-05-07T12:00:00Z",
          reason: 'asleep; wake window invalid: parse wake window "bogus": expected 5 to 6 fields',
        },
      },
    });
    render(<ServerSleepCard gs={gs} />);
    expect(screen.getByText("Asleep")).toBeInTheDocument();
    expect(screen.getByText(/wake window invalid/i)).toBeInTheDocument();
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
    expect(screen.getByText("Empty, sleeping in 5m30s")).toBeInTheDocument();
  });

  // A4: between saving spec.idle.enabled and the next reconcile writing
  // status.idle, none of the state blocks apply — say so instead of
  // rendering a state-less card that looks like the change didn't take.
  it("shows a waiting state when idle was just enabled and the operator hasn't reconciled yet", () => {
    const gs = makeServer({
      spec: {
        templateRef: { name: "minecraft-vanilla" },
        idle: { enabled: true, afterMinutes: 30, wakeWindows: [] },
      },
      status: {},
    });
    render(<ServerSleepCard gs={gs} />);
    expect(screen.getByText("Waiting for the operator")).toBeInTheDocument();
  });

  // A1: the operator's own reason strings are the state — only one of them
  // means the trigger can never fire. Verified against the exact strings
  // idleDecide/idleEligible emit (gameserver_idle.go); a healthy, working
  // state must never read as "Will never sleep".
  describe("renders the operator's reason as the state (A1)", () => {
    it.each([
      ["this game reports no player count", "Will never sleep"],
      ["3 player(s) online", "3 player(s) online"],
      ["stopped by user", "Stopped by user"],
      ["agent heartbeat is stale; player count unknown", "Agent heartbeat is stale; player count unknown"],
      ["another lifecycle operation is in flight", "Another lifecycle operation is in flight"],
      ["server is Starting, not Running", "Server is Starting, not Running"],
      ["woken by request", "Woken by request"],
      ["woken by wake window", "Woken by wake window"],
    ])("reason %j renders as %j", (reason, expectedLabel) => {
      const gs = makeServer({
        status: { idle: { asleep: false, reason } },
      });
      render(<ServerSleepCard gs={gs} />);
      expect(screen.getByText(expectedLabel)).toBeInTheDocument();
    });

    it("shows the operator's reason underneath the 'Will never sleep' label", () => {
      const gs = makeServer({
        status: { idle: { asleep: false, reason: "this game reports no player count" } },
      });
      render(<ServerSleepCard gs={gs} />);
      expect(screen.getByText("Will never sleep")).toBeInTheDocument();
      expect(screen.getByText("This game reports no player count")).toBeInTheDocument();
    });

    it("does not show an explanatory sub-line for a normal working reason", () => {
      const gs = makeServer({
        status: { idle: { asleep: false, reason: "3 player(s) online" } },
      });
      render(<ServerSleepCard gs={gs} />);
      // Only the state row itself should carry the text — no duplicate.
      expect(screen.getAllByText("3 player(s) online")).toHaveLength(1);
    });
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
