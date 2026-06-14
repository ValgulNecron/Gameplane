import { describe, it, expect, vi } from "vitest";
import type { ReactNode } from "react";
import { http, HttpResponse } from "msw";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { server } from "@/test/server";
import { renderWithQuery } from "@/test/render";
import { makeBackup, makeSchedule, makeServer, makeRestore } from "@/test/factories";

vi.mock("@tanstack/react-router", () => ({
  Link: ({ children, to, ...rest }: { children: ReactNode; to: string } & Record<string, unknown>) => (
    <a href={to} {...rest}>{children}</a>
  ),
}));

import { BackupsPage } from "./Backups";

// The native "Back up now" / "New schedule for" <select> is the combobox
// whose options include the "Select a server…" placeholder (the Radix
// filter Selects render their options only when opened).
function serverSelect(): HTMLElement {
  const combos = screen.getAllByRole("combobox");
  const match = combos.find((c) => within(c).queryByRole("option", { name: /select a server/i }));
  if (!match) throw new Error("server <select> not found");
  return match;
}

describe("BackupsPage flows", () => {
  it("disables 'Run snapshot' and explains when no destination is configured", async () => {
    server.use(http.get("/backup-destinations", () => HttpResponse.json({ items: [] })));
    renderWithQuery(<BackupsPage />);
    expect(await screen.findByText(/No backup destinations configured/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Run snapshot/i })).toBeDisabled();
  });

  it("runs a snapshot for the selected server", async () => {
    let posted: unknown = null;
    server.use(
      http.get("/servers", () => HttpResponse.json({ items: [makeServer({ metadata: { name: "alpha" } })] })),
      http.post("/backups", async ({ request }) => {
        posted = await request.json();
        return HttpResponse.json(makeBackup({ metadata: { name: "alpha-new" } }));
      }),
    );
    renderWithQuery(<BackupsPage />);
    await screen.findByText("alpha-2026-05-07"); // default backup row
    await userEvent.selectOptions(serverSelect(), "alpha");
    const run = screen.getByRole("button", { name: /Run snapshot/i });
    await waitFor(() => expect(run).toBeEnabled());
    await userEvent.click(run);
    await waitFor(() => expect(posted).not.toBeNull());
  });

  it("filters the backup list by search text", async () => {
    server.use(
      http.get("/backups", () =>
        HttpResponse.json({
          items: [
            makeBackup({ metadata: { name: "keep-me" }, spec: { serverRef: { name: "alpha" } } }),
            makeBackup({ metadata: { name: "hide-me" }, spec: { serverRef: { name: "beta" } } }),
          ],
        }),
      ),
    );
    renderWithQuery(<BackupsPage />);
    await screen.findByText("keep-me");
    await userEvent.type(screen.getByPlaceholderText(/Search by name or server/i), "keep");
    await waitFor(() => expect(screen.queryByText("hide-me")).toBeNull());
    expect(screen.getByText("keep-me")).toBeInTheDocument();
  });

  it("shows the empty restores state on the Restores tab", async () => {
    server.use(http.get("/restores", () => HttpResponse.json({ items: [] })));
    renderWithQuery(<BackupsPage />);
    await userEvent.click(screen.getByRole("button", { name: /^Restores$/i }));
    expect(await screen.findByText(/No restores have been run/i)).toBeInTheDocument();
  });

  it("lists restores with their target and phase", async () => {
    server.use(
      http.get("/restores", () =>
        HttpResponse.json({
          items: [makeRestore({ metadata: { name: "restore-1" }, spec: { backupRef: { name: "b1" }, serverRef: { name: "alpha" } } })],
        }),
      ),
    );
    renderWithQuery(<BackupsPage />);
    await userEvent.click(screen.getByRole("button", { name: /^Restores$/i }));
    expect(await screen.findByText("restore-1")).toBeInTheDocument();
  });

  it("opens the schedule form for a chosen server", async () => {
    server.use(
      http.get("/servers", () => HttpResponse.json({ items: [makeServer({ metadata: { name: "alpha" } })] })),
    );
    renderWithQuery(<BackupsPage />);
    await userEvent.click(screen.getByRole("button", { name: /^Schedules$/i }));
    await userEvent.selectOptions(serverSelect(), "alpha");
    expect(await screen.findByText(/Schedule \(cron\)/i)).toBeInTheDocument();
  });

  it("prompts before deleting a schedule and toggles suspend", async () => {
    let patched: unknown = null;
    server.use(
      http.get("/schedules", () =>
        HttpResponse.json({ items: [makeSchedule({ metadata: { name: "alpha-daily" } })] }),
      ),
      http.get("/schedules/:name", ({ params }) =>
        HttpResponse.json(makeSchedule({ metadata: { name: String(params.name) } })),
      ),
      http.put("/schedules/:name", async ({ request }) => {
        patched = await request.json();
        return HttpResponse.json(makeSchedule({ metadata: { name: "alpha-daily" } }));
      }),
    );
    renderWithQuery(<BackupsPage />);
    await userEvent.click(screen.getByRole("button", { name: /^Schedules$/i }));
    await screen.findByText("alpha-daily");

    // Toggle the "active" switch → patchSpec (read-modify-write PUT).
    await userEvent.click(screen.getByRole("switch", { name: /Schedule active/i }));
    await waitFor(() => expect(patched).not.toBeNull());

    // Delete prompts a confirm dialog.
    await userEvent.click(screen.getByRole("button", { name: /^Delete$/i }));
    expect(await screen.findByText(/Delete schedule\?/i)).toBeInTheDocument();
  });
});
