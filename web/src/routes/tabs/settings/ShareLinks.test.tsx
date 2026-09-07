import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import type { ReactNode } from "react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { setupServer } from "msw/node";
import { http, HttpResponse } from "msw";
import { ShareLinksSection } from "./ShareLinks";
import type { ShareLink } from "@/types";

const mockLinks: ShareLink[] = [
  {
    id: "link-1",
    createdAt: "2026-07-28T00:00:00Z",
    expiresAt: "2026-08-04T00:00:00Z",
    canStart: true,
  },
  {
    id: "link-2",
    createdAt: "2026-06-01T00:00:00Z",
    expiresAt: "2026-06-08T00:00:00Z",
    canStart: false,
  },
  {
    id: "link-3",
    createdAt: "2026-05-12T00:00:00Z",
    expiresAt: "2026-05-19T00:00:00Z",
    canStart: true,
  },
];

const queryClient = new QueryClient({
  defaultOptions: {
    queries: { retry: false },
    mutations: { retry: false },
  },
});

const Wrapper = ({ children }: { children: ReactNode }) => (
  <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
);

const server = setupServer(
  http.get(/\/servers\/[^/]+:shares$/, () =>
    HttpResponse.json(mockLinks),
  ),
  http.post(/\/servers\/[^/]+:shares$/, async ({ request }) => {
    const body = (await request.json()) as { expiresIn?: string; canStart: boolean };
    const newLink: ShareLink = {
      id: `link-${Date.now()}`,
      createdAt: new Date().toISOString(),
      expiresAt: new Date(Date.now() + 7 * 24 * 60 * 60 * 1000).toISOString(),
      canStart: body.canStart,
      token: "test-token-" + Math.random().toString(36).slice(2),
    };
    return HttpResponse.json(newLink);
  }),
  http.delete("/servers/:name/shares/:id", () =>
    new HttpResponse(null, { status: 204 }),
  ),
);

