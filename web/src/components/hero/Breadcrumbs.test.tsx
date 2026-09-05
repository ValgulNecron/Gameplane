import { describe, it, expect, vi } from "vitest";
import type { ReactNode } from "react";
import { screen, within } from "@testing-library/react";
import { renderWithQuery } from "@/test/render";
import { buildCrumbs, Breadcrumbs } from "./Breadcrumbs";

// Mock TanStack Router
vi.mock("@tanstack/react-router", () => ({
  Link: ({ children, to, ...rest }: { children: ReactNode; to: string } & Record<string, unknown>) => (
    <a href={to} {...rest}>
      {children}
    </a>
  ),
  useLocation: () => ({ pathname: "/" } as ReturnType<typeof import("@tanstack/react-router").useLocation>),
}));

describe("buildCrumbs", () => {
  it("returns home and dashboard crumb for root path", () => {
    const crumbs = buildCrumbs("/");
    expect(crumbs).toEqual([
      { label: "gameplane", to: "/" },
      { label: "Dashboard" },
    ]);
  });

  it("builds breadcrumbs for a single route segment", () => {
    const crumbs = buildCrumbs("/servers");
    expect(crumbs).toEqual([
      { label: "gameplane", to: "/" },
      { label: "Servers", to: "/servers" },
    ]);
  });

  it("builds breadcrumbs for multiple route segments", () => {
    const crumbs = buildCrumbs("/servers/my-server");
    expect(crumbs).toEqual([
      { label: "gameplane", to: "/" },
      { label: "Servers", to: "/servers" },
      { label: "my-server", to: "/servers/my-server" },
    ]);
  });

  it("maps known labels from the label map", () => {
    const crumbs = buildCrumbs("/admin/audit");
    expect(crumbs).toEqual([
      { label: "gameplane", to: "/" },
      { label: "Settings", to: "/admin" },
      { label: "Audit log", to: "/admin/audit" },
    ]);
  });

  it("uses path segment as label for unmapped routes", () => {
    const crumbs = buildCrumbs("/unknown/path");
    expect(crumbs).toEqual([
      { label: "gameplane", to: "/" },
      { label: "unknown", to: "/unknown" },
      { label: "path", to: "/unknown/path" },
    ]);
  });

  it("handles /modules route", () => {
    const crumbs = buildCrumbs("/modules");
    expect(crumbs).toEqual([
      { label: "gameplane", to: "/" },
      { label: "Modules", to: "/modules" },
    ]);
  });

  it("handles /cluster route", () => {
    const crumbs = buildCrumbs("/cluster");
    expect(crumbs).toEqual([
      { label: "gameplane", to: "/" },
      { label: "Cluster", to: "/cluster" },
    ]);
  });

  it("handles /users route", () => {
    const crumbs = buildCrumbs("/users");
    expect(crumbs).toEqual([
      { label: "gameplane", to: "/" },
      { label: "Users & RBAC", to: "/users" },
    ]);
  });

  it("handles /backups route", () => {
    const crumbs = buildCrumbs("/backups");
    expect(crumbs).toEqual([
      { label: "gameplane", to: "/" },
      { label: "Backups", to: "/backups" },
    ]);
  });

  it("handles /admin/logs route", () => {
    const crumbs = buildCrumbs("/admin/logs");
    expect(crumbs).toEqual([
      { label: "gameplane", to: "/" },
      { label: "Settings", to: "/admin" },
      { label: "System logs", to: "/admin/logs" },
    ]);
  });

  it("handles /new route", () => {
    const crumbs = buildCrumbs("/new");
    expect(crumbs).toEqual([
      { label: "gameplane", to: "/" },
      { label: "New", to: "/new" },
    ]);
  });

  it("ignores trailing slashes", () => {
    const crumbs = buildCrumbs("/servers/");
    expect(crumbs).toEqual([
      { label: "gameplane", to: "/" },
      { label: "Servers", to: "/servers" },
    ]);
  });
});

