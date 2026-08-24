import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { api, APIError, Captures, type CaptureStartBody } from "./api";
import { HttpResponse } from "msw";

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

describe("withNS()", () => {
  it("returns path unchanged when namespace is undefined", async () => {
    // withNS is internal to api.ts, so we test it indirectly via Captures
    // methods that use it. Here we verify a namespace-less call.
    fetchMock.mockResolvedValueOnce(jsonRes(200, { items: [] }));
    await Captures.list("alpha");
    const [url] = fetchMock.mock.calls[0];
    expect(url).toBe("/servers/alpha:captures");
  });

  it("appends ?namespace= when path has no query string", async () => {
    fetchMock.mockResolvedValueOnce(jsonRes(200, { items: [] }));
    await Captures.list("alpha", "game-ns");
    const [url] = fetchMock.mock.calls[0];
    expect(url).toBe("/servers/alpha:captures?namespace=game-ns");
  });

  it("appends &namespace= when path already has a query string", async () => {
    fetchMock.mockResolvedValueOnce(jsonRes(200, { captureId: "cap-1" }));
    await Captures.get("alpha", "cap-123", "game-ns");
    const [url] = fetchMock.mock.calls[0];
    expect(url).toBe("/servers/alpha:capture?id=cap-123&namespace=game-ns");
  });

  it("URL-encodes the namespace parameter", async () => {
    fetchMock.mockResolvedValueOnce(jsonRes(200, { items: [] }));
    await Captures.list("alpha", "ns@special/with/slashes");
    const [url] = fetchMock.mock.calls[0];
    expect(url).toBe("/servers/alpha:captures?namespace=ns%40special%2Fwith%2Fslashes");
  });
});

describe("withClusterParam()", () => {
  it("returns path unchanged when cluster is 'local'", async () => {
    const { getCurrentCluster } = await import("./cluster");
    vi.mocked(getCurrentCluster).mockReturnValue("local");
    fetchMock.mockResolvedValueOnce(new HttpResponse(new Blob(["data"]), {}));
    await Captures.download("alpha", "cap-123");
    const [url] = fetchMock.mock.calls[0];
    expect(url).toBe("/servers/alpha:capture-file?id=cap-123");
  });

  it("appends ?cluster= when cluster is non-local", async () => {
    const { getCurrentCluster } = await import("./cluster");
    vi.mocked(getCurrentCluster).mockReturnValue("remote-prod");
    fetchMock.mockResolvedValueOnce(new HttpResponse(new Blob(["data"]), {}));
    await Captures.download("alpha", "cap-123");
    const [url] = fetchMock.mock.calls[0];
    expect(url).toBe("/servers/alpha:capture-file?id=cap-123&cluster=remote-prod");
  });

  it("appends &cluster= when path already has query string", async () => {
    const { getCurrentCluster } = await import("./cluster");
    vi.mocked(getCurrentCluster).mockReturnValue("remote-prod");
    fetchMock.mockResolvedValueOnce(new HttpResponse(new Blob(["data"]), {}));
    await Captures.download("alpha", "cap-123", "game-ns");
    const [url] = fetchMock.mock.calls[0];
    expect(url).toBe(
      "/servers/alpha:capture-file?id=cap-123&namespace=game-ns&cluster=remote-prod"
    );
  });

  it("URL-encodes the cluster id", async () => {
    const { getCurrentCluster } = await import("./cluster");
    vi.mocked(getCurrentCluster).mockReturnValue("cluster@special");
    fetchMock.mockResolvedValueOnce(new HttpResponse(new Blob(["data"]), {}));
    await Captures.download("alpha", "cap-123");
    const [url] = fetchMock.mock.calls[0];
    expect(url).toBe("/servers/alpha:capture-file?id=cap-123&cluster=cluster%40special");
  });
});

describe("Captures.enable()", () => {
  it("POSTs to :capture-enable without namespace", async () => {
    fetchMock.mockResolvedValueOnce(
      jsonRes(200, {
        name: "alpha",
        status: { capture: { enabled: true } },
      })
    );
    await Captures.enable("alpha");
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe("/servers/alpha:capture-enable");
    expect(init.method).toBe("POST");
  });

  it("POSTs to :capture-enable with namespace", async () => {
    fetchMock.mockResolvedValueOnce(
      jsonRes(200, {
        name: "alpha",
        status: { capture: { enabled: true } },
      })
    );
    await Captures.enable("alpha", "game-ns");
    const [url] = fetchMock.mock.calls[0];
    expect(url).toBe("/servers/alpha:capture-enable?namespace=game-ns");
  });
});

