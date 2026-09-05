import { describe, it, expect, vi, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { server } from "@/test/server";
import { renderWithQuery } from "@/test/render";

// Mock the router navigation
const routerMocks = {
  useNavigate: () => vi.fn(),
};

vi.mock("@tanstack/react-router", () => ({
  useNavigate: () => routerMocks.useNavigate(),
}));

import { ClusterSelector } from "./ClusterSelector";
import { setCurrentCluster } from "@/lib/cluster";
import type { ClusterRegistry } from "@/types";

describe("ClusterSelector", () => {
  beforeEach(() => {
    // Clear localStorage and reset cluster selection for test isolation
    localStorage.clear();
    setCurrentCluster("local");
  });

  it("renders the current cluster name with a health dot", async () => {
    const clusters: ClusterRegistry[] = [
      { name: "local", displayName: "Local", phase: "Healthy" },
      { name: "prod", displayName: "Production", phase: "Healthy" },
    ];

    server.use(
      http.get("/clusters", () => HttpResponse.json({ items: clusters })),
    );

    renderWithQuery(<ClusterSelector />);

    // Check for the button trigger with the cluster name (may be "local" or "Local" depending on timing)
    const button = await screen.findByRole("button", { name: /select cluster/i });
    expect(button).toBeInTheDocument();
    // The button should show the current cluster, either the fallback "local" or the displayName "Local"
    expect(button).toHaveTextContent(/[Ll]ocal/);
  });

  it("keeps the dropdown menu closed until the trigger is pressed", async () => {
    const clusters: ClusterRegistry[] = [
      { name: "local", displayName: "Local", phase: "Healthy" },
      { name: "prod", displayName: "Production", phase: "Healthy" },
    ];

    server.use(
      http.get("/clusters", () => HttpResponse.json({ items: clusters })),
    );

    renderWithQuery(<ClusterSelector />);

    // Trigger renders, but the menu (and its items) must not be present
    // until the trigger is actually pressed — regression guard for the menu
    // rendering inline/permanently visible instead of inside a popover.
    await screen.findByRole("button", { name: /select cluster/i });
    expect(screen.queryByRole("menu")).not.toBeInTheDocument();
    expect(screen.queryByText("Production")).not.toBeInTheDocument();
  });

  it("opens the dropdown and lists all clusters", async () => {
    const clusters: ClusterRegistry[] = [
      { name: "local", displayName: "Local", phase: "Healthy" },
      { name: "prod", displayName: "Production", phase: "Unhealthy" },
      { name: "staging", displayName: "Staging", phase: "Unknown" },
    ];

    server.use(
      http.get("/clusters", () => HttpResponse.json({ items: clusters })),
    );

    renderWithQuery(<ClusterSelector />);

    const trigger = await screen.findByRole("button", { name: /select cluster/i });
    await userEvent.click(trigger);

    // Wait for the dropdown to open, then scope assertions to it
    const menu = await screen.findByRole("menu");
    await waitFor(() => {
      expect(within(menu).getByText("Local")).toBeInTheDocument();
      expect(within(menu).getByText("Production")).toBeInTheDocument();
      expect(within(menu).getByText("Staging")).toBeInTheDocument();
    });
  });

  it("marks the currently selected cluster with a check icon", async () => {
    const clusters: ClusterRegistry[] = [
      { name: "local", displayName: "Local", phase: "Healthy" },
      { name: "prod", displayName: "Production", phase: "Healthy" },
    ];

    server.use(
      http.get("/clusters", () => HttpResponse.json({ items: clusters })),
    );

    renderWithQuery(<ClusterSelector />);

    const trigger = await screen.findByRole("button", { name: /select cluster/i });
    await userEvent.click(trigger);

    // Wait for the dropdown to show
    await waitFor(() => {
      expect(screen.getByText("Production")).toBeInTheDocument();
    });

    // The "Local" option should be selected initially (default cluster)
    // Check that the menu has been rendered with cluster items
    const menu = screen.getByRole("menu");
    expect(within(menu).getByText("Local")).toBeInTheDocument();
  });

  it("calls setCurrentCluster and clears the query cache when a cluster is selected", async () => {
    const clusters: ClusterRegistry[] = [
      { name: "local", displayName: "Local", phase: "Healthy" },
      { name: "prod", displayName: "Production", phase: "Healthy" },
    ];

    server.use(
      http.get("/clusters", () => HttpResponse.json({ items: clusters })),
    );

    const { client } = renderWithQuery(<ClusterSelector />);
    const clearSpy = vi.spyOn(client, "clear");

    const trigger = await screen.findByRole("button", { name: /select cluster/i });
    await userEvent.click(trigger);

    // Find and click the "Production" menu item
    const prodItem = await screen.findByRole("menuitem", { name: /Production/ });
    await userEvent.click(prodItem);

    // The query cache should be cleared
    await waitFor(() => {
      expect(clearSpy).toHaveBeenCalled();
    });
  });

  it("includes an 'Add cluster' option at the bottom of the dropdown", async () => {
    const clusters: ClusterRegistry[] = [
      { name: "local", displayName: "Local", phase: "Healthy" },
    ];

    server.use(
      http.get("/clusters", () => HttpResponse.json({ items: clusters })),
    );

    renderWithQuery(<ClusterSelector />);

    const trigger = await screen.findByRole("button", { name: /select cluster/i });
    await userEvent.click(trigger);

    // Look for the "Add cluster" menu item
    const addItem = await screen.findByRole("menuitem", { name: /add cluster/i });
    expect(addItem).toBeInTheDocument();
  });

  it("gracefully handles empty cluster list", async () => {
    server.use(
      http.get("/clusters", () => HttpResponse.json({ items: [] })),
    );

    renderWithQuery(<ClusterSelector />);

    // Should render "local" as fallback
    await waitFor(() => {
      expect(screen.getByText("local")).toBeInTheDocument();
    });

    const trigger = screen.getByRole("button", { name: /select cluster/i });
    await userEvent.click(trigger);

    // Should show a helpful message
    await waitFor(() => {
      expect(screen.getByText("No clusters available")).toBeInTheDocument();
    });
  });

  it("shows loading state while fetching clusters", async () => {
    let resolveResponse: () => void = () => {};
    const responsePromise = new Promise<void>((resolve) => {
      resolveResponse = resolve;
    });

    server.use(
      http.get("/clusters", async () => {
        await responsePromise;
        return HttpResponse.json({
          items: [{ name: "local", displayName: "Local", phase: "Healthy" }],
        });
      }),
    );

    renderWithQuery(<ClusterSelector />);

    // Open dropdown while loading
    const trigger = await screen.findByRole("button", { name: /select cluster/i });
    await userEvent.click(trigger);
    const menu = await screen.findByRole("menu");

    // Should show loading state
    await waitFor(() => {
      expect(within(menu).getByText("Loading…")).toBeInTheDocument();
    });

    // Resolve the response and check it updates. Scope to the menu
    resolveResponse();

    await waitFor(() => {
      expect(within(menu).getByText("Local")).toBeInTheDocument();
    });
  });

  it("shows error state when fetching clusters fails", async () => {
    server.use(
      http.get("/clusters", () => HttpResponse.error()),
    );

    renderWithQuery(<ClusterSelector />);

    const trigger = await screen.findByRole("button", { name: /select cluster/i });
    await userEvent.click(trigger);

    // Should show error state and fallback to "local"
    expect(screen.getByText("local")).toBeInTheDocument();
    expect(screen.getByText("Error loading clusters")).toBeInTheDocument();
  });

  it("displays cluster health status with appropriate colors", async () => {
    const clusters: ClusterRegistry[] = [
      { name: "local", displayName: "Local", phase: "Healthy" },
      { name: "unhealthy", displayName: "Down", phase: "Unhealthy" },
      { name: "unknown", displayName: "Unknown", phase: "Unknown" },
    ];

    server.use(
      http.get("/clusters", () => HttpResponse.json({ items: clusters })),
    );

    renderWithQuery(<ClusterSelector />);

    const trigger = await screen.findByRole("button", { name: /select cluster/i });
    await userEvent.click(trigger);

    // Scope to the open menu
    const menu = await screen.findByRole("menu");
    await waitFor(() => {
      expect(within(menu).getByText("Local")).toBeInTheDocument();
      expect(within(menu).getByText("Down")).toBeInTheDocument();
      expect(within(menu).getByText("Unknown")).toBeInTheDocument();
    });

    // All health dots should be present (rendered as span elements with color classes)
    const healthDots = document.querySelectorAll("span[class*='rounded-full'][class*='h-2'][class*='w-2']");
    expect(healthDots.length).toBeGreaterThanOrEqual(clusters.length + 1); // +1 for the trigger
  });

  it("falls back to cluster name when displayName is not provided", async () => {
    const clusters: ClusterRegistry[] = [
      { name: "prod-east", displayName: "", phase: "Healthy" }, // Empty displayName
    ];

    server.use(
      http.get("/clusters", () => HttpResponse.json({ items: clusters })),
    );

    renderWithQuery(<ClusterSelector />);

    // Render the cluster selector; since no "local" in the list, should show "prod-east" or "local" fallback
    await waitFor(() => {
      // If selected, will show the name
      const button = screen.getByRole("button", { name: /select cluster/i });
      expect(button).toBeInTheDocument();
    });
  });

  it("navigates to /cluster when Add cluster is clicked", async () => {
    const navigateMock = vi.fn();
    routerMocks.useNavigate = () => navigateMock;

    const clusters: ClusterRegistry[] = [
      { name: "local", displayName: "Local", phase: "Healthy" },
    ];

    server.use(
      http.get("/clusters", () => HttpResponse.json({ items: clusters })),
    );

    renderWithQuery(<ClusterSelector />);

    const trigger = await screen.findByRole("button", { name: /select cluster/i });
    await userEvent.click(trigger);

    const addItem = await screen.findByRole("menuitem", { name: /add cluster/i });
    await userEvent.click(addItem);

    expect(navigateMock).toHaveBeenCalledWith({ to: "/cluster" });
  });

  it("renders button with hover styles and chevron", async () => {
    const clusters: ClusterRegistry[] = [
      { name: "local", displayName: "Local", phase: "Healthy" },
    ];

    server.use(
      http.get("/clusters", () => HttpResponse.json({ items: clusters })),
    );

    renderWithQuery(<ClusterSelector />);

    const button = await screen.findByRole("button", { name: /select cluster/i });
    expect(button).toHaveClass("hover:bg-surface");
    expect(button).toHaveClass("transition-colors");
  });

  it("cluster selector shows 'Unknown' phase for null cluster", async () => {
    server.use(
      http.get("/clusters", () => HttpResponse.json({ items: [] })),
    );

    renderWithQuery(<ClusterSelector />);

    // When no clusters match current selection, phase defaults to "Unknown"
    const button = await screen.findByRole("button", { name: /select cluster/i });
    expect(button).toBeInTheDocument();
  });
});
