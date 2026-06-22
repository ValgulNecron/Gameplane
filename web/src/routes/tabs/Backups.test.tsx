import { describe, it, expect } from "vitest";
import { http, HttpResponse } from "msw";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { server } from "@/test/server";
import { renderWithQuery } from "@/test/render";
import { makeBackup, makeRestore, makeDestination } from "@/test/factories";
import { BackupsTab } from "./Backups";

// The default MSW handlers already serve /backups (one Succeeded backup
// for "alpha"), /schedules (one), /restores (empty) and
// /backup-destinations (one). Tests override only what they need.

describe("BackupsTab", () => {
  it("renders schedules, backups, and enables 'Back up now' with one destination", async () => {
    renderWithQuery(<BackupsTab name="alpha" />);
    expect(await screen.findByText("alpha-2026-05-07")).toBeInTheDocument();
    expect(screen.getByText("0 3 * * *")).toBeInTheDocument(); // default schedule cron
    const backupNow = screen.getByRole("button", { name: /back up now/i });
    await waitFor(() => expect(backupNow).toBeEnabled());
  });

  it("disables 'Back up now' with a hint when no destination is configured", async () => {
    server.use(http.get("/backup-destinations", () => HttpResponse.json({ items: [] })));
    renderWithQuery(<BackupsTab name="alpha" />);
    const btn = await screen.findByRole("button", { name: /back up now/i });
    await waitFor(() => expect(btn).toBeDisabled());
    expect(btn.getAttribute("title")).toMatch(/No backup destination/i);
  });

  it("disables 'Back up now' with a multi-destination hint", async () => {
    server.use(
      http.get("/backup-destinations", () =>
        HttpResponse.json({ items: [makeDestination({ name: "a" }), makeDestination({ name: "b" })] }),
      ),
    );
    renderWithQuery(<BackupsTab name="alpha" />);
    const btn = await screen.findByRole("button", { name: /back up now/i });
    await waitFor(() => expect(btn.getAttribute("title")).toMatch(/Multiple destinations/i));
    expect(btn).toBeDisabled();
  });

  it("posts a backup when 'Back up now' is clicked", async () => {
    let posted: unknown = null;
    server.use(
      http.post("/backups", async ({ request }) => {
        posted = await request.json();
        return HttpResponse.json(makeBackup({ metadata: { name: "alpha-new", namespace: "gameplane-games" } }));
      }),
    );
    renderWithQuery(<BackupsTab name="alpha" />);
    const btn = await screen.findByRole("button", { name: /back up now/i });
    await waitFor(() => expect(btn).toBeEnabled());
    await userEvent.click(btn);
    await waitFor(() => expect(posted).not.toBeNull());
  });

  it("shows an error banner when the backup request fails", async () => {
    server.use(http.post("/backups", () => HttpResponse.text("snapshot failed", { status: 500 })));
    renderWithQuery(<BackupsTab name="alpha" />);
    const btn = await screen.findByRole("button", { name: /back up now/i });
    await waitFor(() => expect(btn).toBeEnabled());
    await userEvent.click(btn);
    expect(await screen.findByText("snapshot failed")).toBeInTheDocument();
  });

  it("toggles the schedule form open and closed", async () => {
    renderWithQuery(<BackupsTab name="alpha" />);
    const newBtn = await screen.findByRole("button", { name: /new schedule/i });
    await userEvent.click(newBtn);
    expect(await screen.findByText(/Schedule \(cron\)/i)).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: /cancel/i }));
    await waitFor(() => expect(screen.queryByText(/Schedule \(cron\)/i)).toBeNull());
  });

  it("shows the empty-schedules message when there are none", async () => {
    server.use(http.get("/schedules", () => HttpResponse.json({ items: [] })));
    renderWithQuery(<BackupsTab name="alpha" />);
    expect(await screen.findByText("No schedules yet.")).toBeInTheDocument();
  });

  it("renders recent restores when present", async () => {
    server.use(
      http.get("/backups", () => HttpResponse.json({ items: [] })),
      http.get("/restores", () =>
        HttpResponse.json({
          items: [
            makeRestore({
              spec: { serverRef: { name: "alpha" }, backupRef: { name: "alpha-2026-05-07" } },
            }),
          ],
        }),
      ),
    );
    renderWithQuery(<BackupsTab name="alpha" />);
    expect(await screen.findByText("Recent restores")).toBeInTheDocument();
    expect(screen.getByText("alpha-2026-05-07")).toBeInTheDocument();
  });

  it("opens the detail drawer when a backup row is clicked", async () => {
    renderWithQuery(<BackupsTab name="alpha" />);
    await userEvent.click(await screen.findByText("alpha-2026-05-07"));
    expect(await screen.findByText("Snapshot ID")).toBeInTheDocument();
  });

  it("opens the restore dialog from a restorable backup", async () => {
    // A single succeeded backup → exactly one (enabled) Restore button.
    server.use(http.get("/backups", () => HttpResponse.json({ items: [makeBackup()] })));
    renderWithQuery(<BackupsTab name="alpha" />);
    await screen.findByText("alpha-2026-05-07");
    const restoreBtn = screen.getByRole("button", { name: /^restore$/i });
    expect(restoreBtn).toBeEnabled();
    await userEvent.click(restoreBtn);
    expect(await screen.findByText("Restore from backup")).toBeInTheDocument();
  });

  it("disables Restore for a backup that has not succeeded", async () => {
    server.use(
      http.get("/backups", () => HttpResponse.json({ items: [makeBackup({ status: { phase: "Running" } })] })),
    );
    renderWithQuery(<BackupsTab name="alpha" />);
    await screen.findByText("alpha-2026-05-07");
    expect(screen.getByRole("button", { name: /^restore$/i })).toBeDisabled();
  });

  it("filters out backups, schedules, and restores from other servers", async () => {
    server.use(
      http.get("/backups", () =>
        HttpResponse.json({
          items: [
            makeBackup({ metadata: { name: "mine", namespace: "gameplane-games" }, spec: { serverRef: { name: "alpha" } } }),
            makeBackup({ metadata: { name: "theirs", namespace: "gameplane-games" }, spec: { serverRef: { name: "beta" } } }),
          ],
        }),
      ),
    );
    renderWithQuery(<BackupsTab name="alpha" />);
    expect(await screen.findByText("mine")).toBeInTheDocument();
    expect(screen.queryByText("theirs")).toBeNull();
  });
});
