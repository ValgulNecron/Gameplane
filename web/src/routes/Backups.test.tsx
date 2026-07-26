import { describe, it, expect, vi } from "vitest";
import type { ReactNode } from "react";
import { http, HttpResponse } from "msw";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { server } from "@/test/server";
import { renderWithQuery } from "@/test/render";
import { makeBackup, makeSchedule, makeRestore } from "@/test/factories";

vi.mock("@tanstack/react-router", () => ({
  Link: ({ children, to, ...rest }: { children: ReactNode; to: string } & Record<string, unknown>) => (
    <a href={to} {...rest}>{children}</a>
  ),
}));

import { BackupsPage } from "./Backups";

describe("BackupsPage", () => {
  it("renders the Backups tab and loads rows", async () => {
    server.use(
      http.get("/backups", () =>
        HttpResponse.json({
          items: [
            makeBackup({ metadata: { name: "alpha-2026-05-07" } }),
            makeBackup({ metadata: { name: "alpha-2026-05-06" } }),
          ],
        }),
      ),
    );
    renderWithQuery(<BackupsPage />);
    await screen.findByText("alpha-2026-05-07");
    expect(screen.getByText("alpha-2026-05-06")).toBeInTheDocument();
  });

  it("switches to the Schedules tab and shows rows", async () => {
    server.use(
      http.get("/schedules", () =>
        HttpResponse.json({
          items: [makeSchedule({ metadata: { name: "alpha-daily" } })],
        }),
      ),
    );
    renderWithQuery(<BackupsPage />);
    const schedTab = screen.getByRole("button", { name: /Schedules/i });
    await userEvent.click(schedTab);
    await waitFor(() =>
      expect(screen.getByText("alpha-daily")).toBeInTheDocument(),
    );
  });

  it("runs a snapshot from the 'Back up now' header dialog", async () => {
    type BackupBody = {
      kind?: string;
      spec?: { serverRef?: { name?: string }; repoRef?: { name?: string; key?: string } };
    };
    // No initializer: TS pins a `= null` variable to `null` inside the
    // closure-assigned pattern, so declare it open and assert truthiness.
    let posted: BackupBody | undefined;
    server.use(
      http.post("/backups", async ({ request }) => {
        posted = (await request.json()) as BackupBody;
        return HttpResponse.json(
          makeBackup({ metadata: { name: "alpha-manual-1" } }),
        );
      }),
    );
    renderWithQuery(<BackupsPage />);
    await userEvent.click(screen.getByRole("button", { name: /Back up now/i }));

    const dialog = await screen.findByRole("dialog");
    await within(dialog).findByRole("option", { name: "alpha" });
    await userEvent.selectOptions(within(dialog).getByRole("combobox"), "alpha");

    // Enabled only once the destination auto-selects from the query.
    const run = within(dialog).getByRole("button", { name: /Run snapshot/i });
    await waitFor(() => expect(run).toBeEnabled());
    await userEvent.click(run);

    await waitFor(() => expect(posted).toBeTruthy());
    expect(posted?.kind).toBe("Backup");
    expect(posted?.spec?.serverRef?.name).toBe("alpha");
    expect(posted?.spec?.repoRef).toEqual({ name: "default", key: "repo" });
    // The dialog closes once the snapshot is accepted.
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  });

  it("switches to the Restores tab", async () => {
    renderWithQuery(<BackupsPage />);
    const tab = screen.getByRole("button", { name: /Restores/i });
    await userEvent.click(tab);
    // Empty restore list — page should still render the panel header
    // without crashing.
    await waitFor(() => expect(tab.className).toContain("border-primary"));
  });

  it("filters backups by server name", async () => {
    server.use(
      http.get("/backups", () =>
        HttpResponse.json({
          items: [
            makeBackup({ metadata: { name: "alpha-1" }, spec: { serverRef: { name: "alpha" } } }),
            makeBackup({ metadata: { name: "beta-1" }, spec: { serverRef: { name: "beta" } } }),
          ],
        }),
      ),
      http.get("/schedules", () =>
        HttpResponse.json({ items: [] }),
      ),
      http.get("/restores", () =>
        HttpResponse.json({ items: [] }),
      ),
      http.get("/servers", () =>
        HttpResponse.json({
          items: [
            { metadata: { name: "alpha" } },
            { metadata: { name: "beta" } },
          ],
        }),
      ),
    );
    renderWithQuery(<BackupsPage />);
    await screen.findByText("alpha-1");
    // Select beta server filter
    const serverSelect = screen.getByDisplayValue("Select a server…") as HTMLSelectElement;
    await userEvent.selectOptions(serverSelect, "beta");
    // Only beta-1 should be visible
    expect(screen.getByText("beta-1")).toBeInTheDocument();
    expect(screen.queryByText("alpha-1")).not.toBeInTheDocument();
  });

  it("filters backups by phase", async () => {
    server.use(
      http.get("/backups", () =>
        HttpResponse.json({
          items: [
            makeBackup({ metadata: { name: "backup-1" }, status: { phase: "Succeeded" } }),
            makeBackup({ metadata: { name: "backup-2" }, status: { phase: "Failed" } }),
          ],
        }),
      ),
      http.get("/schedules", () =>
        HttpResponse.json({ items: [] }),
      ),
      http.get("/restores", () =>
        HttpResponse.json({ items: [] }),
      ),
    );
    renderWithQuery(<BackupsPage />);
    await screen.findByText("backup-1");
    // Get phase select (second filter select after server)
    const selects = screen.getAllByDisplayValue("");
    const phaseSelect = selects[1] as HTMLSelectElement;
    await userEvent.selectOptions(phaseSelect, "Succeeded");
    expect(screen.getByText("backup-1")).toBeInTheDocument();
    expect(screen.queryByText("backup-2")).not.toBeInTheDocument();
  });

  it("searches backups by name", async () => {
    server.use(
      http.get("/backups", () =>
        HttpResponse.json({
          items: [
            makeBackup({ metadata: { name: "alpha-search-test" }, spec: { serverRef: { name: "alpha" } } }),
            makeBackup({ metadata: { name: "beta-other" }, spec: { serverRef: { name: "beta" } } }),
          ],
        }),
      ),
      http.get("/schedules", () =>
        HttpResponse.json({ items: [] }),
      ),
      http.get("/restores", () =>
        HttpResponse.json({ items: [] }),
      ),
    );
    renderWithQuery(<BackupsPage />);
    await screen.findByText("alpha-search-test");
    // Search for "search"
    const searchInput = screen.getByPlaceholderText(/search/i) as HTMLInputElement;
    await userEvent.type(searchInput, "search");
    expect(screen.getByText("alpha-search-test")).toBeInTheDocument();
    expect(screen.queryByText("beta-other")).not.toBeInTheDocument();
  });

  it("shows empty state when no backups match filters", async () => {
    server.use(
      http.get("/backups", () =>
        HttpResponse.json({
          items: [
            makeBackup({ metadata: { name: "alpha-1" }, spec: { serverRef: { name: "alpha" } } }),
          ],
        }),
      ),
      http.get("/schedules", () =>
        HttpResponse.json({ items: [] }),
      ),
      http.get("/restores", () =>
        HttpResponse.json({ items: [] }),
      ),
      http.get("/servers", () =>
        HttpResponse.json({
          items: [{ metadata: { name: "beta" } }],
        }),
      ),
    );
    renderWithQuery(<BackupsPage />);
    await screen.findByText("alpha-1");
    // Filter to beta server (which has no backups)
    const serverSelect = screen.getByDisplayValue("Select a server…") as HTMLSelectElement;
    await userEvent.selectOptions(serverSelect, "beta");
    expect(screen.getByText(/No backups match the current filters/)).toBeInTheDocument();
  });

  it("shows error when BackupNow dialog fails to create backup", async () => {
    server.use(
      http.post("/backups", () =>
        HttpResponse.text("backup failed", { status: 500 }),
      ),
    );
    renderWithQuery(<BackupsPage />);
    await userEvent.click(screen.getByRole("button", { name: /Back up now/i }));
    const dialog = await screen.findByRole("dialog");
    await within(dialog).findByRole("option", { name: "alpha" });
    await userEvent.selectOptions(within(dialog).getByRole("combobox"), "alpha");
    const run = within(dialog).getByRole("button", { name: /Run snapshot/i });
    await waitFor(() => expect(run).toBeEnabled());
    await userEvent.click(run);
    await waitFor(() => expect(screen.getByText(/backup failed/)).toBeInTheDocument());
  });

  it("disables BackupNow button when no destination configured", async () => {
    server.use(
      http.get("/backup-destinations", () =>
        HttpResponse.json({ items: [] }),
      ),
    );
    renderWithQuery(<BackupsPage />);
    await userEvent.click(screen.getByRole("button", { name: /Back up now/i }));
    const dialog = await screen.findByRole("dialog");
    const run = within(dialog).getByRole("button", { name: /Run snapshot/i });
    await waitFor(() => expect(run).toBeDisabled());
    expect(screen.getByText(/No backup destinations configured/)).toBeInTheDocument();
  });

  it("toggles schedule suspend status", async () => {
    const toggleHandler = vi.fn(() =>
      HttpResponse.json({})
    );
    server.use(
      http.get("/backups", () =>
        HttpResponse.json({ items: [] }),
      ),
      http.get("/schedules", () =>
        HttpResponse.json({
          items: [
            makeSchedule({
              metadata: { name: "alpha-daily" },
              spec: { serverRef: { name: "alpha" }, schedule: "0 3 * * *", suspend: false },
            }),
          ],
        }),
      ),
      http.get("/restores", () =>
        HttpResponse.json({ items: [] }),
      ),
      http.patch("/schedules/alpha-daily:spec", toggleHandler),
    );
    renderWithQuery(<BackupsPage />);
    const schedTab = screen.getByRole("button", { name: /Schedules/i });
    await userEvent.click(schedTab);
    await screen.findByText("alpha-daily");
    const switchBtn = screen.getByRole("checkbox");
    await userEvent.click(switchBtn);
    await waitFor(() => expect(toggleHandler).toHaveBeenCalled());
  });

  it("shows delete confirmation dialog for schedule", async () => {
    server.use(
      http.get("/backups", () =>
        HttpResponse.json({ items: [] }),
      ),
      http.get("/schedules", () =>
        HttpResponse.json({
          items: [makeSchedule({ metadata: { name: "alpha-daily" } })],
        }),
      ),
      http.get("/restores", () =>
        HttpResponse.json({ items: [] }),
      ),
    );
    renderWithQuery(<BackupsPage />);
    const schedTab = screen.getByRole("button", { name: /Schedules/i });
    await userEvent.click(schedTab);
    await screen.findByText("alpha-daily");
    const deleteBtn = screen.getByRole("button", { name: /Delete/i });
    await userEvent.click(deleteBtn);
    expect(await screen.findByText(/Delete schedule/)).toBeInTheDocument();
  });

  it("filters restores by server and phase", async () => {
    server.use(
      http.get("/backups", () =>
        HttpResponse.json({ items: [] }),
      ),
      http.get("/schedules", () =>
        HttpResponse.json({ items: [] }),
      ),
      http.get("/restores", () =>
        HttpResponse.json({
          items: [
            makeRestore({
              metadata: { name: "restore-1" },
              spec: { serverRef: { name: "alpha" }, backupRef: { name: "backup-1" } },
              status: { phase: "Succeeded" },
            }),
            makeRestore({
              metadata: { name: "restore-2" },
              spec: { serverRef: { name: "beta" }, backupRef: { name: "backup-2" } },
              status: { phase: "Failed" },
            }),
          ],
        }),
      ),
      http.get("/servers", () =>
        HttpResponse.json({
          items: [
            { metadata: { name: "alpha" } },
            { metadata: { name: "beta" } },
          ],
        }),
      ),
    );
    renderWithQuery(<BackupsPage />);
    const restoresTab = screen.getByRole("button", { name: /Restores/i });
    await userEvent.click(restoresTab);
    await screen.findByText("restore-1");
    // Filter by alpha server
    const serverSelect = screen.getByDisplayValue("Select a server…") as HTMLSelectElement;
    await userEvent.selectOptions(serverSelect, "alpha");
    expect(screen.getByText("restore-1")).toBeInTheDocument();
    expect(screen.queryByText("restore-2")).not.toBeInTheDocument();
  });

  it("shows empty restores state", async () => {
    server.use(
      http.get("/restores", () =>
        HttpResponse.json({ items: [] }),
      ),
    );
    renderWithQuery(<BackupsPage />);
    const restoresTab = screen.getByRole("button", { name: /Restores/i });
    await userEvent.click(restoresTab);
    expect(await screen.findByText(/No restores have been run/)).toBeInTheDocument();
  });

  it("searches restores by name", async () => {
    server.use(
      http.get("/restores", () =>
        HttpResponse.json({
          items: [
            makeRestore({
              metadata: { name: "search-restore-1" },
              spec: { serverRef: { name: "alpha" }, backupRef: { name: "backup-1" } },
            }),
            makeRestore({
              metadata: { name: "other-restore" },
              spec: { serverRef: { name: "beta" }, backupRef: { name: "backup-2" } },
            }),
          ],
        }),
      ),
    );
    renderWithQuery(<BackupsPage />);
    const restoresTab = screen.getByRole("button", { name: /Restores/i });
    await userEvent.click(restoresTab);
    await screen.findByText("search-restore-1");
    const searchInput = screen.getByPlaceholderText(/search/i) as HTMLInputElement;
    await userEvent.type(searchInput, "search");
    expect(screen.getByText("search-restore-1")).toBeInTheDocument();
    expect(screen.queryByText("other-restore")).not.toBeInTheDocument();
  });

  it("deletes a schedule", async () => {
    const deleteHandler = vi.fn(() =>
      HttpResponse.json({})
    );
    server.use(
      http.get("/schedules", () =>
        HttpResponse.json({
          items: [makeSchedule({ metadata: { name: "alpha-daily" } })],
        }),
      ),
      http.delete("/schedules/alpha-daily", deleteHandler),
    );
    renderWithQuery(<BackupsPage />);
    const schedTab = screen.getByRole("button", { name: /Schedules/i });
    await userEvent.click(schedTab);
    await screen.findByText("alpha-daily");
    const deleteBtn = screen.getByRole("button", { name: /Delete/i });
    await userEvent.click(deleteBtn);
    await screen.findByText(/Delete schedule/);
    const typeInput = screen.getByRole("textbox") as HTMLInputElement;
    await userEvent.type(typeInput, "alpha-daily");
    const confirmBtn = screen.getByRole("button", { name: /Delete$/ });
    await userEvent.click(confirmBtn);
    await waitFor(() => expect(deleteHandler).toHaveBeenCalled());
  });
});