describe("Captures.disable()", () => {
  it("POSTs to :capture-disable without namespace", async () => {
    fetchMock.mockResolvedValueOnce(
      jsonRes(200, {
        name: "alpha",
        status: { capture: { enabled: false } },
      })
    );
    await Captures.disable("alpha");
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe("/servers/alpha:capture-disable");
    expect(init.method).toBe("POST");
  });

  it("POSTs to :capture-disable with namespace", async () => {
    fetchMock.mockResolvedValueOnce(
      jsonRes(200, {
        name: "alpha",
        status: { capture: { enabled: false } },
      })
    );
    await Captures.disable("alpha", "game-ns");
    const [url] = fetchMock.mock.calls[0];
    expect(url).toBe("/servers/alpha:capture-disable?namespace=game-ns");
  });
});

describe("Captures.start()", () => {
  it("POSTs to :capture-start with body, returns 202", async () => {
    fetchMock.mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          captureId: "cap-12345",
          serverName: "alpha",
          phase: "Pending",
          startedAt: "",
          completedAt: "",
          createdAt: "2026-08-23T00:00:00Z",
          expiresAt: "2026-08-30T00:00:00Z",
          bytesWritten: 0,
          packetsWritten: 0,
          filter: "tcp port 25565",
        }),
        { status: 202, headers: { "Content-Type": "application/json" } }
      )
    );
    const body: CaptureStartBody = {
      maxDurationSeconds: 600,
      maxSizeBytes: 1048576,
      filter: "tcp port 25565",
      ttlSecondsAfterFinished: 3600,
    };
    await Captures.start("alpha", body);
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe("/servers/alpha:capture-start");
    expect(init.method).toBe("POST");
    expect(init.body).toBe(JSON.stringify(body));
  });

  it("POSTs to :capture-start with namespace", async () => {
    fetchMock.mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          captureId: "cap-12345",
          serverName: "alpha",
          phase: "Pending",
          startedAt: "",
          completedAt: "",
          createdAt: "2026-08-23T00:00:00Z",
          expiresAt: "2026-08-30T00:00:00Z",
          bytesWritten: 0,
          packetsWritten: 0,
          filter: "",
        }),
        { status: 202, headers: { "Content-Type": "application/json" } }
      )
    );
    const body: CaptureStartBody = {
      maxDurationSeconds: 300,
      maxSizeBytes: 524288,
    };
    await Captures.start("alpha", body, "game-ns");
    const [url] = fetchMock.mock.calls[0];
    expect(url).toBe("/servers/alpha:capture-start?namespace=game-ns");
  });
});

describe("Captures.stop()", () => {
  it("POSTs to :capture-stop with captureId in body", async () => {
    fetchMock.mockResolvedValueOnce(
      jsonRes(200, {
        captureId: "cap-12345",
        serverName: "alpha",
        phase: "Completed",
        startedAt: "2026-08-23T00:00:00Z",
        completedAt: "2026-08-23T01:00:00Z",
        createdAt: "2026-08-23T00:00:00Z",
        expiresAt: "2026-08-30T00:00:00Z",
        bytesWritten: 1024,
        packetsWritten: 100,
        filter: "tcp port 25565",
      })
    );
    await Captures.stop("alpha", "cap-12345");
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe("/servers/alpha:capture-stop");
    expect(init.method).toBe("POST");
    expect(init.body).toBe(JSON.stringify({ captureId: "cap-12345" }));
  });

  it("POSTs to :capture-stop with namespace", async () => {
    fetchMock.mockResolvedValueOnce(
      jsonRes(200, {
        captureId: "cap-12345",
        serverName: "alpha",
        phase: "Completed",
        startedAt: "2026-08-23T00:00:00Z",
        completedAt: "2026-08-23T01:00:00Z",
        createdAt: "2026-08-23T00:00:00Z",
        expiresAt: "2026-08-30T00:00:00Z",
        bytesWritten: 1024,
        packetsWritten: 100,
        filter: "tcp port 25565",
      })
    );
    await Captures.stop("alpha", "cap-12345", "game-ns");
    const [url] = fetchMock.mock.calls[0];
    expect(url).toBe("/servers/alpha:capture-stop?namespace=game-ns");
  });
});

