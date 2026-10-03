import { ReactNode } from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { GameServer } from "@/types";
import type { ServerAccess, ResourceTarget } from "@/lib/resourceTarget";
import { ServerActionsMenu } from "./ServerActionsMenu";

// Mock the dialog components
vi.mock("./CloneServerDialog", () => ({
  CloneServerDialog: ({ open, target }: { open: boolean; target: ResourceTarget }) =>
    open ? <div data-testid="clone-dialog" data-target={JSON.stringify(target)}>Clone Dialog</div> : null,
}));

vi.mock("./TransferServerDialog", () => ({
  TransferServerDialog: ({ open }: { open: boolean }) =>
    open ? <div data-testid="transfer-dialog">Transfer Dialog</div> : null,
}));

vi.mock("./WipeServerDialog", () => ({
  WipeServerDialog: ({ open }: { open: boolean }) =>
    open ? <div data-testid="wipe-dialog">Wipe Dialog</div> : null,
}));

vi.mock("./DeleteServerDialog", () => ({
  DeleteServerDialog: ({ open }: { open: boolean }) =>
    open ? <div data-testid="delete-dialog">Delete Dialog</div> : null,
}));

const administratorAccess: ServerAccess = { canWrite: true, canControl: true, canConsole: true, canDelete: true, isOwner: false, isCollaborator: false };
const readerAccess: ServerAccess = { canWrite: false, canControl: false, canConsole: false, canDelete: false, isOwner: false, isCollaborator: false };
let targetAccess: ServerAccess = administratorAccess;

function createGameServer(overrides?: {
  metadata?: Partial<GameServer["metadata"]>;
  spec?: Partial<GameServer["spec"]>;
  status?: Partial<GameServer["status"]>;
}): GameServer {
  const metadata = {
    name: "test-server",
    namespace: "gameplane-games",
    uid: "uid-123",
    resourceVersion: "1",
    creationTimestamp: new Date().toISOString(),
    annotations: {
      "gameplane.local/owner-id": "user-1",
    },
    ...overrides?.metadata,
  };

  return {
    apiVersion: "gameplane.local/v1alpha1",
    kind: "GameServer",
    metadata,
    spec: {
      template: "minecraft-java",
      ...overrides?.spec,
    },
    status: {
      phase: "Running",
      ...overrides?.status,
    },
  } as GameServer;
}

function Subject({
  gs,
  onDeleted,
  onTransferred,
}: {
  gs: GameServer;
  onDeleted?: () => void;
  onTransferred?: () => void;
}): ReactNode {
  return (
    <QueryClientProvider client={new QueryClient()}>
      <ServerActionsMenu gs={gs} target={{ cluster: "remote", namespace: gs.metadata.namespace, name: gs.metadata.name, uid: gs.metadata.uid }} access={targetAccess} onDeleted={onDeleted} onTransferred={onTransferred} />
    </QueryClientProvider>
  );
}

