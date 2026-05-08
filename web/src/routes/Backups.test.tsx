import { describe, it, expect, vi } from "vitest";
import type { ReactNode } from "react";
import { http, HttpResponse } from "msw";
import { screen, waitFor } from "@testing-library/react";
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

  it("switches to the Restores tab", async () => {
    renderWithQuery(<BackupsPage />);
    const tab = screen.getByRole("button", { name: /Restores/i });
    await userEvent.click(tab);
    // Empty restore list — page should still render the panel header
    // without crashing.
    await waitFor(() => expect(tab.className).toContain("border-primary"));
  });
});
