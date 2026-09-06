import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { WipeServerDialog } from "./WipeServerDialog";
import * as ServersAPI from "@/lib/endpoints";

// Mock the Servers API
vi.mock("@/lib/endpoints", () => ({
  Servers: {
    wipeData: vi.fn(),
  },
}));

const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
});

function renderWithProviders(component: React.ReactNode) {
  return render(<QueryClientProvider client={queryClient}>{component}</QueryClientProvider>);
}

describe("WipeServerDialog", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("does not render content when closed", () => {
    renderWithProviders(
      <WipeServerDialog
        name="mc-survival"
        open={false}
        onOpenChange={vi.fn()}
      />,
    );
    expect(screen.queryByText("Wipe world?")).not.toBeInTheDocument();
  });

  it("renders title + description when open", () => {
    renderWithProviders(
      <WipeServerDialog
        name="mc-survival"
        open
        onOpenChange={vi.fn()}
      />,
    );
    expect(screen.getByText("Wipe world?")).toBeInTheDocument();
    expect(screen.getByText(/This permanently deletes the world data/)).toBeInTheDocument();
    expect(screen.getByText("mc-survival")).toBeInTheDocument();
  });

  it("renders confirmation checkbox when open", () => {
    renderWithProviders(
      <WipeServerDialog
        name="mc-survival"
        open
        onOpenChange={vi.fn()}
      />,
    );
    expect(
      screen.getByRole("checkbox", {
        name: /I understand this will permanently delete the world data/,
      }),
    ).toBeInTheDocument();
  });

  it("disables confirm button until checkbox is checked", () => {
    renderWithProviders(
      <WipeServerDialog
        name="mc-survival"
        open
        onOpenChange={vi.fn()}
      />,
    );
    const confirmBtn = screen.getByRole("button", { name: "Wipe world" });
    expect(confirmBtn).toBeDisabled();
  });

  it("enables confirm button when checkbox is checked", async () => {
    const user = userEvent.setup();
    renderWithProviders(
      <WipeServerDialog
        name="mc-survival"
        open
        onOpenChange={vi.fn()}
      />,
    );
    const checkbox = screen.getByRole("checkbox", {
      name: /I understand this will permanently delete the world data/,
    });
    const confirmBtn = screen.getByRole("button", { name: "Wipe world" });

    expect(confirmBtn).toBeDisabled();
    await user.click(checkbox);
    expect(confirmBtn).toBeEnabled();
  });

  it("calls mutation when confirm is clicked", async () => {
    const user = userEvent.setup();
    const mockWipeData = vi.mocked(ServersAPI.Servers.wipeData).mockResolvedValue(undefined);

    renderWithProviders(
      <WipeServerDialog
        name="mc-survival"
        open
        onOpenChange={vi.fn()}
      />,
    );

    const checkbox = screen.getByRole("checkbox", {
      name: /I understand this will permanently delete the world data/,
    });
    await user.click(checkbox);

    const confirmBtn = screen.getByRole("button", { name: "Wipe world" });
    await user.click(confirmBtn);

    await waitFor(() => {
      expect(mockWipeData).toHaveBeenCalledWith("mc-survival", "mc-survival", undefined);
    });
  });

  it("calls mutation with namespace if provided", async () => {
    const user = userEvent.setup();
    const mockWipeData = vi.mocked(ServersAPI.Servers.wipeData).mockResolvedValue(undefined);

    renderWithProviders(
      <WipeServerDialog
        name="mc-survival"
        ns="gameplane-games"
        open
        onOpenChange={vi.fn()}
      />,
    );

    const checkbox = screen.getByRole("checkbox", {
      name: /I understand this will permanently delete the world data/,
    });
    await user.click(checkbox);

    const confirmBtn = screen.getByRole("button", { name: "Wipe world" });
    await user.click(confirmBtn);

    await waitFor(() => {
      expect(mockWipeData).toHaveBeenCalledWith("mc-survival", "mc-survival", "gameplane-games");
    });
  });

  it("calls onWiped callback on success", async () => {
    const user = userEvent.setup();
    const onWiped = vi.fn();
    vi.mocked(ServersAPI.Servers.wipeData).mockResolvedValue(undefined);

    renderWithProviders(
      <WipeServerDialog
        name="mc-survival"
        open
        onOpenChange={vi.fn()}
        onWiped={onWiped}
      />,
    );

    const checkbox = screen.getByRole("checkbox", {
      name: /I understand this will permanently delete the world data/,
    });
    await user.click(checkbox);

    const confirmBtn = screen.getByRole("button", { name: "Wipe world" });
    await user.click(confirmBtn);

    await waitFor(() => {
      expect(onWiped).toHaveBeenCalled();
    });
  });

  it("closes dialog on success", async () => {
    const user = userEvent.setup();
    const onOpenChange = vi.fn();
    vi.mocked(ServersAPI.Servers.wipeData).mockResolvedValue(undefined);

    renderWithProviders(
      <WipeServerDialog
        name="mc-survival"
        open
        onOpenChange={onOpenChange}
      />,
    );

    const checkbox = screen.getByRole("checkbox", {
      name: /I understand this will permanently delete the world data/,
    });
    await user.click(checkbox);

    const confirmBtn = screen.getByRole("button", { name: "Wipe world" });
    await user.click(confirmBtn);

    await waitFor(() => {
      expect(onOpenChange).toHaveBeenCalledWith(false);
    });
  });

  it("shows busy label when pending", async () => {
    const user = userEvent.setup();
    vi.mocked(ServersAPI.Servers.wipeData).mockImplementation(
      () => new Promise(() => {}), // Never resolves
    );

    renderWithProviders(
      <WipeServerDialog
        name="mc-survival"
        open
        onOpenChange={vi.fn()}
      />,
    );

    const checkbox = screen.getByRole("checkbox", {
      name: /I understand this will permanently delete the world data/,
    });
    await user.click(checkbox);

    const confirmBtn = screen.getByRole("button", { name: "Wipe world" });
    await user.click(confirmBtn);

    expect(screen.getByRole("button", { name: "Working…" })).toBeDisabled();
  });

  it("Cancel button triggers onOpenChange(false)", async () => {
    const user = userEvent.setup();
    const onOpenChange = vi.fn();
    renderWithProviders(
      <WipeServerDialog
        name="mc-survival"
        open
        onOpenChange={onOpenChange}
      />,
    );
    await user.click(screen.getByRole("button", { name: "Cancel" }));
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("checkbox resets when dialog closes and reopens", async () => {
    const user = userEvent.setup();
    const onOpenChange = vi.fn();
    const { rerender } = renderWithProviders(
      <WipeServerDialog
        name="mc-survival"
        open
        onOpenChange={onOpenChange}
      />,
    );

    const checkbox = screen.getByRole("checkbox", {
      name: /I understand this will permanently delete the world data/,
    }) as HTMLInputElement;
    await user.click(checkbox);
    expect(checkbox.checked).toBe(true);

    // Close the dialog
    rerender(
      <WipeServerDialog
        name="mc-survival"
        open={false}
        onOpenChange={onOpenChange}
      />,
    );

    // Reopen the dialog
    rerender(
      <WipeServerDialog
        name="mc-survival"
        open
        onOpenChange={onOpenChange}
      />,
    );

    // Checkbox should be unchecked
    const newCheckbox = screen.getByRole("checkbox", {
      name: /I understand this will permanently delete the world data/,
    }) as HTMLInputElement;
    expect(newCheckbox.checked).toBe(false);
  });

  it("disables buttons when pending", async () => {
    const user = userEvent.setup();
    vi.mocked(ServersAPI.Servers.wipeData).mockImplementation(
      () => new Promise(() => {}), // Never resolves
    );

    renderWithProviders(
      <WipeServerDialog
        name="mc-survival"
        open
        onOpenChange={vi.fn()}
      />,
    );

    const checkbox = screen.getByRole("checkbox", {
      name: /I understand this will permanently delete the world data/,
    });
    await user.click(checkbox);

    const confirmBtn = screen.getByRole("button", { name: "Wipe world" });
    const cancelBtn = screen.getByRole("button", { name: "Cancel" });

    await user.click(confirmBtn);

    expect(cancelBtn).toBeDisabled();
    expect(screen.getByRole("button", { name: "Working…" })).toBeDisabled();
  });
});