describe("Breadcrumbs component", () => {
  it("renders home link and dashboard crumb", () => {
    const crumbs = buildCrumbs("/");
    renderWithQuery(<Breadcrumbs items={crumbs} />);

    const nav = screen.getByRole("navigation");
    expect(nav).toBeInTheDocument();

    const homeLink = screen.getByRole("link", { name: /gameplane/i });
    expect(homeLink).toHaveAttribute("href", "/");

    const dashboardCrumb = within(nav).getByText("Dashboard");
    expect(dashboardCrumb).toBeInTheDocument();
  });

  it("renders navigation links for non-final crumbs", () => {
    const crumbs = buildCrumbs("/servers/my-server");
    renderWithQuery(<Breadcrumbs items={crumbs} />);

    // Find links by their href attribute (from TanStack Router Link mock)
    const homeLink = screen.getByRole("link", { name: /gameplane/i });
    expect(homeLink).toHaveAttribute("href", "/");

    const serversLink = screen.getByRole("link", { name: /servers/i });
    expect(serversLink).toHaveAttribute("href", "/servers");

    // Last item should not be a link
    const myServerText = screen.getByText("my-server");
    expect(myServerText.tagName).not.toBe("A");
  });

  it("marks the last crumb with aria-current='page'", () => {
    const crumbs = buildCrumbs("/servers/my-server");
    renderWithQuery(<Breadcrumbs items={crumbs} />);

    const nav = screen.getByRole("navigation");
    const items = within(nav).getAllByRole("listitem");

    // Verify we have 3 items
    expect(items).toHaveLength(3);

    // First item (home) should not have aria-current
    expect(items[0]).not.toHaveAttribute("aria-current");

    // Middle item (servers) should not have aria-current
    expect(items[1]).not.toHaveAttribute("aria-current");

    // Last item (my-server) should have aria-current='page'
    expect(items[2]).toHaveAttribute("aria-current", "page");
  });

  it("renders the dashboard crumb with aria-current when on root", () => {
    const crumbs = buildCrumbs("/");
    renderWithQuery(<Breadcrumbs items={crumbs} />);

    const nav = screen.getByRole("navigation");
    const items = within(nav).getAllByRole("listitem");

    expect(items).toHaveLength(2);

    // Home link should not have aria-current
    expect(items[0]).not.toHaveAttribute("aria-current");

    // Dashboard (last) should have aria-current='page'
    expect(items[1]).toHaveAttribute("aria-current", "page");
  });

  it("renders custom route labels from label map", () => {
    const crumbs = buildCrumbs("/admin/audit");
    renderWithQuery(<Breadcrumbs items={crumbs} />);

    expect(screen.getByRole("link", { name: /gameplane/i })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /settings/i })).toBeInTheDocument();
    expect(screen.getByText("Audit log")).toBeInTheDocument();
  });

  it("renders unmapped labels as-is", () => {
    const crumbs = buildCrumbs("/unknown/path");
    renderWithQuery(<Breadcrumbs items={crumbs} />);

    expect(screen.getByRole("link", { name: /gameplane/i })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /unknown/i })).toBeInTheDocument();
    expect(screen.getByText("path")).toBeInTheDocument();
  });

  it("renders all crumbs in order", () => {
    const crumbs = buildCrumbs("/admin/audit");
    renderWithQuery(<Breadcrumbs items={crumbs} />);

    const nav = screen.getByRole("navigation");
    const items = within(nav).getAllByRole("listitem");

    expect(items).toHaveLength(3);
    expect(items[0]).toHaveTextContent("gameplane");
    expect(items[1]).toHaveTextContent("Settings");
    expect(items[2]).toHaveTextContent("Audit log");
  });

  it("accepts custom crumb items", () => {
    const customCrumbs = [
      { label: "Home", to: "/" },
      { label: "Custom", to: "/custom" },
      { label: "Page" },
    ];
    renderWithQuery(<Breadcrumbs items={customCrumbs} />);

    const homeLink = screen.getByRole("link", { name: /home/i });
    expect(homeLink).toHaveAttribute("href", "/");

    const customLink = screen.getByRole("link", { name: /custom/i });
    expect(customLink).toHaveAttribute("href", "/custom");

    expect(screen.getByText("Page")).toBeInTheDocument();
  });

  it("handles empty items array gracefully", () => {
    renderWithQuery(<Breadcrumbs items={[]} />);

    const nav = screen.getByRole("navigation");
    expect(nav).toBeInTheDocument();
    expect(within(nav).queryAllByRole("listitem")).toHaveLength(0);
  });
});
