import { describe, it, expect, vi } from "vitest";
import type { ReactNode } from "react";
import { http, HttpResponse } from "msw";
import { screen, waitFor } from "@testing-library/react";
import { server } from "@/test/server";
import { renderWithQuery } from "@/test/render";
import { makeServer, makeBackup, makeAudit, makeClusterView, makeClusterStats, makeUser } from "@/test/factories";

// TanStack Router's Link needs a router context the test doesn't supply.
// Replace it with a plain anchor — the attention/feed links keep the same
// DOM contract for what we assert.
vi.mock("@tanstack/react-router", () => ({
  Link: ({ children, to, ...rest }: { children: ReactNode; to: string } & Record<string, unknown>) => (
    <a href={to} {...rest}>
      {children}
    </a>
  ),
}));

import { DashboardPage } from "./Dashboard";

describe("DashboardPage", () => {
  it("renders the overview shell: KPIs and section cards", async () => {
    renderWithQuery(<DashboardPage />);
    await screen.findByText("Dashboard");
    // KPI tiles ("Storage" also appears as a cluster-resources meter label,
    // so assert at least one match rather than a unique one).
    expect(screen.getByText("Running")).toBeInTheDocument();
    expect(screen.getByText("Players online")).toBeInTheDocument();
    expect(screen.getAllByText("Storage").length).toBeGreaterThan(0);
    expect(screen.getByText("Nodes ready")).toBeInTheDocument();
    // Section cards
    expect(screen.getByText("Fleet status")).toBeInTheDocument();
    expect(screen.getByText("Cluster resources")).toBeInTheDocument();
    expect(screen.getByText("Recent backups")).toBeInTheDocument();
    // Admin (default /users/me) has audit:read → activity card shown.
    expect(await screen.findByText("Recent activity")).toBeInTheDocument();
  });

  it("surfaces servers needing attention and links to their detail", async () => {
    server.use(
      http.get("/servers", () =>
        HttpResponse.json({
          items: [
            makeServer({ metadata: { name: "ok-srv" }, status: { phase: "Running" } }),
            makeServer({ metadata: { name: "broken-srv" }, status: { phase: "Failed" } }),
            makeServer({
              metadata: { name: "stale-srv" },
              status: { phase: "Running", agent: { stale: true } },
            }),
          ],
        }),
      ),
    );
    renderWithQuery(<DashboardPage />);
    await screen.findByText("Needs attention");

    expect(screen.getByText("Failed 1")).toBeInTheDocument();
    const failedLink = screen.getByRole("link", { name: /broken-srv/i });
    expect(failedLink).toHaveAttribute("href", "/servers/$name");
    expect(screen.getByText("Failed — check logs")).toBeInTheDocument();
    // Stale agent is flagged even though the phase is Running.
    expect(screen.getByRole("link", { name: /stale-srv/i })).toBeInTheDocument();
    expect(screen.getByText("Agent heartbeat stale")).toBeInTheDocument();
  });

  it("shows a healthy state when nothing needs attention", async () => {
    server.use(
      http.get("/servers", () =>
        HttpResponse.json({
          items: [makeServer({ metadata: { name: "happy" }, status: { phase: "Running" } })],
        }),
      ),
    );
    renderWithQuery(<DashboardPage />);
    expect(await screen.findByText(/Everything looks healthy/i)).toBeInTheDocument();
  });

  it("renders cluster resource meters and the node list", async () => {
    server.use(
      http.get("/cluster", () =>
        HttpResponse.json(
          makeClusterView({
            ready: 1,
            total: 1,
            nodes: [
              {
                name: "node-7",
                status: "Ready",
                cpu: { used: 4, capacity: 8 },
                memory: { used: 8_000_000_000, capacity: 16_000_000_000 },
                pods: { used: 9, capacity: 110 },
              },
            ],
          }),
        ),
      ),
      http.get("/cluster/stats", () =>
        HttpResponse.json(makeClusterStats({ usedStorageBytes: 780_000_000_000, totalStorageBytes: 1_000_000_000_000 })),
      ),
    );
    renderWithQuery(<DashboardPage />);
    await screen.findByText("Cluster resources");
    expect(screen.getByText("CPU")).toBeInTheDocument();
    expect(screen.getByText("Memory")).toBeInTheDocument();
    // "Storage" appears as both a KPI tile and a meter — both are expected.
    expect(screen.getAllByText("Storage").length).toBeGreaterThan(0);
    expect(await screen.findByText("node-7")).toBeInTheDocument();
  });

  it("lists recent backups with their phase", async () => {
    server.use(
      http.get("/backups", () =>
        HttpResponse.json({
          items: [
            makeBackup({
              metadata: { name: "valheim-snap" },
              spec: { serverRef: { name: "valheim-01" } },
              status: { phase: "Succeeded", startTime: "2026-05-07T03:00:00Z", size: "2.1 GiB" },
            }),
          ],
        }),
      ),
    );
    renderWithQuery(<DashboardPage />);
    await screen.findByText("Recent backups");
    expect(await screen.findByText("valheim-01")).toBeInTheDocument();
    expect(screen.getByText("2.1 GiB")).toBeInTheDocument();
  });

  it("humanizes audit events across action types", async () => {
    server.use(
      http.get("/admin/audit", () =>
        HttpResponse.json([
          makeAudit({ id: 1, actor: "ada", method: "POST", path: "/api/v1/servers/mc:start", target: "mc" }),
          makeAudit({ id: 2, actor: "ada", method: "POST", path: "/api/v1/servers/mc:stop", target: "mc" }),
          makeAudit({ id: 3, actor: "ada", method: "POST", path: "/api/v1/servers/mc:restart", target: "mc" }),
          makeAudit({ id: 4, actor: "ada", method: "POST", path: "/api/v1/backups", target: "mc" }),
          makeAudit({ id: 5, actor: "ada", method: "POST", path: "/api/v1/users", target: "liam" }),
          makeAudit({ id: 6, actor: "ada", method: "DELETE", path: "/api/v1/servers/old", target: "old" }),
          makeAudit({ id: 7, actor: "ada", method: "POST", path: "/api/v1/servers", target: "new" }),
          makeAudit({ id: 8, actor: "ada", method: "PUT", path: "/api/v1/servers/mc", target: "mc" }),
        ]),
      ),
    );
    renderWithQuery(<DashboardPage />);
    await screen.findByText("ada started mc");
    for (const text of [
      "ada stopped mc",
      "ada restarted mc",
      "ada backed up mc",
      "ada updated a user liam",
      "ada deleted old",
      "ada created new",
      "ada updated mc",
    ]) {
      expect(screen.getByText(text)).toBeInTheDocument();
    }
  });

  it("renders empty states for activity and backups", async () => {
    server.use(
      http.get("/admin/audit", () => HttpResponse.json([])),
      http.get("/backups", () => HttpResponse.json({ items: [] })),
    );
    renderWithQuery(<DashboardPage />);
    expect(await screen.findByText("No recent activity.")).toBeInTheDocument();
    expect(screen.getByText("No backups yet.")).toBeInTheDocument();
  });

  it("handles missing cluster data without nodes", async () => {
    server.use(
      http.get("/cluster", () => HttpResponse.json({})),
      http.get("/cluster/stats", () => HttpResponse.json({})),
    );
    renderWithQuery(<DashboardPage />);
    await screen.findByText("Cluster resources");
    // Nodes KPI tile falls back to "no node data" and the node list is omitted.
    expect(screen.getByText("no node data")).toBeInTheDocument();
  });

  it("hides recent activity from users without audit:read", async () => {
    server.use(
      http.get("/users/me", () => HttpResponse.json(makeUser({ role: "viewer" }))),
    );
    const { client } = renderWithQuery(<DashboardPage />);
    // Wait until the viewer identity has actually loaded before asserting
    // the audit-gated card is absent (otherwise we'd pass during loading).
    await waitFor(() => expect(client.getQueryData(["me"])).toBeTruthy());
    await screen.findByText("Recent backups");
    expect(screen.queryByText("Recent activity")).not.toBeInTheDocument();
    // Viewer lacks servers:write → no "View cluster" link either.
    expect(screen.queryByRole("link", { name: /view cluster/i })).not.toBeInTheDocument();
  });
});