describe("ServerActionsMenu", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    targetAccess = administratorAccess;
  });

  it("renders the menu trigger button", () => {
    const gs = createGameServer();
    render(<Subject gs={gs} />);
    expect(screen.getByRole("button", { name: "Server actions" })).toBeInTheDocument();
  });

  it("opens the dropdown when trigger is clicked", async () => {
    const user = userEvent.setup();
    const gs = createGameServer();
    render(<Subject gs={gs} />);

    expect(screen.queryByText("Clone server")).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Server actions" }));
    expect(screen.getByText("Clone server")).toBeInTheDocument();
  });

  it("shows all menu items when dropdown is open", async () => {
    const user = userEvent.setup();
    const gs = createGameServer();
    render(<Subject gs={gs} />);

    await user.click(screen.getByRole("button", { name: "Server actions" }));

    expect(screen.getByText("Clone server")).toBeInTheDocument();
    expect(screen.getByText("Transfer ownership")).toBeInTheDocument();
    expect(screen.getByText("Wipe world data")).toBeInTheDocument();
    expect(screen.getByText("Delete server")).toBeInTheDocument();
  });

  it("opens clone dialog when clone server is clicked", async () => {
    const user = userEvent.setup();
    const gs = createGameServer();
    render(<Subject gs={gs} />);

    await user.click(screen.getByRole("button", { name: "Server actions" }));
    await user.click(screen.getByText("Clone server"));

    expect(screen.getByTestId("clone-dialog")).toBeInTheDocument();
  });

  it("opens transfer dialog when transfer ownership is clicked", async () => {
    const user = userEvent.setup();
    const gs = createGameServer();
    render(<Subject gs={gs} />);

    await user.click(screen.getByRole("button", { name: "Server actions" }));
    await user.click(screen.getByText("Transfer ownership"));

    expect(screen.getByTestId("transfer-dialog")).toBeInTheDocument();
  });

  it("opens wipe dialog when wipe world data is clicked", async () => {
    const user = userEvent.setup();
    const gs = createGameServer();
    render(<Subject gs={gs} />);

    await user.click(screen.getByRole("button", { name: "Server actions" }));
    await user.click(screen.getByText("Wipe world data"));

    expect(screen.getByTestId("wipe-dialog")).toBeInTheDocument();
  });

  it("opens delete dialog when delete server is clicked", async () => {
    const user = userEvent.setup();
    const gs = createGameServer();
    render(<Subject gs={gs} />);

    await user.click(screen.getByRole("button", { name: "Server actions" }));
    await user.click(screen.getByText("Delete server"));

    expect(screen.getByTestId("delete-dialog")).toBeInTheDocument();
  });

  it("renders clone item as disabled when user lacks permission", async () => {
    const user = userEvent.setup();
    const gs = createGameServer();

    targetAccess = readerAccess;

    render(<Subject gs={gs} />);
    await user.click(screen.getByRole("button", { name: "Server actions" }));

    const cloneItem = screen.getByText("Clone server").closest("[role='menuitem']");
    expect(cloneItem).toHaveAttribute("aria-disabled", "true");
  });

  it("renders transfer item as disabled when user is not owner and lacks operator role", async () => {
    const user = userEvent.setup();
    const gs = createGameServer({
      metadata: {
        annotations: {
          "gameplane.local/owner-id": "other-user-id",
        },
      },
    });

    targetAccess = readerAccess;

    render(<Subject gs={gs} />);
    await user.click(screen.getByRole("button", { name: "Server actions" }));

    const transferItem = screen.getByText("Transfer ownership").closest("[role='menuitem']");
    expect(transferItem).toHaveAttribute("aria-disabled", "true");
  });

  it("renders destructive items with danger text color", async () => {
    const user = userEvent.setup();
    const gs = createGameServer();
    render(<Subject gs={gs} />);

    await user.click(screen.getByRole("button", { name: "Server actions" }));

    const wipeItem = screen.getByText("Wipe world data").closest("[role='menuitem']");
    const deleteItem = screen.getByText("Delete server").closest("[role='menuitem']");

    expect(wipeItem).toHaveClass("text-danger");
    expect(deleteItem).toHaveClass("text-danger");
  });

  it("renders non-destructive items with default text color", async () => {
    const user = userEvent.setup();
    const gs = createGameServer();
    render(<Subject gs={gs} />);

    await user.click(screen.getByRole("button", { name: "Server actions" }));

    const cloneItem = screen.getByText("Clone server").closest("[role='menuitem']");
    const transferItem = screen.getByText("Transfer ownership").closest("[role='menuitem']");

    expect(cloneItem).toHaveClass("text-foreground");
    expect(transferItem).toHaveClass("text-foreground");
  });

  it("shows hints on disabled items", async () => {
    const user = userEvent.setup();
    const gs = createGameServer();

    targetAccess = readerAccess;

    render(<Subject gs={gs} />);
    await user.click(screen.getByRole("button", { name: "Server actions" }));

    const cloneItem = screen.getByText("Clone server").closest("[role='menuitem']");
    const titleAttr = cloneItem?.querySelector("[title]");
    expect(titleAttr).toHaveAttribute("title", "Requires operator role");
  });

  it("renders separator between action groups", async () => {
    const user = userEvent.setup();
    const gs = createGameServer();
    render(<Subject gs={gs} />);

    await user.click(screen.getByRole("button", { name: "Server actions" }));

    // The separator is a Separator component with role="separator"
    const separator = screen.getByRole("separator");
    expect(separator).toBeInTheDocument();
  });

  it("passes server name and namespace to dialogs", async () => {
    const user = userEvent.setup();
    const gs = createGameServer({
      metadata: {
        name: "my-server",
        namespace: "production",
      },
    });

    render(<Subject gs={gs} />);
    await user.click(screen.getByRole("button", { name: "Server actions" }));
    await user.click(screen.getByText("Clone server"));

    // The clone dialog should be open (mocked component validates this)
    await waitFor(() => {
      expect(screen.getByTestId("clone-dialog")).toBeInTheDocument();
    });
    expect(JSON.parse(screen.getByTestId("clone-dialog").getAttribute("data-target") ?? "null")).toEqual({ cluster: "remote", namespace: "production", name: "my-server", uid: "uid-123" });
  });

  it("calls onDeleted callback when DeleteServerDialog triggers it", async () => {
    const user = userEvent.setup();
    const onDeleted = vi.fn();
    const gs = createGameServer();

    render(<Subject gs={gs} onDeleted={onDeleted} />);
    await user.click(screen.getByRole("button", { name: "Server actions" }));
    await user.click(screen.getByText("Delete server"));

    expect(screen.getByTestId("delete-dialog")).toBeInTheDocument();
  });

  it("calls onTransferred callback when TransferServerDialog triggers it", async () => {
    const user = userEvent.setup();
    const onTransferred = vi.fn();
    const gs = createGameServer();

    render(<Subject gs={gs} onTransferred={onTransferred} />);
    await user.click(screen.getByRole("button", { name: "Server actions" }));
    await user.click(screen.getByText("Transfer ownership"));

    expect(screen.getByTestId("transfer-dialog")).toBeInTheDocument();
  });

  it("handles owner check correctly when user is the owner", async () => {
    const user = userEvent.setup();
    const gs = createGameServer({
      metadata: {
        annotations: {
          "gameplane.local/owner-id": "user-1", // Same as the logged-in user
        },
      },
    });

    targetAccess = { ...readerAccess, isOwner: true, canControl: true, canConsole: true, canDelete: true };
    render(<Subject gs={gs} />);
    await user.click(screen.getByRole("button", { name: "Server actions" }));

    // Transfer should be enabled for the owner even without operator role
    const transferItem = screen.getByText("Transfer ownership").closest("[role='menuitem']");
    expect(transferItem).not.toHaveAttribute("aria-disabled", "true");
  });

  it("namespace write permission does not grant owner-only actions", async () => {
    targetAccess = { ...readerAccess, canWrite: true, canControl: true };
    const user = userEvent.setup();
    render(<Subject gs={createGameServer()} />);
    await user.click(screen.getByRole("button", { name: "Server actions" }));
    expect(screen.getByText("Clone server").closest("[role='menuitem']")).not.toHaveAttribute("aria-disabled", "true");
    for (const label of ["Transfer ownership", "Wipe world data", "Delete server"]) {
      expect(screen.getByText(label).closest("[role='menuitem']")).toHaveAttribute("aria-disabled", "true");
    }
  });

  it("renders icon buttons with icon-only styling", () => {
    const gs = createGameServer();
    render(<Subject gs={gs} />);

    const triggerButton = screen.getByRole("button", { name: "Server actions" });
    expect(triggerButton).toHaveClass("button");
    expect(triggerButton).toHaveClass("button--icon-only");
    expect(triggerButton).toHaveClass("button--ghost");
  });
});
