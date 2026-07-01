import { describe, it, expect, vi } from "vitest";
import type { ReactNode } from "react";
import { http, HttpResponse } from "msw";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { server } from "@/test/server";
import { renderWithQuery } from "@/test/render";
import { makeBackup, makeSchedule } from "@/test/factories";

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
});
