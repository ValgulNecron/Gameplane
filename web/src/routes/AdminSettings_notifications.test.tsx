import { describe, it, expect, vi } from "vitest";
import type { ReactNode } from "react";
import { http, HttpResponse } from "msw";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { server } from "@/test/server";
import { renderWithQuery } from "@/test/render";
import { makeConfig } from "@/test/factories";
import type { NotificationsCfg } from "@/lib/config";

vi.mock("@tanstack/react-router", () => ({
  Link: ({ children, to, ...rest }: { children: ReactNode; to: string } & Record<string, unknown>) => (
    <a href={to} {...rest}>{children}</a>
  ),
}));

import { AdminSettingsPage } from "./AdminSettings";

function seedSinks(notifications: NotificationsCfg) {
  server.use(
    http.get("/admin/config", () => HttpResponse.json(makeConfig({ notifications }))),
  );
}

async function gotoNotifications() {
  await screen.findByRole("heading", { name: /Admin settings/i });
  await userEvent.click(await screen.findByRole("button", { name: /Notifications/i }));
}

describe("AdminSettings notifications", () => {
  it("flags a legacy sink without a Secret and disables its test button", async () => {
    seedSinks({ sinks: [{ name: "old-hook", kind: "discord", enabled: true }] });
    renderWithQuery(<AdminSettingsPage />);
    await gotoNotifications();
    expect(await screen.findByText("Needs secret")).toBeInTheDocument();
    expect(screen.getByText(/no Secret configured/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Send test/i })).toBeDisabled();
  });

  it("test-fires a configured sink and shows the outcome", async () => {
    seedSinks({
      sinks: [{ name: "team-alerts", kind: "webhook", enabled: true, configRef: "team-hook" }],
    });
    renderWithQuery(<AdminSettingsPage />);
    await gotoNotifications();
    await userEvent.click(await screen.findByRole("button", { name: /Send test/i }));
    expect(await screen.findByText(/delivered/i)).toBeInTheDocument();
  });

  it("surfaces a failed test delivery", async () => {
    seedSinks({
      sinks: [{ name: "team-alerts", kind: "slack", enabled: true, configRef: "team-hook" }],
    });
    server.use(
      http.post("/admin/notifications/sinks/:name/test", () =>
        HttpResponse.text("deliver test to \"team-alerts\": endpoint returned 404", {
          status: 502,
        }),
      ),
    );
    renderWithQuery(<AdminSettingsPage />);
    await gotoNotifications();
    await userEvent.click(await screen.findByRole("button", { name: /Send test/i }));
    expect(await screen.findByText(/endpoint returned 404/i)).toBeInTheDocument();
  });

  it("disables test-send while the draft has unsaved changes", async () => {
    seedSinks({
      sinks: [{ name: "team-alerts", kind: "discord", enabled: true, configRef: "team-hook" }],
    });
    renderWithQuery(<AdminSettingsPage />);
    await gotoNotifications();
    const testBtn = await screen.findByRole("button", { name: /Send test/i });
    expect(testBtn).toBeEnabled();
    await userEvent.click(screen.getByRole("switch", { name: /Disable sink team-alerts/i }));
    expect(testBtn).toBeDisabled();
  });

  it("adds a sink with a Secret ref and event filter, then saves it", async () => {
    let saved: NotificationsCfg | undefined;
    server.use(
      http.put("/admin/config/notifications", async ({ request }) => {
        saved = (await request.json()) as NotificationsCfg;
        return new HttpResponse(null, { status: 204 });
      }),
    );
    renderWithQuery(<AdminSettingsPage />);
    await gotoNotifications();
    await userEvent.click(await screen.findByRole("button", { name: /Add sink/i }));
    // Name and Secret name share the placeholder; they render in that order.
    const [nameInput, secretInput] = screen.getAllByPlaceholderText("team-alerts");
    await userEvent.type(nameInput, "ops-alerts");
    await userEvent.selectOptions(screen.getByRole("combobox", { name: /Sink kind/i }), "slack");
    await userEvent.type(secretInput, "ops-hook");
    // Defaults have server.recovered checked; narrow the filter.
    await userEvent.click(screen.getByRole("checkbox", { name: /server\.recovered/i }));
    await userEvent.click(screen.getByRole("button", { name: /^Add sink$/i }));
    expect(await screen.findByText(/slack · Secret: ops-hook/i)).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: /Save changes/i }));
    await waitFor(() => expect(saved).toBeDefined());
    expect(saved?.sinks).toEqual([
      {
        name: "ops-alerts",
        kind: "slack",
        enabled: true,
        configRef: "ops-hook",
        events: ["server.unhealthy", "backup.failed", "restore.failed"],
      },
    ]);
  });

  it("deletes a sink from the draft", async () => {
    seedSinks({
      sinks: [{ name: "team-alerts", kind: "discord", enabled: true, configRef: "team-hook" }],
    });
    renderWithQuery(<AdminSettingsPage />);
    await gotoNotifications();
    await screen.findByText("team-alerts");
    await userEvent.click(screen.getByRole("button", { name: /Delete sink team-alerts/i }));
    expect(await screen.findByText(/No notification sinks configured/i)).toBeInTheDocument();
    expect(screen.queryByText("team-alerts")).not.toBeInTheDocument();
  });
});
