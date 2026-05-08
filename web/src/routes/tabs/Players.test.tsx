import { describe, it, expect } from "vitest";
import { http, HttpResponse } from "msw";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { server } from "@/test/server";
import { renderWithQuery } from "@/test/render";
import { makePlayers } from "@/test/factories";
import { PlayersTab } from "./Players";

describe("PlayersTab", () => {
  it("shows the online count and player list", async () => {
    server.use(
      http.get("/servers/alpha/players", () =>
        HttpResponse.json(makePlayers({ online: 2, max: 20, players: ["alice", "bob"] })),
      ),
    );
    renderWithQuery(<PlayersTab name="alpha" />);
    expect(await screen.findByText(/2 \/ 20 online/)).toBeInTheDocument();
    expect(screen.getByText("alice")).toBeInTheDocument();
    expect(screen.getByText("bob")).toBeInTheDocument();
  });

  it("shows loading state initially", () => {
    server.use(
      http.get("/servers/alpha/players", async () => {
        await new Promise((r) => setTimeout(r, 50));
        return HttpResponse.json(makePlayers());
      }),
    );
    renderWithQuery(<PlayersTab name="alpha" />);
    expect(screen.getByText(/Loading/i)).toBeInTheDocument();
  });

  it("offers a refresh button", async () => {
    let callCount = 0;
    server.use(
      http.get("/servers/alpha/players", () => {
        callCount++;
        return HttpResponse.json(makePlayers());
      }),
    );
    renderWithQuery(<PlayersTab name="alpha" />);
    await screen.findByText(/online/);
    expect(callCount).toBe(1);
    const btn = screen.getByTitle(/Refresh/i);
    await userEvent.click(btn);
    await waitFor(() => expect(callCount).toBeGreaterThan(1));
  });

  it("renders empty state when no players online", async () => {
    server.use(
      http.get("/servers/alpha/players", () =>
        HttpResponse.json(makePlayers({ online: 0, players: [] })),
      ),
    );
    renderWithQuery(<PlayersTab name="alpha" />);
    expect(await screen.findByText(/0 \/ 20 online/)).toBeInTheDocument();
  });
});
