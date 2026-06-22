import { describe, it, expect } from "vitest";
import { http, HttpResponse } from "msw";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { server } from "@/test/server";
import { renderWithQuery } from "@/test/render";
import { makeClusterView } from "@/test/factories";
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
});
