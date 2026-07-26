import { afterEach, describe, it, expect, vi } from "vitest";
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

afterEach(() => {
  server.resetHandlers();
});

describe("AdminSettingsPage", () => {
  it("renders the General section by default", async () => {
    server.use(
      http.get("/admin/config", () =>
        HttpResponse.json({
          general: {
            instanceName: "gameplane-prod",
            externalURL: "https://example.com",
            defaultNamespace: "gameplane-games",
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

  it("shows loading state when config is loading", async () => {
    server.use(
      http.get("/admin/config", async () => {
        await new Promise((r) => setTimeout(r, 100));
        return HttpResponse.json({
          general: { instanceName: "", externalURL: "", defaultNamespace: "" },
        });
      }),
    );
    renderWithQuery(<AdminSettingsPage />);
    expect(screen.getByText("Loading configuration…")).toBeInTheDocument();
  });

  it("shows error state when config fails to load", async () => {
    server.use(
      http.get("/admin/config", () =>
        new HttpResponse("Server error", { status: 500 }),
      ),
    );
    renderWithQuery(<AdminSettingsPage />);
    expect(await screen.findByText(/Failed to load configuration/i)).toBeInTheDocument();
  });

  it("navigates to Auth section and shows auth form", async () => {
    server.use(
      http.get("/admin/config", () =>
        HttpResponse.json({
          auth: {
            providers: [
              { name: "Local accounts", kind: "local", enabled: true },
            ],
          },
          general: { instanceName: "", externalURL: "", defaultNamespace: "" },
        }),
      ),
      http.get("/auth/providers", () =>
        HttpResponse.json({
          providers: [{ name: "Local accounts", kind: "local", label: "Local Accounts" }],
        }),
      ),
    );
    renderWithQuery(<AdminSettingsPage />);
    await screen.findByRole("heading", { name: /Admin settings/i });
    await userEvent.click(screen.getByRole("button", { name: /Authentication/i }));
    // Exact string, not a case-insensitive regex: the section subtitle
    // ("Built-in local accounts plus federated identity providers…") also
    // matches /Local accounts/i, so a regex query is ambiguous here.
    expect(await screen.findByText("Local accounts")).toBeInTheDocument();
  });

  it("saves general section changes", async () => {
    const saveHandler = vi.fn(() => new HttpResponse(null, { status: 204 }));
    server.use(
      http.get("/admin/config", () =>
        HttpResponse.json({
          general: {
            instanceName: "old-name",
            externalURL: "https://old.example.com",
            defaultNamespace: "gameplane-games",
          },
        }),
      ),
      http.put("/admin/config/general", saveHandler),
    );
    renderWithQuery(<AdminSettingsPage />);
    const input = await screen.findByDisplayValue("old-name");
    await userEvent.clear(input);
    await userEvent.type(input, "new-name");
    await userEvent.click(screen.getByRole("button", { name: /Save changes/i }));
    await waitFor(() => expect(screen.getByText("Saved")).toBeInTheDocument());
    expect(saveHandler).toHaveBeenCalled();
  });

  it("shows error message on save failure", async () => {
    server.use(
      http.get("/admin/config", () =>
        HttpResponse.json({
          general: {
            instanceName: "",
            externalURL: "",
            defaultNamespace: "",
          },
        }),
      ),
      http.put("/admin/config/general", () =>
        new HttpResponse("Validation error", { status: 400 }),
      ),
    );
    renderWithQuery(<AdminSettingsPage />);
    // All three General fields are blank in this fixture, so
    // findByDisplayValue("") would match all of them ambiguously — wait
    // for the section's own Save button to render instead.
    await screen.findByRole("button", { name: /Save changes/i });
    await userEvent.click(screen.getByRole("button", { name: /Save changes/i }));
    await waitFor(() =>
      expect(screen.getByText(/Validation error/i)).toBeInTheDocument(),
    );
  });

  it("shows telemetry toggle and saves setting", async () => {
    server.use(
      http.get("/admin/config", () =>
        HttpResponse.json({
          general: { instanceName: "", externalURL: "", defaultNamespace: "" },
          telemetry: { sendMetrics: false },
        }),
      ),
      http.put("/admin/config/telemetry", () => new HttpResponse(null, { status: 204 })),
    );
    renderWithQuery(<AdminSettingsPage />);
    await userEvent.click(screen.getByRole("button", { name: /Telemetry/i }));
    // The Switch component renders role="switch" (a <button>), not a
    // native checkbox input; assert via aria-checked, matching
    // switch.test.tsx's own convention for this component.
    const toggle = await screen.findByRole("switch");
    expect(toggle).toHaveAttribute("aria-checked", "false");
    await userEvent.click(toggle);
    await userEvent.click(screen.getByRole("button", { name: /Save changes/i }));
    await waitFor(() => expect(screen.getByText("Saved")).toBeInTheDocument());
  });

  it("navigates to Updates section and shows update channel", async () => {
    server.use(
      http.get("/cluster/info", () =>
        HttpResponse.json({
          updateChannel: "stable",
          version: "v1.28.0",
          gameplaneVersion: "v0.2.0",
        }),
      ),
    );
    renderWithQuery(<AdminSettingsPage />);
    await userEvent.click(screen.getByRole("button", { name: /Updates/i }));
    expect(await screen.findByText("stable")).toBeInTheDocument();
  });

  it("navigates to About section and shows version info", async () => {
    server.use(
      http.get("/cluster/info", () =>
        HttpResponse.json({
          gameplaneVersion: "v0.2.0-beta.7",
          version: "v1.28.0",
        }),
      ),
    );
    renderWithQuery(<AdminSettingsPage />);
    await userEvent.click(screen.getByRole("button", { name: /About/i }));
    expect(await screen.findByText("v0.2.0-beta.7")).toBeInTheDocument();
    expect(screen.getByText("v1.28.0")).toBeInTheDocument();
  });

  it("navigates to Notifications section and shows sink list", async () => {
    server.use(
      http.get("/admin/config", () =>
        HttpResponse.json({
          notifications: {
            sinks: [
              {
                name: "discord-alerts",
                kind: "discord",
                enabled: true,
                configRef: "gameplane-notify-discord-alerts",
                events: ["server.unhealthy"],
              },
            ],
          },
        }),
      ),
    );
    renderWithQuery(<AdminSettingsPage />);
    await userEvent.click(screen.getByRole("button", { name: /Notifications/i }));
    expect(await screen.findByText("discord-alerts")).toBeInTheDocument();
  });

  it("toggles notification sink enabled state", async () => {
    const updateHandler = vi.fn(() => new HttpResponse(null, { status: 204 }));
    server.use(
      http.get("/admin/config", () =>
        HttpResponse.json({
          notifications: {
            sinks: [
              {
                name: "test-sink",
                kind: "slack",
                enabled: true,
                configRef: "gameplane-notify-test",
                events: ["server.unhealthy"],
              },
            ],
          },
        }),
      ),
      http.put("/admin/config/notifications", updateHandler),
    );
    renderWithQuery(<AdminSettingsPage />);
    await userEvent.click(screen.getByRole("button", { name: /Notifications/i }));
    const toggle = await screen.findByLabelText(/Disable sink test-sink/i);
    await userEvent.click(toggle);
    await userEvent.click(screen.getByRole("button", { name: /Save changes/i }));
    await waitFor(() => expect(screen.getByText("Saved")).toBeInTheDocument());
  });

  it("shows unsaved changes warning when test is clicked with dirty form", async () => {
    server.use(
      http.get("/admin/config", () =>
        HttpResponse.json({
          general: { instanceName: "", externalURL: "", defaultNamespace: "" },
          notifications: {
            sinks: [
              {
                name: "test-sink",
                kind: "discord",
                enabled: true,
                configRef: "gameplane-notify-test",
                events: [],
              },
            ],
          },
        }),
      ),
    );
    renderWithQuery(<AdminSettingsPage />);
    await userEvent.click(screen.getByRole("button", { name: /Notifications/i }));
    const toggle = await screen.findByLabelText(/Disable sink test-sink/i);
    await userEvent.click(toggle);
    const testBtn = screen.getByRole("button", { name: /Send test/i });
    // toHaveAttribute compares the attribute value with strict equality
    // unless the expected value is an asymmetric matcher — a bare RegExp
    // literal never matches, so wrap it in stringMatching.
    expect(testBtn).toHaveAttribute("title", expect.stringMatching(/Save changes first/i));
  });

  it("runs test on notification sink", async () => {
    const testHandler = vi.fn(() => new HttpResponse(null, { status: 200 }));
    server.use(
      http.get("/admin/config", () =>
        HttpResponse.json({
          notifications: {
            sinks: [
              {
                name: "test-sink",
                kind: "slack",
                enabled: true,
                configRef: "gameplane-notify-test",
                events: ["server.unhealthy"],
              },
            ],
          },
        }),
      ),
      http.post("/notifications/test-sink:test", testHandler),
    );
    renderWithQuery(<AdminSettingsPage />);
    await userEvent.click(screen.getByRole("button", { name: /Notifications/i }));
    const testBtn = await screen.findByRole("button", { name: /Send test/i });
    await userEvent.click(testBtn);
    await waitFor(() =>
      expect(screen.getByText("✓ delivered")).toBeInTheDocument(),
    );
  });

  it("shows different event chips based on events list", async () => {
    server.use(
      http.get("/admin/config", () =>
        HttpResponse.json({
          notifications: {
            sinks: [
              {
                name: "many-events",
                kind: "webhook",
                enabled: true,
                configRef: "gameplane-notify-many",
                events: ["server.unhealthy", "server.recovered", "backup.failed", "backup.succeeded"],
              },
            ],
          },
        }),
      ),
    );
    renderWithQuery(<AdminSettingsPage />);
    await userEvent.click(screen.getByRole("button", { name: /Notifications/i }));
    expect(await screen.findByText(/\+2/)).toBeInTheDocument();
  });

  it("navigates to Mod Registries section", async () => {
    server.use(
      http.get("/admin/config", () =>
        HttpResponse.json({
          modRegistries: { registries: [] },
        }),
      ),
    );
    renderWithQuery(<AdminSettingsPage />);
    await userEvent.click(screen.getByRole("button", { name: /Mod registries/i }));
    expect(await screen.findByText(/CurseForge/i)).toBeInTheDocument();
    expect(screen.getByText(/Steam Workshop/i)).toBeInTheDocument();
    expect(screen.getByText(/Nexus Mods/i)).toBeInTheDocument();
  });

  it("shows configured state for mod registry with key set", async () => {
    server.use(
      http.get("/admin/config", () =>
        HttpResponse.json({
          modRegistries: {
            registries: [
              { provider: "curseforge", configRef: "gameplane-modreg-curseforge" },
            ],
          },
        }),
      ),
    );
    renderWithQuery(<AdminSettingsPage />);
    await userEvent.click(screen.getByRole("button", { name: /Mod registries/i }));
    expect(await screen.findByText("Configured")).toBeInTheDocument();
  });

  it("navigates to Backup Destinations section", async () => {
    server.use(
      http.get("/admin/config", () =>
        HttpResponse.json({}),
      ),
      // BackupDestinations.list() hits /backup-destinations, not the
      // /admin-prefixed path — that prefix is only used for the auth
      // provider / mod registry secret write endpoints.
      http.get("/backup-destinations", () =>
        HttpResponse.json({
          items: [
            {
              name: "primary-backup",
              url: "s3:s3.example.com/bucket",
              hasPassword: true,
              createdAt: "2026-01-01T00:00:00Z",
            },
          ],
        }),
      ),
    );
    renderWithQuery(<AdminSettingsPage />);
    await userEvent.click(screen.getByRole("button", { name: /Backup destinations/i }));
    expect(await screen.findByText("primary-backup")).toBeInTheDocument();
  });

  it("shows error when backup destinations fail to load", async () => {
    server.use(
      http.get("/admin/config", () =>
        HttpResponse.json({}),
      ),
      http.get("/backup-destinations", () =>
        new HttpResponse("Server error", { status: 500 }),
      ),
    );
    renderWithQuery(<AdminSettingsPage />);
    await userEvent.click(screen.getByRole("button", { name: /Backup destinations/i }));
    expect(await screen.findByText(/error/i)).toBeInTheDocument();
  });

  it("shows empty state for backup destinations", async () => {
    server.use(
      http.get("/admin/config", () =>
        HttpResponse.json({}),
      ),
      http.get("/backup-destinations", () =>
        HttpResponse.json({ items: [] }),
      ),
    );
    renderWithQuery(<AdminSettingsPage />);
    await userEvent.click(screen.getByRole("button", { name: /Backup destinations/i }));
    expect(
      await screen.findByText(/No backup destinations configured/i),
    ).toBeInTheDocument();
  });

  it("disables last enabled auth provider toggle", async () => {
    server.use(
      http.get("/admin/config", () =>
        HttpResponse.json({
          auth: {
            providers: [
              { name: "Local accounts", kind: "local", enabled: true },
            ],
          },
        }),
      ),
      http.get("/admin/auth/providers", () =>
        HttpResponse.json({ providers: [] }),
      ),
    );
    renderWithQuery(<AdminSettingsPage />);
    await userEvent.click(screen.getByRole("button", { name: /Authentication/i }));
    const enabledBtn = await screen.findByRole("button", { name: /Enabled/i });
    expect(enabledBtn).toBeDisabled();
  });

  it("allows disabling provider when helm provider exists", async () => {
    server.use(
      http.get("/admin/config", () =>
        HttpResponse.json({
          auth: {
            providers: [
              { name: "Local accounts", kind: "local", enabled: true },
            ],
          },
        }),
      ),
      // AuthSection's runtime-providers query hits /auth/providers, not
      // the /admin-prefixed secret-management path.
      http.get("/auth/providers", () =>
        HttpResponse.json({
          providers: [
            { name: "helm", kind: "oidc", label: "Helm-managed" },
          ],
        }),
      ),
    );
    renderWithQuery(<AdminSettingsPage />);
    await userEvent.click(screen.getByRole("button", { name: /Authentication/i }));
    const enabledBtn = await screen.findByRole("button", { name: /Enabled/i });
    expect(enabledBtn).not.toBeDisabled();
  });

  it("shows warning when last provider would be disabled", async () => {
    server.use(
      http.get("/admin/config", () =>
        HttpResponse.json({
          auth: {
            providers: [
              { name: "Local accounts", kind: "local", enabled: true },
            ],
          },
        }),
      ),
      http.get("/admin/auth/providers", () =>
        HttpResponse.json({ providers: [] }),
      ),
    );
    renderWithQuery(<AdminSettingsPage />);
    await userEvent.click(screen.getByRole("button", { name: /Authentication/i }));
    expect(
      await screen.findByText(/At least one identity provider must stay enabled/i),
    ).toBeInTheDocument();
  });
});
