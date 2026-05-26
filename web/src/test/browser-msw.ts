// Browser-side MSW setup, only loaded when VITE_E2E_MOCK is true.
// main.tsx dynamically imports this module so it never appears in a
// production build (Vite tree-shakes the dead branch).
//
// Reuses the same `handlers` array as the vitest suite — the contract
// the dashboard expects from the API is declared once and exercised
// from both unit tests and Playwright mock-mode runs.

import { setupWorker } from "msw/browser";
import { handlers } from "./handlers";

const worker = setupWorker(...handlers);

export async function startMSW(): Promise<void> {
  await worker.start({
    // Don't error on requests we haven't mocked — pass them through to
    // the real network. Lets us mix mocked auth/list endpoints with a
    // real fetch for things we don't care to mock.
    onUnhandledRequest: "bypass",
    serviceWorker: { url: "/mockServiceWorker.js" },
  });
}
