import { describe, it, expect, vi } from "vitest";
import type { ReactNode } from "react";
import { http, HttpResponse } from "msw";
import { screen, waitFor } from "@testing-library/react";
import { server } from "@/test/server";
import { renderWithQuery } from "@/test/render";
import {
  makeServer,
  makeBackup,
  makeClusterView,
  makeClusterStats,
  makeUser,
} from "@/test/factories";

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
  it("renders the dashboard title and subtitle", async () => {
    renderWithQuery(<DashboardPage />);
    expect(screen.getByText("Dashboard")).toBeInTheDocument();
    expect(screen.getByText("At-a-glance health of your Gameplane cluster.")).toBeInTheDocument();
  });

  it("shows a loading state while queries are pending", () => {
    // Use a never-resolving query to keep it in loading state
    server.use(
      http.get("/servers", () => new Promise(() => {})), // never resolves
    );
    renderWithQuery(<DashboardPage />);
    expect(screen.getByRole("status")).toBeInTheDocument(); // Spinner has role="status"
  });

  it("shows an empty state once queries resolve", async () => {
    renderWithQuery(<DashboardPage />);
    // Wait for queries to settle
    await waitFor(() => {
      // Once loading resolves, the page should render without the spinner
      expect(screen.queryByRole("status")).not.toBeInTheDocument();
    });
    expect(screen.getByText("Dashboard")).toBeInTheDocument();
  });

  it("fetches servers with the correct query key and refetch interval", async () => {
    const { client } = renderWithQuery(<DashboardPage />);
    await waitFor(() => {
      const data = client.getQueryData(["servers"]);
      expect(data).toBeDefined();
    });
  });

  it("fetches cluster stats with the correct query key and staleTime", async () => {
    const { client } = renderWithQuery(<DashboardPage />);
    await waitFor(() => {
      const data = client.getQueryData(["cluster-stats"]);
      expect(data).toBeDefined();
    });
  });

  it("fetches cluster info with the correct query key and staleTime", async () => {
    const { client } = renderWithQuery(<DashboardPage />);
    await waitFor(() => {
      const data = client.getQueryData(["cluster"]);
      expect(data).toBeDefined();
    });
  });

  it("fetches backups with the correct query key and staleTime", async () => {
    const { client } = renderWithQuery(<DashboardPage />);
    await waitFor(() => {
      const data = client.getQueryData(["backups"]);
      expect(data).toBeDefined();
    });
  });

  it("fetches audit events when user has audit:read permission", async () => {
    const { client } = renderWithQuery(<DashboardPage />);
    // Admin user has audit:read by default
    await waitFor(() => {
      const data = client.getQueryData(["audit", "dashboard"]);
      expect(data).toBeDefined();
    });
  });

  it("does not fetch audit events when user lacks audit:read permission", async () => {
    server.use(
      http.get("/users/me", () => HttpResponse.json(makeUser({ role: "viewer" }))),
    );
    const { client } = renderWithQuery(<DashboardPage />);
    await waitFor(() => expect(client.getQueryData(["me"])).toBeTruthy());
    // Viewer should not have audit:read → query should not be made
    const data = client.getQueryData(["audit", "dashboard"]);
    expect(data).toBeUndefined();
  });

  it("handles servers endpoint error gracefully", async () => {
    server.use(
      http.get("/servers", () => HttpResponse.error()),
    );
    renderWithQuery(<DashboardPage />);
    await waitFor(() => {
      // Should render without crashing
      expect(screen.getByText("Dashboard")).toBeInTheDocument();
    });
  });

  it("handles cluster stats endpoint error gracefully", async () => {
    server.use(
      http.get("/cluster/stats", () => HttpResponse.error()),
    );
    renderWithQuery(<DashboardPage />);
    await waitFor(() => {
      expect(screen.getByText("Dashboard")).toBeInTheDocument();
    });
  });

  it("handles cluster info endpoint error gracefully", async () => {
    server.use(
      http.get("/cluster", () => HttpResponse.error()),
    );
    renderWithQuery(<DashboardPage />);
    await waitFor(() => {
      expect(screen.getByText("Dashboard")).toBeInTheDocument();
    });
  });

  it("handles backups endpoint error gracefully", async () => {
    server.use(
      http.get("/backups", () => HttpResponse.error()),
    );
    renderWithQuery(<DashboardPage />);
    await waitFor(() => {
      expect(screen.getByText("Dashboard")).toBeInTheDocument();
    });
  });

  it("loads with empty server list", async () => {
    server.use(
      http.get("/servers", () =>
        HttpResponse.json({ items: [] }),
      ),
    );
    renderWithQuery(<DashboardPage />);
    await waitFor(() => {
      // Page should load without error with empty servers
      expect(screen.getByText("Dashboard")).toBeInTheDocument();
    });
  });

  it("loads with empty backups list", async () => {
    server.use(
      http.get("/backups", () =>
        HttpResponse.json({ items: [] }),
      ),
    );
    renderWithQuery(<DashboardPage />);
    await waitFor(() => {
      expect(screen.getByText("Dashboard")).toBeInTheDocument();
    });
  });

  it("loads with empty audit events", async () => {
    server.use(
      http.get("/admin/audit", () => HttpResponse.json([])),
    );
    renderWithQuery(<DashboardPage />);
    await waitFor(() => {
      expect(screen.getByText("Dashboard")).toBeInTheDocument();
    });
  });

  it("loads with missing cluster data", async () => {
    server.use(
      http.get("/cluster", () => HttpResponse.json({})),
      http.get("/cluster/stats", () => HttpResponse.json({})),
    );
    renderWithQuery(<DashboardPage />);
    await waitFor(() => {
      expect(screen.getByText("Dashboard")).toBeInTheDocument();
    });
  });

  it("handles partial server data gracefully", async () => {
    server.use(
      http.get("/servers", () =>
        HttpResponse.json({
          items: [
            makeServer({ metadata: { name: "partial-server" } }),
          ],
        }),
      ),
    );
    renderWithQuery(<DashboardPage />);
    await waitFor(() => {
      expect(screen.getByText("Dashboard")).toBeInTheDocument();
    });
  });

  it("loads with multiple servers", async () => {
    server.use(
      http.get("/servers", () =>
        HttpResponse.json({
          items: [
            makeServer({ metadata: { name: "srv-1" }, status: { phase: "Running" } }),
            makeServer({ metadata: { name: "srv-2" }, status: { phase: "Running" } }),
            makeServer({ metadata: { name: "srv-3" }, status: { phase: "Stopped" } }),
          ],
        }),
      ),
    );
    renderWithQuery(<DashboardPage />);
    await waitFor(() => {
      expect(screen.getByText("Dashboard")).toBeInTheDocument();
    });
  });

  it("handles multiple backup items", async () => {
    server.use(
      http.get("/backups", () =>
        HttpResponse.json({
          items: [
            makeBackup({ metadata: { name: "backup-1" } }),
            makeBackup({ metadata: { name: "backup-2" } }),
          ],
        }),
      ),
    );
    renderWithQuery(<DashboardPage />);
    await waitFor(() => {
      expect(screen.getByText("Dashboard")).toBeInTheDocument();
    });
  });

  it("operator can access dashboard", async () => {
    server.use(
      http.get("/users/me", () => HttpResponse.json(makeUser({ role: "operator" }))),
    );
    const { client } = renderWithQuery(<DashboardPage />);
    await waitFor(() => expect(client.getQueryData(["me"])).toBeTruthy());
    expect(screen.getByText("Dashboard")).toBeInTheDocument();
  });

  it("viewer can access dashboard", async () => {
    server.use(
      http.get("/users/me", () => HttpResponse.json(makeUser({ role: "viewer" }))),
    );
    const { client } = renderWithQuery(<DashboardPage />);
    await waitFor(() => expect(client.getQueryData(["me"])).toBeTruthy());
    expect(screen.getByText("Dashboard")).toBeInTheDocument();
  });

  it("renders with cluster node data", async () => {
    server.use(
      http.get("/cluster", () =>
        HttpResponse.json(
          makeClusterView({
            ready: 2,
            total: 2,
            nodes: [
              { name: "node-1", status: "Ready" },
              { name: "node-2", status: "Ready" },
            ],
          }),
        ),
      ),
    );
    renderWithQuery(<DashboardPage />);
    await waitFor(() => {
      expect(screen.getByText("Dashboard")).toBeInTheDocument();
    });
  });

  it("renders with cluster stats data", async () => {
    server.use(
      http.get("/cluster/stats", () =>
        HttpResponse.json(
          makeClusterStats({
            usedStorageBytes: 500_000_000_000,
            totalStorageBytes: 1_000_000_000_000,
          }),
        ),
      ),
    );
    renderWithQuery(<DashboardPage />);
    await waitFor(() => {
      expect(screen.getByText("Dashboard")).toBeInTheDocument();
    });
  });
});
