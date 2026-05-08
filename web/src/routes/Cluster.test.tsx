import { describe, it, expect } from "vitest";
import { http, HttpResponse } from "msw";
import { screen, waitFor } from "@testing-library/react";
import { server } from "@/test/server";
import { renderWithQuery } from "@/test/render";
import { makeClusterView } from "@/test/factories";
import { ClusterPage } from "./Cluster";

describe("ClusterPage", () => {
  it("renders node cards from /cluster", async () => {
    renderWithQuery(<ClusterPage />);
    await screen.findByText("node-1");
    expect(screen.getByText(/kestrel-prod/)).toBeInTheDocument();
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
});
