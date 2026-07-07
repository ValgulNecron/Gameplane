import { describe, it, expect, vi } from "vitest";
import type { ReactNode } from "react";
import { http, HttpResponse } from "msw";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { server } from "@/test/server";
import { renderWithQuery } from "@/test/render";
import { makeServer, makeClusterStats } from "@/test/factories";

// TanStack Router's Link needs a router context the test doesn't supply.
// Replace it with a plain anchor — same DOM contract for what we assert.
// Extract search params and build the full href so route-parameter assertions work.
vi.mock("@tanstack/react-router", () => ({
  Link: ({ children, to, search, ...rest }: { children: ReactNode; to: string; search?: Record<string, unknown> } & Record<string, unknown>) => {
    let href = to;
    if (search && Object.keys(search).length > 0) {
      const params = new URLSearchParams();
      Object.entries(search).forEach(([key, value]) => {
        if (value !== undefined && value !== null) {
          params.set(key, String(value));
        }
      });
      href = `${to}?${params.toString()}`;
    }
    return (
      <a href={href} {...rest}>
        {children}
      </a>
    );
  },
}));

import { ServersPage } from "./Servers";

describe("ServersPage", () => {
  it("renders the server list and cluster stats", async () => {
    server.use(
      http.get("/servers", () =>
        HttpResponse.json({
          items: [
            makeServer({ metadata: { name: "alpha", namespace: "gameplane-games" }, status: { phase: "Running" } }),
            makeServer({ metadata: { name: "beta", namespace: "gameplane-games" }, status: { phase: "Stopped" } }),
          ],
        }),
      ),
      http.get("/cluster/stats", () => HttpResponse.json(makeClusterStats())),
    );
    renderWithQuery(<ServersPage />);
    await screen.findByText("alpha");
    expect(screen.getByText("beta")).toBeInTheDocument();
  });

  it("filters by name via the search box", async () => {
    server.use(
      http.get("/servers", () =>
        HttpResponse.json({
          items: [
            makeServer({ metadata: { name: "alpha", namespace: "gameplane-games" } }),
            makeServer({ metadata: { name: "beta", namespace: "gameplane-games" } }),
          ],
        }),
      ),
    );
    renderWithQuery(<ServersPage />);
    await screen.findByText("alpha");
    const search = screen.getByPlaceholderText(/Search/i);
    await userEvent.type(search, "alpha");
    await waitFor(() => expect(screen.queryByText("beta")).not.toBeInTheDocument());
    expect(screen.getByText("alpha")).toBeInTheDocument();
  });

  it("never sums unknown player counts into a negative total", async () => {
    server.use(
      http.get("/servers", () =>
        HttpResponse.json({
          items: [
            // legacy -1 sentinel and the new null "unknown" — neither may
            // drag the aggregate below zero.
            makeServer({ metadata: { name: "a", namespace: "gameplane-games" }, status: { phase: "Running", agent: { playersOnline: -1 } } }),
            makeServer({ metadata: { name: "b", namespace: "gameplane-games" }, status: { phase: "Running", agent: { playersOnline: null } } }),
          ],
        }),
      ),
    );
    renderWithQuery(<ServersPage />);
    await screen.findByText("a");
    expect(screen.queryByText("-1")).not.toBeInTheDocument();
    expect(screen.queryByText("-2")).not.toBeInTheDocument();
  });

  it("shows CPU and memory usage from the agent heartbeat", async () => {
    server.use(
      http.get("/servers", () =>
        HttpResponse.json({
          items: [
            // Telemetry lives under status.agent (cgroup + statfs); the table
            // shows it as a percent of the limit when a limit is reported.
            makeServer({
              metadata: { name: "metrics-on", namespace: "gameplane-games" },
              status: {
                phase: "Running",
                agent: {
                  cpuMillicores: 500,
                  cpuLimitMillicores: 2000, // 25%
                  memoryBytes: 536870912,
                  memoryLimitBytes: 1073741824, // 50%
                },
              },
            }),
          ],
        }),
      ),
    );
    renderWithQuery(<ServersPage />);
    await screen.findByText("metrics-on");
    expect(screen.getByText("25%")).toBeInTheDocument();
    expect(screen.getByText("50%")).toBeInTheDocument();
  });

  it("falls back to absolute cores when CPU has no limit", async () => {
    server.use(
      http.get("/servers", () =>
        HttpResponse.json({
          items: [
            makeServer({
              metadata: { name: "no-limit", namespace: "gameplane-games" },
              status: { phase: "Running", agent: { cpuMillicores: 1500 } },
            }),
          ],
        }),
      ),
    );
    renderWithQuery(<ServersPage />);
    await screen.findByText("no-limit");
    expect(screen.getByText("1.50 cores")).toBeInTheDocument();
  });

  it("renders dashes for CPU and memory when the heartbeat omits them", async () => {
    server.use(
      http.get("/servers", () =>
        HttpResponse.json({
          items: [
            makeServer({
              metadata: { name: "metrics-off", namespace: "gameplane-games" },
              status: { phase: "Running", agent: { playersOnline: 0 } },
            }),
          ],
        }),
      ),
    );
    renderWithQuery(<ServersPage />);
    const row = (await screen.findByText("metrics-off")).closest("tr") as HTMLElement;
    // Both the CPU and Memory cells fall back to "—" (so does Node).
    expect(within(row).getAllByText("—").length).toBeGreaterThanOrEqual(2);
  });

  it("renders empty stats gracefully when /cluster/stats fails", async () => {
    server.use(
      http.get("/servers", () => HttpResponse.json({ items: [] })),
      http.get("/cluster/stats", () => HttpResponse.error()),
    );
    renderWithQuery(<ServersPage />);
    await screen.findByText(/Servers/i);
  });

  it("renders shared servers under a 'Shared with you' header", async () => {
    server.use(
      http.get("/servers", () =>
        HttpResponse.json({
          items: [
            makeServer({ metadata: { name: "owned" }, status: { phase: "Running" } }),
          ],
        }),
      ),
      http.get("/users/me/servers", () =>
        HttpResponse.json({
          items: [
            makeServer({ metadata: { name: "owned" }, status: { phase: "Running" } }),
            makeServer({ metadata: { name: "shared" }, status: { phase: "Stopped" } }),
          ],
        }),
      ),
    );
    renderWithQuery(<ServersPage />);
    await screen.findByText("owned");
    expect(screen.getByText(/Shared with you/i)).toBeInTheDocument();
    expect(screen.getByText("shared")).toBeInTheDocument();
  });

  it("does not show 'Shared with you' header when no shared servers", async () => {
    server.use(
      http.get("/servers", () =>
        HttpResponse.json({
          items: [
            makeServer({ metadata: { name: "owned" }, status: { phase: "Running" } }),
          ],
        }),
      ),
      http.get("/users/me/servers", () =>
        HttpResponse.json({
          items: [
            makeServer({ metadata: { name: "owned" }, status: { phase: "Running" } }),
          ],
        }),
      ),
    );
    renderWithQuery(<ServersPage />);
    await screen.findByText("owned");
    expect(screen.queryByText(/Shared with you/i)).not.toBeInTheDocument();
  });

  it("filters shared servers by search and phase", async () => {
    server.use(
      http.get("/servers", () =>
        HttpResponse.json({
          items: [],
        }),
      ),
      http.get("/users/me/servers", () =>
        HttpResponse.json({
          items: [
            makeServer({ metadata: { name: "shared-alpha" }, status: { phase: "Running" } }),
            makeServer({ metadata: { name: "shared-beta" }, status: { phase: "Stopped" } }),
          ],
        }),
      ),
    );
    renderWithQuery(<ServersPage />);
    await screen.findByText(/Shared with you/i);
    const search = screen.getByPlaceholderText(/Search/i);
    await userEvent.type(search, "alpha");
    await waitFor(() => expect(screen.queryByText("shared-beta")).not.toBeInTheDocument());
    expect(screen.getByText("shared-alpha")).toBeInTheDocument();
  });

  it("deduplicates servers by namespace and name", async () => {
    server.use(
      http.get("/servers", () =>
        HttpResponse.json({
          items: [
            makeServer({
              metadata: { name: "dup", namespace: "gameplane-games" },
              status: { phase: "Running" },
            }),
          ],
        }),
      ),
      http.get("/users/me/servers", () =>
        HttpResponse.json({
          items: [
            makeServer({
              metadata: { name: "dup", namespace: "gameplane-games" },
              status: { phase: "Running" },
            }),
            makeServer({
              metadata: { name: "shared", namespace: "gameplane-games" },
              status: { phase: "Running" },
            }),
          ],
        }),
      ),
    );
    renderWithQuery(<ServersPage />);
    await screen.findByText("dup");
    // "dup" should appear only once in the document (in the main list, not shared)
    const dupElements = screen.getAllByText("dup");
    expect(dupElements).toHaveLength(1);
    // "shared" should appear once in the shared section
    expect(screen.getByText("shared")).toBeInTheDocument();
  });

  it("renders shared servers from non-default namespaces as enabled links with namespace param", async () => {
    server.use(
      http.get("/servers", () =>
        HttpResponse.json({
          items: [
            makeServer({
              metadata: { name: "owned", namespace: "gameplane-games" },
              status: { phase: "Running" },
            }),
          ],
        }),
      ),
      http.get("/users/me/servers", () =>
        HttpResponse.json({
          items: [
            makeServer({
              metadata: { name: "owned", namespace: "gameplane-games" },
              status: { phase: "Running" },
            }),
            makeServer({
              metadata: { name: "other-ns-server", namespace: "other-namespace" },
              status: { phase: "Running" },
            }),
          ],
        }),
      ),
    );
    renderWithQuery(<ServersPage />);
    await screen.findByText("owned");
    const sharedLink = screen.getByText("other-ns-server").closest("a") as HTMLAnchorElement;
    expect(sharedLink).toBeInTheDocument();
    expect(sharedLink.href).toContain("ns=other-namespace");
  });
});
