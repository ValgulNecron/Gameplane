import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { api, APIError } from "./api";

// Mock the cluster module to control getCurrentCluster
vi.mock("./cluster", () => ({
  getCurrentCluster: vi.fn(() => "local"),
  setCurrentCluster: vi.fn(),
  subscribeCluster: vi.fn(),
  useCurrentCluster: vi.fn(),
}));

const fetchMock = vi.fn();

beforeEach(() => {
  vi.stubGlobal("fetch", fetchMock);
  document.cookie = "";
});

afterEach(() => {
  fetchMock.mockReset();
  vi.unstubAllGlobals();
});

function jsonRes(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

describe("api()", () => {
  it("does not send a CSRF header on GET", async () => {
    fetchMock.mockResolvedValueOnce(jsonRes(200, { ok: true }));
    await api("/users/me");
    const [, init] = fetchMock.mock.calls[0];
    expect(init.method).toBe("GET");
    expect(init.headers["X-Gameplane-CSRF"]).toBeUndefined();
  });

  it("sends the CSRF cookie value as header on POST", async () => {
    document.cookie = "gameplane_csrf=tok123";
    fetchMock.mockResolvedValueOnce(jsonRes(200, { ok: true }));
    await api("/auth/login", { method: "POST", body: { u: "x" } });
    const [, init] = fetchMock.mock.calls[0];
    expect(init.method).toBe("POST");
    expect(init.headers["X-Gameplane-CSRF"]).toBe("tok123");
    expect(init.body).toBe(JSON.stringify({ u: "x" }));
  });

  it("returns parsed JSON on 2xx", async () => {
    fetchMock.mockResolvedValueOnce(jsonRes(200, { hello: "world" }));
    const r = await api<{ hello: string }>("/x");
    expect(r).toEqual({ hello: "world" });
  });

  it("returns undefined on 204", async () => {
    fetchMock.mockResolvedValueOnce(new Response(null, { status: 204 }));
    const r = await api("/x", { method: "DELETE" });
    expect(r).toBeUndefined();
  });

  it("throws APIError with body on 4xx", async () => {
    fetchMock.mockImplementation(
      async () => new Response("nope", { status: 401 }),
    );
    await expect(api("/users/me")).rejects.toMatchObject({
      status: 401,
      body: "nope",
    });
    await expect(api("/users/me")).rejects.toBeInstanceOf(APIError);
  });

  it("does not append cluster param when cluster is 'local'", async () => {
    fetchMock.mockResolvedValueOnce(jsonRes(200, { ok: true }));
    await api("/servers");
    const [url] = fetchMock.mock.calls[0];
    expect(url).toBe("/servers");
  });

  it("appends ?cluster= when cluster is non-local", async () => {
    const { getCurrentCluster } = await import("./cluster");
    vi.mocked(getCurrentCluster).mockReturnValueOnce("remote-prod");
    fetchMock.mockResolvedValueOnce(jsonRes(200, { ok: true }));
    await api("/servers");
    const [url] = fetchMock.mock.calls[0];
    expect(url).toBe("/servers?cluster=remote-prod");
  });

  it("appends &cluster= when path already has query string", async () => {
    const { getCurrentCluster } = await import("./cluster");
    vi.mocked(getCurrentCluster).mockReturnValueOnce("remote-prod");
    fetchMock.mockResolvedValueOnce(jsonRes(200, { ok: true }));
    await api("/servers?namespace=games");
    const [url] = fetchMock.mock.calls[0];
    expect(url).toBe("/servers?namespace=games&cluster=remote-prod");
  });

  it("URL-encodes the cluster id", async () => {
    const { getCurrentCluster } = await import("./cluster");
    vi.mocked(getCurrentCluster).mockReturnValueOnce("cluster@special");
    fetchMock.mockResolvedValueOnce(jsonRes(200, { ok: true }));
    await api("/servers");
    const [url] = fetchMock.mock.calls[0];
    expect(url).toBe("/servers?cluster=cluster%40special");
  });
});
