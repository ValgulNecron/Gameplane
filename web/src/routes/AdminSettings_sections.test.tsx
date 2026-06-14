import { describe, it, expect, vi } from "vitest";
import type { ReactNode } from "react";
import { http, HttpResponse } from "msw";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { server } from "@/test/server";
import { renderWithQuery } from "@/test/render";
import { makeConfig } from "@/test/factories";

vi.mock("@tanstack/react-router", () => ({
  Link: ({ children, to, ...rest }: { children: ReactNode; to: string } & Record<string, unknown>) => (
    <a href={to} {...rest}>{children}</a>
  ),
}));

import { AdminSettingsPage } from "./AdminSettings";

async function gotoSection(name: RegExp) {
  await screen.findByRole("heading", { name: /Admin settings/i });
  await userEvent.click(await screen.findByRole("button", { name }));
}

describe("AdminSettings sections", () => {
  it("saves an edited General field", async () => {
    renderWithQuery(<AdminSettingsPage />);
    const nameInput = await screen.findByDisplayValue("Kestrel (mock)");
    await userEvent.clear(nameInput);
    await userEvent.type(nameInput, "kestrel-prod");
    await userEvent.click(screen.getByRole("button", { name: /Save changes/i }));
    expect(await screen.findByText("Saved")).toBeInTheDocument();
  });

  it("surfaces a save error from the server", async () => {
    server.use(
      http.put("/admin/config/general", () => HttpResponse.text("namespace invalid", { status: 400 })),
    );
    renderWithQuery(<AdminSettingsPage />);
    const nameInput = await screen.findByDisplayValue("Kestrel (mock)");
    await userEvent.clear(nameInput);
    await userEvent.type(nameInput, "x");
    await userEvent.click(screen.getByRole("button", { name: /Save changes/i }));
    expect(await screen.findByText(/namespace invalid/i)).toBeInTheDocument();
  });

  it("toggles an authentication provider", async () => {
    renderWithQuery(<AdminSettingsPage />);
    await gotoSection(/Authentication/i);
    // The seeded local provider starts enabled; toggling flips the badge.
    const toggle = await screen.findByRole("button", { name: /^Enabled$/i });
    await userEvent.click(toggle);
    expect(await screen.findByRole("button", { name: /^Disabled$/i })).toBeInTheDocument();
  });

  it("toggles telemetry and saves", async () => {
    renderWithQuery(<AdminSettingsPage />);
    await gotoSection(/Telemetry/i);
    const sw = await screen.findByRole("switch", { name: /Enable telemetry/i });
    await userEvent.click(sw);
    await userEvent.click(screen.getByRole("button", { name: /Save changes/i }));
    expect(await screen.findByText("Saved")).toBeInTheDocument();
  });

  it("changes the update channel and saves", async () => {
    renderWithQuery(<AdminSettingsPage />);
    await gotoSection(/Updates/i);
    const select = await screen.findByRole("combobox");
    await userEvent.selectOptions(select, "beta");
    await userEvent.click(screen.getByRole("button", { name: /Save changes/i }));
    expect(await screen.findByText("Saved")).toBeInTheDocument();
  });

  it("toggles a configured notification sink", async () => {
    server.use(
      http.get("/admin/config", () =>
        HttpResponse.json(
          makeConfig({ notifications: { sinks: [{ name: "ops-slack", kind: "slack", enabled: false }] } }),
        ),
      ),
    );
    renderWithQuery(<AdminSettingsPage />);
    await gotoSection(/Notifications/i);
    expect(await screen.findByText("ops-slack")).toBeInTheDocument();
    await userEvent.click(await screen.findByRole("switch", { name: /Enable sink/i }));
    expect(await screen.findByRole("switch", { name: /Disable sink/i })).toBeInTheDocument();
  });

  it("lists backup destinations and opens the add form", async () => {
    renderWithQuery(<AdminSettingsPage />);
    await gotoSection(/Backup destinations/i);
    // The default handler returns one destination ("default").
    await userEvent.click(await screen.findByRole("button", { name: /Add destination/i }));
    const nameField = await screen.findByPlaceholderText("kestrel-backup-repo");
    await userEvent.type(nameField, "repo1");
    await userEvent.type(screen.getByPlaceholderText(/s3:s3.example.com/i), "s3:host/bucket");
    await userEvent.type(screen.getByPlaceholderText(/passphrase/i), "a-strong-passphrase");
    const save = screen.getByRole("button", { name: /Save destination/i });
    await waitFor(() => expect(save).toBeEnabled());
    await userEvent.click(save);
    // On success the form closes; the Add button returns.
    expect(await screen.findByRole("button", { name: /Add destination/i })).toBeInTheDocument();
  });

  it("prompts before deleting a backup destination", async () => {
    renderWithQuery(<AdminSettingsPage />);
    await gotoSection(/Backup destinations/i);
    await userEvent.click(await screen.findByRole("button", { name: /Delete default/i }));
    expect(await screen.findByText(/Delete backup destination\?/i)).toBeInTheDocument();
  });

  it("renders the About section with the license", async () => {
    renderWithQuery(<AdminSettingsPage />);
    await gotoSection(/About/i);
    expect(await screen.findByText("AGPL-3.0")).toBeInTheDocument();
    expect(screen.getByText("Kestrel")).toBeInTheDocument();
  });
});
