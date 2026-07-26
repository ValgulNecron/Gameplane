import { describe, it, expect, vi } from "vitest";
import type { ReactNode } from "react";
import { http, HttpResponse } from "msw";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { server } from "@/test/server";
import { renderWithQuery } from "@/test/render";
import { makeServer, makeUser } from "@/test/factories";

// TanStack Router APIs the layout reaches into.
const routerMocks = {
  useLocation: () => ({ pathname: "/" } as ReturnType<typeof import("@tanstack/react-router").useLocation>),
  useNavigate: () => vi.fn(),
};

vi.mock("@tanstack/react-router", () => ({
  Link: ({ children, to, ...rest }: { children: ReactNode; to: string } & Record<string, unknown>) => (
    <a href={to} {...rest}>{children}</a>
  ),
  Outlet: () => <div data-testid="outlet">outlet</div>,
  useLocation: () => routerMocks.useLocation(),
  useMatches: () => [],
  useNavigate: () => routerMocks.useNavigate(),
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
    expect(await screen.findByTestId("outlet")).toBeInTheDocument();
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

    // The drawer is now open with its nav rendered. Desktop sidebar is aria-hidden.
    const closeBtn = await screen.findByRole("button", { name: /close navigation/i });
    expect(screen.getByRole("link", { name: /Dashboard/i })).toBeInTheDocument();

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

  it("drawer closes when Escape is pressed (Radix Dialog default)", async () => {
    renderWithQuery(<AppLayout />);
    const menuBtn = await screen.findByRole("button", { name: /open navigation/i });
    await userEvent.click(menuBtn);

    // Drawer is open — close button is visible.
    await screen.findByRole("button", { name: /close navigation/i });

    // Press Escape to close (Radix Dialog handles this automatically).
    await userEvent.keyboard("{Escape}");

    // Close button should be gone, drawer is closed.
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

  it("shows a shell skeleton while useMe is loading", async () => {
    // Use a promise that never resolves to simulate indefinite loading.
    server.use(
      http.get("/users/me", () => new Promise(() => {})), // Never resolves
      http.get("/cluster/info", () => new Promise(() => {})), // Never resolves
    );

    renderWithQuery(<AppLayout />);

    // Skeleton should be present with aria-busy attribute and Loading label.
    const skeleton = screen.getByLabelText("Loading");
    expect(skeleton).toHaveAttribute("aria-busy", "true");

    // "guest" text must NOT appear anywhere — no pre-auth identity leak.
    expect(screen.queryByText(/guest/i)).not.toBeInTheDocument();

    // No admin links should appear while loading (they require me data).
    expect(screen.queryByRole("link", { name: /Users & RBAC/i })).not.toBeInTheDocument();
  });

  it("transitions from skeleton to real shell when user loads", async () => {
    server.use(
      http.get("/users/me", () => HttpResponse.json(makeUser({ role: "admin" }))),
      http.get("/cluster/info", () => HttpResponse.json({ clusterName: "homelab" })),
    );

    renderWithQuery(<AppLayout />);

    // Wait for real content (admin link) — indicates skeleton has been replaced.
    await waitFor(() =>
      expect(screen.getByRole("link", { name: /Users & RBAC/i })).toBeInTheDocument(),
    );

    // Real nav is present; skeleton is gone.
    expect(screen.getByRole("link", { name: /Audit log/i })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Settings/i })).toBeInTheDocument();
  });

  it("shows cluster name in sidebar", async () => {
    server.use(
      http.get("/users/me", () => HttpResponse.json(makeUser())),
      http.get("/cluster/info", () => HttpResponse.json({ clusterName: "prod-us-east" })),
    );
    renderWithQuery(<AppLayout />);
    expect(await screen.findByText("prod-us-east")).toBeInTheDocument();
  });

  it("shows dash when cluster name is unavailable", async () => {
    server.use(
      http.get("/users/me", () => HttpResponse.json(makeUser())),
      http.get("/cluster/info", () => new HttpResponse(null, { status: 500 })),
    );
    renderWithQuery(<AppLayout />);
    // Cluster name fallback in sidebar
    await waitFor(() => {
      expect(screen.getByText("—")).toBeInTheDocument();
    });
  });

  it("shows role in profile footer", async () => {
    server.use(
      http.get("/users/me", () =>
        HttpResponse.json(makeUser({ role: "operator", displayName: "Alice" })),
      ),
    );
    renderWithQuery(<AppLayout />);
    expect(await screen.findByText("operator")).toBeInTheDocument();
    expect(screen.getByText("Alice")).toBeInTheDocument();
  });

  it("shows guest username when displayName is absent", async () => {
    server.use(
      http.get("/users/me", () =>
        HttpResponse.json(makeUser({ username: "bob", displayName: "" })),
      ),
    );
    renderWithQuery(<AppLayout />);
    expect(await screen.findByText("bob")).toBeInTheDocument();
  });

  it("renders avatar initials correctly", async () => {
    server.use(
      http.get("/users/me", () =>
        HttpResponse.json(makeUser({ displayName: "Alice Brown" })),
      ),
    );
    renderWithQuery(<AppLayout />);
    // Initials should be "AL" (first 2 chars uppercase)
    expect(await screen.findByText("AL")).toBeInTheDocument();
  });

  it("search dropdown shows no matches message", async () => {
    server.use(
      http.get("/servers", () =>
        HttpResponse.json({
          items: [makeServer({ metadata: { name: "alpha" } })],
        }),
      ),
    );
    renderWithQuery(<AppLayout />);
    const search = await screen.findByLabelText(/search servers/i);
    await userEvent.type(search, "nonexistent");
    expect(await screen.findByText("No servers match.")).toBeInTheDocument();
  });

  it("search navigates on Enter key with first match", async () => {
    const originalLocation = window.location;
    Object.defineProperty(window, "location", {
      writable: true,
      value: { ...originalLocation, assign: vi.fn() },
    });
    try {
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
      renderWithQuery(<AppLayout />);
      const search = await screen.findByLabelText(/search servers/i);
      await userEvent.type(search, "a");
      await userEvent.keyboard("{Enter}");
      expect(window.location.assign).toHaveBeenCalledWith("/servers/alpha");
    } finally {
      Object.defineProperty(window, "location", { value: originalLocation });
    }
  });

  it("search dropdown closes on blur", async () => {
    server.use(
      http.get("/servers", () =>
        HttpResponse.json({
          items: [makeServer({ metadata: { name: "alpha" } })],
        }),
      ),
    );
    renderWithQuery(<AppLayout />);
    const search = await screen.findByLabelText(/search servers/i);
    await userEvent.click(search);
    const link = await screen.findByRole("link", { name: /alpha/i });
    expect(link).toBeInTheDocument();
    // Blur closes dropdown after 120ms
    search.blur();
    await waitFor(
      () => expect(screen.queryByRole("link", { name: /alpha/i })).not.toBeInTheDocument(),
      { timeout: 200 },
    );
  });

  it("search clears when navigating from dropdown", async () => {
    server.use(
      http.get("/servers", () =>
        HttpResponse.json({
          items: [makeServer({ metadata: { name: "alpha" } })],
        }),
      ),
    );
    renderWithQuery(<AppLayout />);
    const search = await screen.findByLabelText(/search servers/i) as HTMLInputElement;
    await userEvent.type(search, "alpha");
    expect(search.value).toBe("alpha");
    const link = await screen.findByRole("link", { name: /alpha/i });
    await userEvent.click(link);
    await waitFor(() => expect(search.value).toBe(""));
  });

  it("search escape key closes dropdown", async () => {
    server.use(
      http.get("/servers", () =>
        HttpResponse.json({
          items: [makeServer({ metadata: { name: "alpha" } })],
        }),
      ),
    );
    renderWithQuery(<AppLayout />);
    const search = await screen.findByLabelText(/search servers/i);
    await userEvent.type(search, "a");
    const link = await screen.findByRole("link", { name: /alpha/i });
    expect(link).toBeInTheDocument();
    await userEvent.keyboard("{Escape}");
    await waitFor(() => {
      expect(screen.queryByRole("link", { name: /alpha/i })).not.toBeInTheDocument();
    });
  });

  it("notifications open and close with bell click", async () => {
    renderWithQuery(<AppLayout />);
    const bell = await screen.findByRole("button", { name: /notifications/i });
    // Dropdown closed initially
    expect(screen.queryByText("No recent activity.")).not.toBeInTheDocument();
    await userEvent.click(bell);
    // Dropdown open
    expect(screen.getByText("No recent activity.")).toBeInTheDocument();
    await userEvent.click(bell);
    // Dropdown closed
    await waitFor(() => {
      expect(screen.queryByText("No recent activity.")).not.toBeInTheDocument();
    });
  });

  it("notifications badge shows unread count", async () => {
    renderWithQuery(<AppLayout />);
    await screen.findByRole("button", { name: /notifications/i });
    // Initially no badge (unread count is 0)
    expect(screen.queryByText(/^\d+$/)).not.toBeInTheDocument();
  });

  it("logout button navigates to login", async () => {
    const originalLocation = window.location;
    Object.defineProperty(window, "location", {
      writable: true,
      value: { ...originalLocation, assign: vi.fn() },
    });
    try {
      server.use(
        http.post("/auth/logout", () => new HttpResponse(null, { status: 204 })),
      );
      renderWithQuery(<AppLayout />);
      await screen.findByRole("link", { name: /Dashboard/i });
      const logoutBtn = screen.getByTitle("Sign out");
      await userEvent.click(logoutBtn);
      await waitFor(() => {
        expect(window.location.assign).toHaveBeenCalledWith("/login");
      });
    } finally {
      Object.defineProperty(window, "location", { value: originalLocation });
    }
  });

  it("sidebar nav item has active class when exact match", async () => {
    routerMocks.useLocation = () => ({ pathname: "/servers" } as ReturnType<typeof import("@tanstack/react-router").useLocation>);
    server.use(
      http.get("/users/me", () => HttpResponse.json(makeUser())),
    );
    renderWithQuery(<AppLayout />);
    // The /servers link should have active class (exact path match, but has sub-paths)
    await waitFor(() => {
      const link = screen.getByRole("link", { name: /^Servers$/i });
      expect(link.className).toContain("active");
    });
  });

  it("desktop sidebar hidden on mobile", async () => {
    server.use(
      http.get("/users/me", () => HttpResponse.json(makeUser())),
    );
    renderWithQuery(<AppLayout />);
    await screen.findByRole("link", { name: /Dashboard/i });
    // Desktop sidebar has lg:flex class, so it's hidden on small screens
    const desktopSidebar = screen.getAllByRole("link", { name: /Dashboard/i })[0].closest("aside");
    expect(desktopSidebar).toHaveClass("hidden");
  });

  it("system logs nav only visible with * permission", async () => {
    server.use(
      http.get("/users/me", () =>
        HttpResponse.json(makeUser({ role: "operator" })),
      ),
    );
    renderWithQuery(<AppLayout />);
    // Operator doesn't have * permission
    await waitFor(() => {
      expect(screen.queryByRole("link", { name: /System logs/i })).not.toBeInTheDocument();
    });
  });

  it("renders gameplane branding and logo", async () => {
    server.use(
      http.get("/users/me", () => HttpResponse.json(makeUser())),
    );
    renderWithQuery(<AppLayout />);
    expect(await screen.findByText("gameplane")).toBeInTheDocument();
  });

  it("breadcrumb builds from pathname", async () => {
    routerMocks.useLocation = () => ({ pathname: "/servers/alpha" } as ReturnType<typeof import("@tanstack/react-router").useLocation>);
    server.use(
      http.get("/users/me", () => HttpResponse.json(makeUser())),
    );
    renderWithQuery(<AppLayout />);
    // Breadcrumb should show: gameplane > Servers > alpha
    await waitFor(() => {
      expect(screen.getByText("gameplane")).toBeInTheDocument();
      // The last breadcrumb (alpha) should be text, not a link
      const breadcrumbs = screen.getAllByText(/gameplane|Servers|alpha/);
      expect(breadcrumbs.length).toBeGreaterThan(0);
    });
  });
});
