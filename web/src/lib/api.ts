// Thin fetch wrapper for the Gameplane API. Handles:
//   - base URL (relative; vite proxy forwards in dev, same-origin in prod)
//   - CSRF header injection for mutating requests (cookie → header)
//   - uniform error throwing so TanStack Query .error works consistently
//   - cluster query param threading for multi-cluster support

import { getCurrentCluster } from "./cluster";

const CSRF_COOKIE = "gameplane_csrf";
const CSRF_HEADER = "X-Gameplane-CSRF";

function csrfToken(): string {
  const match = document.cookie.match(new RegExp("(?:^|; )" + CSRF_COOKIE + "=([^;]+)"));
  return match ? decodeURIComponent(match[1]) : "";
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
