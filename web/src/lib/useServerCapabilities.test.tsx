import type { ReactNode } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, onTestFinished, vi } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "@/test/server";
import { resourceKey, type ResourceTarget } from "./resourceTarget";
import { useServerCapabilities } from "./useServerCapabilities";

const remote: ResourceTarget = { cluster: "remote", namespace: "games", name: "same", uid: "remote-uid" };
const ready = { enabled: true, files: true, state: "ready" as const,
  defaultRetentionSeconds: 86400, maxRetentionSeconds: 604800,
  defaultMaxDurationSeconds: 300, defaultMaxSizeBytes: 5 * 1024 * 1024 * 1024 };

function renderCapabilities(target = remote, enabled = true) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: Infinity, staleTime: Infinity } } });
  onTestFinished(() => client.clear());
  const wrapper = ({ children }: { children: ReactNode }) => <QueryClientProvider client={client}>{children}</QueryClientProvider>;
  return {
    client,
    ...renderHook(({ target: selected, enabled: allowed }) => useServerCapabilities(selected, allowed), {
      wrapper, initialProps: { target, enabled },
    }),
  };
}

afterEach(() => server.resetHandlers());

describe("useServerCapabilities", () => {
  it("waits for capabilities from the exact remote server and namespace", async () => {
    let release!: () => void;
    const gate = new Promise<void>((resolve) => { release = resolve; });
    const requests: URL[] = [];
    server.use(http.get("/servers/:name/capabilities", async ({ request }) => {
      requests.push(new URL(request.url));
      await gate;
      return HttpResponse.json({ target: remote, capture: ready });
    }));
    const { result } = renderCapabilities();
    try {
      await waitFor(() => expect(requests).toHaveLength(1));
      expect(result.current.isPending).toBe(true);
      expect(result.current.data).toBeUndefined();
      expect(requests[0].pathname).toBe("/servers/same/capabilities");
      expect(requests[0].searchParams.get("cluster")).toBe("remote");
      expect(requests[0].searchParams.get("namespace")).toBe("games");
    } finally {
      release();
    }
    await waitFor(() => expect(result.current.data).toEqual(ready));
  });

  it.each([
    { cluster: "local" },
    { namespace: "another-namespace" },
    { name: "another-server" },
    { uid: "replacement-uid" },
    { uid: undefined },
  ])("refuses capabilities for a mismatched identity: %j", async (mismatch) => {
    server.use(http.get("/servers/:name/capabilities", () => HttpResponse.json({
      target: { ...remote, ...mismatch }, capture: ready,
    })));
    const { result } = renderCapabilities();
    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.error?.message).toMatch(/server identity changed/i);
    expect(result.current.data).toBeUndefined();
  });

  it("does not fetch when capture permission is absent", async () => {
    const read = vi.fn(() => HttpResponse.json({ target: remote, capture: ready }));
    server.use(http.get("/servers/:name/capabilities", read));
    const { result } = renderCapabilities(remote, false);
    await act(async () => { await Promise.resolve(); });
    expect(result.current.fetchStatus).toBe("idle");
    expect(result.current.data).toBeUndefined();
    expect(read).not.toHaveBeenCalled();
  });

  it("surfaces a failed capability request without retrying", async () => {
    const read = vi.fn(() => new HttpResponse("unavailable", { status: 503 }));
    server.use(http.get("/servers/:name/capabilities", read));
    const { result } = renderCapabilities();
    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.data).toBeUndefined();
    expect(read).toHaveBeenCalledOnce();
  });

  it.each([
    { enabled: false, files: false, state: "unsupported" },
    { enabled: false, files: false, state: "unavailable" },
    { enabled: false, files: true, state: "ready" },
  ] as const)("preserves the declared capture support state %j", async (capture) => {
    server.use(http.get("/servers/:name/capabilities", () => HttpResponse.json({ target: remote, capture: { ...ready, ...capture } })));
    const { result } = renderCapabilities();
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toEqual({ ...ready, ...capture });
  });

  it.each([
    { defaultRetentionSeconds: undefined },
    { defaultRetentionSeconds: 0 },
    { defaultRetentionSeconds: 59 },
    { maxRetentionSeconds: 3600 },
    { defaultMaxDurationSeconds: 3601 },
    { defaultMaxSizeBytes: -1 },
    { defaultMaxSizeBytes: Number.MAX_SAFE_INTEGER + 1 },
  ])("does not substitute home defaults for invalid ready site limits: %j", async (invalid) => {
    server.use(http.get("/servers/:name/capabilities", () => HttpResponse.json({ target: remote, capture: { ...ready, ...invalid } })));
    const { result } = renderCapabilities();
    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.error?.message).toMatch(/capture limits are unavailable/i);
    expect(result.current.data).toBeUndefined();
  });

  it("isolates same-named local, remote and replacement server cache entries", async () => {
    const local: ResourceTarget = { ...remote, cluster: "local", uid: "local-uid" };
    const replacement: ResourceTarget = { ...remote, uid: "replacement-uid" };
    const disabled = { enabled: false, files: false, state: "unsupported" as const };
    let currentRemote = remote;
    const requests: string[] = [];
    server.use(http.get("/servers/:name/capabilities", ({ request }) => {
      const url = new URL(request.url);
      const selected = url.searchParams.get("cluster") === "remote" ? currentRemote : local;
      requests.push(selected.uid!);
      return HttpResponse.json({ target: selected, capture: selected === remote ? ready : disabled });
    }));
    const { result, rerender, client } = renderCapabilities();
    await waitFor(() => expect(result.current.data).toEqual(ready));
    rerender({ target: local, enabled: true });
    await waitFor(() => expect(result.current.data).toEqual(disabled));
    currentRemote = replacement;
    rerender({ target: replacement, enabled: true });
    await waitFor(() => expect(requests).toHaveLength(3));
    await waitFor(() => expect(result.current.data).toEqual(disabled));
    expect(client.getQueryData(resourceKey(remote, "capabilities"))).toEqual(ready);
    expect(client.getQueryData(resourceKey(local, "capabilities"))).toEqual(disabled);
    expect(client.getQueryData(resourceKey(replacement, "capabilities"))).toEqual(disabled);
    expect(requests).toEqual(["remote-uid", "local-uid", "replacement-uid"]);
  });
});
