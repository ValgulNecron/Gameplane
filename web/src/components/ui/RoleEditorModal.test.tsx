import { describe, it, expect, vi, beforeEach } from "vitest";
import type { ReactNode } from "react";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { Role, PermissionGroup } from "@/types";
import { RoleEditorModal } from "./RoleEditorModal";

// Mock the endpoints
vi.mock("@/lib/endpoints", () => ({
  Roles: {
    update: vi.fn(),
    create: vi.fn(),
  },
}));

import { Roles } from "@/lib/endpoints";
import { APIError } from "@/lib/api";

const queryClient = new QueryClient({
  defaultOptions: {
    queries: { retry: false },
    mutations: { retry: false },
  },
});

const Wrapper = ({ children }: { children: ReactNode }) => (
  <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
);

const mockRole: Role = {
  name: "operator",
  description: "Manage game servers and backups",
  builtin: false,
  permissions: ["servers:read", "servers:write", "backups:read"],
};

const mockGroups: PermissionGroup[] = [
  {
    resource: "servers",
    label: "Game Servers",
    permissions: [
      { key: "servers:read", label: "Read", namespaced: true },
      { key: "servers:write", label: "Write", namespaced: true },
      { key: "servers:console", label: "Console", namespaced: true },
    ],
  },
  {
    resource: "backups",
    label: "Backups",
    permissions: [
      { key: "backups:read", label: "Read", namespaced: true },
      { key: "backups:write", label: "Write", namespaced: true },
    ],
  },
];

describe("RoleEditorModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    queryClient.clear();
  });

  it("renders when open with role and groups", () => {
    render(
      <RoleEditorModal
        open
        onOpenChange={() => {}}
        role={mockRole}
        groups={mockGroups}
      />,
      { wrapper: Wrapper },
    );
    expect(screen.getByText("Edit role: operator")).toBeInTheDocument();
    expect(
      screen.getByText("Grant a curated set of permissions."),
    ).toBeInTheDocument();
  });

  it("does not render when closed", () => {
    render(
      <RoleEditorModal
        open={false}
        onOpenChange={() => {}}
        role={mockRole}
        groups={mockGroups}
      />,
      { wrapper: Wrapper },
    );
    expect(screen.queryByText("Edit role: operator")).not.toBeInTheDocument();
  });

  it("populates description field with role description", () => {
    render(
      <RoleEditorModal
        open
        onOpenChange={() => {}}
        role={mockRole}
        groups={mockGroups}
      />,
      { wrapper: Wrapper },
    );
    const input = screen.getByDisplayValue("Manage game servers and backups");
    expect(input).toBeInTheDocument();
  });

  it("displays permission groups with correct labels", () => {
    render(
      <RoleEditorModal
        open
        onOpenChange={() => {}}
        role={mockRole}
        groups={mockGroups}
      />,
      { wrapper: Wrapper },
    );
    expect(screen.getByText("Game Servers")).toBeInTheDocument();
    expect(screen.getByText("Backups")).toBeInTheDocument();
  });

  it("displays all permissions for each group", () => {
    render(
      <RoleEditorModal
        open
        onOpenChange={() => {}}
        role={mockRole}
        groups={mockGroups}
      />,
      { wrapper: Wrapper },
    );
    expect(screen.getByText("servers:read")).toBeInTheDocument();
    expect(screen.getByText("servers:write")).toBeInTheDocument();
    expect(screen.getByText("servers:console")).toBeInTheDocument();
    expect(screen.getByText("backups:read")).toBeInTheDocument();
    expect(screen.getByText("backups:write")).toBeInTheDocument();
  });

  it("shows 'ns' badge for namespaced permissions", () => {
    render(
      <RoleEditorModal
        open
        onOpenChange={() => {}}
        role={mockRole}
        groups={mockGroups}
      />,
      { wrapper: Wrapper },
    );
    const nsBadges = screen.getAllByText("ns");
    expect(nsBadges.length).toBeGreaterThan(0);
  });

  it("renders Cancel and Save buttons", () => {
    render(
      <RoleEditorModal
        open
        onOpenChange={() => {}}
        role={mockRole}
        groups={mockGroups}
      />,
      { wrapper: Wrapper },
    );
    expect(screen.getByText("Cancel")).toBeInTheDocument();
    expect(screen.getByText("Save role")).toBeInTheDocument();
  });

  it("calls onOpenChange when Cancel button is clicked", async () => {
    const user = userEvent.setup();
    const onOpenChange = vi.fn();
    render(
      <RoleEditorModal
        open
        onOpenChange={onOpenChange}
        role={mockRole}
        groups={mockGroups}
      />,
      { wrapper: Wrapper },
    );
    await user.click(screen.getByText("Cancel"));
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("toggles permission checkbox on click", async () => {
    const user = userEvent.setup();
    render(
      <RoleEditorModal
        open
        onOpenChange={() => {}}
        role={mockRole}
        groups={mockGroups}
      />,
      { wrapper: Wrapper },
    );
    const firstCheckbox = screen.getByRole("checkbox", { name: /servers:read/ });
    // Initially should be checked (since it's in mockRole.permissions)
    expect(firstCheckbox).toBeChecked();
    // Uncheck it
    await user.click(firstCheckbox);
    expect(firstCheckbox).not.toBeChecked();
  });

  it("allows updating description", async () => {
    const user = userEvent.setup();
    render(
      <RoleEditorModal
        open
        onOpenChange={() => {}}
        role={mockRole}
        groups={mockGroups}
      />,
      { wrapper: Wrapper },
    );
    const input = screen.getByDisplayValue("Manage game servers and backups");
    await user.clear(input);
    await user.type(input, "New description");
    expect(input).toHaveValue("New description");
  });

  // F-126: creating a role with a name that already exists 409s, and the
  // save mutation had no error handler — the modal just stayed open silent.
  it("shows an error banner when saving a duplicate role name 409s", async () => {
    const user = userEvent.setup();
    vi.mocked(Roles.create).mockRejectedValueOnce(
      new APIError(409, "role \"operator\" already exists"),
    );
    render(
      <RoleEditorModal
        open
        onOpenChange={() => {}}
        role={null}
        groups={mockGroups}
      />,
      { wrapper: Wrapper },
    );
    await user.type(screen.getByLabelText("Name"), "operator");
    await user.click(screen.getByText("Create role"));
    expect(await screen.findByText(/already exists/i)).toBeInTheDocument();
  });
});
