import "@testing-library/jest-dom";
import { configure } from "@testing-library/react";
import { afterAll, afterEach, beforeAll } from "vitest";
import { server } from "./server";

// Raise the default async-utility timeout. On the loaded CI runner, components
// backed by TanStack Query can take well over the 1000ms default to settle, so
// waitFor/findBy intermittently time out (e.g. ServerDetail's lifecycle buttons
// asserting not-disabled). 5s only raises the ceiling — happy-path tests still
// resolve as fast as the data does — and removes a recurring slow-CI flake.
configure({ asyncUtilTimeout: 5000 });

// Radix UI components (Dropdown, Dialog) call into pointer-capture and
// scrollIntoView APIs that jsdom doesn't implement. Without these
// stubs, fireEvent against a Radix trigger throws or silently no-ops.
if (typeof window !== "undefined") {
  if (!Element.prototype.hasPointerCapture) {
    Element.prototype.hasPointerCapture = () => false;
  }
  if (!Element.prototype.releasePointerCapture) {
    Element.prototype.releasePointerCapture = () => {};
  }
  if (!Element.prototype.scrollIntoView) {
    Element.prototype.scrollIntoView = () => {};
  }

  // ResizeObserver / IntersectionObserver / matchMedia — frequently used
  // by virtualized lists, popovers, and theme detection. jsdom omits all
  // three; the stubs no-op so renders that depend on them succeed.
  if (!("ResizeObserver" in window)) {
    (window as unknown as { ResizeObserver: typeof ResizeObserver }).ResizeObserver =
      class {
        observe() {}
        unobserve() {}
        disconnect() {}
      } as unknown as typeof ResizeObserver;
  }
  if (!("IntersectionObserver" in window)) {
    (window as unknown as { IntersectionObserver: typeof IntersectionObserver }).IntersectionObserver =
      class {
        observe() {}
        unobserve() {}
        disconnect() {}
        takeRecords() { return []; }
        root = null;
        rootMargin = "";
        thresholds = [];
      } as unknown as typeof IntersectionObserver;
  }
  if (!window.matchMedia) {
    window.matchMedia = (query: string) =>
      ({
        matches: false,
        media: query,
        onchange: null,
        addListener: () => {},
        removeListener: () => {},
        addEventListener: () => {},
        removeEventListener: () => {},
        dispatchEvent: () => false,
      }) as MediaQueryList;
  }
}

// MSW lifecycle. onUnhandledRequest:"error" makes a missed handler fail
// loudly rather than the test hanging on a real network call.
beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
afterEach(() => server.resetHandlers());
afterAll(() => server.close());
