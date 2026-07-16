import { describe, it, expect } from "vitest";
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
});
