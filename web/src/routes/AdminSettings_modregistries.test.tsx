import { describe, it, expect, vi } from "vitest";
import type { ReactNode } from "react";
import { http, HttpResponse } from "msw";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { server } from "@/test/server";
import { renderWithQuery } from "@/test/render";
import { makeConfig } from "@/test/factories";
import type { ModRegistriesCfg } from "@/lib/config";

vi.mock("@tanstack/react-router", () => ({
  Link: ({ children, to, ...rest }: { children: ReactNode; to: string } & Record<string, unknown>) => (
    <a href={to} {...rest}>{children}</a>
  ),
}));

import { AdminSettingsPage } from "./AdminSettings";

function seedRegistries(modRegistries: ModRegistriesCfg) {
  server.use(
    http.get("/admin/config", () => HttpResponse.json(makeConfig({ modRegistries }))),
  );
}

async function gotoModRegistries() {
  await screen.findByRole("heading", { name: /Admin settings/i });
  await userEvent.click(await screen.findByRole("button", { name: /Mod registries/i }));
}

describe("AdminSettings mod registries", () => {
  it("shows a configured provider as Configured with Replace/Remove actions", async () => {
    seedRegistries({ registries: [{ provider: "curseforge", configRef: "gameplane-modreg-curseforge" }] });
    renderWithQuery(<AdminSettingsPage />);
    await gotoModRegistries();
    // Scope to the CurseForge row: Steam and Nexus are legitimately
    // unconfigured in this fixture and render their own "Set API key"
    // buttons, so an unscoped query would see more than one.
    const row = (await screen.findByText("CurseForge")).closest("li")!;
    expect(within(row).getByText("Configured")).toBeInTheDocument();
    expect(within(row).getByText(/Active in the Mods browser/i)).toBeInTheDocument();
    expect(within(row).getByRole("button", { name: /Replace/i })).toBeInTheDocument();
    expect(within(row).getByRole("button", { name: /Remove/i })).toBeInTheDocument();
    expect(within(row).queryByRole("button", { name: /Set API key/i })).not.toBeInTheDocument();
  });

  it("shows unconfigured providers as Not configured, hidden from the Mods browser", async () => {
    seedRegistries({ registries: [] });
    renderWithQuery(<AdminSettingsPage />);
    await gotoModRegistries();
    expect(await screen.findByText("Steam Workshop")).toBeInTheDocument();
    expect(screen.getByText("Nexus Mods")).toBeInTheDocument();
    // Scope to the registries list: the section's own subtitle copy
    // ("...stay hidden from the Mods browser until a key is configured")
    // also satisfies the /Hidden from the Mods browser/i regex, so an
    // unscoped query over-counts by one.
    const list = screen.getByRole("list");
    const notConfigured = within(list).getAllByText("Not configured");
    expect(notConfigured).toHaveLength(3); // curseforge, steam, nexus
    expect(within(list).getAllByText(/Hidden from the Mods browser/i)).toHaveLength(3);
    expect(within(list).getAllByRole("button", { name: /Set API key/i })).toHaveLength(3);
    expect(screen.getByText(
      /Always available: Modrinth, Thunderstore, Hangar, Factorio, Spigot, GitHub, uMod/i,
    )).toBeInTheDocument();
  });

  it("sets an API key: stores the Secret first, then saves the config row on Save changes", async () => {
    const calls: string[] = [];
    let secretBody: Record<string, string> | undefined;
    let saved: ModRegistriesCfg | undefined;
    server.use(
      http.put("/admin/registries/:provider/secret", async ({ params, request }) => {
        calls.push("secret");
        secretBody = (await request.json()) as Record<string, string>;
        return HttpResponse.json({ name: `gameplane-modreg-${String(params.provider)}`, keys: ["apiKey"] });
      }),
      http.put("/admin/config/modRegistries", async ({ request }) => {
        calls.push("config");
        saved = (await request.json()) as ModRegistriesCfg;
        return new HttpResponse(null, { status: 204 });
      }),
    );
    seedRegistries({ registries: [] });
    renderWithQuery(<AdminSettingsPage />);
    await gotoModRegistries();
    const steamRow = (await screen.findByText("Steam Workshop")).closest("li")!;
    await userEvent.click(within(steamRow).getByRole("button", { name: /Set API key/i }));
    await userEvent.type(screen.getByLabelText(/API key/i), "steam-secret-key");
    await userEvent.click(screen.getByRole("button", { name: /^Save$/i }));
    // The row flips to Configured locally, but the config section itself
    // isn't persisted until the section's own Save changes.
    expect(await screen.findByText(/Active in the Mods browser/i)).toBeInTheDocument();
    expect(secretBody).toEqual({ apiKey: "steam-secret-key" });
    expect(calls).toEqual(["secret"]);
    await userEvent.click(screen.getByRole("button", { name: /^Save changes$/i }));
    await waitFor(() => expect(saved).toBeDefined());
    expect(calls).toEqual(["secret", "config"]);
    expect(saved?.registries).toEqual([
      { provider: "steam", configRef: "gameplane-modreg-steam" },
    ]);
  });

  it("removes a configured key: deletes the managed Secret and updates the draft immediately", async () => {
    let deletedProvider: string | null = null;
    seedRegistries({ registries: [{ provider: "curseforge", configRef: "gameplane-modreg-curseforge" }] });
    server.use(
      http.delete("/admin/registries/:provider/secret", ({ params }) => {
        deletedProvider = String(params.provider);
        return new HttpResponse(null, { status: 204 });
      }),
    );
    renderWithQuery(<AdminSettingsPage />);
    await gotoModRegistries();
    await screen.findByText("Configured");
    await userEvent.click(screen.getByRole("button", { name: /Remove/i }));
    // Scope to the registries list — see the note in the "unconfigured
    // providers" test above about the section subtitle colliding with the
    // /Hidden from the Mods browser/i regex.
    const list = screen.getByRole("list");
    expect(await within(list).findAllByText("Not configured")).toHaveLength(3);
    expect(within(list).getAllByText(/Hidden from the Mods browser/i)).toHaveLength(3);
    await waitFor(() => expect(deletedProvider).toBe("curseforge"));
  });

  it("does not delete a Secret the section didn't create when removing a provider", async () => {
    let deleteCalled = false;
    seedRegistries({ registries: [{ provider: "nexus", configRef: "my-own-secret" }] });
    server.use(
      http.delete("/admin/registries/:provider/secret", () => {
        deleteCalled = true;
        return new HttpResponse(null, { status: 204 });
      }),
    );
    renderWithQuery(<AdminSettingsPage />);
    await gotoModRegistries();
    await screen.findByText("Configured");
    await userEvent.click(screen.getByRole("button", { name: /Remove/i }));
    expect(await screen.findAllByText("Not configured")).toHaveLength(3);
    expect(deleteCalled).toBe(false);
  });

  it("never renders the stored API key anywhere, including the Replace form", async () => {
    seedRegistries({ registries: [{ provider: "curseforge", configRef: "gameplane-modreg-curseforge" }] });
    renderWithQuery(<AdminSettingsPage />);
    await gotoModRegistries();
    // GET /admin/config never carries key material — only the configRef.
    expect(screen.queryByText(/apiKey/i)).not.toBeInTheDocument();
    await userEvent.click(await screen.findByRole("button", { name: /Replace/i }));
    const input = screen.getByLabelText(/API key/i) as HTMLInputElement;
    // Write-only: the Replace form never prefills a value to reveal.
    expect(input).toHaveValue("");
    expect(input).toHaveAttribute("type", "password");
  });
});
