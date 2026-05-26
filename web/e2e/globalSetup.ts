// Live-mode globalSetup. Runs before Playwright spawns the webServer
// (and tests) and is responsible for:
//
//   1. Reading the admin password the Go E2E suite handed off via
//      test/e2e/.tmp/admin-password.
//   2. Spawning `kubectl port-forward` on a fixed port (18080) so
//      vite's dev-server proxy and Playwright tests both have a
//      stable target. Pinned at 18080 because vite reads its env at
//      config-load time (which is before this function runs).
//   3. Polling /healthz on the port-forward until it answers 200.
//   4. Logging in once via the API to obtain session + CSRF cookies,
//      and writing them to e2e/.auth/storage.json so every test reuses
//      a single authenticated context (storageState).
//
// Each Playwright test gets a fresh browser context by default, so we
// can't rely on a beforeEach login — cookies wouldn't survive context
// boundaries. storageState is the documented mechanism.
//
// The port-forward pid is stashed in e2e/.portforward.pid for
// globalTeardown.

import { spawn } from "node:child_process";
import { existsSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const passwordFile = join(here, "..", "..", "test", "e2e", ".tmp", "admin-password");
const pidFile = join(here, ".portforward.pid");
const authDir = join(here, ".auth");
const storageFile = join(authDir, "storage.json");

const PORT = 18080;
const CTX = process.env.KESTREL_E2E_CLUSTER ?? "kestrel-e2e";

export default async function globalSetup(): Promise<void> {
  if (!existsSync(passwordFile)) {
    throw new Error(
      `live-mode setup: admin password file not found at ${passwordFile}. ` +
        `Run the Go e2e suite first (it bootstraps the admin and writes the file).`,
    );
  }
  const pw = readFileSync(passwordFile, "utf8").trim();
  if (!pw) {
    throw new Error(`live-mode setup: admin password file at ${passwordFile} is empty`);
  }
  const username = process.env.KESTREL_E2E_ADMIN_USERNAME ?? "e2e-admin";

  // Spawn kubectl port-forward as a detached subprocess. Stdio is
  // ignored so a successful run is silent; we surface failure via the
  // /healthz polling loop below.
  const child = spawn(
    "kubectl",
    [
      "--context",
      `kind-${CTX}`,
      "port-forward",
      "-n",
      "kestrel-system",
      "svc/kestrel-api",
      `${PORT}:80`,
    ],
    { stdio: "ignore", detached: true },
  );
  child.unref();
  if (!child.pid) {
    throw new Error("live-mode setup: failed to spawn kubectl port-forward");
  }
  writeFileSync(pidFile, String(child.pid), { mode: 0o600 });

  // Poll /healthz. fetch() throws ECONNREFUSED until the listener is up.
  const deadline = Date.now() + 30_000;
  let healthyOK = false;
  while (Date.now() < deadline) {
    try {
      const res = await fetch(`http://localhost:${PORT}/healthz`);
      if (res.ok) {
        healthyOK = true;
        break;
      }
    } catch {
      // not ready yet
    }
    await new Promise((r) => setTimeout(r, 500));
  }
  if (!healthyOK) {
    throw new Error(`live-mode setup: /healthz never returned 200 within 30s on localhost:${PORT}`);
  }

  // Log in once and capture both cookies. We hit the API directly
  // (not through vite) because vite isn't up yet — webServer hasn't
  // started. Cookies stored under host "localhost" so they're sent
  // when Playwright navigates to localhost:5173/auth/login (vite
  // proxies but the cookie host is the same).
  const loginRes = await fetch(`http://localhost:${PORT}/auth/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ username, password: pw }),
  });
  if (!loginRes.ok) {
    const text = await loginRes.text().catch(() => "");
    throw new Error(`live-mode setup: login failed (${loginRes.status}): ${text}`);
  }
  const setCookies = loginRes.headers.getSetCookie();
  if (setCookies.length === 0) {
    throw new Error("live-mode setup: login response did not include Set-Cookie headers");
  }

  // Translate raw Set-Cookie strings into the Playwright storageState
  // shape. We strip the Secure flag because vite serves http (not
  // https) — Playwright's chromium would otherwise refuse to send the
  // cookies to a non-secure origin.
  const cookies = setCookies
    .map((raw) => parseSetCookie(raw))
    .filter((c): c is PlaywrightCookie => c !== null);

  if (!cookies.find((c) => c.name === "kestrel_session")) {
    throw new Error("live-mode setup: kestrel_session cookie missing from login response");
  }

  if (!existsSync(authDir)) mkdirSync(authDir, { recursive: true, mode: 0o700 });
  writeFileSync(
    storageFile,
    JSON.stringify({ cookies, origins: [] }, null, 2),
    { mode: 0o600 },
  );

  // Surface to the spec files for any flow that wants to re-login or
  // probe with raw fetch.
  process.env.ADMIN_USERNAME = username;
  process.env.ADMIN_PASSWORD = pw;
}

interface PlaywrightCookie {
  name: string;
  value: string;
  domain: string;
  path: string;
  expires: number;
  httpOnly: boolean;
  secure: boolean;
  sameSite: "Strict" | "Lax" | "None";
}

// parseSetCookie turns a single Set-Cookie response header into the
// shape Playwright's storageState expects. It's intentionally minimal —
// we only support the attributes our API actually emits (Path,
// HttpOnly, Secure, SameSite, Expires).
function parseSetCookie(raw: string): PlaywrightCookie | null {
  const parts = raw.split(";").map((p) => p.trim());
  const [first, ...attrs] = parts;
  if (!first) return null;
  const eq = first.indexOf("=");
  if (eq <= 0) return null;
  const name = first.slice(0, eq);
  const value = first.slice(eq + 1);

  const cookie: PlaywrightCookie = {
    name,
    value,
    domain: "localhost",
    path: "/",
    expires: -1,
    httpOnly: false,
    // Strip Secure so chromium accepts it on http://localhost:5173.
    // The cookie still authenticates the same session on the server.
    secure: false,
    sameSite: "Lax",
  };
  for (const a of attrs) {
    const lower = a.toLowerCase();
    if (lower === "httponly") cookie.httpOnly = true;
    else if (lower === "secure") {
      // intentionally ignored — see comment above
    } else if (lower.startsWith("path=")) cookie.path = a.slice("path=".length);
    else if (lower.startsWith("samesite=")) {
      const v = a.slice("samesite=".length).toLowerCase();
      cookie.sameSite =
        v === "strict" ? "Strict" : v === "none" ? "None" : "Lax";
    } else if (lower.startsWith("expires=")) {
      const t = Date.parse(a.slice("expires=".length));
      if (!Number.isNaN(t)) cookie.expires = Math.floor(t / 1000);
    }
  }
  return cookie;
}
