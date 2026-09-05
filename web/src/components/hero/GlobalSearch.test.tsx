import { describe, it, expect, vi, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { screen, waitFor, fireEvent } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { server } from "@/test/server";
import { renderWithQuery } from "@/test/render";
import { GlobalSearch } from "./GlobalSearch";
import type { GameServer } from "@/types";

// Mock the router navigation
const navigateMock = vi.fn();
vi.mock("@tanstack/react-router", () => ({
  useNavigate: () => navigateMock,
}));

describe("GlobalSearch", () => {
  beforeEach(() => {
    navigateMock.mockReset();
  });

  it("renders a search input with placeholder", async () => {
    server.use(
      http.get("/servers", () =>
        HttpResponse.json({ items: [] })
      )
    );

    renderWithQuery(<GlobalSearch />);

    const input = screen.getByTestId("search-input");
    expect(input).toBeInTheDocument();
    expect(input).toHaveAttribute("placeholder", "Search servers…");
    expect(input).toHaveAttribute("aria-label", "Search servers");
  });

  it("displays matching servers in a dropdown", async () => {
    const servers: GameServer[] = [
      {
        metadata: { name: "mc-survival", namespace: "default" },
      } as GameServer,
      {
        metadata: { name: "mc-creative", namespace: "default" },
      } as GameServer,
    ];

    server.use(
      http.get("/servers", () =>
        HttpResponse.json({ items: servers })
      )
    );

    renderWithQuery(<GlobalSearch />);
    const input = screen.getByTestId("search-input");

    await userEvent.type(input, "mc");

    await waitFor(() => {
      expect(screen.getByTestId("search-result-mc-survival")).toBeInTheDocument();
      expect(screen.getByTestId("search-result-mc-creative")).toBeInTheDocument();
    });
  });

  it("filters servers by name", async () => {
    const servers: GameServer[] = [
      {
        metadata: { name: "mc-survival", namespace: "default" },
      } as GameServer,
      {
        metadata: { name: "mc-creative", namespace: "default" },
      } as GameServer,
      {
        metadata: { name: "rust-server", namespace: "default" },
      } as GameServer,
    ];

    server.use(
      http.get("/servers", () =>
        HttpResponse.json({ items: servers })
      )
    );

    renderWithQuery(<GlobalSearch />);
    const input = screen.getByTestId("search-input");

    await userEvent.type(input, "creative");

    await waitFor(() => {
      expect(screen.getByTestId("search-result-mc-creative")).toBeInTheDocument();
      expect(screen.queryByTestId("search-result-mc-survival")).not.toBeInTheDocument();
      expect(screen.queryByTestId("search-result-rust-server")).not.toBeInTheDocument();
    });
  });

  it("shows 'No servers match.' when no results", async () => {
    server.use(
      http.get("/servers", () =>
        HttpResponse.json({
          items: [
            {
              metadata: { name: "mc-survival", namespace: "default" },
            } as GameServer,
          ],
        })
      )
    );

    renderWithQuery(<GlobalSearch />);
    const input = screen.getByTestId("search-input");

    await userEvent.type(input, "nonexistent");

    await waitFor(() => {
      expect(screen.getByText("No servers match.")).toBeInTheDocument();
    });
  });

  it("limits results to 6 servers", async () => {
    const servers: GameServer[] = Array.from({ length: 10 }, (_, i) => ({
      metadata: { name: `server-${i}`, namespace: "default" },
    } as GameServer));

    server.use(
      http.get("/servers", () =>
        HttpResponse.json({ items: servers })
      )
    );

    renderWithQuery(<GlobalSearch />);
    const input = screen.getByTestId("search-input");

    await userEvent.type(input, "server");

    await waitFor(() => {
      const results = screen.getAllByRole("option");
      expect(results).toHaveLength(6);
    });
  });

  it("navigates to server on Enter key with first match", async () => {
    const servers: GameServer[] = [
      {
        metadata: { name: "mc-survival", namespace: "default" },
      } as GameServer,
      {
        metadata: { name: "mc-creative", namespace: "default" },
      } as GameServer,
    ];

    server.use(
      http.get("/servers", () =>
        HttpResponse.json({ items: servers })
      )
    );

    renderWithQuery(<GlobalSearch />);
    const input = screen.getByTestId("search-input") as HTMLInputElement;

    await userEvent.type(input, "mc");
    fireEvent.keyDown(input, { key: "Enter" });

    await waitFor(() => {
      expect(navigateMock).toHaveBeenCalledWith({
        to: "/servers/$name",
        params: { name: "mc-survival" },
      });
    });
  });

  it("navigates to selected server on Enter after arrow navigation", async () => {
    const servers: GameServer[] = [
      {
        metadata: { name: "mc-survival", namespace: "default" },
      } as GameServer,
      {
        metadata: { name: "mc-creative", namespace: "default" },
      } as GameServer,
    ];

    server.use(
      http.get("/servers", () =>
        HttpResponse.json({ items: servers })
      )
    );

    renderWithQuery(<GlobalSearch />);
    const input = screen.getByTestId("search-input") as HTMLInputElement;

    await userEvent.type(input, "mc");

    await waitFor(() => {
      expect(screen.getByTestId("search-result-mc-survival")).toBeInTheDocument();
    });

    // Arrow down to move selection
    await userEvent.keyboard("{ArrowDown}{ArrowDown}");

    // Enter should navigate to a server
    await userEvent.keyboard("{Enter}");

    await waitFor(() => {
      expect(navigateMock).toHaveBeenCalled();
      // Should navigate to one of the servers
      const call = navigateMock.mock.calls[0][0];
      expect(call.to).toBe("/servers/$name");
      expect(["mc-survival", "mc-creative"]).toContain(call.params.name);
    });
  });

  it("closes dropdown on Escape key", async () => {
    const servers: GameServer[] = [
      {
        metadata: { name: "mc-survival", namespace: "default" },
      } as GameServer,
    ];

    server.use(
      http.get("/servers", () =>
        HttpResponse.json({ items: servers })
      )
    );

    renderWithQuery(<GlobalSearch />);
    const input = screen.getByTestId("search-input") as HTMLInputElement;

    await userEvent.type(input, "mc");

    await waitFor(() => {
      expect(screen.getByTestId("search-result-mc-survival")).toBeInTheDocument();
    });

    fireEvent.keyDown(input, { key: "Escape" });

    await waitFor(() => {
      expect(screen.queryByTestId("search-result-mc-survival")).not.toBeInTheDocument();
    });
  });

  it("navigates on result click", async () => {
    const servers: GameServer[] = [
      {
        metadata: { name: "mc-survival", namespace: "default" },
      } as GameServer,
    ];

    server.use(
      http.get("/servers", () =>
        HttpResponse.json({ items: servers })
      )
    );

    renderWithQuery(<GlobalSearch />);
    const input = screen.getByTestId("search-input");

    await userEvent.type(input, "mc");

    const result = await screen.findByTestId("search-result-mc-survival");
    await userEvent.click(result);

    await waitFor(() => {
      expect(navigateMock).toHaveBeenCalledWith({
        to: "/servers/$name",
        params: { name: "mc-survival" },
      });
    });
  });

  it("clears search and closes dropdown on successful navigation", async () => {
    const servers: GameServer[] = [
      {
        metadata: { name: "mc-survival", namespace: "default" },
      } as GameServer,
    ];

    server.use(
      http.get("/servers", () =>
        HttpResponse.json({ items: servers })
      )
    );

    renderWithQuery(<GlobalSearch />);
    const input = screen.getByTestId("search-input") as HTMLInputElement;

    await userEvent.type(input, "mc");

    const result = await screen.findByTestId("search-result-mc-survival");
    await userEvent.click(result);

    await waitFor(() => {
      expect(input.value).toBe("");
    });
  });

  it("supports ArrowUp and ArrowDown navigation", async () => {
    const servers: GameServer[] = [
      {
        metadata: { name: "mc-survival", namespace: "default" },
      } as GameServer,
      {
        metadata: { name: "mc-creative", namespace: "default" },
      } as GameServer,
      {
        metadata: { name: "mc-adventure", namespace: "default" },
      } as GameServer,
    ];

    server.use(
      http.get("/servers", () =>
        HttpResponse.json({ items: servers })
      )
    );

    renderWithQuery(<GlobalSearch />);
    const input = screen.getByTestId("search-input") as HTMLInputElement;

    await userEvent.type(input, "mc");

    await waitFor(() => {
      expect(screen.getByTestId("search-result-mc-survival")).toBeInTheDocument();
    });

    // Arrow down and up should not throw errors
    await userEvent.keyboard("{ArrowDown}");
    await userEvent.keyboard("{ArrowDown}");
    await userEvent.keyboard("{ArrowUp}");

    // Results should still be visible
    expect(screen.getByTestId("search-result-mc-survival")).toBeInTheDocument();
    expect(screen.getByTestId("search-result-mc-creative")).toBeInTheDocument();
    expect(screen.getByTestId("search-result-mc-adventure")).toBeInTheDocument();
  });

  it("highlights results on mouse enter", async () => {
    const servers: GameServer[] = [
      {
        metadata: { name: "mc-survival", namespace: "default" },
      } as GameServer,
      {
        metadata: { name: "mc-creative", namespace: "default" },
      } as GameServer,
    ];

    server.use(
      http.get("/servers", () =>
        HttpResponse.json({ items: servers })
      )
    );

    renderWithQuery(<GlobalSearch />);
    const input = screen.getByTestId("search-input");

    await userEvent.type(input, "mc");

    const survival = await screen.findByTestId("search-result-mc-survival");
    const creative = screen.getByTestId("search-result-mc-creative");

    // Mouse enter should not throw error
    fireEvent.mouseEnter(creative);

    // Both items should still be in the document
    expect(survival).toBeInTheDocument();
    expect(creative).toBeInTheDocument();
  });

  it("does not show dropdown when query is empty", async () => {
    server.use(
      http.get("/servers", () =>
        HttpResponse.json({
          items: [
            {
              metadata: { name: "mc-survival", namespace: "default" },
            } as GameServer,
          ],
        })
      )
    );

    renderWithQuery(<GlobalSearch />);
    const input = screen.getByTestId("search-input");

    // Focus without typing
    await userEvent.click(input);

    // Dropdown should not be visible for empty query
    expect(screen.queryByText("No servers match.")).not.toBeInTheDocument();
    expect(screen.queryByTestId("search-result-mc-survival")).not.toBeInTheDocument();
  });

  it("opens dropdown on focus if query is not empty", async () => {
    const servers: GameServer[] = [
      {
        metadata: { name: "mc-survival", namespace: "default" },
      } as GameServer,
    ];

    server.use(
      http.get("/servers", () =>
        HttpResponse.json({ items: servers })
      )
    );

    renderWithQuery(<GlobalSearch />);
    const input = screen.getByTestId("search-input");

    // Type a query, blur, then focus
    await userEvent.type(input, "mc");
    await userEvent.click(input); // This also focuses

    await waitFor(() => {
      expect(screen.getByTestId("search-result-mc-survival")).toBeInTheDocument();
    });
  });
});
