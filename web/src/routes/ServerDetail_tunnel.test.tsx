import { describe, it, expect } from "vitest";
import { screen } from "@testing-library/react";
import { renderWithQuery } from "@/test/render";
import { makeServer } from "@/test/factories";
import type { GameServer } from "@/types";
import { OverviewTab } from "./tabs/Overview";

describe("OverviewTab tunnel connection", () => {
  it("shows tunnel endpoint with provider label when tunnel is enabled and has an address", async () => {
    const gs: GameServer = makeServer({
      status: {
        endpoints: [
          {
            name: "game",
            host: "tunnel.example.com",
            port: 25565,
            tunnelProvider: "frp",
          },
        ],
      },
    });
    renderWithQuery(<OverviewTab gs={gs} name="test-server" />);
    expect(screen.getByText("frp tunnel")).toBeInTheDocument();
    expect(screen.getByText("tunnel.example.com")).toBeInTheDocument();
  });

  it("displays Tailnet warning badge when Tailscale endpoint is private", async () => {
    const gs: GameServer = makeServer({
      status: {
        endpoints: [
          {
            name: "game",
            host: "game-server.tail1234.ts.net",
            port: 25565,
            tunnelProvider: "tailscale",
            private: true,
          },
        ],
      },
    });
    renderWithQuery(<OverviewTab gs={gs} name="test-server" />);
    expect(screen.getByText("Tailnet only — not public")).toBeInTheDocument();
  });

  it("shows cluster address below tunnel endpoint", async () => {
    const gs: GameServer = makeServer({
      status: {
        endpoints: [
          {
            name: "game",
            host: "tunnel.example.com",
            port: 25565,
            tunnelProvider: "frp",
          },
          {
            name: "game",
            host: "10.0.0.5",
            port: 25565,
          },
        ],
      },
    });
    renderWithQuery(<OverviewTab gs={gs} name="test-server" />);
    expect(screen.getByText("Cluster address")).toBeInTheDocument();
    // The cluster address should still be visible
    expect(screen.getByText("10.0.0.5")).toBeInTheDocument();
  });

  it("shows waiting message when tunnel is awaiting address", async () => {
    const gs: GameServer = makeServer({
      status: {
        endpoints: [
          {
            name: "game",
            host: "",
            port: 0,
            tunnelProvider: "playit",
          },
        ],
        conditions: [
          {
            type: "TunnelReady",
            status: "False",
            reason: "AwaitingAddress",
            message: "Waiting for playit.gg to assign a public address",
          },
        ],
      },
    });
    renderWithQuery(<OverviewTab gs={gs} name="test-server" />);
    expect(
      screen.getByText("Waiting for playit.gg to assign a public address"),
    ).toBeInTheDocument();
  });

  it("uses default 'Waiting for tunnel address' when TunnelReady condition lacks message", async () => {
    const gs: GameServer = makeServer({
      status: {
        endpoints: [
          {
            name: "game",
            host: "",
            port: 0,
            tunnelProvider: "playit",
          },
        ],
        conditions: [
          {
            type: "TunnelReady",
            status: "False",
          },
        ],
      },
    });
    renderWithQuery(<OverviewTab gs={gs} name="test-server" />);
    expect(screen.getByText("Waiting for tunnel address…")).toBeInTheDocument();
  });

  it("renders cluster address normally when no tunnel endpoint exists", async () => {
    const gs: GameServer = makeServer({
      status: {
        endpoints: [
          {
            name: "game",
            host: "mc.internal",
            port: 25565,
          },
        ],
      },
    });
    renderWithQuery(<OverviewTab gs={gs} name="test-server" />);
    // Should not have tunnel provider label
    expect(screen.queryByText("frp tunnel")).not.toBeInTheDocument();
    expect(screen.queryByText("tailscale tunnel")).not.toBeInTheDocument();
    expect(screen.getByText("mc.internal")).toBeInTheDocument();
  });

  it("has copy button for tunnel endpoint", async () => {
    const gs: GameServer = makeServer({
      status: {
        endpoints: [
          {
            name: "game",
            host: "tunnel.example.com",
            port: 25565,
            tunnelProvider: "frp",
          },
        ],
      },
    });
    renderWithQuery(<OverviewTab gs={gs} name="test-server" />);
    const copyButtons = screen.getAllByRole("button", { name: "Copy" });
    expect(copyButtons.length).toBeGreaterThan(0);
  });
});
