import { afterEach, describe, it, expect, vi } from "vitest";
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
  useNavigate: () => vi.fn(),
  useSearch: () => ({}),
  useParams: () => ({}),
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

  // Regression: usedStorageBytes/totalStorageBytes are provisioned-vs-physical,
  // not used-vs-total, so networked storage can legitimately read >100% — that
  // must present as an explicit overcommit state, not a silently broken meter.
  it("flags storage as overcommitted when provisioned exceeds physical capacity", async () => {
    server.use(
      http.get("/servers", () => HttpResponse.json({ items: [] })),
      http.get("/cluster/stats", () =>
        HttpResponse.json(makeClusterStats({ usedStorageBytes: 102_000_000_000, totalStorageBytes: 86_000_000_000 })),
      ),
    );
    renderWithQuery(<ServersPage />);
    await screen.findByText("Storage provisioned");
    expect(await screen.findAllByText(/overcommitted/i)).not.toHaveLength(0);
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

  it("renders the Filter button with no badge when no facets are applied", async () => {
    server.use(
      http.get("/servers", () =>
        HttpResponse.json({
          items: [
            makeServer({ metadata: { name: "alpha", namespace: "gameplane-games" } }),
          ],
        }),
      ),
    );
    renderWithQuery(<ServersPage />);
    await screen.findByText("alpha");
    const filterButton = screen.getByRole("button", { name: /Filter/i });
    expect(filterButton).toBeInTheDocument();
    // Badge text should not be a number when no facets are applied
    expect(filterButton.textContent).not.toMatch(/\d/);
  });

  it("opens the popover and lists distinct games and namespaces", async () => {
    server.use(
      http.get("/servers", () =>
        HttpResponse.json({
          items: [
            makeServer({
              metadata: { name: "alpha", namespace: "gameplane-games" },
              spec: { templateRef: { name: "minecraft-java" } },
            }),
            makeServer({
              metadata: { name: "beta", namespace: "other-ns" },
              spec: { templateRef: { name: "valheim" } },
            }),
            makeServer({
              metadata: { name: "gamma", namespace: "gameplane-games" },
              spec: { templateRef: { name: "minecraft-java" } },
            }),
          ],
        }),
      ),
    );
    renderWithQuery(<ServersPage />);
    await screen.findByText("alpha");

    const filterButton = screen.getByRole("button", { name: /Filter/i });
    await userEvent.click(filterButton);

    const menu = await screen.findByRole("menu");
    expect(within(menu).getByRole("menuitemcheckbox", { name: "minecraft-java" })).toBeInTheDocument();
    expect(within(menu).getByRole("menuitemcheckbox", { name: "valheim" })).toBeInTheDocument();
    expect(within(menu).getByRole("menuitemcheckbox", { name: "gameplane-games" })).toBeInTheDocument();
    expect(within(menu).getByRole("menuitemcheckbox", { name: "other-ns" })).toBeInTheDocument();
  });

  it("filters servers by game when a game is selected and Apply is clicked", async () => {
    server.use(
      http.get("/servers", () =>
        HttpResponse.json({
          items: [
            makeServer({
              metadata: { name: "alpha", namespace: "gameplane-games" },
              spec: { templateRef: { name: "minecraft-java" } },
            }),
            makeServer({
              metadata: { name: "beta", namespace: "gameplane-games" },
              spec: { templateRef: { name: "valheim" } },
            }),
          ],
        }),
      ),
    );
    renderWithQuery(<ServersPage />);
    await screen.findByText("alpha");

    // Open filter
    const filterButton = screen.getByRole("button", { name: /Filter/i });
    await userEvent.click(filterButton);

    // Select minecraft-java
    const minecraftCheckbox = screen.getByRole("menuitemcheckbox", { name: "minecraft-java" });
    await userEvent.click(minecraftCheckbox);

    // Click Apply
    const applyButton = screen.getByRole("button", { name: /^Apply$/i });
    await userEvent.click(applyButton);

    // Only alpha should be visible, beta should not
    await waitFor(() => expect(screen.queryByText("beta")).not.toBeInTheDocument());
    expect(screen.getByText("alpha")).toBeInTheDocument();
  });

  it("clears selected facets when Clear is clicked", async () => {
    server.use(
      http.get("/servers", () =>
        HttpResponse.json({
          items: [
            makeServer({
              metadata: { name: "alpha", namespace: "gameplane-games" },
              spec: { templateRef: { name: "minecraft-java" } },
            }),
            makeServer({
              metadata: { name: "beta", namespace: "gameplane-games" },
              spec: { templateRef: { name: "valheim" } },
            }),
          ],
        }),
      ),
    );
    renderWithQuery(<ServersPage />);
    await screen.findByText("alpha");

    // Open filter
    const filterButton = screen.getByRole("button", { name: /Filter/i });
    await userEvent.click(filterButton);

    // Select minecraft-java
    const minecraftCheckbox = screen.getByRole("menuitemcheckbox", { name: "minecraft-java" });
    await userEvent.click(minecraftCheckbox);

    // Click Clear (should keep popover open)
    const clearButton = screen.getByRole("button", { name: /^Clear$/i });
    await userEvent.click(clearButton);

    // minecraft-java should no longer be checked
    expect(minecraftCheckbox).not.toHaveAttribute("data-state", "checked");
  });

  it("shows count badge when facets are applied", async () => {
    server.use(
      http.get("/servers", () =>
        HttpResponse.json({
          items: [
            makeServer({
              metadata: { name: "alpha", namespace: "gameplane-games" },
              spec: { templateRef: { name: "minecraft-java" } },
            }),
            makeServer({
              metadata: { name: "beta", namespace: "other-ns" },
              spec: { templateRef: { name: "valheim" } },
            }),
          ],
        }),
      ),
    );
    renderWithQuery(<ServersPage />);
    await screen.findByText("alpha");

    // Open filter
    const filterButton = screen.getByRole("button", { name: /Filter/i });
    await userEvent.click(filterButton);

    // Select one game
    const minecraftCheckbox = screen.getByRole("menuitemcheckbox", { name: "minecraft-java" });
    await userEvent.click(minecraftCheckbox);

    // Select one namespace
    const otherNsCheckbox = screen.getByRole("menuitemcheckbox", { name: "other-ns" });
    await userEvent.click(otherNsCheckbox);

    // Click Apply
    const applyButton = screen.getByRole("button", { name: /^Apply$/i });
    await userEvent.click(applyButton);

    // Badge should show "2" (1 game + 1 namespace) — scope to the Filter
    // button so it doesn't collide with other "2"s (stat tiles, tab counts).
    await waitFor(() => {
      expect(within(filterButton).getByText("2")).toBeInTheDocument();
    });
  });

  it("composes status filter and game facet filter together", async () => {
    server.use(
      http.get("/servers", () =>
        HttpResponse.json({
          items: [
            makeServer({
              metadata: { name: "mc-running", namespace: "gameplane-games" },
              spec: { templateRef: { name: "minecraft-java" } },
              status: { phase: "Running" },
            }),
            makeServer({
              metadata: { name: "mc-stopped", namespace: "gameplane-games" },
              spec: { templateRef: { name: "minecraft-java" } },
              status: { phase: "Stopped" },
            }),
            makeServer({
              metadata: { name: "val-running", namespace: "gameplane-games" },
              spec: { templateRef: { name: "valheim" } },
              status: { phase: "Running" },
            }),
          ],
        }),
      ),
    );
    renderWithQuery(<ServersPage />);
    await screen.findByText("mc-running");

    // Apply game filter for minecraft-java
    const filterButton = screen.getByRole("button", { name: /Filter/i });
    await userEvent.click(filterButton);
    const minecraftCheckbox = screen.getByRole("menuitemcheckbox", { name: "minecraft-java" });
    await userEvent.click(minecraftCheckbox);
    const applyButton = screen.getByRole("button", { name: /^Apply$/i });
    await userEvent.click(applyButton);

    // Now apply status filter to "Running"
    const runningButton = screen.getByRole("button", { name: /Running/i });
    await userEvent.click(runningButton);

    // Only mc-running should be visible
    expect(screen.getByText("mc-running")).toBeInTheDocument();
    expect(screen.queryByText("mc-stopped")).not.toBeInTheDocument();
    expect(screen.queryByText("val-running")).not.toBeInTheDocument();
  });

  // The asleep flag threads through serverRowData -> ServerRow ->
  // ServerLifecycleActions untested before this — cover the Wake/Start
  // swap and the :wake call the same way ServerDetail.test.tsx does.
  it("shows Wake (not Start) on an asleep row and calls the :wake endpoint when clicked", async () => {
    const wakeHandler = vi.fn(() => new HttpResponse(null, { status: 202 }));
    server.use(
      http.get("/servers", () =>
        HttpResponse.json({
          items: [
            makeServer({
              metadata: { name: "sleepy", namespace: "gameplane-games" },
              status: {
                phase: "Suspended",
                idle: { asleep: true, asleepSince: "2026-05-07T12:00:00Z" },
              },
            }),
          ],
        }),
      ),
      http.post(/\/servers\/[^/]+:wake$/, wakeHandler),
    );
    renderWithQuery(<ServersPage />);
    const row = (await screen.findByText("sleepy")).closest("tr") as HTMLElement;
    expect(within(row).getByTitle("Wake")).toBeInTheDocument();
    expect(within(row).queryByTitle("Start")).not.toBeInTheDocument();
    await userEvent.click(within(row).getByTitle("Wake"));
    await waitFor(() => expect(wakeHandler).toHaveBeenCalled());
  });

  // C1: an asleep server is phase Suspended, but :stop is still a real
  // action (it patches spec.suspend=true) — Stop must not be dead here.
  it("keeps Stop enabled for an asleep server", async () => {
    server.use(
      http.get("/servers", () =>
        HttpResponse.json({
          items: [
            makeServer({
              metadata: { name: "sleepy", namespace: "gameplane-games" },
              status: {
                phase: "Suspended",
                idle: { asleep: true, asleepSince: "2026-05-07T12:00:00Z" },
              },
            }),
          ],
        }),
      ),
    );
    renderWithQuery(<ServersPage />);
    const row = (await screen.findByText("sleepy")).closest("tr") as HTMLElement;
    expect(within(row).getByTitle("Stop")).not.toBeDisabled();
  });
});

