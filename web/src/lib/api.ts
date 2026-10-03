// Thin fetch wrapper for the Gameplane API. Handles:
//   - base URL (relative; vite proxy forwards in dev, same-origin in prod)
//   - CSRF header injection for mutating requests (cookie → header)
//   - uniform error throwing so TanStack Query .error works consistently
//   - cluster query param threading for multi-cluster support

import type {
  CaptureStatus,
  NetworkCapture,
  NetworkCaptureList,
  ShareLink,
  ShareLinkPublic,
} from "@/types";

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

// These APIs administer the central installation. The similarly named
// /users/me/servers endpoint is a selected-cluster workload query.
export function isCentralAPI(path: string): boolean {
  const pathname = path.split("?")[0];
  if (pathname === "/users/me/servers") return false;
  return ["/auth", "/users", "/roles", "/admin", "/modules", "/clusters", "/fleet", "/shares"].some(
    (prefix) => pathname === prefix || pathname.startsWith(prefix + "/"),
  );
}

export function withClusterParam(path: string, clusterId = "local"): string {
  if (isCentralAPI(path)) return path;
  // Explicit resource URLs are already scoped; never append a second selector.
  if (new URL(path, "http://gameplane.invalid").searchParams.has("cluster")) return path;
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
  /** Capture the intended cluster when a query or operation is created. */
  cluster?: string;
}

export async function api<T>(path: string, opts: Options = {}): Promise<T> {
  const method = opts.method ?? "GET";
  const headers: Record<string, string> = {
    Accept: "application/json",
    "Content-Type": "application/json",
    ...(opts.headers ?? {}),
  };
  const mutating = method !== "GET" && method !== "HEAD" && method !== "OPTIONS";
  if (mutating) headers[CSRF_HEADER] = csrfToken();

  // Thread cluster query param for multi-cluster support.
  // Only append when non-local to preserve back-compat (local = default, omit param).
  const requestPath = withClusterParam(path, opts.cluster ?? "local");

  const res = await fetch(requestPath, {
    method,
    headers,
    credentials: "include",
    // Older web deployments cached HTML at these same URLs without Vary.
    // Bypass those entries; TanStack Query owns application data caching.
    cache: "no-store",
    signal: opts.signal,
    body: opts.body === undefined ? undefined : JSON.stringify(opts.body),
  });
  if (!res.ok) {
    const text = await res.text().catch(() => "");
    throw new APIError(res.status, text);
  }
  if (res.status === 204) return undefined as T;
  // Some 2xx responses (e.g. 202 Accepted from fire-and-forget actions) carry
  // no body at all — res.json() throws on an empty string, which would look
  // like a request failure to callers. Treat an empty body as success with
  // no payload instead of attempting to parse it.
  const text = await res.text();
  if (text === "") return undefined as T;
  return JSON.parse(text) as T;
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

function makeCaptures(request: typeof api, scopedURL: (path: string) => string, raw: (path: string, init?: RequestInit) => Promise<Response> = (path, init) => fetch(path, init)) {
  const api = request;
  const withClusterParam = scopedURL;
  return {
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
    const res = await raw(path, { method: "GET", credentials: "include" });
    if (!res.ok) {
      const text = await res.text().catch(() => "");
      throw new APIError(res.status, text);
    }
    return res.blob();
  },
};
}

export const Captures = makeCaptures(api, (path) => withClusterParam(path));

// ConfigUpdateResponse mirrors the envelope returned by PUT /admin/config/{section}
// and DELETE /admin/config/auth/role-mappings/{role}, containing the updated section
// and its new value (as a JSON object, not raw message — api<T>() parses it).
export interface ConfigUpdateResponse {
  section: string;
  value: Record<string, unknown>;
}

export const Config = {
  // DELETE /admin/config/auth/role-mappings/{role} — resets one role's override
  // to its Helm-seeded value, returning the updated auth section.
  deleteRoleMapping: (role: string) =>
    api<ConfigUpdateResponse>(
      `/admin/config/auth/role-mappings/${encodeURIComponent(role)}`,
      { method: "DELETE" },
    ),
};

// ShareLinkCreateBody is the discriminated create-request shape callers
// build: exactly one of an absolute `expiresAt` instant or `neverExpires:
// true` (OD-1/OD-5), or the deprecated-for-one-release relative `expiresIn`
// duration string. `canStart` is always required. This narrows
// ShareLinkCreateRequest's flat optional fields into a shape the compiler
// can enforce at call sites, while the wire type (and the deprecated path)
// stay defined in `types.ts`.
export type ShareLinkCreateBody = { canStart: boolean } & (
  | { expiresAt: string; neverExpires?: never; expiresIn?: never }
  | { neverExpires: true; expiresAt?: never; expiresIn?: never }
  | { expiresIn: string; expiresAt?: never; neverExpires?: never }
  // All three omitted: legacy callers relying on the server's existing
  // omitted-expiry default (OD-7), e.g. api.test.ts's URL-encoding test.
  | { expiresAt?: never; neverExpires?: never; expiresIn?: never }
);

