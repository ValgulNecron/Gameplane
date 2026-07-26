import { describe, it, expect, vi } from "vitest";
import { http, HttpResponse } from "msw";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { server } from "@/test/server";
import { renderWithQuery } from "@/test/render";
import { makeClusterInfo, makeClusterView } from "@/test/factories";
import { ClusterPage } from "./Cluster";

describe("ClusterPage", () => {
  it("renders node cards from /cluster", async () => {
    renderWithQuery(<ClusterPage />);
    await screen.findByText("node-1");
    expect(screen.getByText(/gameplane-prod/)).toBeInTheDocument();
    expect(screen.getByText(/v1\.31\.0/)).toBeInTheDocument();
  });

  it("shows empty state when no nodes", async () => {
    server.use(
      http.get("/cluster", () => HttpResponse.json(makeClusterView({ nodes: [] }))),
    );
    renderWithQuery(<ClusterPage />);
    await waitFor(() => {
      expect(screen.getByText(/No node data yet/)).toBeInTheDocument();
    });
  });

  // Regression: a node whose `used` is absent (no metrics-server) used to
  // render as a 0% bar, indistinguishable from a genuinely idle node.
  it("shows unknown (—) for CPU/memory when a node has no `used` reading", async () => {
    server.use(
      http.get("/cluster", () =>
        HttpResponse.json(
          makeClusterView({
            nodes: [
              {
                name: "no-metrics-node",
                status: "Ready",
                cpu: { capacity: 8 },
                memory: { capacity: 16_000_000_000 },
              },
            ],
          }),
        ),
      ),
    );
    renderWithQuery(<ClusterPage />);
    await screen.findByText("no-metrics-node");
    // Two meters (CPU, Memory) both read unknown.
    expect(screen.getAllByText("—").length).toBeGreaterThanOrEqual(2);
    expect(screen.queryByText("0%")).not.toBeInTheDocument();
  });

  it("falls back gracefully on API error", async () => {
    server.use(http.get("/cluster", () => HttpResponse.error()));
    renderWithQuery(<ClusterPage />);
    // The query handler swallows the error to {} so the page renders
    // its empty-state subtitle.
    await waitFor(() => {
      expect(screen.getByText(/No node data yet/)).toBeInTheDocument();
    });
  });

  it("Add node shows the join command on success", async () => {
    server.use(
      http.post("/cluster/nodes:join", () =>
        HttpResponse.json({
          command: "kubeadm join api.test:6443 --token abcdef.0123456789abcdef --discovery-token-ca-cert-hash sha256:deadbeef",
          token: "abcdef.0123456789abcdef",
          caCertHash: "sha256:deadbeef",
          endpoint: "api.test:6443",
          expiresAt: "2026-06-15T00:00:00Z",
        }),
      ),
    );
    renderWithQuery(<ClusterPage />);
    await userEvent.click(await screen.findByRole("button", { name: /add node/i }));
    expect(await screen.findByText(/kubeadm join api\.test:6443/)).toBeInTheDocument();
  });

  it("Add node surfaces a clear message when clusterOps is disabled (501)", async () => {
    server.use(
      http.post("/cluster/nodes:join", () => new HttpResponse("not enabled", { status: 501 })),
    );
    renderWithQuery(<ClusterPage />);
    await userEvent.click(await screen.findByRole("button", { name: /add node/i }));
    expect(await screen.findByText(/aren't enabled/i)).toBeInTheDocument();
  });

  it("disables both ops buttons with a hint when /cluster/info reports clusterOps off", async () => {
    server.use(
      http.get("/cluster/info", () =>
        HttpResponse.json(makeClusterInfo({ clusterOps: false })),
      ),
    );
    renderWithQuery(<ClusterPage />);
    await waitFor(() => {
      expect(screen.getByRole("button", { name: /add node/i })).toBeDisabled();
    });
    expect(screen.getByRole("button", { name: /download kubeconfig/i })).toBeDisabled();
    expect(screen.getByText(/clusterOps\.enabled/)).toBeInTheDocument();
  });

  it("keeps the ops buttons active when clusterOps is on", async () => {
    renderWithQuery(<ClusterPage />);
    // Default handler reports clusterOps: true — no hint, buttons live.
    await screen.findByText("node-1");
    expect(screen.getByRole("button", { name: /add node/i })).toBeEnabled();
    expect(screen.getByRole("button", { name: /download kubeconfig/i })).toBeEnabled();
    expect(screen.queryByText(/clusterOps\.enabled/)).not.toBeInTheDocument();
  });

  it("keeps the ops buttons active when the info fetch fails (older API)", async () => {
    server.use(http.get("/cluster/info", () => HttpResponse.error()));
    renderWithQuery(<ClusterPage />);
    await screen.findByText("node-1");
    expect(screen.getByRole("button", { name: /add node/i })).toBeEnabled();
    expect(screen.getByRole("button", { name: /download kubeconfig/i })).toBeEnabled();
  });

  it("shows error when Add node fails with 403", async () => {
    server.use(
      http.post("/cluster/nodes:join", () =>
        new HttpResponse("forbidden", { status: 403 })
      ),
    );
    renderWithQuery(<ClusterPage />);
    await userEvent.click(await screen.findByRole("button", { name: /add node/i }));
    expect(await screen.findByText(/don't have permission/i)).toBeInTheDocument();
  });

  it("shows generic error when Add node fails with 500", async () => {
    server.use(
      http.post("/cluster/nodes:join", () =>
        new HttpResponse("server error", { status: 500 })
      ),
    );
    renderWithQuery(<ClusterPage />);
    await userEvent.click(await screen.findByRole("button", { name: /add node/i }));
    expect(await screen.findByText(/operation failed/i)).toBeInTheDocument();
  });

  it("downloads kubeconfig blob", async () => {
    const createObjectURLMock = vi.fn(() => "blob:mock-url");
    const revokeObjectURLMock = vi.fn();
    const clickMock = vi.fn();

    globalThis.URL.createObjectURL = createObjectURLMock;
    globalThis.URL.revokeObjectURL = revokeObjectURLMock;

    server.use(
      http.post("/cluster/kubeconfig", () =>
        // A raw `Blob` body throws inside msw's Response construction on this
        // environment's jsdom (its Blob lacks `.stream()`, which msw's
        // Response proxy calls while building the mocked Response) — use a
        // string body instead. The client's `res.blob()` still round-trips it
        // into a real Blob regardless of the original body shape (same fix
        // as AuditLog.test.tsx's CSV export test).
        new HttpResponse("kubeconfig content", { headers: { "Content-Type": "text/plain" } })
      ),
    );

    renderWithQuery(<ClusterPage />);
    const downloadBtn = await screen.findByRole("button", { name: /download kubeconfig/i });

    // Mock createElement and click
    const originalCreateElement = document.createElement.bind(document);
    document.createElement = vi.fn((tag) => {
      if (tag === "a") {
        const el = originalCreateElement("a");
        el.click = clickMock;
        return el;
      }
      return originalCreateElement(tag);
    });

    await userEvent.click(downloadBtn);
    await waitFor(() => {
      expect(clickMock).toHaveBeenCalled();
    });

    // Cleanup
    document.createElement = originalCreateElement;
  });

  it("shows error when kubeconfig download fails", async () => {
    server.use(
      http.post("/cluster/kubeconfig", () =>
        new HttpResponse("forbidden", { status: 403 })
      ),
    );
    renderWithQuery(<ClusterPage />);
    const downloadBtn = await screen.findByRole("button", { name: /download kubeconfig/i });
    await userEvent.click(downloadBtn);
    expect(await screen.findByText(/don't have permission/i, {}, { timeout: 10000 })).toBeInTheDocument();
  });

  it("shows generic error when kubeconfig download fails with 500", async () => {
    server.use(
      http.get("/cluster/kubeconfig", () =>
        new HttpResponse("internal error", { status: 500 })
      ),
    );
    renderWithQuery(<ClusterPage />);
    const downloadBtn = await screen.findByRole("button", { name: /download kubeconfig/i });
    await userEvent.click(downloadBtn);
    expect(await screen.findByText(/operation failed/i)).toBeInTheDocument();
  });

  it("dismisses join info card", async () => {
    server.use(
      http.post("/cluster/nodes:join", () =>
        HttpResponse.json({
          command: "kubeadm join api.test:6443 --token abcdef.0123456789abcdef --discovery-token-ca-cert-hash sha256:deadbeef",
          token: "abcdef.0123456789abcdef",
          caCertHash: "sha256:deadbeef",
          endpoint: "api.test:6443",
          expiresAt: "2026-06-15T00:00:00Z",
        }),
      ),
    );
    renderWithQuery(<ClusterPage />);
    await userEvent.click(await screen.findByRole("button", { name: /add node/i }));
    expect(await screen.findByText(/kubeadm join/)).toBeInTheDocument();
    const dismissBtn = screen.getByRole("button", { name: /dismiss/i });
    await userEvent.click(dismissBtn);
    await waitFor(() => {
      expect(screen.queryByText(/kubeadm join/)).not.toBeInTheDocument();
    });
  });

  it("copies join command to clipboard", async () => {
    const clipboardMock = vi.fn();
    Object.assign(navigator, {
      clipboard: {
        writeText: clipboardMock,
      },
    });

    server.use(
      http.post("/cluster/nodes:join", () =>
        HttpResponse.json({
          command: "kubeadm join api.test:6443 --token abcdef.0123456789abcdef --discovery-token-ca-cert-hash sha256:deadbeef",
          token: "abcdef.0123456789abcdef",
          caCertHash: "sha256:deadbeef",
          endpoint: "api.test:6443",
          expiresAt: "2026-06-15T00:00:00Z",
        }),
      ),
    );
    renderWithQuery(<ClusterPage />);
    await userEvent.click(await screen.findByRole("button", { name: /add node/i }));
    const copyBtn = await screen.findByRole("button", { name: /copy/i });
    await userEvent.click(copyBtn);
    await waitFor(() => {
      expect(clipboardMock).toHaveBeenCalledWith(expect.stringContaining("kubeadm join"));
    });
  });

  it("renders node with roles and labels", async () => {
    server.use(
      http.get("/cluster", () =>
        HttpResponse.json(
          makeClusterView({
            nodes: [
              {
                name: "master-1",
                status: "Ready",
                roles: ["master", "control-plane"],
                labels: ["node-type=control", "zone=us-east-1"],
                cpu: { used: 2, capacity: 8 },
                memory: { used: 4_000_000_000, capacity: 16_000_000_000 },
                uptime: "45d 3h 22m",
              },
            ],
          }),
        ),
      ),
    );
    renderWithQuery(<ClusterPage />);
    await screen.findByText("master-1");
    expect(screen.getByText(/master.*control-plane/)).toBeInTheDocument();
    expect(screen.getByText(/node-type=control/)).toBeInTheDocument();
    expect(screen.getByText(/zone=us-east-1/)).toBeInTheDocument();
  });

  it("renders node without labels", async () => {
    server.use(
      http.get("/cluster", () =>
        HttpResponse.json(
          makeClusterView({
            nodes: [
              {
                name: "worker-1",
                status: "Ready",
                cpu: { used: 1, capacity: 4 },
                memory: { used: 2_000_000_000, capacity: 8_000_000_000 },
              },
            ],
          }),
        ),
      ),
    );
    renderWithQuery(<ClusterPage />);
    await screen.findByText("worker-1");
    // Scope query to the node card to avoid matching the "worker" role badge.
    // The card's own wrapper carries "space-y-4" (see NodeCard in Cluster.tsx)
    // — "space-y-2" only wraps the inner meters block, which doesn't contain
    // the node name, so `.closest(".space-y-2")` never matched.
    const nodeCard = screen.getByText("worker-1").closest(".space-y-4");
    expect(nodeCard?.textContent).toMatch(/worker/i);
  });

  it("renders node with missing status as Unknown", async () => {
    server.use(
      http.get("/cluster", () =>
        HttpResponse.json(
          makeClusterView({
            nodes: [
              {
                name: "unknown-node",
                status: undefined,
                cpu: { capacity: 8 },
                memory: { capacity: 16_000_000_000 },
              },
            ],
          }),
        ),
      ),
    );
    renderWithQuery(<ClusterPage />);
    await screen.findByText("unknown-node");
    expect(screen.getByText(/Unknown/)).toBeInTheDocument();
  });

  it("renders node with pending/NotReady status", async () => {
    server.use(
      http.get("/cluster", () =>
        HttpResponse.json(
          makeClusterView({
            nodes: [
              {
                name: "pending-node",
                status: "NotReady",
                cpu: { used: 0, capacity: 4 },
                memory: { used: 512_000_000, capacity: 8_000_000_000 },
              },
            ],
          }),
        ),
      ),
    );
    renderWithQuery(<ClusterPage />);
    await screen.findByText("pending-node");
    const badge = screen.getByText(/NotReady/);
    expect(badge.className).toContain("danger");
  });

  it("renders multiple nodes in grid", async () => {
    server.use(
      http.get("/cluster", () =>
        HttpResponse.json(
          makeClusterView({
            nodes: [
              {
                name: "node-1",
                status: "Ready",
                cpu: { used: 1, capacity: 4 },
                memory: { used: 2_000_000_000, capacity: 8_000_000_000 },
              },
              {
                name: "node-2",
                status: "Ready",
                cpu: { used: 2, capacity: 8 },
                memory: { used: 4_000_000_000, capacity: 16_000_000_000 },
              },
            ],
          }),
        ),
      ),
    );
    renderWithQuery(<ClusterPage />);
    await screen.findByText("node-1");
    expect(screen.getByText("node-2")).toBeInTheDocument();
  });

  it("renders subtitle with cluster info when available", async () => {
    server.use(
      http.get("/cluster", () =>
        HttpResponse.json(
          makeClusterView({
            name: "production",
            version: "v1.31.0",
            ready: 3,
            total: 3,
            nodes: [
              { name: "node-1", status: "Ready", cpu: { capacity: 4 }, memory: { capacity: 8_000_000_000 } },
              { name: "node-2", status: "Ready", cpu: { capacity: 4 }, memory: { capacity: 8_000_000_000 } },
              { name: "node-3", status: "Ready", cpu: { capacity: 4 }, memory: { capacity: 8_000_000_000 } },
            ],
          }),
        ),
      ),
    );
    renderWithQuery(<ClusterPage />);
    await waitFor(() => {
      // The subtitle is one interpolated string ("production · v1.31.0 ·
      // 3/3 nodes healthy"), so it renders as a single text node — an exact
      // string match against just "production" can never succeed. Use a
      // partial regex, like the other substring assertions in this file.
      expect(screen.getByText(/production/)).toBeInTheDocument();
    });
    expect(screen.getByText(/3\/3 nodes healthy/)).toBeInTheDocument();
  });

  it("clears error when operations succeed", async () => {
    let callCount = 0;
    server.use(
      http.post("/cluster/nodes:join", () => {
        callCount++;
        if (callCount === 1) {
          return new HttpResponse("error", { status: 500 });
        }
        return HttpResponse.json({
          command: "kubeadm join ...",
          token: "abc",
          caCertHash: "def",
          endpoint: "api.test",
          expiresAt: "2026-06-15T00:00:00Z",
        });
      }),
    );
    renderWithQuery(<ClusterPage />);
    const addBtn = await screen.findByRole("button", { name: /add node/i });

    // First attempt fails
    await userEvent.click(addBtn);
    expect(await screen.findByText(/operation failed/i)).toBeInTheDocument();

    // Second attempt succeeds
    await userEvent.click(addBtn);
    await waitFor(() => {
      expect(screen.queryByText(/operation failed/i)).not.toBeInTheDocument();
      expect(screen.getByText(/kubeadm join/)).toBeInTheDocument();
    });
  });
});
