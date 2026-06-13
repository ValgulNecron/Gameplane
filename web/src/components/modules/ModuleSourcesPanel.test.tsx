import { describe, it, expect } from "vitest";
import { http, HttpResponse } from "msw";
import { screen, fireEvent, waitFor } from "@testing-library/react";
// (waitFor used for both the timing path and the empty-state DOM scan.)
import { server } from "@/test/server";
import { renderWithQuery } from "@/test/render";
import { makeModuleSource } from "@/test/factories";
import { ModuleSourcesPanel, sourceLocation } from "./ModuleSourcesPanel";

describe("ModuleSourcesPanel", () => {
  it("renders the listing", async () => {
    server.use(
      http.get("/modules/sources", () =>
        HttpResponse.json({
          items: [
            makeModuleSource({
              metadata: { name: "upstream" },
              spec: { type: "oci", oci: { url: "ghcr.io/x", modules: [{ name: "minecraft" }] } },
              status: {
                conditions: [
                  { type: "Synced", status: "True", lastTransitionTime: "2026-01-01T00:00:00Z" },
                ],
                modules: [{ name: "minecraft" }],
              },
            }),
          ],
        }),
      ),
    );
    renderWithQuery(<ModuleSourcesPanel />);
    expect(await screen.findByText("upstream")).toBeInTheDocument();
    expect(screen.getByText("ghcr.io/x")).toBeInTheDocument();
  });

  it("shows the empty state when no sources", async () => {
    server.use(
      http.get("/modules/sources", () => HttpResponse.json({ items: [] })),
    );
    renderWithQuery(<ModuleSourcesPanel />);
    // The empty-state message wraps "ModuleSource" in a <code> tag, so
    // a single text matcher has to span elements via textContent.
    await waitFor(() => {
      const empty = Array.from(document.querySelectorAll("div")).find((d) =>
        /No.*ModuleSource.*resources configured/i.test(d.textContent ?? ""),
      );
      expect(empty).toBeTruthy();
    });
  });

  it("shows the loading state initially", () => {
    server.use(
      http.get("/modules/sources", async () => {
        await new Promise((r) => setTimeout(r, 50));
        return HttpResponse.json({ items: [] });
      }),
    );
    renderWithQuery(<ModuleSourcesPanel />);
    expect(screen.getByText(/Loading module sources/i)).toBeInTheDocument();
  });

  it("shows error UI on failure", async () => {
    server.use(
      http.get("/modules/sources", () => HttpResponse.text("boom", { status: 500 })),
    );
    renderWithQuery(<ModuleSourcesPanel />);
    await waitFor(() =>
      expect(screen.getByText(/Failed to load module sources/i)).toBeInTheDocument(),
    );
  });

  it("formats the location per source type", () => {
    expect(sourceLocation({ type: "oci", oci: { url: "ghcr.io/x", modules: [] } })).toBe("ghcr.io/x");
    expect(sourceLocation({ oci: { url: "ghcr.io/y", modules: [] } })).toBe("ghcr.io/y"); // type defaults to oci
    expect(sourceLocation({ type: "git", git: { url: "https://g/x.git", ref: "main" } })).toBe(
      "https://g/x.git@main",
    );
    expect(sourceLocation({ type: "git", git: { url: "https://g/x.git" } })).toBe("https://g/x.git");
    expect(sourceLocation({ type: "http", http: { url: "https://e/m.tar.gz" } })).toBe("https://e/m.tar.gz");
    expect(sourceLocation({ type: "local", local: { path: "bundles" } })).toBe("local:bundles");
    expect(sourceLocation({ type: "local", local: {} })).toBe("local");
    expect(sourceLocation({ type: "upload" })).toBe("uploaded bundles");
    expect(sourceLocation({ type: "git" })).toBe(""); // malformed spec degrades to empty
    expect(sourceLocation({ type: "http" })).toBe("");
  });

  it("renders a git source row with type badge", async () => {
    server.use(
      http.get("/modules/sources", () =>
        HttpResponse.json({
          items: [
            makeModuleSource({
              metadata: { name: "community" },
              spec: { type: "git", git: { url: "https://github.com/x/mods", ref: "stable" } },
            }),
          ],
        }),
      ),
    );
    renderWithQuery(<ModuleSourcesPanel />);
    expect(await screen.findByText("community")).toBeInTheDocument();
    expect(screen.getByText("git")).toBeInTheDocument();
    expect(screen.getByText("https://github.com/x/mods@stable")).toBeInTheDocument();
  });

  it("creates a source through the dialog", async () => {
    let created: unknown = null;
    server.use(
      http.get("/modules/sources", () => HttpResponse.json({ items: [] })),
      http.post("/modules/sources", async ({ request }) => {
        created = await request.json();
        return HttpResponse.json({ metadata: { name: "uploads" }, spec: { type: "upload" } }, { status: 201 });
      }),
    );
    renderWithQuery(<ModuleSourcesPanel />);
    fireEvent.click(await screen.findByRole("button", { name: /Add source/ }));
    fireEvent.change(screen.getByPlaceholderText("community"), { target: { value: "uploads" } });
    fireEvent.change(screen.getByRole("combobox", { name: "Type" }), { target: { value: "upload" } });
    fireEvent.click(screen.getByRole("button", { name: "Add source" }));
    await waitFor(() => expect(created).not.toBeNull());
    expect(created).toMatchObject({ name: "uploads", type: "upload" });
  });

  it("opens the edit dialog prefilled for a row", async () => {
    server.use(
      http.get("/modules/sources", () =>
        HttpResponse.json({
          items: [
            makeModuleSource({
              metadata: { name: "community" },
              spec: { type: "git", git: { url: "https://g/x.git", ref: "main" } },
            }),
          ],
        }),
      ),
    );
    renderWithQuery(<ModuleSourcesPanel />);
    fireEvent.click(await screen.findByRole("button", { name: "Edit community" }));
    expect(await screen.findByText("Edit source community")).toBeInTheDocument();
    expect(screen.getByDisplayValue("https://g/x.git")).toBeInTheDocument();
  });

  it("surfaces a delete conflict on the panel", async () => {
    server.use(
      http.get("/modules/sources", () =>
        HttpResponse.json({ items: [makeModuleSource({ metadata: { name: "upstream" } })] }),
      ),
      http.delete("/modules/sources/upstream", () =>
        HttpResponse.text('source "upstream" is still used by installed module(s): mc', {
          status: 409,
        }),
      ),
    );
    renderWithQuery(<ModuleSourcesPanel />);
    fireEvent.click(await screen.findByRole("button", { name: "Delete upstream" }));
    expect(await screen.findByText(/still used by installed module/)).toBeInTheDocument();
  });

  it("renders the insecure pill when set", async () => {
    server.use(
      http.get("/modules/sources", () =>
        HttpResponse.json({
          items: [
            makeModuleSource({
              spec: { type: "oci", oci: { url: "localhost:5000", modules: [], insecure: true } },
            }),
          ],
        }),
      ),
    );
    renderWithQuery(<ModuleSourcesPanel />);
    expect(await screen.findByText(/insecure/)).toBeInTheDocument();
  });

  it("renders a keyless verify pill for a signed oci source", async () => {
    server.use(
      http.get("/modules/sources", () =>
        HttpResponse.json({
          items: [
            makeModuleSource({
              metadata: { name: "upstream" },
              spec: {
                type: "oci",
                oci: { url: "ghcr.io/x", modules: [{ name: "mc" }] },
                verify: { keyless: { issuer: "https://issuer", identity: "id@example.com" } },
              },
            }),
          ],
        }),
      ),
    );
    renderWithQuery(<ModuleSourcesPanel />);
    expect(await screen.findByText("keyless")).toBeInTheDocument();
  });

  it("renders a keyed verify pill for a key-pinned source", async () => {
    server.use(
      http.get("/modules/sources", () =>
        HttpResponse.json({
          items: [
            makeModuleSource({
              metadata: { name: "upstream" },
              spec: {
                type: "oci",
                oci: { url: "ghcr.io/x", modules: [{ name: "mc" }] },
                verify: { key: { name: "cosign-pub" } },
              },
            }),
          ],
        }),
      ),
    );
    renderWithQuery(<ModuleSourcesPanel />);
    expect(await screen.findByText("keyed")).toBeInTheDocument();
  });

  it("flags a source serving a stale catalog", async () => {
    server.use(
      http.get("/modules/sources", () =>
        HttpResponse.json({
          items: [
            makeModuleSource({
              metadata: { name: "community" },
              spec: { type: "git", git: { url: "https://g/x.git", ref: "main" } },
              status: {
                lastSync: "2026-01-01T00:00:00Z",
                modules: [{ name: "mc" }],
                conditions: [
                  { type: "Synced", status: "False", reason: "IndexFailed" },
                  { type: "Ready", status: "True", reason: "ServingStaleCatalog" },
                ],
              },
            }),
          ],
        }),
      ),
    );
    renderWithQuery(<ModuleSourcesPanel />);
    expect(await screen.findByText("serving stale catalog")).toBeInTheDocument();
  });
});
