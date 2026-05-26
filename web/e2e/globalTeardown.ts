// Live-mode globalTeardown. Counterpart to globalSetup — kills the
// kubectl port-forward we spawned. Best-effort: a failure here
// shouldn't fail the test run.

import { existsSync, readFileSync, unlinkSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const pidFile = join(here, ".portforward.pid");

export default async function globalTeardown(): Promise<void> {
  if (!existsSync(pidFile)) return;
  const raw = readFileSync(pidFile, "utf8").trim();
  const pid = Number.parseInt(raw, 10);
  if (Number.isFinite(pid) && pid > 0) {
    try {
      process.kill(pid);
    } catch {
      // already gone
    }
  }
  try {
    unlinkSync(pidFile);
  } catch {
    // best-effort cleanup
  }
}