describe("ShareLinksSection", () => {
  beforeEach(() => {
    server.listen();
    queryClient.clear();
    vi.clearAllMocks();
  });

  afterEach(() => {
    server.close();
  });

  // List state tests
  it("renders title and description", async () => {
    render(<ShareLinksSection name="mc-survival" />, { wrapper: Wrapper });
    expect(screen.getByText("Share links")).toBeInTheDocument();
    expect(
      screen.getByText(/Let people without a Gameplane account/),
    ).toBeInTheDocument();
  });

  it("renders Create link button in header", () => {
    render(<ShareLinksSection name="mc-survival" />, { wrapper: Wrapper });
    const btn = screen.getByRole("button", { name: /Create link/ });
    expect(btn).toBeInTheDocument();
  });

  // Empty state test
  it("shows empty state when no links exist", async () => {
    server.use(
      http.get(/\/servers\/[^/]+:shares$/, () =>
        HttpResponse.json([]),
      ),
    );
    render(<ShareLinksSection name="mc-survival" />, { wrapper: Wrapper });
    await waitFor(() => {
      expect(screen.getByText("No share links yet")).toBeInTheDocument();
    });
    expect(
      screen.getByText(/Create a link so a friend without a Gameplane/),
    ).toBeInTheDocument();
  });

  // Table rendering tests
  it("renders table with share links when they exist", async () => {
    render(<ShareLinksSection name="mc-survival" />, { wrapper: Wrapper });
    await waitFor(() => {
      expect(screen.getByRole("table")).toBeInTheDocument();
    });
    // Table should contain date strings
    expect(screen.getByText("Jul 28, 2026")).toBeInTheDocument();
    expect(screen.getByText("Jun 1, 2026")).toBeInTheDocument();
  });

  it("renders column headers", async () => {
    render(<ShareLinksSection name="mc-survival" />, { wrapper: Wrapper });
    await waitFor(() => {
      expect(screen.getByText("Created")).toBeInTheDocument();
      expect(screen.getByText("Expires")).toBeInTheDocument();
    });
  });

  it("displays can-start capability as 'Can start' chip", async () => {
    render(<ShareLinksSection name="mc-survival" />, { wrapper: Wrapper });
    await waitFor(() => {
      expect(screen.getByText("Can start")).toBeInTheDocument();
    });
  });

  it("displays view-only capability as 'View only' chip", async () => {
    render(<ShareLinksSection name="mc-survival" />, { wrapper: Wrapper });
    await waitFor(() => {
      expect(screen.getByText("View only")).toBeInTheDocument();
    });
  });

  it("renders Revoke button for each link", async () => {
    render(<ShareLinksSection name="mc-survival" />, { wrapper: Wrapper });
    await waitFor(() => {
      const revokeButtons = screen.getAllByRole("button", { name: "Revoke" });
      expect(revokeButtons).toHaveLength(3);
    });
  });

  // Create dialog tests
  it("opens create dialog when Create link button is clicked", async () => {
    const user = userEvent.setup();
    render(<ShareLinksSection name="mc-survival" />, { wrapper: Wrapper });
    const createBtn = screen.getByRole("button", { name: /Create link/ });
    await user.click(createBtn);
    await waitFor(() => {
      expect(screen.getByText("Create share link for mc-survival")).toBeInTheDocument();
    });
  });

  it("shows expiry options in create dialog", async () => {
    const user = userEvent.setup();
    render(<ShareLinksSection name="mc-survival" />, { wrapper: Wrapper });
    const createBtn = screen.getByRole("button", { name: /Create link/ });
    await user.click(createBtn);
    await waitFor(() => {
      expect(screen.getByText("Expires in")).toBeInTheDocument();
    });
  });

  it("includes allow-start switch in create dialog", async () => {
    const user = userEvent.setup();
    render(<ShareLinksSection name="mc-survival" />, { wrapper: Wrapper });
    const createBtn = screen.getByRole("button", { name: /Create link/ });
    await user.click(createBtn);
    await waitFor(() => {
      expect(screen.getByText("Allow starting the server")).toBeInTheDocument();
    });
  });

  it("closes create dialog on Cancel", async () => {
    const user = userEvent.setup();
    render(<ShareLinksSection name="mc-survival" />, { wrapper: Wrapper });
    const createBtn = screen.getByRole("button", { name: /Create link/ });
    await user.click(createBtn);
    await waitFor(() => {
      expect(screen.getByText("Create share link for mc-survival")).toBeInTheDocument();
    });
    const cancelBtn = screen.getByRole("button", { name: "Cancel" });
    await user.click(cancelBtn);
    await waitFor(() => {
      expect(screen.queryByText("Create share link for mc-survival")).not.toBeInTheDocument();
    });
  });

  it("creates link with selected expiry and canStart", async () => {
    const user = userEvent.setup();
    render(<ShareLinksSection name="mc-survival" />, { wrapper: Wrapper });
    const createBtn = screen.getByRole("button", { name: /Create link/ });
    await user.click(createBtn);

    // Find and select expiry dropdown
    const selectEl = screen.getByDisplayValue("168h") as HTMLSelectElement;
    await user.selectOptions(selectEl, "720h");

    // Toggle allow-start switch
    const switchEl = screen.getByRole("checkbox") as HTMLInputElement;
    await user.click(switchEl);

    // Submit
    const createConfirmBtn = screen.getByRole("button", { name: "Create link" });
    await user.click(createConfirmBtn);

    // Should open created dialog
    await waitFor(() => {
      expect(screen.getByText("Share link created")).toBeInTheDocument();
    });
  });

  it("shows error on failed create", async () => {
    const user = userEvent.setup();
    server.use(
      http.post(/\/servers\/[^/]+:shares$/, () =>
        HttpResponse.json({ error: "Permission denied" }, { status: 403 }),
      ),
    );
    render(<ShareLinksSection name="mc-survival" />, { wrapper: Wrapper });
    const createBtn = screen.getByRole("button", { name: /Create link/ });
    await user.click(createBtn);
    await waitFor(() => {
      expect(screen.getByText("Create share link for mc-survival")).toBeInTheDocument();
    });
    const createConfirmBtn = screen.getByRole("button", { name: "Create link" });
    await user.click(createConfirmBtn);
    await waitFor(() => {
      expect(screen.getByText(/Failed to create link/)).toBeInTheDocument();
    });
  });

  // Created dialog tests
  it("displays created link URL in created dialog", async () => {
    const user = userEvent.setup();
    render(<ShareLinksSection name="mc-survival" />, { wrapper: Wrapper });
    const createBtn = screen.getByRole("button", { name: /Create link/ });
    await user.click(createBtn);
    await waitFor(() => {
      expect(screen.getByText("Create share link for mc-survival")).toBeInTheDocument();
    });
    const createConfirmBtn = screen.getByRole("button", { name: "Create link" });
    await user.click(createConfirmBtn);
    await waitFor(() => {
      expect(screen.getByText("Share link created")).toBeInTheDocument();
      expect(screen.getByText(/You will not see this link again/)).toBeInTheDocument();
    });
  });

  it("copies link URL to clipboard", async () => {
    const user = userEvent.setup();
    const clipboardSpy = vi.spyOn(navigator.clipboard, "writeText");

    render(<ShareLinksSection name="mc-survival" />, { wrapper: Wrapper });
    const createBtn = screen.getByRole("button", { name: /Create link/ });
    await user.click(createBtn);
    await waitFor(() => {
      expect(screen.getByText("Create share link for mc-survival")).toBeInTheDocument();
    });
    const createConfirmBtn = screen.getByRole("button", { name: "Create link" });
    await user.click(createConfirmBtn);

    await waitFor(() => {
      expect(screen.getByText("Share link created")).toBeInTheDocument();
    });

    const copyBtn = screen.getByRole("button", { name: /Copy link/ });
    await user.click(copyBtn);

    await waitFor(() => {
      expect(clipboardSpy).toHaveBeenCalled();
    });
    clipboardSpy.mockRestore();
  });

  it("closes created dialog on Done", async () => {
    const user = userEvent.setup();
    render(<ShareLinksSection name="mc-survival" />, { wrapper: Wrapper });
    const createBtn = screen.getByRole("button", { name: /Create link/ });
    await user.click(createBtn);
    await waitFor(() => {
      expect(screen.getByText("Create share link for mc-survival")).toBeInTheDocument();
    });
    const createConfirmBtn = screen.getByRole("button", { name: "Create link" });
    await user.click(createConfirmBtn);

    await waitFor(() => {
      expect(screen.getByText("Share link created")).toBeInTheDocument();
    });

    const doneBtn = screen.getByRole("button", { name: "Done" });
    await user.click(doneBtn);

    await waitFor(() => {
      expect(screen.queryByText("Share link created")).not.toBeInTheDocument();
    });
  });

  // Revoke dialog tests
  it("opens revoke dialog when Revoke button is clicked", async () => {
    const user = userEvent.setup();
    render(<ShareLinksSection name="mc-survival" />, { wrapper: Wrapper });
    await waitFor(() => {
      expect(screen.getByRole("table")).toBeInTheDocument();
    });
    const revokeButtons = screen.getAllByRole("button", { name: "Revoke" });
    await user.click(revokeButtons[0]);
    await waitFor(() => {
      expect(screen.getByText("Revoke this share link?")).toBeInTheDocument();
    });
  });

  it("shows confirmation message in revoke dialog", async () => {
    const user = userEvent.setup();
    render(<ShareLinksSection name="mc-survival" />, { wrapper: Wrapper });
    await waitFor(() => {
      expect(screen.getByRole("table")).toBeInTheDocument();
    });
    const revokeButtons = screen.getAllByRole("button", { name: "Revoke" });
    await user.click(revokeButtons[0]);
    await waitFor(() => {
      expect(
        screen.getByText(/will immediately lose access to/),
      ).toBeInTheDocument();
    });
  });

  it("closes revoke dialog on Cancel", async () => {
    const user = userEvent.setup();
    render(<ShareLinksSection name="mc-survival" />, { wrapper: Wrapper });
    await waitFor(() => {
      expect(screen.getByRole("table")).toBeInTheDocument();
    });
    const revokeButtons = screen.getAllByRole("button", { name: "Revoke" });
    await user.click(revokeButtons[0]);
    await waitFor(() => {
      expect(screen.getByText("Revoke this share link?")).toBeInTheDocument();
    });
    const cancelBtn = screen.getAllByRole("button", { name: "Cancel" })[0];
    await user.click(cancelBtn);
    await waitFor(() => {
      expect(screen.queryByText("Revoke this share link?")).not.toBeInTheDocument();
    });
  });

  it("revokes link on confirmation", async () => {
    const user = userEvent.setup();
    render(<ShareLinksSection name="mc-survival" />, { wrapper: Wrapper });
    await waitFor(() => {
      expect(screen.getByRole("table")).toBeInTheDocument();
    });
    const revokeButtons = screen.getAllByRole("button", { name: "Revoke" });
    await user.click(revokeButtons[0]);
    await waitFor(() => {
      expect(screen.getByText("Revoke this share link?")).toBeInTheDocument();
    });
    const revokeConfirmBtn = screen.getByRole("button", { name: "Revoke link" });
    await user.click(revokeConfirmBtn);
    await waitFor(() => {
      expect(screen.queryByText("Revoke this share link?")).not.toBeInTheDocument();
    });
  });

  it("shows error on failed revoke", async () => {
    const user = userEvent.setup();
    server.use(
      http.delete("/servers/:name/shares/:id", () =>
        HttpResponse.json({ error: "Permission denied" }, { status: 403 }),
      ),
    );
    render(<ShareLinksSection name="mc-survival" />, { wrapper: Wrapper });
    await waitFor(() => {
      expect(screen.getByRole("table")).toBeInTheDocument();
    });
    const revokeButtons = screen.getAllByRole("button", { name: "Revoke" });
    await user.click(revokeButtons[0]);
    await waitFor(() => {
      expect(screen.getByText("Revoke this share link?")).toBeInTheDocument();
    });
    const revokeConfirmBtn = screen.getByRole("button", { name: "Revoke link" });
    await user.click(revokeConfirmBtn);
    await waitFor(() => {
      expect(screen.getByText(/Failed to revoke link/)).toBeInTheDocument();
    });
  });

  // Multi-cluster tests
  it("accepts namespace prop", () => {
    render(<ShareLinksSection name="mc-survival" ns="custom-ns" />, { wrapper: Wrapper });
    expect(screen.getByText("Share links")).toBeInTheDocument();
  });

  // Status determination tests
  it("shows Active status for non-expired links", async () => {
    render(<ShareLinksSection name="mc-survival" />, { wrapper: Wrapper });
    await waitFor(() => {
      expect(screen.getByText("Active")).toBeInTheDocument();
    });
  });

  it("shows Expired status for past-expiry links", async () => {
    server.use(
      http.get(/\/servers\/[^/]+:shares$/, () =>
        HttpResponse.json([
          {
            id: "expired-link",
            createdAt: "2026-01-01T00:00:00Z",
            expiresAt: "2024-01-01T00:00:00Z",
            canStart: false,
          },
        ]),
      ),
    );
    render(<ShareLinksSection name="mc-survival" />, { wrapper: Wrapper });
    await waitFor(() => {
      expect(screen.getByText("Expired")).toBeInTheDocument();
    });
  });
});
