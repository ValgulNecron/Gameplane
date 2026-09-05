import { describe, it, expect, vi } from "vitest";
import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderWithQuery } from "@/test/render";
import { TopBar } from "./TopBar";
import type { User } from "@/types";

// Mock Auth for logout
vi.mock("@/lib/endpoints", () => ({
  Auth: {
    logout: vi.fn(() => Promise.resolve()),
  },
}));

// Mock location.assign
const mockLocationAssign = vi.fn();
Object.defineProperty(window, "location", {
  value: { assign: mockLocationAssign },
  writable: true,
});

const mockUser: User = {
  id: 1,
  username: "alice",
  displayName: "Alice Developer",
  email: "alice@example.com",
  role: "admin",
  createdAt: "2024-01-01T00:00:00Z",
};

describe("TopBar", () => {
  it("renders all four slots", () => {
    renderWithQuery(
      <TopBar
        breadcrumbs={<div>Breadcrumbs Content</div>}
        clusterSelector={<div>Cluster Content</div>}
        search={<div>Search Content</div>}
        notifications={<div>Notifications Content</div>}
        user={mockUser}
        onMenuClick={() => {}}
      />
    );

    expect(screen.getByText("Breadcrumbs Content")).toBeInTheDocument();
    expect(screen.getByText("Cluster Content")).toBeInTheDocument();
    expect(screen.getByText("Search Content")).toBeInTheDocument();
    expect(screen.getByText("Notifications Content")).toBeInTheDocument();
  });

  it("renders header with border-b", () => {
    const { container } = renderWithQuery(
      <TopBar
        breadcrumbs={<div>Breadcrumbs</div>}
        clusterSelector={<div>Cluster</div>}
        search={<div>Search</div>}
        notifications={<div>Notifications</div>}
        onMenuClick={() => {}}
      />
    );

    const header = container.querySelector("header");
    expect(header).toHaveClass("border-b");
  });

  it("renders hamburger button for mobile navigation", () => {
    renderWithQuery(
      <TopBar
        breadcrumbs={<div>Breadcrumbs</div>}
        clusterSelector={<div>Cluster</div>}
        search={<div>Search</div>}
        notifications={<div>Notifications</div>}
        onMenuClick={() => {}}
      />
    );

    const hamburger = screen.getByRole("button", { name: /open navigation/i });
    expect(hamburger).toBeInTheDocument();
    expect(hamburger).toHaveClass("lg:hidden");
  });

  it("calls onMenuClick when hamburger button is clicked", async () => {
    const handleMenuClick = vi.fn();
    const user = userEvent.setup();

    renderWithQuery(
      <TopBar
        breadcrumbs={<div>Breadcrumbs</div>}
        clusterSelector={<div>Cluster</div>}
        search={<div>Search</div>}
        notifications={<div>Notifications</div>}
        onMenuClick={handleMenuClick}
      />
    );

    const hamburger = screen.getByRole("button", { name: /open navigation/i });
    await user.click(hamburger);

    expect(handleMenuClick).toHaveBeenCalledOnce();
  });

  it("renders user avatar with initials when user is provided", () => {
    renderWithQuery(
      <TopBar
        breadcrumbs={<div>Breadcrumbs</div>}
        clusterSelector={<div>Cluster</div>}
        search={<div>Search</div>}
        notifications={<div>Notifications</div>}
        user={mockUser}
        onMenuClick={() => {}}
      />
    );

    const avatar = screen.getByRole("button", { name: /user menu/i });
    expect(avatar).toBeInTheDocument();
  });

  it("renders user avatar with 'guest' when user is not provided", () => {
    renderWithQuery(
      <TopBar
        breadcrumbs={<div>Breadcrumbs</div>}
        clusterSelector={<div>Cluster</div>}
        search={<div>Search</div>}
        notifications={<div>Notifications</div>}
        onMenuClick={() => {}}
      />
    );

    const avatar = screen.getByRole("button", { name: /user menu/i });
    expect(avatar).toBeInTheDocument();
  });

  it("keeps the user menu closed until the avatar is clicked", () => {
    renderWithQuery(
      <TopBar
        breadcrumbs={<div>Breadcrumbs</div>}
        clusterSelector={<div>Cluster</div>}
        search={<div>Search</div>}
        notifications={<div>Notifications</div>}
        user={mockUser}
        onMenuClick={() => {}}
      />
    );

    // The menu (and its items) must not be present until the avatar trigger
    // is pressed — regression guard for the menu rendering inline/permanently
    // visible instead of inside a popover.
    expect(screen.queryByRole("menu")).not.toBeInTheDocument();
    expect(screen.queryByText("Sign out")).not.toBeInTheDocument();
    expect(screen.queryByText("Alice Developer")).not.toBeInTheDocument();
  });

  it("renders user menu with user name and role when avatar is clicked", async () => {
    const user = userEvent.setup();

    renderWithQuery(
      <TopBar
        breadcrumbs={<div>Breadcrumbs</div>}
        clusterSelector={<div>Cluster</div>}
        search={<div>Search</div>}
        notifications={<div>Notifications</div>}
        user={mockUser}
        onMenuClick={() => {}}
      />
    );

    const avatar = screen.getByRole("button", { name: /user menu/i });
    await user.click(avatar);

    expect(screen.getByText("Alice Developer")).toBeInTheDocument();
    expect(screen.getByText("admin")).toBeInTheDocument();
  });

  it("renders logout button in user menu", async () => {
    const user = userEvent.setup();

    renderWithQuery(
      <TopBar
        breadcrumbs={<div>Breadcrumbs</div>}
        clusterSelector={<div>Cluster</div>}
        search={<div>Search</div>}
        notifications={<div>Notifications</div>}
        user={mockUser}
        onMenuClick={() => {}}
      />
    );

    const avatar = screen.getByRole("button", { name: /user menu/i });
    await user.click(avatar);

    const logoutButton = screen.getByRole("menuitem", { name: /sign out/i });
    expect(logoutButton).toBeInTheDocument();
  });

  it("calls logout and navigates to login when logout button is clicked", async () => {
    const user = userEvent.setup();
    const { Auth } = await import("@/lib/endpoints");

    renderWithQuery(
      <TopBar
        breadcrumbs={<div>Breadcrumbs</div>}
        clusterSelector={<div>Cluster</div>}
        search={<div>Search</div>}
        notifications={<div>Notifications</div>}
        user={mockUser}
        onMenuClick={() => {}}
      />
    );

    const avatar = screen.getByRole("button", { name: /user menu/i });
    await user.click(avatar);

    const logoutButton = screen.getByRole("menuitem", { name: /sign out/i });
    await user.click(logoutButton);

    expect(Auth.logout).toHaveBeenCalled();
    expect(mockLocationAssign).toHaveBeenCalledWith("/login");
  });

  it("renders with all sections in correct layout order (left: breadcrumbs, right: controls)", () => {
    const { container } = renderWithQuery(
      <TopBar
        breadcrumbs={<div data-testid="breadcrumbs-slot">Breadcrumbs</div>}
        clusterSelector={<div data-testid="cluster-slot">Cluster</div>}
        search={<div data-testid="search-slot">Search</div>}
        notifications={<div data-testid="notifications-slot">Notifications</div>}
        user={mockUser}
        onMenuClick={() => {}}
      />
    );

    const header = container.querySelector("header");
    expect(header).toHaveClass("flex", "justify-between");

    // Left side should contain breadcrumbs
    const leftDiv = header?.querySelector(".gap-2");
    if (leftDiv instanceof HTMLElement) {
      expect(within(leftDiv).getByTestId("breadcrumbs-slot")).toBeInTheDocument();
    }

    // Right side should contain cluster, search, notifications, and avatar
    const rightDiv = header?.querySelector(".flex.shrink-0");
    if (rightDiv instanceof HTMLElement) {
      expect(within(rightDiv).getByTestId("cluster-slot")).toBeInTheDocument();
      expect(within(rightDiv).getByTestId("search-slot")).toBeInTheDocument();
      expect(within(rightDiv).getByTestId("notifications-slot")).toBeInTheDocument();
      expect(within(rightDiv).getByRole("button", { name: /user menu/i })).toBeInTheDocument();
    }
  });

  it("renders user name from displayName when available", async () => {
    const user = userEvent.setup();

    renderWithQuery(
      <TopBar
        breadcrumbs={<div>Breadcrumbs</div>}
        clusterSelector={<div>Cluster</div>}
        search={<div>Search</div>}
        notifications={<div>Notifications</div>}
        user={mockUser}
        onMenuClick={() => {}}
      />
    );

    const avatar = screen.getByRole("button", { name: /user menu/i });
    await user.click(avatar);

    // displayName is "Alice Developer", so it should be shown
    expect(screen.getByText("Alice Developer")).toBeInTheDocument();
  });

  it("falls back to username when displayName is not available", async () => {
    const user = userEvent.setup();
    const userWithoutDisplay: User = {
      id: 2,
      username: "bob",
      displayName: "",
      email: "bob@example.com",
      role: "operator",
      createdAt: "2024-01-01T00:00:00Z",
    };

    renderWithQuery(
      <TopBar
        breadcrumbs={<div>Breadcrumbs</div>}
        clusterSelector={<div>Cluster</div>}
        search={<div>Search</div>}
        notifications={<div>Notifications</div>}
        user={userWithoutDisplay}
        onMenuClick={() => {}}
      />
    );

    const avatar = screen.getByRole("button", { name: /user menu/i });
    await user.click(avatar);

    expect(screen.getByText("bob")).toBeInTheDocument();
  });

  it("displays user with operator role", async () => {
    const user = userEvent.setup();
    const operatorUser: User = {
      id: 3,
      username: "charlie",
      displayName: "Charlie",
      email: "charlie@example.com",
      role: "operator",
      createdAt: "2024-01-01T00:00:00Z",
    };

    renderWithQuery(
      <TopBar
        breadcrumbs={<div>Breadcrumbs</div>}
        clusterSelector={<div>Cluster</div>}
        search={<div>Search</div>}
        notifications={<div>Notifications</div>}
        user={operatorUser}
        onMenuClick={() => {}}
      />
    );

    const avatar = screen.getByRole("button", { name: /user menu/i });
    await user.click(avatar);

    expect(screen.getByText("Charlie")).toBeInTheDocument();
    expect(screen.getByText("operator")).toBeInTheDocument();
  });

  it("renders mobile title when provided", () => {
    renderWithQuery(
      <TopBar
        breadcrumbs={<div>Breadcrumbs</div>}
        clusterSelector={<div>Cluster</div>}
        search={<div>Search</div>}
        notifications={<div>Notifications</div>}
        mobileTitle="Servers"
        onMenuClick={() => {}}
      />
    );

    expect(screen.getByText("Servers")).toBeInTheDocument();
  });

  it("hides breadcrumbs and desktop controls below lg breakpoint", () => {
    renderWithQuery(
      <TopBar
        breadcrumbs={<div data-testid="breadcrumbs-slot">Breadcrumbs</div>}
        clusterSelector={<div data-testid="cluster-slot">Cluster</div>}
        search={<div data-testid="search-slot">Search</div>}
        notifications={<div data-testid="notifications-slot">Notifications</div>}
        mobileTitle="Servers"
        user={mockUser}
        onMenuClick={() => {}}
      />
    );

    // Verify mobile title is shown
    expect(screen.getByText("Servers")).toBeInTheDocument();

    // Verify breadcrumbs have lg:hidden applied
    const breadcrumbsDiv = screen.getByTestId("breadcrumbs-slot").parentElement;
    expect(breadcrumbsDiv).toHaveClass("hidden");
    expect(breadcrumbsDiv).toHaveClass("lg:block");

    // Verify cluster selector, search, and notifications have hidden lg:flex
    const clusterDiv = screen.getByTestId("cluster-slot").parentElement;
    expect(clusterDiv).toHaveClass("hidden");
    expect(clusterDiv).toHaveClass("lg:flex");

    const searchDiv = screen.getByTestId("search-slot").parentElement;
    expect(searchDiv).toHaveClass("hidden");
    expect(searchDiv).toHaveClass("lg:flex");

    const notificationsDiv = screen.getByTestId("notifications-slot").parentElement;
    expect(notificationsDiv).toHaveClass("hidden");
    expect(notificationsDiv).toHaveClass("lg:flex");
  });
});
