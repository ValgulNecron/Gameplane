import { describe, it, expect, vi } from "vitest";
// @ts-expect-error jsdom lacks type definitions in this project
import { JSDOM } from "jsdom";
import html from "../../index.html?raw";
const bootScript = /<script id="theme-boot">([\s\S]*?)<\/script>/.exec(html)?.[1];
if (!bootScript) throw new Error("theme boot script not found in index.html");

describe("theme boot script", () => {
  function runBootScript(dom: JSDOM, setupFn?: (window: Window) => void) {
    if (setupFn) {
      setupFn(dom.window as unknown as Window);
    }
    // Execute the script in the window realm by creating a real <script> element.
    // This is necessary because dom.window.eval() evaluates in the Node scope, not
    // the window scope, so localStorage and document would be undefined.
    const script = dom.window.document.createElement("script");
    script.textContent = bootScript;
    dom.window.document.head.appendChild(script);
  }

  describe("applies stored preference", () => {
    it("should apply stored 'dark' preference", () => {
      const dom = new JSDOM(
        `<!doctype html>
         <html lang="en" class="dark" data-theme="dark">
         <head></head>
         <body></body>
         </html>`,
        { url: "http://localhost", runScripts: "dangerously" },
      );

      const { window } = dom;
      window.localStorage.setItem("gameplane-theme", "dark");
      runBootScript(dom);

      expect(window.document.documentElement.classList.contains("dark")).toBe(true);
      expect(window.document.documentElement.classList.contains("light")).toBe(false);
      expect(window.document.documentElement.dataset.theme).toBe("dark");
    });

    it("should apply stored 'light' preference", () => {
      const dom = new JSDOM(
        `<!doctype html>
         <html lang="en" class="dark" data-theme="dark">
         <head></head>
         <body></body>
         </html>`,
        { url: "http://localhost", runScripts: "dangerously" },
      );

      const { window } = dom;
      window.localStorage.setItem("gameplane-theme", "light");
      runBootScript(dom);

      expect(window.document.documentElement.classList.contains("light")).toBe(true);
      expect(window.document.documentElement.classList.contains("dark")).toBe(false);
      expect(window.document.documentElement.dataset.theme).toBe("light");
    });
  });

  describe("respects 'system' preference with matchMedia", () => {
    it("should resolve to 'dark' when system prefers dark", () => {
      const dom = new JSDOM(
        `<!doctype html>
         <html lang="en" class="dark" data-theme="dark">
         <head></head>
         <body></body>
         </html>`,
        { url: "http://localhost", runScripts: "dangerously" },
      );

      // No stored preference → defaults to "system"
      // Mock matchMedia to return dark
      runBootScript(dom, (window) => {
        window.matchMedia = vi.fn((query: string) => ({
          matches: query === "(prefers-color-scheme: dark)",
          media: query,
          onchange: null,
          addListener: vi.fn(),
          removeListener: vi.fn(),
          addEventListener: vi.fn(),
          removeEventListener: vi.fn(),
          dispatchEvent: vi.fn(),
        })) as unknown as typeof window.matchMedia;
      });

      const { window } = dom;
      expect(window.document.documentElement.classList.contains("dark")).toBe(true);
      expect(window.document.documentElement.classList.contains("light")).toBe(false);
      expect(window.document.documentElement.dataset.theme).toBe("dark");
    });

    it("should resolve to 'light' when system prefers light", () => {
      const dom = new JSDOM(
        `<!doctype html>
         <html lang="en" class="dark" data-theme="dark">
         <head></head>
         <body></body>
         </html>`,
        { url: "http://localhost", runScripts: "dangerously" },
      );

      // No stored preference → defaults to "system"
      // Mock matchMedia to return light
      runBootScript(dom, (window) => {
        window.matchMedia = vi.fn((query: string) => ({
          matches: query === "(prefers-color-scheme: light)",
          media: query,
          onchange: null,
          addListener: vi.fn(),
          removeListener: vi.fn(),
          addEventListener: vi.fn(),
          removeEventListener: vi.fn(),
          dispatchEvent: vi.fn(),
        })) as unknown as typeof window.matchMedia;
      });

      const { window } = dom;
      expect(window.document.documentElement.classList.contains("light")).toBe(true);
      expect(window.document.documentElement.classList.contains("dark")).toBe(false);
      expect(window.document.documentElement.dataset.theme).toBe("light");
    });

    it("should default to 'dark' when no preference is stored and matchMedia is unavailable", () => {
      const dom = new JSDOM(
        `<!doctype html>
         <html lang="en" class="dark" data-theme="dark">
         <head></head>
         <body></body>
         </html>`,
        { url: "http://localhost", runScripts: "dangerously" },
      );

      // No stored preference
      // Remove matchMedia
      runBootScript(dom, (window) => {
        delete (window as unknown as { matchMedia?: typeof window.matchMedia }).matchMedia;
      });

      const { window } = dom;
      // Should keep the default dark
      expect(window.document.documentElement.classList.contains("dark")).toBe(true);
      expect(window.document.documentElement.dataset.theme).toBe("dark");
    });
  });

  describe("handles localStorage unavailability gracefully", () => {
    it("should keep dark default when localStorage throws", () => {
      const dom = new JSDOM(
        `<!doctype html>
         <html lang="en" class="dark" data-theme="dark">
         <head></head>
         <body></body>
         </html>`,
        { url: "http://localhost", runScripts: "dangerously" },
      );

      // Make localStorage throw
      runBootScript(dom, (window) => {
        Object.defineProperty(window, "localStorage", {
          get: () => {
            throw new Error("localStorage is not available");
          },
        });
      });

      const { window } = dom;
      // Should keep the default dark
      expect(window.document.documentElement.classList.contains("dark")).toBe(true);
      expect(window.document.documentElement.dataset.theme).toBe("dark");
    });
  });

  describe("initializes data-theme attribute correctly", () => {
    it("should set data-theme to match the resolved class", () => {
      const dom = new JSDOM(
        `<!doctype html>
         <html lang="en" class="dark" data-theme="dark">
         <head></head>
         <body></body>
         </html>`,
        { url: "http://localhost", runScripts: "dangerously" },
      );

      const { window } = dom;
      window.localStorage.setItem("gameplane-theme", "light");
      runBootScript(dom);

      expect(window.document.documentElement.dataset.theme).toBe("light");
    });

    it("should remove both dark and light classes before adding the resolved one", () => {
      const dom = new JSDOM(
        `<!doctype html>
         <html lang="en" class="dark light" data-theme="dark">
         <head></head>
         <body></body>
         </html>`,
        { url: "http://localhost", runScripts: "dangerously" },
      );

      const { window } = dom;
      window.localStorage.setItem("gameplane-theme", "light");
      runBootScript(dom);

      // Should have only light, not both
      expect(window.document.documentElement.classList.contains("light")).toBe(true);
      expect(window.document.documentElement.classList.contains("dark")).toBe(false);
    });
  });
});
