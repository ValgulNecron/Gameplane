import { describe, it, expect, vi } from "vitest";
import type { ReactNode } from "react";
import { http, HttpResponse } from "msw";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { server } from "@/test/server";
import { renderWithQuery } from "@/test/render";

vi.mock("@tanstack/react-router", () => ({
  Link: ({ children, to, ...rest }: { children: ReactNode; to: string } & Record<string, unknown>) => (
    <a href={to} {...rest}>{children}</a>
  ),
}));

import { AdminSettingsPage } from "./AdminSettings";

describe("AdminSettingsPage", () => {
  it("renders the General section by default", async () => {
    server.use(
      http.get("/admin/config", () =>
        HttpResponse.json({
          general: {
            instanceName: "kestrel-prod",
            externalURL: "https://example.com",
            defaultNamespace: "kestrel-games",
          },
        }),
      ),
    );
    renderWithQuery(<AdminSettingsPage />);
    expect(await screen.findByRole("heading", { name: /Admin settings/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /General/i })).toBeInTheDocument();
  });

  it("switches to a different section on click", async () => {
    renderWithQuery(<AdminSettingsPage />);
    const authBtn = await screen.findByRole("button", { name: /Authentication/i });
    await userEvent.click(authBtn);
    // The Authentication tab is now selected — its label is rendered in
    // the active style. Sanity-check via the nav button class change.
    await waitFor(() => {
      expect(authBtn.className).toContain("bg-surface");
    });
  });

  it("renders all section nav buttons", async () => {
    renderWithQuery(<AdminSettingsPage />);
    await screen.findByRole("heading", { name: /Admin settings/i });
    for (const label of [
      "General",
      "Authentication",
      "Backup destinations",
      "Module sources",
      "Notifications",
      "Telemetry",
      "Updates",
      "About",
    ]) {
      expect(screen.getByRole("button", { name: new RegExp(label, "i") })).toBeInTheDocument();
    }
  });
});