// Share link management. Authenticated operations (create, list, revoke) require
// an active session and server ownership; public operations (resolve, start) are
// rate-limited but require no auth. All operations map rate-limit and invalid-link
// errors to neutral responses per FR-005 (privacy: no error detail).
function makeShares(request: typeof api) {
  const api = request;
  return {
  // POST /servers/{name}:shares (authenticated, owner-only).
  // Creates a new share link with an absolute expiry, no expiry, or (deprecated
  // for one release) a relative expiry, plus the start permission.
  create: (server: string, body: ShareLinkCreateBody, ns?: string) =>
    api<ShareLink>(withNS(`/servers/${encodeURIComponent(server)}:shares`, ns), {
      method: "POST",
      body,
    }),

  // GET /servers/{name}:shares (authenticated, owner-only).
  // Lists all share links for a server. The list never includes raw tokens.
  list: (server: string, ns?: string) =>
    api<ShareLink[]>(withNS(`/servers/${encodeURIComponent(server)}:shares`, ns)),

  // DELETE /servers/{name}/shares/{id} (authenticated, owner-only).
  // Revokes a share link, rendering its token invalid immediately.
  revoke: (server: string, id: string, ns?: string) =>
    api<void>(
      withNS(`/servers/${encodeURIComponent(server)}/shares/${encodeURIComponent(id)}`, ns),
      { method: "DELETE" }
    ),

  // GET /shares/{token} (public, no auth, rate-limited).
  // Resolves a share link token to its public view: server name, status, address,
  // player count (if exposed). Returns the same response for invalid, expired, and
  // revoked tokens — callers cannot distinguish (FR-005 privacy rule). Rate-limit
  // errors (429) are also mapped to the same neutral response.
  resolve: async (token: string): Promise<ShareLinkPublic> => {
    try {
      return await api<ShareLinkPublic>(`/shares/${encodeURIComponent(token)}`);
    } catch (err) {
      if (err instanceof APIError && (err.status === 404 || err.status === 429)) {
        // Map 404 (invalid/expired/revoked) and 429 (rate-limited) to a neutral response.
        // This prevents callers from distinguishing between missing and rate-limited states.
        return {
          serverName: "",
          status: "Unknown",
        };
      }
      throw err;
    }
  },

  // POST /shares/{token}/start (public, no auth, rate-limited, only if canStart=true).
  // Wakes a sleeping server if the link permits it. Returns 202 Accepted on success.
  // Returns the same response for invalid/expired/revoked/no-permission states (FR-005).
  // Rate-limit errors (429) are also mapped to neutral.
  start: async (token: string): Promise<void> => {
    try {
      return await api<void>(`/shares/${encodeURIComponent(token)}/start`, {
        method: "POST",
      });
    } catch (err) {
      if (err instanceof APIError && (err.status === 404 || err.status === 429)) {
        // Map 404 and 429 to void (no error thrown). Callers see success either way.
        return;
      }
      throw err;
    }
  },
};
}

export const Shares = makeShares(api);


export interface ResourceScope {
  readonly cluster: string;
  readonly namespace?: string;
}

/** Immutable request scope for a resource, independent of list filters and storage. */
export function createRequestClient(input: ResourceScope, signal?: AbortSignal) {
  const scope = Object.freeze({ cluster: input.cluster, namespace: input.namespace });
  if (!scope.cluster) throw new Error("A resource cluster is required.");
  const url = (path: string): string => {
    if (!path.startsWith("/") || path.startsWith("//")) throw new Error("Expected an API path.");
    if (isCentralAPI(path)) return path;
    const parsed = new URL(path, "http://gameplane.invalid");
    const clusters = parsed.searchParams.getAll("cluster");
    if (clusters.length > 1 || (clusters.length === 1 && clusters[0] !== scope.cluster)) {
      throw new Error("The request cluster does not match its resource.");
    }
    const namespaced = ["/servers", "/backups", "/schedules", "/restores", "/backup-destinations", "/ws/servers"].some(
      (prefix) => parsed.pathname === prefix || parsed.pathname.startsWith(prefix + "/"),
    );
    if (namespaced && scope.namespace) {
      const namespaces = parsed.searchParams.getAll("namespace");
      if (namespaces.length > 1 || (namespaces.length === 1 && namespaces[0] !== scope.namespace)) {
        throw new Error("The request namespace does not match its resource.");
      }
      parsed.searchParams.set("namespace", scope.namespace);
    }
    if (scope.cluster !== "local") parsed.searchParams.set("cluster", scope.cluster);
    return parsed.pathname + parsed.search;
  };
  const request = <T>(path: string, options: Options = {}): Promise<T> => {
    if (!isCentralAPI(path) && options.cluster && options.cluster !== scope.cluster) {
      throw new Error("The request cluster does not match its resource.");
    }
    const requestSignal = signal && options.signal ? AbortSignal.any([signal, options.signal]) : options.signal ?? signal;
    return api<T>(url(path), { ...options, cluster: scope.cluster, signal: requestSignal });
  };
  const raw = (path: string, init: RequestInit = {}): Promise<Response> => {
    const requestSignal = signal && init.signal ? AbortSignal.any([signal, init.signal]) : init.signal ?? signal;
    return fetch(url(path), { credentials: "include", cache: "no-store", ...init, signal: requestSignal });
  };
  return { scope, api: request, url, raw, Captures: makeCaptures(request, url, raw), Shares: makeShares(request) };
}
