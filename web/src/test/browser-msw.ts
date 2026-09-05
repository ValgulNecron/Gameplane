// Browser-side MSW setup, only loaded when VITE_E2E_MOCK is true.
// main.tsx dynamically imports this module so it never appears in a
// production build (Vite tree-shakes the dead branch).
//
// Reuses the same `handlers` array as the vitest suite — the contract
// the dashboard expects from the API is declared once and exercised
// from both unit tests and Playwright mock-mode runs.
//
// When localStorage.getItem("gameplane-e2e-dataset") === "screenshots",
// uses an enriched handler set with diverse test data for demo/screenshot runs.
// When it is "sso-only", overrides /auth/providers to report no local-login
// provider, so Login.tsx renders its SSO-only branch.

import { setupWorker } from "msw/browser";
import { handlers, buildScreenshotHandlers, buildSsoOnlyHandlers } from "./handlers";

function getHandlerSet(): Parameters<typeof setupWorker>[0][] {
  // Wrap in try/catch in case localStorage is unavailable (e.g., sandboxed iframe)
  try {
    if (typeof window !== "undefined") {
      const dataset = window.localStorage.getItem("gameplane-e2e-dataset");
      if (dataset === "screenshots") {
        return buildScreenshotHandlers();
      }
      if (dataset === "sso-only") {
        return buildSsoOnlyHandlers();
      }
    }
  } catch {
    // localStorage unavailable; fall through to default
  }
  return handlers;
}

const worker = setupWorker(...getHandlerSet());

export async function startMSW(): Promise<void> {
  await worker.start({
    // Don't error on requests we haven't mocked — pass them through to
    // the real network. Lets us mix mocked auth/list endpoints with a
    // real fetch for things we don't care to mock.
    onUnhandledRequest: "bypass",
    serviceWorker: { url: "/mockServiceWorker.js" },
  });
}
