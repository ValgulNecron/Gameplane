// Thin fetch wrapper for the Gameplane API. Handles:
//   - base URL (relative; vite proxy forwards in dev, same-origin in prod)
//   - CSRF header injection for mutating requests (cookie → header)
//   - uniform error throwing so TanStack Query .error works consistently
//   - cluster query param threading for multi-cluster support

import { getCurrentCluster } from "./cluster";
import type { CaptureStatus, NetworkCapture, NetworkCaptureList } from "@/types";

const CSRF_COOKIE = "gameplane_csrf";
const CSRF_HEADER = "X-Gameplane-CSRF";

function csrfToken(): string {
  const match = document.cookie.match(new RegExp("(?:^|; )" + CSRF_COOKIE + "=([^;]+)"));
  return match ? decodeURIComponent(match[1]) : "";
}

// withNS appends the namespace query param when provided. Local to this
// file so the capture client calls below don't need a dependency on
// web/src/lib/endpoints.ts (which itself depends on this file — importing
// back would be circular). Mirrors endpoints.ts's withNS exactly.
function withNS(path: string, ns?: string): string {
  if (!ns) return path;
  const sep = path.includes("?") ? "&" : "?";
  return `${path}${sep}namespace=${encodeURIComponent(ns)}`;
}

// withClusterParam appends the ?cluster= query param for multi-cluster
// support, the same rule api<T>() applies internally below. Used only
// internally by Captures.download().
function withClusterParam(path: string): string {
  const clusterId = getCurrentCluster();
  if (clusterId === "local") return path;
  const sep = path.includes("?") ? "&" : "?";
  return `${path}${sep}cluster=${encodeURIComponent(clusterId)}`;
}

// csrfHeaders returns the CSRF header for callers that bypass api() —
// raw fetch with a non-JSON body (multipart upload, plaintext write) still
// needs the same token the API enforces on all mutating requests.
export function csrfHeaders(): Record<string, string> {
  return { [CSRF_HEADER]: csrfToken() };
}

export class APIError extends Error {
  status: number;
  body: string;
  constructor(status: number, body: string) {
    super(`${status}: ${body}`);
    this.status = status;
    this.body = body;
  }
}

export interface Options {
  method?: string;
  body?: unknown;
  headers?: Record<string, string>;
  signal?: AbortSignal;
}

export async function api<T>(path: string, opts: Options = {}): Promise<T> {
  const method = opts.method ?? "GET";
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...(opts.headers ?? {}),
  };
  const mutating = method !== "GET" && method !== "HEAD" && method !== "OPTIONS";
  if (mutating) headers[CSRF_HEADER] = csrfToken();

  // Thread cluster query param for multi-cluster support.
  // Only append when non-local to preserve back-compat (local = default, omit param).
  const clusterId = getCurrentCluster();
  let requestPath = path;
  if (clusterId !== "local") {
    const sep = path.includes("?") ? "&" : "?";
    requestPath = `${path}${sep}cluster=${encodeURIComponent(clusterId)}`;
  }

  const res = await fetch(requestPath, {
    method,
    headers,
    credentials: "include",
    signal: opts.signal,
    body: opts.body === undefined ? undefined : JSON.stringify(opts.body),
  });
  if (!res.ok) {
    const text = await res.text().catch(() => "");
    throw new APIError(res.status, text);
  }
  if (res.status === 204) return undefined as T;
  return res.json() as Promise<T>;
}

// ---------------------------------------------------------------------
// Capture (network-capture sidecar) REST client calls — contracts/
// rest-api.md, verified against the response structs in
// api/internal/handlers/capture.go (the code wins where they disagree;
// see the T086 report for the two places that happened).
// ---------------------------------------------------------------------

// captureToggleResponse mirrors captureHandler's captureToggleResp
// (api/internal/handlers/capture.go) — the shape returned by both
// capture-enable and capture-disable. Not exported from web/src/types.ts,
// since it nests CaptureStatus under {name, status.capture} only for
// these two endpoints.
export interface CaptureToggleResponse {
  name: string;
  status: {
    capture: CaptureStatus;
  };
}

export interface CaptureStartBody {
  filter?: string;
  maxDurationSeconds: number;
  maxSizeBytes: number;
  ttlSecondsAfterFinished?: number;
}

// captureDeleteResponse mirrors captureDeleteResp (capture.go).
export interface CaptureDeleteResponse {
  deleted: boolean;
  captureId: string;
}

export const Captures = {
  // POST /servers/{name}:capture-enable
  enable: (name: string, ns?: string) =>
    api<CaptureToggleResponse>(withNS(`/servers/${name}:capture-enable`, ns), { method: "POST" }),
  // POST /servers/{name}:capture-disable
  disable: (name: string, ns?: string) =>
    api<CaptureToggleResponse>(withNS(`/servers/${name}:capture-disable`, ns), { method: "POST" }),
  // POST /servers/{name}:capture-start — 202 Accepted; response fields
  // beyond captureStartResp's (ttlSecondsAfterFinished is present here,
  // stoppingReason is not) are typed optional on NetworkCapture.
  start: (name: string, body: CaptureStartBody, ns?: string) =>
    api<NetworkCapture>(withNS(`/servers/${name}:capture-start`, ns), { method: "POST", body }),
  // POST /servers/{name}:capture-stop — captureId goes in the JSON body,
  // not a query param (captureStopReq, capture.go).
  stop: (name: string, captureId: string, ns?: string) =>
    api<NetworkCapture>(withNS(`/servers/${name}:capture-stop`, ns), {
      method: "POST",
      body: { captureId },
    }),
  // GET /servers/{name}:captures
  list: (name: string, ns?: string) =>
    api<NetworkCaptureList>(withNS(`/servers/${name}:captures`, ns)),
  // GET /servers/{name}:capture?id={id}
  get: (name: string, id: string, ns?: string) =>
    api<NetworkCapture>(withNS(`/servers/${name}:capture?id=${encodeURIComponent(id)}`, ns)),
  // DELETE /servers/{name}:capture?id={id}
  remove: (name: string, id: string, ns?: string) =>
    api<CaptureDeleteResponse>(
      withNS(`/servers/${name}:capture?id=${encodeURIComponent(id)}`, ns),
      { method: "DELETE" },
    ),
  // GET /servers/{name}:capture-file?id={id} — binary PCAPNG (research.md
  // Decision 1: streamed straight through from the sidecar). Bypasses
  // api<T>()'s JSON handling, matching Cluster.kubeconfig /
  // Audit.exportCsv's Blob-download pattern (web/src/lib/endpoints.ts).
  // A GET carries no CSRF header, consistent with api<T>()'s own rule
  // (mutating = method !== GET/HEAD/OPTIONS).
  download: async (name: string, id: string, ns?: string): Promise<Blob> => {
    const path = withClusterParam(
      withNS(`/servers/${name}:capture-file?id=${encodeURIComponent(id)}`, ns),
    );
    const res = await fetch(path, { method: "GET", credentials: "include" });
    if (!res.ok) {
      const text = await res.text().catch(() => "");
      throw new APIError(res.status, text);
    }
    return res.blob();
  },
};
