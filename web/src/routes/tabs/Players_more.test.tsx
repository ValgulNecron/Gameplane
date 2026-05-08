import { describe, it, expect, vi } from "vitest";
import { http, HttpResponse } from "msw";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { server } from "@/test/server";
import { renderWithQuery } from "@/test/render";
import { makePlayers } from "@/test/factories";
import { PlayersTab } from "./Players";

describe("PlayersTab moderation actions", () => {
  it("shows kick/ban controls when capabilities allow it", async () => {
    server.use(
      http.get("/servers/alpha/players", () =>
        HttpResponse.json(
          makePlayers({
            online: 1,
            players: ["alice"],
            capabilities: { kick: true, ban: true, unban: true },
          }),
        ),
      ),
    );
    renderWithQuery(<PlayersTab name="alpha" />);
    await screen.findByText("alice");
    expect(screen.getByTitle("Kick")).toBeInTheDocument();
    expect(screen.getByTitle("Ban")).toBeInTheDocument();
  });

  it("Kick button opens the confirm action panel", async () => {
    server.use(
      http.get("/servers/alpha/players", () =>
        HttpResponse.json(makePlayers({ players: ["alice"] })),
      ),
    );
    renderWithQuery(<PlayersTab name="alpha" />);
    await screen.findByText("alice");
    await userEvent.click(screen.getByTitle("Kick"));
    // The confirmation panel renders a "Reason" input — that's unique
    // and only appears when the panel is up.
    expect(await screen.findByPlaceholderText(/Reason/i)).toBeInTheDocument();
  });

  it("kick happy path POSTs to /players/kick and refetches", async () => {
    let kicked = false;
    server.use(
      http.get("/servers/alpha/players", () =>
        HttpResponse.json(makePlayers({ players: ["alice"] })),
      ),
      http.post("/servers/alpha/players/kick", async () => {
        kicked = true;
        return HttpResponse.json({ ok: true, raw: "Kicked alice" });
      }),
    );
    renderWithQuery(<PlayersTab name="alpha" />);
    await screen.findByText("alice");
    await userEvent.click(screen.getByTitle("Kick"));
    // The "Confirm" + verb button confirms the action.
    const confirmBtn = await waitFor(() => {
      const all = screen.getAllByRole("button");
      const m = all.filter(
        (b) => b.textContent?.trim() === "Kick" || b.textContent?.trim() === "Kicking…",
      );
      if (m.length === 0) throw new Error("confirm Kick not found");
      return m[m.length - 1];
    });
    await userEvent.click(confirmBtn);
    await waitFor(() => expect(kicked).toBe(true));
  });

  it("kick failure surfaces an error message", async () => {
    server.use(
      http.get("/servers/alpha/players", () =>
        HttpResponse.json(makePlayers({ players: ["alice"] })),
      ),
      http.post("/servers/alpha/players/kick", () =>
        HttpResponse.text("rcon down", { status: 502 }),
      ),
    );
    renderWithQuery(<PlayersTab name="alpha" />);
    await screen.findByText("alice");
    await userEvent.click(screen.getByTitle("Kick"));
    const confirmBtn = await waitFor(() => {
      const all = screen.getAllByRole("button");
      const m = all.filter(
        (b) => b.textContent?.trim() === "Kick" || b.textContent?.trim() === "Kicking…",
      );
      if (m.length === 0) throw new Error("confirm Kick not found");
      return m[m.length - 1];
    });
    await userEvent.click(confirmBtn);
    await waitFor(() => expect(screen.getByText(/rcon down/i)).toBeInTheDocument());
  });

  it("Banned section toggles open and shows entries", async () => {
    server.use(
      http.get("/servers/alpha/players", () =>
        HttpResponse.json(makePlayers({ online: 0, players: [] })),
      ),
      http.get("/servers/alpha/players/banned", () =>
        HttpResponse.json([
          { name: "griefer", source: "Server", reason: "x-ray" },
        ]),
      ),
    );
    renderWithQuery(<PlayersTab name="alpha" />);
    await screen.findByText(/0 \/ 20 online/);
    await userEvent.click(screen.getByRole("button", { name: /Banned/i }));
    expect(await screen.findByText("griefer")).toBeInTheDocument();
    expect(screen.getByText(/x-ray/)).toBeInTheDocument();
  });

  it("Unban button POSTs to /players/unban", async () => {
    let unbanned = false;
    server.use(
      http.get("/servers/alpha/players", () =>
        HttpResponse.json(makePlayers({ players: [] })),
      ),
      http.get("/servers/alpha/players/banned", () =>
        HttpResponse.json([{ name: "griefer", source: "Server" }]),
      ),
      http.post("/servers/alpha/players/unban", () => {
        unbanned = true;
        return HttpResponse.json({ ok: true });
      }),
    );
    renderWithQuery(<PlayersTab name="alpha" />);
    await userEvent.click(await screen.findByRole("button", { name: /Banned/i }));
    await screen.findByText("griefer");
    await userEvent.click(screen.getByTitle("Unban"));
    await waitFor(() => expect(unbanned).toBe(true));
  });

  it("renders a non-moderation message when capabilities are all false", async () => {
    server.use(
      http.get("/servers/alpha/players", () =>
        HttpResponse.json(
          makePlayers({
            players: ["alice"],
            capabilities: { kick: false, ban: false, unban: false },
          }),
        ),
      ),
    );
    renderWithQuery(<PlayersTab name="alpha" />);
    await screen.findByText("alice");
    expect(
      screen.getByText(/Player moderation isn't supported/i),
    ).toBeInTheDocument();
  });

  it("'Nobody online' renders when no players are reported", async () => {
    server.use(
      http.get("/servers/alpha/players", () =>
        HttpResponse.json(makePlayers({ online: 0, players: [] })),
      ),
    );
    renderWithQuery(<PlayersTab name="alpha" />);
    expect(await screen.findByText(/Nobody online/i)).toBeInTheDocument();
  });
});
