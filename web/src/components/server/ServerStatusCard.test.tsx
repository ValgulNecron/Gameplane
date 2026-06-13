import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { screen } from "@testing-library/react";
import { renderWithQuery } from "@/test/render";
import { ServerStatusCard } from "./ServerStatusCard";
import type { GameTemplate, StatusReading } from "@/types";

const fetchMock = vi.fn();

beforeEach(() => vi.stubGlobal("fetch", fetchMock));
afterEach(() => {
  fetchMock.mockReset();
  vi.unstubAllGlobals();
});

function jsonRes(body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  });
}

function tmpl(metrics: Array<{ id: string; displayName: string; unit?: string }>, rcon = "source"): GameTemplate {
  return {
    metadata: { name: "minecraft-java" },
    spec: {
      displayName: "Minecraft",
      game: "minecraft-java",
      version: "1",
      image: "img",
      rcon: { protocol: rcon },
      capabilities: { status: { metrics } },
    },
  };
}

describe("ServerStatusCard", () => {
  it("renders nothing when no metrics are declared", () => {
    const { container } = renderWithQuery(
      <ServerStatusCard name="s1" tmpl={tmpl([])} running />,
    );
    expect(container).toBeEmptyDOMElement();
  });

  it("renders nothing when the game has no rcon", () => {
    const { container } = renderWithQuery(
      <ServerStatusCard name="s1" tmpl={tmpl([{ id: "seed", displayName: "Seed" }], "none")} running />,
    );
    expect(container).toBeEmptyDOMElement();
  });

  it("shows declared metrics with their resolved values", async () => {
    fetchMock.mockResolvedValue(
      jsonRes([
        { id: "seed", displayName: "World seed", value: "-4096" },
        { id: "tps", displayName: "TPS", value: "19.8", unit: "tps" },
      ] satisfies StatusReading[]),
    );
    renderWithQuery(
      <ServerStatusCard
        name="s1"
        tmpl={tmpl([
          { id: "seed", displayName: "World seed" },
          { id: "tps", displayName: "TPS", unit: "tps" },
        ])}
        running
      />,
    );
    expect(await screen.findByText("-4096")).toBeInTheDocument();
    expect(await screen.findByText("19.8")).toBeInTheDocument();
    expect(screen.getByText("World seed")).toBeInTheDocument();
  });

  it("shows a placeholder for a metric with no current reading", async () => {
    fetchMock.mockResolvedValue(jsonRes([] satisfies StatusReading[]));
    renderWithQuery(
      <ServerStatusCard name="s1" tmpl={tmpl([{ id: "seed", displayName: "World seed" }])} running />,
    );
    // The declared metric still renders its label; value falls back to —.
    expect(await screen.findByText("World seed")).toBeInTheDocument();
    expect(screen.getByText("—")).toBeInTheDocument();
  });
});
