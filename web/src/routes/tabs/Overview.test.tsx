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

describe("OverviewTab players card", () => {
  it("shows 'No players connected.' when online is 0", async () => {
    fetchMock.mockResolvedValue(jsonRes({
      online: 0, max: 20, players: [], asOf: "now",
      capabilities: { kick: true, ban: true, unban: true },
    } satisfies PlayersResp));
    render(withClient(<OverviewTab gs={gs()} name="s1" />));
    expect(await screen.findByText("No players connected.")).toBeInTheDocument();
  });

  it("renders real player names from the snapshot", async () => {
    fetchMock.mockResolvedValue(jsonRes({
      online: 2, max: 20, players: ["alice", "bob"], asOf: "now",
      capabilities: { kick: true, ban: true, unban: true },
    } satisfies PlayersResp));
    render(withClient(<OverviewTab gs={gs()} name="s1" />));
    expect(await screen.findByText("alice")).toBeInTheDocument();
    expect(await screen.findByText("bob")).toBeInTheDocument();
    expect(screen.queryByText("player1")).toBeNull();
  });

  it("shows '+ N more' when there are more than 5 players", async () => {
    const players = ["a", "b", "c", "d", "e", "f", "g"];
    fetchMock.mockResolvedValue(jsonRes({
      online: players.length, max: 20, players, asOf: "now",
      capabilities: { kick: true, ban: true, unban: true },
    } satisfies PlayersResp));
    render(withClient(<OverviewTab gs={gs()} name="s1" />));
    expect(await screen.findByText("+ 2 more")).toBeInTheDocument();
  });
});