// The page renders either a <table> (desktop) or a stacked card list
// (mobile), decided by useMediaQuery("(max-width: 767px)"). Everywhere else
// in this suite window.matchMedia is left at the global jsdom stub (always
// `matches: false`, see src/test/setup.ts), which is why the table has been
// what every test above exercises. These cases override matchMedia to force
// the mobile branch so ServerCard/StatChip get real coverage too.
describe("ServersPage mobile layout", () => {
  const ORIGINAL_MATCH_MEDIA = window.matchMedia;

  function setMobileViewport() {
    window.matchMedia = ((query: string) => ({
      matches: query === "(max-width: 767px)",
      media: query,
      onchange: null,
      addListener: () => {},
      removeListener: () => {},
      addEventListener: () => {},
      removeEventListener: () => {},
      dispatchEvent: () => false,
    })) as typeof window.matchMedia;
  }

  afterEach(() => {
    window.matchMedia = ORIGINAL_MATCH_MEDIA;
  });

  it("renders a stacked card list (with stat chips and lifecycle actions) instead of the table", async () => {
    setMobileViewport();
    server.use(
      http.get("/servers", () =>
        HttpResponse.json({
          items: [
            makeServer({
              metadata: { name: "mobile-alpha", namespace: "gameplane-games" },
              status: {
                phase: "Running",
                agent: {
                  cpuMillicores: 500,
                  cpuLimitMillicores: 2000,
                  memoryBytes: 536870912,
                  memoryLimitBytes: 1073741824,
                  playersOnline: 3,
                  playersMax: 10,
                },
              },
            }),
          ],
        }),
      ),
    );
    renderWithQuery(<ServersPage />);
    await screen.findByText("mobile-alpha");

    expect(screen.queryByRole("table")).not.toBeInTheDocument();
    expect(screen.getByText("3/10")).toBeInTheDocument();
    expect(screen.getByText("25%")).toBeInTheDocument();
    expect(screen.getByText("50%")).toBeInTheDocument();
    expect(screen.getByTitle("Start")).toBeInTheDocument();
    expect(screen.getByTitle("Stop")).toBeInTheDocument();
    expect(screen.getByTitle("Restart")).toBeInTheDocument();
  });

  it("shows an empty-state card and a 'Shared with you' section on mobile", async () => {
    setMobileViewport();
    server.use(
      http.get("/servers", () => HttpResponse.json({ items: [] })),
      http.get("/users/me/servers", () =>
        HttpResponse.json({
          items: [makeServer({ metadata: { name: "mobile-shared" }, status: { phase: "Stopped" } })],
        }),
      ),
    );
    renderWithQuery(<ServersPage />);
    expect(await screen.findByText(/Shared with you/i)).toBeInTheDocument();
    expect(screen.getByText("mobile-shared")).toBeInTheDocument();
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
  });

  it("shows the 'No servers match' card when the list is empty", async () => {
    setMobileViewport();
    server.use(http.get("/servers", () => HttpResponse.json({ items: [] })));
    renderWithQuery(<ServersPage />);
    expect(await screen.findByText("No servers match.")).toBeInTheDocument();
  });
});
