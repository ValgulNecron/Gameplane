import { describe, it, expect, vi } from "vitest";
import type { ReactNode } from "react";
import { http, HttpResponse } from "msw";
import { screen, waitFor } from "@testing-library/react";
import { server } from "@/test/server";
import { renderWithQuery } from "@/test/render";
import { makeUser } from "@/test/factories";

// TanStack Router APIs the layout reaches into.
vi.mock("@tanstack/react-router", () => ({
  Link: ({ children, to, ...rest }: { children: ReactNode; to: string } & Record<string, unknown>) => (
    <a href={to} {...rest}>{children}</a>
  ),
  Outlet: () => <div data-testid="outlet">outlet</div>,
  useLocation: () => ({ pathname: "/" }),
  useMatches: () => [],
}));

import { AppLayout } from "./AppLayout";

describe("AppLayout", () => {
  it("renders the sidebar nav items shared by all roles", async () => {
    server.use(
      http.get("/users/me", () => HttpResponse.json(makeUser({ role: "viewer" }))),
      http.get("/cluster/info", () => HttpResponse.json({ clusterName: "homelab" })),
    );
    renderWithQuery(<AppLayout />);
    await waitFor(() =>
      expect(screen.getByRole("link", { name: /Dashboard/i })).toBeInTheDocument(),
    );
    expect(screen.getByRole("link", { name: /Servers/i })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Modules/i })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Backups/i })).toBeInTheDocument();
    // Viewer-restricted: /cluster, /users, /admin nav not rendered.
    expect(screen.queryByRole("link", { name: /Audit log/i })).not.toBeInTheDocument();
  });

  it("operator role unlocks the Cluster nav", async () => {
    server.use(
      http.get("/users/me", () => HttpResponse.json(makeUser({ role: "operator" }))),
    );
    renderWithQuery(<AppLayout />);
    await waitFor(() =>
      expect(screen.getByRole("link", { name: /Cluster/i })).toBeInTheDocument(),
    );
  });

  it("admin role unlocks Users / Audit / Settings nav", async () => {
    server.use(
      http.get("/users/me", () => HttpResponse.json(makeUser({ role: "admin" }))),
    );
    renderWithQuery(<AppLayout />);
    await waitFor(() =>
      expect(screen.getByRole("link", { name: /Users & RBAC/i })).toBeInTheDocument(),
    );
    expect(screen.getByRole("link", { name: /Audit log/i })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Settings/i })).toBeInTheDocument();
  });

  it("renders the outlet for child routes", async () => {
    renderWithQuery(<AppLayout />);
    expect(screen.getByTestId("outlet")).toBeInTheDocument();
  });
});