describe("Captures.list()", () => {
  it("GETs from :captures without namespace", async () => {
    fetchMock.mockResolvedValueOnce(
      jsonRes(200, {
        captures: [],
        total: 0,
        limit: 100,
        offset: 0,
      })
    );
    await Captures.list("alpha");
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe("/servers/alpha:captures");
    expect(init.method).toBe("GET");
  });

  it("GETs from :captures with namespace", async () => {
    fetchMock.mockResolvedValueOnce(
      jsonRes(200, {
        captures: [
          {
            captureId: "cap-1",
            serverName: "alpha",
            phase: "Completed",
            startedAt: "2026-08-23T00:00:00Z",
            completedAt: "2026-08-23T01:00:00Z",
            createdAt: "2026-08-23T00:00:00Z",
            expiresAt: "2026-08-30T00:00:00Z",
            bytesWritten: 1024,
            packetsWritten: 100,
            filter: "",
          },
        ],
        total: 1,
        limit: 100,
        offset: 0,
      })
    );
    await Captures.list("alpha", "game-ns");
    const [url] = fetchMock.mock.calls[0];
    expect(url).toBe("/servers/alpha:captures?namespace=game-ns");
  });
});

describe("Captures.get()", () => {
  it("GETs from :capture?id= without namespace", async () => {
    fetchMock.mockResolvedValueOnce(
      jsonRes(200, {
        captureId: "cap-12345",
        serverName: "alpha",
        phase: "Completed",
        startedAt: "2026-08-23T00:00:00Z",
        completedAt: "2026-08-23T01:00:00Z",
        createdAt: "2026-08-23T00:00:00Z",
        expiresAt: "2026-08-30T00:00:00Z",
        bytesWritten: 1024,
        packetsWritten: 100,
        filter: "tcp port 25565",
      })
    );
    await Captures.get("alpha", "cap-12345");
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe("/servers/alpha:capture?id=cap-12345");
    expect(init.method).toBe("GET");
  });

  it("GETs from :capture?id= with namespace", async () => {
    fetchMock.mockResolvedValueOnce(
      jsonRes(200, {
        captureId: "cap-12345",
        serverName: "alpha",
        phase: "Completed",
        startedAt: "2026-08-23T00:00:00Z",
        completedAt: "2026-08-23T01:00:00Z",
        createdAt: "2026-08-23T00:00:00Z",
        expiresAt: "2026-08-30T00:00:00Z",
        bytesWritten: 1024,
        packetsWritten: 100,
        filter: "tcp port 25565",
      })
    );
    await Captures.get("alpha", "cap-12345", "game-ns");
    const [url] = fetchMock.mock.calls[0];
    expect(url).toBe("/servers/alpha:capture?id=cap-12345&namespace=game-ns");
  });

  it("URL-encodes the capture id", async () => {
    fetchMock.mockResolvedValueOnce(
      jsonRes(200, {
        captureId: "cap@special/id",
        serverName: "alpha",
        phase: "Completed",
        startedAt: "2026-08-23T00:00:00Z",
        completedAt: "2026-08-23T01:00:00Z",
        createdAt: "2026-08-23T00:00:00Z",
        expiresAt: "2026-08-30T00:00:00Z",
        bytesWritten: 1024,
        packetsWritten: 100,
        filter: "tcp port 25565",
      })
    );
    await Captures.get("alpha", "cap@special/id");
    const [url] = fetchMock.mock.calls[0];
    expect(url).toBe("/servers/alpha:capture?id=cap%40special%2Fid");
  });
});

