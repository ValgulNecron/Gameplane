import { describe, it, expect, vi } from "vitest";
import type { ReactNode } from "react";
import { http, HttpResponse } from "msw";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { server } from "@/test/server";
import { renderWithQuery } from "@/test/render";
import { makeServer, makeClusterStats } from "@/test/factories";

// TanStack Router's Link needs a router context the test doesn't supply.
// Replace it with a plain anchor — same DOM contract for what we assert.
vi.mock("@tanstack/react-router", () => ({
  Link: ({ children, to, ...rest }: { children: ReactNode; to: string } & Record<string, unknown>) => (
    <a href={to} {...rest}>
      {children}
    </a>
  ),
}));

import { DashboardPage } from "./Dashboard";

describe("DashboardPage", () => {
  it("renders the server list and cluster stats", async () => {
    server.use(
      http.get("/servers", () =>
        HttpResponse.json({
          items: [
            makeServer({ metadata: { name: "alpha" }, status: { phase: "Running" } }),
            makeServer({ metadata: { name: "beta" }, status: { phase: "Stopped" } }),
          ],
        }),
      ),
      http.get("/cluster/stats", () => HttpResponse.json(makeClusterStats())),
    );
    renderWithQuery(<DashboardPage />);
    await screen.findByText("alpha");
    expect(screen.getByText("beta")).toBeInTheDocument();
  });

  it("filters by name via the search box", async () => {
    server.use(
      http.get("/servers", () =>
        HttpResponse.json({
          items: [
            makeServer({ metadata: { name: "alpha" } }),
            makeServer({ metadata: { name: "beta" } }),
          ],
        }),
      ),
    );
    renderWithQuery(<DashboardPage />);
    await screen.findByText("alpha");
    const search = screen.getByPlaceholderText(/Search/i);
    await userEvent.type(search, "alpha");
    await waitFor(() => expect(screen.queryByText("beta")).not.toBeInTheDocument());
    expect(screen.getByText("alpha")).toBeInTheDocument();
  });

  it("renders empty stats gracefully when /cluster/stats fails", async () => {
    server.use(
      http.get("/servers", () => HttpResponse.json({ items: [] })),
      http.get("/cluster/stats", () => HttpResponse.error()),
    );
    renderWithQuery(<DashboardPage />);
    await screen.findByText(/Servers/i);
  });
});
