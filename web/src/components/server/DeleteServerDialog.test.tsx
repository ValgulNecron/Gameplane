import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { DeleteServerDialog } from "./DeleteServerDialog";
import * as ServersAPI from "@/lib/endpoints";

// Mock the Servers API
vi.mock("@/lib/endpoints", () => ({
  Servers: {
    remove: vi.fn(),
  },
}));

const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
});

function renderWithProviders(component: React.ReactNode) {
  return render(<QueryClientProvider client={queryClient}>{component}</QueryClientProvider>);
}

describe("DeleteServerDialog", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("does not render content when closed", () => {
    renderWithProviders(
      <DeleteServerDialog
        name="test-server"
        open={false}
        onOpenChange={() => {}}
      />,
    );
    expect(screen.queryByText(/Delete test-server\?/)).not.toBeInTheDocument();
  });

  it("renders title with server name when open", () => {
    renderWithProviders(
      <DeleteServerDialog
        name="my-game-server"
        open
        onOpenChange={() => {}}
      />,
    );
    expect(screen.getByText("Delete my-game-server?")).toBeInTheDocument();
  });

  it("renders description warning about permanent deletion", () => {
    renderWithProviders(
      <DeleteServerDialog
        name="srv1"
        open
        onOpenChange={() => {}}
      />,
    );
    expect(
      screen.getByText(/This will permanently remove the GameServer resource/),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/persistent volume/),
    ).toBeInTheDocument();
  });

  it("requires typing server name to enable delete", async () => {
    renderWithProviders(
      <DeleteServerDialog
        name="test-server"
        open
        onOpenChange={() => {}}
      />,
    );
    const deleteBtn = screen.getByRole("button", { name: "Delete server" });
    expect(deleteBtn).toBeDisabled();

    const input = screen.getByPlaceholderText("Type to confirm") as HTMLInputElement;
    await userEvent.type(input, "test-server");
    expect(deleteBtn).toBeEnabled();
  });

  it("calls mutation on confirm", async () => {
    const mockRemove = vi.mocked(ServersAPI.Servers.remove).mockResolvedValue({});
    const onDeleted = vi.fn();

    renderWithProviders(
      <DeleteServerDialog
        name="test-server"
        open
        onOpenChange={() => {}}
        onDeleted={onDeleted}
      />,
    );

    const input = screen.getByPlaceholderText("Type to confirm");
    await userEvent.type(input, "test-server");

    const deleteBtn = screen.getByRole("button", { name: "Delete server" });
    await userEvent.click(deleteBtn);

    await waitFor(() => {
      expect(mockRemove).toHaveBeenCalledWith("test-server", undefined);
      expect(onDeleted).toHaveBeenCalled();
    });
  });

  it("passes namespace when provided", async () => {
    const mockRemove = vi.mocked(ServersAPI.Servers.remove).mockResolvedValue({});

    renderWithProviders(
      <DeleteServerDialog
        name="test-server"
        ns="default"
        open
        onOpenChange={() => {}}
      />,
    );

    const input = screen.getByPlaceholderText("Type to confirm");
    await userEvent.type(input, "test-server");

    const deleteBtn = screen.getByRole("button", { name: "Delete server" });
    await userEvent.click(deleteBtn);

    await waitFor(() => {
      expect(mockRemove).toHaveBeenCalledWith("test-server", "default");
    });
  });

  it("displays error message when deletion fails", async () => {
    const error = new Error("API error");
    vi.mocked(ServersAPI.Servers.remove).mockRejectedValue(error);

    renderWithProviders(
      <DeleteServerDialog
        name="test-server"
        open
        onOpenChange={() => {}}
      />,
    );

    const input = screen.getByPlaceholderText("Type to confirm");
    await userEvent.type(input, "test-server");

    const deleteBtn = screen.getByRole("button", { name: "Delete server" });
    await userEvent.click(deleteBtn);

    await waitFor(() => {
      expect(screen.getByText(/delete failed/)).toBeInTheDocument();
    });
  });

  it("disables buttons while mutation is in flight", async () => {
    const mockRemove = vi.mocked(ServersAPI.Servers.remove).mockImplementation(
      () => new Promise((resolve) => setTimeout(resolve, 100)),
    );

    renderWithProviders(
      <DeleteServerDialog
        name="test-server"
        open
        onOpenChange={() => {}}
      />,
    );

    const input = screen.getByPlaceholderText("Type to confirm");
    await userEvent.type(input, "test-server");

    const deleteBtn = screen.getByRole("button", { name: "Delete server" });
    await userEvent.click(deleteBtn);

    // Button should show "Working…" and be disabled
    expect(screen.getByRole("button", { name: "Working…" })).toBeDisabled();
    const cancelBtn = screen.getByRole("button", { name: "Cancel" });
    expect(cancelBtn).toBeDisabled();
  });

  it("closes dialog on cancel", async () => {
    const onOpenChange = vi.fn();
    renderWithProviders(
      <DeleteServerDialog
        name="test-server"
        open
        onOpenChange={onOpenChange}
      />,
    );

    const cancelBtn = screen.getByRole("button", { name: "Cancel" });
    await userEvent.click(cancelBtn);

    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("clears confirmation text when dialog reopens", async () => {
    const { rerender } = renderWithProviders(
      <DeleteServerDialog
        name="test-server"
        open
        onOpenChange={() => {}}
      />,
    );

    const input = screen.getByPlaceholderText("Type to confirm") as HTMLInputElement;
    await userEvent.type(input, "test-server");
    expect(input.value).toBe("test-server");

    // Close dialog
    rerender(
      <QueryClientProvider client={queryClient}>
        <DeleteServerDialog
          name="test-server"
          open={false}
          onOpenChange={() => {}}
        />
      </QueryClientProvider>,
    );

    // Reopen dialog
    rerender(
      <QueryClientProvider client={queryClient}>
        <DeleteServerDialog
          name="test-server"
          open
          onOpenChange={() => {}}
        />
      </QueryClientProvider>,
    );

    // Confirmation text should be cleared
    const newInput = screen.getByPlaceholderText("Type to confirm") as HTMLInputElement;
    expect(newInput.value).toBe("");
  });

  it("clears mutation state when dialog closes without confirming", async () => {
    const mockRemove = vi.mocked(ServersAPI.Servers.remove).mockRejectedValue(
      new Error("API error"),
    );

    const onOpenChange = vi.fn();
    const { rerender } = renderWithProviders(
      <DeleteServerDialog
        name="test-server"
        open
        onOpenChange={onOpenChange}
      />,
    );

    const input = screen.getByPlaceholderText("Type to confirm");
    await userEvent.type(input, "test-server");

    const deleteBtn = screen.getByRole("button", { name: "Delete server" });
    await userEvent.click(deleteBtn);

    // Wait for error to appear
    await waitFor(() => {
      expect(screen.getByText(/delete failed/)).toBeInTheDocument();
    });

    // Close the dialog
    const cancelBtn = screen.getByRole("button", { name: "Cancel" });
    await userEvent.click(cancelBtn);

    expect(onOpenChange).toHaveBeenCalledWith(false);

    // Reopen dialog
    rerender(
      <QueryClientProvider client={queryClient}>
        <DeleteServerDialog
          name="test-server"
          open
          onOpenChange={onOpenChange}
        />
      </QueryClientProvider>,
    );

    // Error message should be cleared
    expect(screen.queryByText(/delete failed/)).not.toBeInTheDocument();
  });
});