describe("Captures.remove()", () => {
  it("DELETEs from :capture?id= without namespace", async () => {
    fetchMock.mockResolvedValueOnce(
      jsonRes(200, {
        deleted: true,
        captureId: "cap-12345",
      })
    );
    await Captures.remove("alpha", "cap-12345");
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe("/servers/alpha:capture?id=cap-12345");
    expect(init.method).toBe("DELETE");
  });

  it("DELETEs from :capture?id= with namespace", async () => {
    fetchMock.mockResolvedValueOnce(
      jsonRes(200, {
        deleted: true,
        captureId: "cap-12345",
      })
    );
    await Captures.remove("alpha", "cap-12345", "game-ns");
    const [url] = fetchMock.mock.calls[0];
    expect(url).toBe("/servers/alpha:capture?id=cap-12345&namespace=game-ns");
  });

  it("URL-encodes the capture id", async () => {
    fetchMock.mockResolvedValueOnce(
      jsonRes(200, {
        deleted: true,
        captureId: "cap@special/id",
      })
    );
    await Captures.remove("alpha", "cap@special/id");
    const [url] = fetchMock.mock.calls[0];
    expect(url).toBe("/servers/alpha:capture?id=cap%40special%2Fid");
  });
});

describe("Captures.download()", () => {
  it("GETs from :capture-file?id= and returns Blob without namespace", async () => {
    const blobData = new Blob(["pcapng mock data"], {
      type: "application/octet-stream",
    });
    fetchMock.mockResolvedValueOnce(
      new Response(blobData, {
        status: 200,
        headers: { "Content-Type": "application/octet-stream" },
      })
    );
    const result = await Captures.download("alpha", "cap-12345");
    expect(result).toBeInstanceOf(Blob);
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe("/servers/alpha:capture-file?id=cap-12345");
    expect(init.method).toBe("GET");
  });

  it("GETs from :capture-file?id= with namespace and returns Blob", async () => {
    const blobData = new Blob(["pcapng mock data"], {
      type: "application/octet-stream",
    });
    fetchMock.mockResolvedValueOnce(
      new Response(blobData, {
        status: 200,
        headers: { "Content-Type": "application/octet-stream" },
      })
    );
    const result = await Captures.download("alpha", "cap-12345", "game-ns");
    expect(result).toBeInstanceOf(Blob);
    const [url] = fetchMock.mock.calls[0];
    expect(url).toBe("/servers/alpha:capture-file?id=cap-12345&namespace=game-ns");
  });

  it("includes cluster param on non-local cluster", async () => {
    const { getCurrentCluster } = await import("./cluster");
    vi.mocked(getCurrentCluster).mockReturnValue("remote-prod");
    const blobData = new Blob(["pcapng mock data"], {
      type: "application/octet-stream",
    });
    fetchMock.mockResolvedValueOnce(
      new Response(blobData, {
        status: 200,
        headers: { "Content-Type": "application/octet-stream" },
      })
    );
    await Captures.download("alpha", "cap-12345");
    const [url] = fetchMock.mock.calls[0];
    expect(url).toBe("/servers/alpha:capture-file?id=cap-12345&cluster=remote-prod");
  });

  it("throws APIError on non-ok response with error body", async () => {
    fetchMock.mockResolvedValueOnce(
      new Response("File not found", { status: 404 })
    );
    await expect(Captures.download("alpha", "cap-12345")).rejects.toMatchObject({
      status: 404,
      body: "File not found",
    });
    await expect(Captures.download("alpha", "cap-12345")).rejects.toBeInstanceOf(APIError);
  });

  it("throws APIError on non-ok response with empty body", async () => {
    fetchMock.mockResolvedValueOnce(new Response("", { status: 500 }));
    await expect(Captures.download("alpha", "cap-12345")).rejects.toMatchObject({
      status: 500,
      body: "",
    });
    await expect(Captures.download("alpha", "cap-12345")).rejects.toBeInstanceOf(APIError);
  });

  it("throws APIError when response.text() fails but catches gracefully", async () => {
    const failingResponse = new Response("error", { status: 500 });
    vi.spyOn(failingResponse, "text").mockRejectedValueOnce(new Error("text failed"));
    fetchMock.mockResolvedValueOnce(failingResponse);
    await expect(Captures.download("alpha", "cap-12345")).rejects.toMatchObject({
      status: 500,
      body: "",
    });
  });

  it("URL-encodes the capture id in download URL", async () => {
    const blobData = new Blob(["pcapng mock data"], {
      type: "application/octet-stream",
    });
    fetchMock.mockResolvedValueOnce(
      new Response(blobData, {
        status: 200,
        headers: { "Content-Type": "application/octet-stream" },
      })
    );
    await Captures.download("alpha", "cap@special/id");
    const [url] = fetchMock.mock.calls[0];
    expect(url).toBe("/servers/alpha:capture-file?id=cap%40special%2Fid");
  });
});
