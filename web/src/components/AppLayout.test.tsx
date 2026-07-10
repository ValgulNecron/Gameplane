import { describe, it, expect, vi } from "vitest";
import type { ReactNode } from "react";
import { http, HttpResponse } from "msw";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { server } from "@/test/server";
import { renderWithQuery } from "@/test/render";
import { makeServer, makeUser } from "@/test/factories";

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

  it("notifications bell toggles a panel", async () => {
    renderWithQuery(<AppLayout />);
    const bell = await screen.findByRole("button", { name: /notifications/i });
    await userEvent.click(bell);
    expect(await screen.findByText("Recent activity")).toBeInTheDocument();
  });

  it("hamburger opens the mobile nav drawer; scrim and nav clicks close it", async () => {
    renderWithQuery(<AppLayout />);
    // Closed by default — no close button (drawer-only) in the document.
    expect(screen.queryByRole("button", { name: /close navigation/i })).not.toBeInTheDocument();

    const menuBtn = await screen.findByRole("button", { name: /open navigation/i });
    await userEvent.click(menuBtn);

    // The drawer mounts a second "Dashboard" link (desktop sidebar + drawer).
    const closeBtn = await screen.findByRole("button", { name: /close navigation/i });
    expect(screen.getAllByRole("link", { name: /Dashboard/i }).length).toBeGreaterThan(1);

    await userEvent.click(closeBtn);
    await waitFor(() =>
      expect(screen.queryByRole("button", { name: /close navigation/i })).not.toBeInTheDocument(),
    );

    // Re-open, then close via a nav-item click instead of the X button.
    await userEvent.click(menuBtn);
    await screen.findByRole("button", { name: /close navigation/i });
    const drawerLinks = screen.getAllByRole("link", { name: /Servers/i });
    await userEvent.click(drawerLinks[drawerLinks.length - 1]);
    await waitFor(() =>
      expect(screen.queryByRole("button", { name: /close navigation/i })).not.toBeInTheDocument(),
    );
  });

  it("global search filters servers by name and links to detail", async () => {
    server.use(
      http.get("/servers", () =>
        HttpResponse.json({
          items: [makeServer({ metadata: { name: "alpha" } }), makeServer({ metadata: { name: "beta" } })],
        }),
      ),
    );
    renderWithQuery(<AppLayout />);
    const search = await screen.findByLabelText(/search servers/i);
    await userEvent.type(search, "alph");
    const link = await screen.findByRole("link", { name: /alpha/i });
    expect(link).toHaveAttribute("href", "/servers/$name");
    expect(screen.queryByText("beta")).not.toBeInTheDocument();
  });
});
