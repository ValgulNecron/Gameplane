import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { OverviewTab } from "./Overview";
import type { GameServer, PlayersResp } from "@/types";

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

function gs(overrides: Partial<GameServer["status"]> = {}): GameServer {
  return {
    metadata: { name: "s1" },
    spec: { templateRef: { name: "minecraft-java" } },
    status: { phase: "Running", ...overrides },
  };
}

function jsonRes(body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  });
}

describe("OverviewTab recent events", () => {
  it("renders mapped Kubernetes events including a Warning", async () => {
    fetchMock.mockImplementation((path: string) => {
      if (String(path).includes("/events")) {
        return Promise.resolve(jsonRes([
          {
            id: "e1", time: "2026-01-01T00:00:00Z", type: "Warning",
            reason: "Failed", message: "Back-off pulling image",
            source: "kubelet", object: "Pod/s1-0", count: 3,
          },
          {
            id: "e2", time: "2026-01-01T00:00:00Z", type: "Normal",
            reason: "Pulling", message: "pulling image itzg/minecraft",
            source: "kubelet", object: "Pod/s1-0", count: 1,
          },
        ]));
      }
      return Promise.resolve(jsonRes({ online: 0, max: 20, players: [], asOf: "now" }));
    });
    render(withClient(<OverviewTab gs={gs({ phase: "Starting" })} name="s1" />));
    expect(await screen.findByText("Failed: Back-off pulling image")).toBeInTheDocument();
    expect(screen.getByText("Pulling: pulling image itzg/minecraft")).toBeInTheDocument();
  });
});

describe("OverviewTab metric tiles", () => {
  it("renders real CPU, memory and disk usage from the heartbeat", async () => {
    fetchMock.mockImplementation(() =>
      Promise.resolve(jsonRes({ online: 0, max: 20, players: [], asOf: "now" })));
    render(withClient(<OverviewTab name="s1" gs={gs({
      agent: {
        lastHeartbeat: "now",
        cpuMillicores: 500, cpuLimitMillicores: 2000, // 25%
        memoryBytes: 536870912, memoryLimitBytes: 1073741824, // 512 MB / 1.0 GB = 50%
        diskUsedBytes: 2147483648, diskTotalBytes: 10737418240, // 2.0 GB / 10 GB = 20%
      },
    })} />));
    expect(await screen.findByText("Disk")).toBeInTheDocument();
    expect(screen.getByText("25%")).toBeInTheDocument(); // CPU 500/2000
    expect(screen.getByText("0.5 / 2.0 cores")).toBeInTheDocument();
    expect(screen.getByText("50%")).toBeInTheDocument();
    // formatBytes gives one decimal below 10 units: 1 GiB → "1.0 GB".
    expect(screen.getByText("512 MB / 1.0 GB")).toBeInTheDocument();
    expect(screen.getByText("20%")).toBeInTheDocument();
    expect(screen.getByText("2.0 GB / 10 GB")).toBeInTheDocument();
    // No Network tile (no stock-K8s per-pod source), and Players lives in
    // the sidebar card, not the metric row.
    expect(screen.queryByText("Network")).toBeNull();
  });

  it("renders '—' for usage when the agent heartbeat is stale/absent", async () => {
    fetchMock.mockImplementation(() =>
      Promise.resolve(jsonRes({ online: 0, max: 20, players: [], asOf: "now" })));
    // A stale heartbeat: the API drops the usage readings and flags stale,
    // so the tiles must render "—" rather than a frozen value or NaN.
    render(withClient(<OverviewTab name="s1" gs={gs({
      agent: { lastHeartbeat: "old", stale: true },
    })} />));
    expect(await screen.findByText("Disk")).toBeInTheDocument();
    // CPU, Memory and Disk all unknown → at least three em-dashes.
    expect(screen.getAllByText("—").length).toBeGreaterThanOrEqual(3);
  });

  it("shows cores/bytes and '—' limits when usage is known but limits are absent", async () => {
    fetchMock.mockImplementation(() =>
      Promise.resolve(jsonRes({ online: 0, max: 20, players: [], asOf: "now" })));
    // phase Starting disables the roster query, so the Players card falls
    // back to status.agent.playersOnline. Usage values are set but the
    // limits/total are absent/0, exercising the no-limit tile branches.
    render(withClient(<OverviewTab name="s1" gs={gs({
      phase: "Starting",
      agent: {
        lastHeartbeat: "now",
        cpuMillicores: 1500,                          // no limit → "1.50 cores", no bar
        memoryBytes: 536870912,                       // no limit → "512 MB" primary + "512 MB / —"
        diskUsedBytes: 1073741824, diskTotalBytes: 0, // no total → "1.0 GB" primary + "1.0 GB / —"
        playersOnline: 3,                             // exercises the players fallback branch
      },
    })} />));
    expect(await screen.findByText("1.50 cores")).toBeInTheDocument();
    expect(screen.getByText("512 MB")).toBeInTheDocument();
    expect(screen.getByText("512 MB / —")).toBeInTheDocument();
    expect(screen.getByText("1.0 GB")).toBeInTheDocument();
    expect(screen.getByText("1.0 GB / —")).toBeInTheDocument();
    // Players fallback (phase != Running) feeds the sidebar card.
    expect(screen.getByText(/names not yet available/)).toBeInTheDocument();
  });
});

describe("OverviewTab players card", () => {
  it("shows 'No players connected.' when online is 0", async () => {
    fetchMock.mockImplementation(() => Promise.resolve(jsonRes({
      online: 0, max: 20, players: [], asOf: "now",
      capabilities: { kick: true, ban: true, unban: true },
    } satisfies PlayersResp)));
    render(withClient(<OverviewTab gs={gs()} name="s1" />));
    expect(await screen.findByText("No players connected.")).toBeInTheDocument();
  });

  it("renders real player names from the snapshot", async () => {
    fetchMock.mockImplementation(() => Promise.resolve(jsonRes({
      online: 2, max: 20, players: ["alice", "bob"], asOf: "now",
      capabilities: { kick: true, ban: true, unban: true },
    } satisfies PlayersResp)));
    render(withClient(<OverviewTab gs={gs()} name="s1" />));
    expect(await screen.findByText("alice")).toBeInTheDocument();
    expect(await screen.findByText("bob")).toBeInTheDocument();
    expect(screen.queryByText("player1")).toBeNull();
  });

  it("shows '+ N more' when there are more than 5 players", async () => {
    const players = ["a", "b", "c", "d", "e", "f", "g"];
    fetchMock.mockImplementation(() => Promise.resolve(jsonRes({
      online: players.length, max: 20, players, asOf: "now",
      capabilities: { kick: true, ban: true, unban: true },
    } satisfies PlayersResp)));
    render(withClient(<OverviewTab gs={gs()} name="s1" />));
    expect(await screen.findByText("+ 2 more")).toBeInTheDocument();
  });
});
