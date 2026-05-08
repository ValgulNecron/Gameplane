import { describe, it, expect } from "vitest";
import { http, HttpResponse } from "msw";
import { screen, waitFor } from "@testing-library/react";
// (waitFor used for both the timing path and the empty-state DOM scan.)
import { server } from "@/test/server";
import { renderWithQuery } from "@/test/render";
import { makeModuleSource } from "@/test/factories";
import { ModuleSourcesPanel } from "./ModuleSourcesPanel";

describe("ModuleSourcesPanel", () => {
  it("renders the listing", async () => {
    server.use(
      http.get("/modules/sources", () =>
        HttpResponse.json({
          items: [
            makeModuleSource({
              metadata: { name: "upstream" },
              spec: { url: "ghcr.io/x", modules: [{ name: "minecraft" }] },
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

  it("renders the insecure pill when set", async () => {
    server.use(
      http.get("/modules/sources", () =>
        HttpResponse.json({
          items: [
            makeModuleSource({
              spec: { url: "localhost:5000", modules: [], insecure: true },
            }),
          ],
        }),
      ),
    );
    renderWithQuery(<ModuleSourcesPanel />);
    expect(await screen.findByText(/insecure/)).toBeInTheDocument();
  });
});
