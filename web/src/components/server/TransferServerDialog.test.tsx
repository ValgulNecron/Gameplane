import { describe, it, expect, vi, beforeEach } from "vitest";
import type { ReactNode } from "react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { TransferServerDialog } from "./TransferServerDialog";

const mockUsers = [
  { id: 1, username: "alice" },
  { id: 2, username: "bob" },
  { id: 3, username: "charlie" },
];

const mockServer = {
  metadata: {
    name: "test-server",
    namespace: "default",
    annotations: {
      "gameplane.local/owner": "alice",
    },
  },
};

const mockGetServer = vi.fn();
const mockListUsers = vi.fn();
const mockTransfer = vi.fn();

vi.mock("@/lib/endpoints", () => ({
  Servers: {
    get: mockGetServer,
    transfer: mockTransfer,
  },
  Users: {
    list: mockListUsers,
  },
}));

function renderWithQuery(component: ReactNode) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      {component}
    </QueryClientProvider>,
  );
}

describe("TransferServerDialog", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockGetServer.mockResolvedValue(mockServer);
    mockListUsers.mockResolvedValue(mockUsers);
  });

  it("does not render content when closed", () => {
    renderWithQuery(
      <TransferServerDialog
        name="test-server"
        open={false}
        onOpenChange={() => {}}
      />,
    );

    expect(screen.queryByText(/Transfer test-server/)).not.toBeInTheDocument();
  });

  it("renders title and current owner when open", async () => {
    renderWithQuery(
      <TransferServerDialog
        name="test-server"
        open
        onOpenChange={() => {}}
      />,
    );

    await waitFor(() => {
      expect(screen.getByText("Transfer test-server")).toBeInTheDocument();
    });
    expect(screen.getByText(/Current owner: alice/)).toBeInTheDocument();
  });

  it("displays 'unassigned' when server has no owner", async () => {
    const serverNoOwner = {
      metadata: {
        name: "test-server",
        namespace: "default",
        annotations: {},
      },
    };
    mockGetServer.mockResolvedValueOnce(serverNoOwner);

    renderWithQuery(
      <TransferServerDialog
        name="test-server"
        open
        onOpenChange={() => {}}
      />,
    );

    await waitFor(() => {
      expect(screen.getByText(/Current owner: unassigned/)).toBeInTheDocument();
    });
  });

  it("renders user list in popover when trigger is clicked", async () => {
    renderWithQuery(
      <TransferServerDialog
        name="test-server"
        open
        onOpenChange={() => {}}
      />,
    );

    await waitFor(() => {
      expect(screen.getByText("Select a user…")).toBeInTheDocument();
    });

    const trigger = screen.getByText("Select a user…").closest("[role='button']");
    if (trigger) {
      await userEvent.click(trigger);
    }

    await waitFor(() => {
      const options = screen.getAllByRole("option");
      expect(options.length).toBeGreaterThan(0);
    });
  });

  it("selects a user from the list", async () => {
    renderWithQuery(
      <TransferServerDialog
        name="test-server"
        open
        onOpenChange={() => {}}
      />,
    );

    await waitFor(() => {
      expect(screen.getByText("Select a user…")).toBeInTheDocument();
    });

    const trigger = screen.getByText("Select a user…").closest("[role='button']");
    if (trigger) {
      await userEvent.click(trigger);
    }

    const bobOption = await waitFor(() => {
      const option = screen.getAllByRole("option").find((el) => el.textContent === "bob");
      if (!option) throw new Error("bob option not found");
      return option;
    });
    await userEvent.click(bobOption);

    // After selection, the trigger text should update
    await waitFor(() => {
      const trigger2 = screen.queryByText("bob");
      expect(trigger2).toBeInTheDocument();
    });
  });

  it("disables Transfer button until user is selected", async () => {
    renderWithQuery(
      <TransferServerDialog
        name="test-server"
        open
        onOpenChange={() => {}}
      />,
    );

    await waitFor(() => {
      const btn = screen.getByRole("button", { name: "Transfer" });
      expect(btn).toBeDisabled();
    });
  });

  it("enables Transfer button after user is selected", async () => {
    renderWithQuery(
      <TransferServerDialog
        name="test-server"
        open
        onOpenChange={() => {}}
      />,
    );

    await waitFor(() => {
      expect(screen.getByText("Select a user…")).toBeInTheDocument();
    });

    const trigger = screen.getByText("Select a user…").closest("[role='button']");
    if (trigger) {
      await userEvent.click(trigger);
    }

    const bobOption = await waitFor(() => {
      const option = screen.getAllByRole("option").find((el) => el.textContent === "bob");
      if (!option) throw new Error("bob option not found");
      return option;
    });
    await userEvent.click(bobOption);

    await waitFor(() => {
      const btn = screen.getByRole("button", { name: "Transfer" });
      expect(btn).not.toBeDisabled();
    });
  });

  it("calls onOpenChange when Cancel is clicked", async () => {
    const onOpenChange = vi.fn();

    renderWithQuery(
      <TransferServerDialog
        name="test-server"
        open
        onOpenChange={onOpenChange}
      />,
    );

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Cancel" })).toBeInTheDocument();
    });

    await userEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("displays error when users list fails", async () => {
    const error = new Error("Permission denied");
    mockListUsers.mockRejectedValueOnce(error);

    renderWithQuery(
      <TransferServerDialog
        name="test-server"
        open
        onOpenChange={() => {}}
      />,
    );

    await waitFor(() => {
      expect(screen.getByText(/You need permission to list users/)).toBeInTheDocument();
    });
  });

  it("displays transfer error message when mutation fails", async () => {
    mockTransfer.mockRejectedValueOnce(new Error("Transfer failed"));

    const onOpenChange = vi.fn();

    renderWithQuery(
      <TransferServerDialog
        name="test-server"
        open
        onOpenChange={onOpenChange}
      />,
    );

    await waitFor(() => {
      expect(screen.getByText("Select a user…")).toBeInTheDocument();
    });

    const trigger = screen.getByText("Select a user…").closest("[role='button']");
    if (trigger) {
      await userEvent.click(trigger);
    }

    const bobOption = await waitFor(() => {
      const option = screen.getAllByRole("option").find((el) => el.textContent === "bob");
      if (!option) throw new Error("bob option not found");
      return option;
    });
    await userEvent.click(bobOption);

    const transferBtn = screen.getByRole("button", { name: "Transfer" });
    await userEvent.click(transferBtn);

    await waitFor(() => {
      expect(screen.getByText(/transfer failed/)).toBeInTheDocument();
    });
  });

  it("shows busy label during transfer", async () => {
    mockTransfer.mockImplementation(
      () =>
        new Promise((resolve) => {
          setTimeout(() => resolve(mockServer), 1000);
        }),
    );

    renderWithQuery(
      <TransferServerDialog
        name="test-server"
        open
        onOpenChange={() => {}}
      />,
    );

    await waitFor(() => {
      expect(screen.getByText("Select a user…")).toBeInTheDocument();
    });

    const trigger = screen.getByText("Select a user…").closest("[role='button']");
    if (trigger) {
      await userEvent.click(trigger);
    }

    const bobOption = await waitFor(() => {
      const option = screen.getAllByRole("option").find((el) => el.textContent === "bob");
      if (!option) throw new Error("bob option not found");
      return option;
    });
    await userEvent.click(bobOption);

    const transferBtn = screen.getByRole("button", { name: "Transfer" });
    await userEvent.click(transferBtn);

    await waitFor(() => {
      expect(screen.getByRole("button", { name: /Transferring/ })).toBeInTheDocument();
    });
  });

  it("calls onTransferred callback on success", async () => {
    const onTransferred = vi.fn();
    const onOpenChange = vi.fn();

    renderWithQuery(
      <TransferServerDialog
        name="test-server"
        open
        onOpenChange={onOpenChange}
        onTransferred={onTransferred}
      />,
    );

    await waitFor(() => {
      expect(screen.getByText("Select a user…")).toBeInTheDocument();
    });

    const trigger = screen.getByText("Select a user…").closest("[role='button']");
    if (trigger) {
      await userEvent.click(trigger);
    }

    const bobOption = await waitFor(() => {
      const option = screen.getAllByRole("option").find((el) => el.textContent === "bob");
      if (!option) throw new Error("bob option not found");
      return option;
    });
    await userEvent.click(bobOption);

    const transferBtn = screen.getByRole("button", { name: "Transfer" });
    await userEvent.click(transferBtn);

    await waitFor(() => {
      expect(onTransferred).toHaveBeenCalled();
      expect(onOpenChange).toHaveBeenCalledWith(false);
    });
  });
});
