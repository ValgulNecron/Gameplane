import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { UsersPage } from "./Users";
import type { ExtendedUser } from "@/types";

// The page calls Users.* and Roles.* from `@/lib/endpoints`. Stub the
// whole module so we can assert the right calls were made without
// spinning up a fetch mock.
const list = vi.fn();
const update = vi.fn();
const remove = vi.fn();
const resetPassword = vi.fn();
const create = vi.fn();
const bindings = vi.fn();
const addBinding = vi.fn();
const removeBinding = vi.fn();
const rolesList = vi.fn();
const catalog = vi.fn();
const rolesCreate = vi.fn();
const rolesUpdate = vi.fn();
const rolesRemove = vi.fn();
vi.mock("@/lib/endpoints", () => ({
  Users: {
    list: () => list(),
    update: (id: number, body: unknown) => update(id, body),
    remove: (id: number) => remove(id),
    resetPassword: (id: number, pw: string) => resetPassword(id, pw),
    create: (body: unknown) => create(body),
    bindings: (id: number) => bindings(id),
    addBinding: (id: number, body: unknown) => addBinding(id, body),
    removeBinding: (id: number, r: string, ns: string) => removeBinding(id, r, ns),
  },
  Roles: {
    list: () => rolesList(),
    catalog: () => catalog(),
    create: (body: unknown) => rolesCreate(body),
    update: (name: string, body: unknown) => rolesUpdate(name, body),
    remove: (name: string) => rolesRemove(name),
  },
}));

const useMeMock = vi.fn();
// Keep the real can(); only stub useMe.
vi.mock("@/lib/auth", async (orig) => ({
  ...(await orig<typeof import("@/lib/auth")>()),
  useMe: () => useMeMock(),
}));

const ROLE_DEFS = [
  { name: "admin", description: "Full access.", builtin: true, permissions: ["*"] },
  { name: "operator", description: "Manage servers.", builtin: true, permissions: ["servers:read", "servers:write"] },
  { name: "viewer", description: "Read-only.", builtin: true, permissions: ["servers:read"] },
];
const CATALOG = {
  groups: [
    {
      resource: "servers",
      label: "Game servers",
      permissions: [
        { key: "servers:read", label: "View", namespaced: true },
        { key: "servers:write", label: "Manage", namespaced: true },
      ],
    },
  ],
};

vi.mock("@tanstack/react-router", () => ({
  Link: ({ children, ...rest }: { children: ReactNode } & Record<string, unknown>) => (
    <a {...rest}>{children}</a>
  ),
}));

const ALICE: ExtendedUser = {
  id: 2,
  username: "alice",
  displayName: "Alice",
  email: "alice@example.com",
  role: "operator",
  provider: "local",
  createdAt: "2026-01-01T00:00:00Z",
};
const ME: ExtendedUser = {
  id: 1,
  username: "root",
  displayName: "Root",
  email: "root@example.com",
  role: "admin",
  provider: "local",
  createdAt: "2026-01-01T00:00:00Z",
  permissions: { "*": ["*"] },
};

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <UsersPage />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  list.mockResolvedValue([ME, ALICE]);
  rolesList.mockResolvedValue(ROLE_DEFS);
  catalog.mockResolvedValue(CATALOG);
  bindings.mockResolvedValue([]);
  useMeMock.mockReturnValue({ data: ME, error: null, isLoading: false });
});

afterEach(() => {
  vi.resetAllMocks();
});

describe("UsersPage", () => {
  it("renders the row for each user", async () => {
    renderPage();
    expect(await screen.findByText("alice")).toBeInTheDocument();
    expect(screen.getByText("root")).toBeInTheDocument();
    // Provider renders as a normalized badge (local → "Local").
    expect(screen.getAllByText("Local").length).toBeGreaterThan(0);
  });

  it("shows the email under the display name without duplicating it", async () => {
    const bob: ExtendedUser = {
      id: 3,
      username: "bob",
      displayName: "",
      email: "bob@example.com",
      role: "viewer",
      provider: "local",
      createdAt: "2026-01-01T00:00:00Z",
    };
    list.mockResolvedValue([ME, ALICE, bob]);
    renderPage();
    // Alice has a display name, so her email renders as a subline.
    expect(await screen.findByText("alice@example.com")).toBeInTheDocument();
    // Bob has no display name — his email already IS the name line, so it
    // must not repeat as a subline.
    expect(screen.getAllByText("bob@example.com").length).toBe(1);
  });

  it("opens the action menu for a row", async () => {
    const user = userEvent.setup();
    renderPage();
    await user.click(await screen.findByLabelText("Actions for alice"));
    expect(await screen.findByText("Edit user")).toBeInTheDocument();
    expect(screen.getByText("Reset password")).toBeInTheDocument();
    expect(screen.getByText("Delete user")).toBeInTheDocument();
  });

  it("submits an edit via PATCH", async () => {
    const user = userEvent.setup();
    update.mockResolvedValue({ ...ALICE, role: "admin" });
    renderPage();
    await user.click(await screen.findByLabelText("Actions for alice"));
    await user.click(await screen.findByText("Edit user"));

    const select = await screen.findByDisplayValue("operator");
    await user.selectOptions(select, "admin");
    await user.click(screen.getByRole("button", { name: /Save changes/i }));

    await waitFor(() => expect(update).toHaveBeenCalledWith(2, { role: "admin" }));
  });

  it("submits a password reset", async () => {
    const user = userEvent.setup();
    resetPassword.mockResolvedValue(undefined);
    renderPage();
    await user.click(await screen.findByLabelText("Actions for alice"));
    await user.click(await screen.findByText("Reset password"));

    const pw = await screen.findByPlaceholderText(/At least 12 characters/i);
    await user.type(pw, "brand-new-password-1");
    await user.click(screen.getByRole("button", { name: /Set new password/i }));

    await waitFor(() =>
      expect(resetPassword).toHaveBeenCalledWith(2, "brand-new-password-1"),
    );
  });

  it("blocks self-delete via the menu", async () => {
    const user = userEvent.setup();
    renderPage();
    await user.click(await screen.findByLabelText("Actions for root"));
    const deleteItem = await screen.findByText("Delete user");
    const item = deleteItem.closest("[role='menuitem']");
    expect(item).toHaveAttribute("aria-disabled", "true");
    // Clicking a disabled Radix menu item is a no-op; assert the
    // underlying mutation never fires.
    await user.click(deleteItem);
    expect(remove).not.toHaveBeenCalled();
  });

  it("confirms before deleting another user", async () => {
    const user = userEvent.setup();
    remove.mockResolvedValue(undefined);
    renderPage();
    await user.click(await screen.findByLabelText("Actions for alice"));
    await user.click(await screen.findByText("Delete user"));

    expect(await screen.findByText("Delete alice?")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Delete user" }));
    await waitFor(() => expect(remove).toHaveBeenCalledWith(2));
  });
});

describe("UsersPage roles tab", () => {
  async function openRolesTab() {
    const user = userEvent.setup();
    renderPage();
    await user.click(await screen.findByRole("button", { name: /Roles/i }));
    return user;
  }

  it("lists roles with a built-in badge", async () => {
    await openRolesTab();
    expect(await screen.findByText("operator")).toBeInTheDocument();
    expect(screen.getAllByText("built-in").length).toBeGreaterThan(0);
  });

  it("creates a custom role with selected permissions", async () => {
    rolesCreate.mockResolvedValue({
      name: "support",
      description: "",
      builtin: false,
      permissions: ["servers:read"],
    });
    const user = await openRolesTab();
    await user.click(await screen.findByRole("button", { name: /New role/i }));
    await user.type(await screen.findByPlaceholderText("support"), "support");
    await user.click(await screen.findByLabelText(/servers:read/));
    await user.click(screen.getByRole("button", { name: /Create role/i }));
    await waitFor(() =>
      expect(rolesCreate).toHaveBeenCalledWith(
        expect.objectContaining({ name: "support", permissions: ["servers:read"] }),
      ),
    );
  });

  it("does not offer to delete a built-in role", async () => {
    await openRolesTab();
    // operator is built-in: it can be edited but not deleted.
    await screen.findByText("operator");
    expect(screen.queryByLabelText("Remove operator in")).toBeNull();
    // The admin card offers neither edit nor delete.
    const cards = screen.getAllByText(/^built-in$/);
    expect(cards.length).toBe(3);
  });
});

describe("UsersPage audit quick-link", () => {
  it("shows an Audit log link for users with audit:read", async () => {
    renderPage();
    const link = await screen.findByText("Audit log");
    expect(link.closest("a")).toHaveAttribute("to", "/admin/audit");
  });

  it("hides the Audit log link from users without audit:read", async () => {
    useMeMock.mockReturnValue({
      data: { ...ME, permissions: { "*": ["users:manage"] } },
      error: null,
      isLoading: false,
    });
    renderPage();
    await screen.findByText("Invite user");
    expect(screen.queryByText("Audit log")).toBeNull();
  });
});

describe("UsersPage error handling", () => {
  it("displays error when users list fails to load", async () => {
    const error = new Error("Network error");
    list.mockRejectedValue(error);
    renderPage();
    await screen.findByText(/Failed to load users/i);
  });
});

describe("UsersPage search/filter", () => {
  it("filters users by username", async () => {
    const user = userEvent.setup();
    renderPage();
    await user.type(await screen.findByLabelText("Search users"), "alice");
    expect(await screen.findByText("alice")).toBeInTheDocument();
    expect(screen.queryByText("root")).not.toBeInTheDocument();
  });

  it("filters users by email", async () => {
    const user = userEvent.setup();
    renderPage();
    await user.type(await screen.findByLabelText("Search users"), "alice@");
    expect(await screen.findByText("alice")).toBeInTheDocument();
  });

  it("shows all users when search is cleared", async () => {
    const user = userEvent.setup();
    renderPage();
    const search = await screen.findByLabelText("Search users");
    await user.type(search, "alice");
    expect(screen.queryByText("root")).not.toBeInTheDocument();
    await user.clear(search);
    expect(await screen.findByText("root")).toBeInTheDocument();
  });

  it("shows no results message when filter matches nothing", async () => {
    const user = userEvent.setup();
    renderPage();
    await user.type(await screen.findByLabelText("Search users"), "nonexistent");
    expect(await screen.findByText("No entries.")).toBeInTheDocument();
  });
});

describe("UsersPage user row display", () => {
  it("shows role badge with correct styling for known roles", async () => {
    renderPage();
    const badge = await screen.findByText("operator");
    expect(badge).toHaveClass("bg-violet/15", "text-violet");
  });

  it("shows 'you' badge next to current user", async () => {
    renderPage();
    await screen.findByText("root");
    expect(screen.getByText("you")).toBeInTheDocument();
  });

  it("displays OIDC provider badge for OIDC users", async () => {
    const oidcUser: ExtendedUser = {
      id: 3,
      username: "oidc-user",
      displayName: "OIDC User",
      email: "oidc@example.com",
      role: "viewer",
      provider: "oidc",
      createdAt: "2026-01-01T00:00:00Z",
    };
    list.mockResolvedValue([ME, oidcUser]);
    renderPage();
    expect(await screen.findByText("OIDC")).toBeInTheDocument();
  });

  it("displays Pending provider badge for pending users", async () => {
    const pendingUser: ExtendedUser = {
      id: 4,
      username: "pending-user",
      displayName: "",
      email: "pending@example.com",
      role: "viewer",
      provider: "pending",
      createdAt: "2026-01-01T00:00:00Z",
    };
    list.mockResolvedValue([ME, pendingUser]);
    renderPage();
    expect(await screen.findByText("Pending")).toBeInTheDocument();
  });

  it("disables reset password for OIDC users", async () => {
    const user = userEvent.setup();
    const oidcUser: ExtendedUser = {
      id: 3,
      username: "oidc-user",
      displayName: "OIDC User",
      email: "oidc@example.com",
      role: "viewer",
      provider: "oidc",
      createdAt: "2026-01-01T00:00:00Z",
    };
    list.mockResolvedValue([ME, oidcUser]);
    renderPage();
    await user.click(await screen.findByLabelText("Actions for oidc-user"));
    const resetItem = await screen.findByText("Reset password");
    const item = resetItem.closest("[role='menuitem']");
    expect(item).toHaveAttribute("aria-disabled", "true");
    expect(screen.getByText("Account is OIDC-managed")).toBeInTheDocument();
  });
});

describe("UsersPage tabs", () => {
  it("switches to roles tab", async () => {
    const user = userEvent.setup();
    renderPage();
    await user.click(await screen.findByRole("button", { name: /Roles/i }));
    expect(await screen.findByText("operator")).toBeInTheDocument();
    expect(screen.queryByText("alice")).not.toBeInTheDocument();
  });

  it("switches to service accounts tab", async () => {
    const user = userEvent.setup();
    renderPage();
    await user.click(await screen.findByRole("button", { name: /Service accounts/i }));
    expect(await screen.findByText(/tracked for v1.1/i)).toBeInTheDocument();
  });

  it("switches to identity providers tab", async () => {
    const user = userEvent.setup();
    renderPage();
    await user.click(await screen.findByRole("button", { name: /Identity providers/i }));
    expect(await screen.findByText(/configured in Helm values/i)).toBeInTheDocument();
  });
});

describe("InviteModal", () => {
  it("creates a user with all fields", async () => {
    const user = userEvent.setup();
    create.mockResolvedValue(ALICE);
    renderPage();
    await user.click(await screen.findByRole("button", { name: /Invite user/i }));

    await user.type(await screen.findByPlaceholderText("alice"), "newuser");
    await user.type(screen.getByPlaceholderText("Alice Operator"), "New User");
    await user.type(screen.getByPlaceholderText("alice@example.com"), "new@example.com");
    await user.type(screen.getAllByPlaceholderText(/At least 12 characters/)[0], "password-1234");

    await user.click(screen.getByRole("button", { name: /Create user/i }));

    await waitFor(() =>
      expect(create).toHaveBeenCalledWith(
        expect.objectContaining({
          username: "newuser",
          displayName: "New User",
          email: "new@example.com",
          password: "password-1234",
        }),
      ),
    );
  });

  it("allows creating without password (for OIDC invite)", async () => {
    const user = userEvent.setup();
    create.mockResolvedValue(ALICE);
    renderPage();
    await user.click(await screen.findByRole("button", { name: /Invite user/i }));

    await user.type(await screen.findByPlaceholderText("alice"), "oidc-user");
    await user.click(screen.getByRole("button", { name: /Create user/i }));

    await waitFor(() =>
      expect(create).toHaveBeenCalledWith(
        expect.objectContaining({
          username: "oidc-user",
          password: undefined,
        }),
      ),
    );
  });

  it("disables create button when password is too short", async () => {
    const user = userEvent.setup();
    renderPage();
    await user.click(await screen.findByRole("button", { name: /Invite user/i }));

    await user.type(await screen.findByPlaceholderText("alice"), "newuser");
    await user.type(screen.getAllByPlaceholderText(/At least 12 characters/)[0], "short");

    const button = screen.getByRole("button", { name: /Create user/i });
    expect(button).toBeDisabled();
    expect(screen.getByText(/At least 12 characters/i)).toBeInTheDocument();
  });

  it("displays error from API", async () => {
    const user = userEvent.setup();
    create.mockRejectedValue(new Error("User already exists"));
    renderPage();
    await user.click(await screen.findByRole("button", { name: /Invite user/i }));

    await user.type(await screen.findByPlaceholderText("alice"), "duplicate");
    await user.type(screen.getAllByPlaceholderText(/At least 12 characters/)[0], "password-1234");
    await user.click(screen.getByRole("button", { name: /Create user/i }));

    expect(await screen.findByText("User already exists")).toBeInTheDocument();
  });

  it("closes modal on cancel", async () => {
    const user = userEvent.setup();
    renderPage();
    await user.click(await screen.findByRole("button", { name: /Invite user/i }));
    expect(await screen.findByText("Invite user")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /Cancel/i }));
    expect(screen.queryByText("Invite user")).not.toBeInTheDocument();
  });
});

describe("EditUserModal", () => {
  it("updates display name only", async () => {
    const user = userEvent.setup();
    update.mockResolvedValue({ ...ALICE, displayName: "Alice Updated" });
    renderPage();
    await user.click(await screen.findByLabelText("Actions for alice"));
    await user.click(await screen.findByText("Edit user"));

    const displayInput = await screen.findByDisplayValue("Alice");
    await user.clear(displayInput);
    await user.type(displayInput, "Alice Updated");
    await user.click(screen.getByRole("button", { name: /Save changes/i }));

    await waitFor(() =>
      expect(update).toHaveBeenCalledWith(2, { displayName: "Alice Updated" }),
    );
  });

  it("disables save when no changes are made", async () => {
    const user = userEvent.setup();
    renderPage();
    await user.click(await screen.findByLabelText("Actions for alice"));
    await user.click(await screen.findByText("Edit user"));

    const button = await screen.findByRole("button", { name: /Save changes/i });
    expect(button).toBeDisabled();
  });

  it("blocks self-demotion from user management role", async () => {
    const user = userEvent.setup();
    useMeMock.mockReturnValue({
      data: { ...ME, role: "operator", permissions: { "*": ["users:manage"] } },
      error: null,
      isLoading: false,
    });
    list.mockResolvedValue([ME, ALICE]);
    renderPage();
    await user.click(await screen.findByLabelText("Actions for root"));
    await user.click(await screen.findByText("Edit user"));

    const select = await screen.findByDisplayValue("operator");
    await user.selectOptions(select, "viewer");

    expect(
      await screen.findByText(/can't remove your own ability to manage users/i),
    ).toBeInTheDocument();
    const button = screen.getByRole("button", { name: /Save changes/i });
    expect(button).toBeDisabled();
  });

  it("displays error from API", async () => {
    const user = userEvent.setup();
    update.mockRejectedValue(new Error("Permission denied"));
    renderPage();
    await user.click(await screen.findByLabelText("Actions for alice"));
    await user.click(await screen.findByText("Edit user"));

    const displayInput = await screen.findByDisplayValue("Alice");
    await user.clear(displayInput);
    await user.type(displayInput, "Alice New");
    await user.click(screen.getByRole("button", { name: /Save changes/i }));

    expect(await screen.findByText("Permission denied")).toBeInTheDocument();
  });

  it("closes modal on cancel", async () => {
    const user = userEvent.setup();
    renderPage();
    await user.click(await screen.findByLabelText("Actions for alice"));
    await user.click(await screen.findByText("Edit user"));

    expect(await screen.findByText("alice")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: /Cancel/i }));

    expect(screen.queryByDisplayValue("Alice")).not.toBeInTheDocument();
  });
});

describe("ResetPasswordModal", () => {
  it("disables button when password is too short", async () => {
    const user = userEvent.setup();
    renderPage();
    await user.click(await screen.findByLabelText("Actions for alice"));
    await user.click(await screen.findByText("Reset password"));

    const pw = await screen.findByPlaceholderText(/At least 12 characters/i);
    await user.type(pw, "short");

    const button = screen.getByRole("button", { name: /Set new password/i });
    expect(button).toBeDisabled();
    expect(screen.getByText(/At least 12 characters/i)).toBeInTheDocument();
  });

  it("displays error from API", async () => {
    const user = userEvent.setup();
    resetPassword.mockRejectedValue(new Error("Cannot reset password"));
    renderPage();
    await user.click(await screen.findByLabelText("Actions for alice"));
    await user.click(await screen.findByText("Reset password"));

    const pw = await screen.findByPlaceholderText(/At least 12 characters/i);
    await user.type(pw, "brand-new-password-1");
    await user.click(screen.getByRole("button", { name: /Set new password/i }));

    expect(await screen.findByText("Cannot reset password")).toBeInTheDocument();
  });

  it("closes modal on cancel", async () => {
    const user = userEvent.setup();
    renderPage();
    await user.click(await screen.findByLabelText("Actions for alice"));
    await user.click(await screen.findByText("Reset password"));

    const pw = await screen.findByPlaceholderText(/At least 12 characters/i);
    await user.type(pw, "password-1234");
    await user.click(screen.getByRole("button", { name: /Cancel/i }));

    expect(screen.queryByPlaceholderText(/At least 12 characters/i)).not.toBeInTheDocument();
  });
});

describe("DeleteUserDialog", () => {
  it("shows warning when user tries to delete themselves", async () => {
    const user = userEvent.setup();
    renderPage();
    await user.click(await screen.findByLabelText("Actions for root"));
    await user.click(await screen.findByText("Delete user"));

    expect(
      await screen.findByText(/You can't delete your own account/i),
    ).toBeInTheDocument();
  });

  it("displays error from API for other users", async () => {
    const user = userEvent.setup();
    remove.mockRejectedValue(new Error("Cannot delete admin user"));
    renderPage();
    await user.click(await screen.findByLabelText("Actions for alice"));
    await user.click(await screen.findByText("Delete user"));

    await user.click(await screen.findByRole("button", { name: "Delete user" }));

    expect(await screen.findByText("Cannot delete admin user")).toBeInTheDocument();
  });

  it("closes modal on cancel", async () => {
    const user = userEvent.setup();
    renderPage();
    await user.click(await screen.findByLabelText("Actions for alice"));
    await user.click(await screen.findByText("Delete user"));

    expect(await screen.findByText("Delete alice?")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: /Cancel/i }));

    expect(screen.queryByText("Delete alice?")).not.toBeInTheDocument();
  });
});

describe("UsersPage roles tab extended", () => {
  async function openRolesTab() {
    const user = userEvent.setup();
    renderPage();
    await user.click(await screen.findByRole("button", { name: /Roles/i }));
    return user;
  }

  it("hides new role button for users without roles:manage", async () => {
    useMeMock.mockReturnValue({
      data: { ...ME, permissions: { "*": ["audit:read"] } },
      error: null,
      isLoading: false,
    });
    await openRolesTab();
    expect(screen.queryByRole("button", { name: /New role/i })).toBeNull();
  });

  it("shows role description when available", async () => {
    await openRolesTab();
    expect(await screen.findByText("Full access.")).toBeInTheDocument();
    expect(screen.getByText("Manage servers.")).toBeInTheDocument();
  });

  it("shows permission count for non-wildcard roles", async () => {
    await openRolesTab();
    expect(await screen.findByText(/2 permissions/)).toBeInTheDocument();
  });

  it("shows 'all permissions' badge for admin role", async () => {
    await openRolesTab();
    expect(await screen.findByText("all permissions")).toBeInTheDocument();
  });

  it("allows editing role description", async () => {
    const user = await openRolesTab();
    rolesUpdate.mockResolvedValue({
      name: "operator",
      description: "Updated description",
      builtin: true,
      permissions: ["servers:read", "servers:write"],
    });

    const editBtn = await screen.findByRole("button", { name: /Edit/ });
    await user.click(editBtn);
    const descInput = await screen.findByDisplayValue("Manage servers.");
    await user.clear(descInput);
    await user.type(descInput, "Updated description");
    await user.click(screen.getByRole("button", { name: /Save role/i }));

    await waitFor(() =>
      expect(rolesUpdate).toHaveBeenCalledWith(
        "operator",
        expect.objectContaining({ description: "Updated description" }),
      ),
    );
  });

  it("closes role editor on cancel", async () => {
    const user = await openRolesTab();
    const editBtn = await screen.findByRole("button", { name: /Edit/ });
    await user.click(editBtn);

    expect(await screen.findByText(/Edit role/i)).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: /Cancel/i }));

    expect(screen.queryByText(/Edit role/i)).not.toBeInTheDocument();
  });
});

describe("NamespaceGrants", () => {
  it("displays namespace grants in edit modal", async () => {
    const user = userEvent.setup();
    const binding = {
      roleName: "operator",
      namespace: "gameplane-games",
    };
    bindings.mockResolvedValue([binding]);
    renderPage();
    await user.click(await screen.findByLabelText("Actions for alice"));
    await user.click(await screen.findByText("Edit user"));

    expect(await screen.findByText("Namespace grants")).toBeInTheDocument();
    expect(screen.getByText("operator")).toBeInTheDocument();
    expect(screen.getByText("gameplane-games")).toBeInTheDocument();
  });

  it("displays error when removing binding fails", async () => {
    const user = userEvent.setup();
    const binding = {
      roleName: "viewer",
      namespace: "prod",
    };
    bindings.mockResolvedValue([binding]);
    removeBinding.mockRejectedValue(new Error("Permission denied"));
    renderPage();
    await user.click(await screen.findByLabelText("Actions for alice"));
    await user.click(await screen.findByText("Edit user"));

    const removeBtn = await screen.findByLabelText(/Remove viewer in prod/);
    await user.click(removeBtn);

    expect(await screen.findByText("Permission denied")).toBeInTheDocument();
  });

  it("shows message when no namespace grants exist", async () => {
    const user = userEvent.setup();
    bindings.mockResolvedValue([]);
    renderPage();
    await user.click(await screen.findByLabelText("Actions for alice"));
    await user.click(await screen.findByText("Edit user"));

    expect(await screen.findByText("No per-namespace grants.")).toBeInTheDocument();
  });

  it("removes a namespace grant", async () => {
    const user = userEvent.setup();
    const binding = {
      roleName: "operator",
      namespace: "gameplane-games",
    };
    bindings.mockResolvedValue([binding]);
    removeBinding.mockResolvedValue(undefined);
    renderPage();
    await user.click(await screen.findByLabelText("Actions for alice"));
    await user.click(await screen.findByText("Edit user"));

    const removeBtn = await screen.findByLabelText(
      /Remove operator in gameplane-games/,
    );
    await user.click(removeBtn);

    await waitFor(() =>
      expect(removeBinding).toHaveBeenCalledWith(2, "operator", "gameplane-games"),
    );
  });

  it("adds a namespace grant", async () => {
    const user = userEvent.setup();
    bindings.mockResolvedValue([]);
    addBinding.mockResolvedValue({ roleName: "viewer", namespace: "custom-ns" });
    renderPage();
    await user.click(await screen.findByLabelText("Actions for alice"));
    await user.click(await screen.findByText("Edit user"));

    const select = await screen.findByDisplayValue("operator");
    await user.selectOptions(select, "viewer");

    const nsInput = await screen.findByPlaceholderText("namespace");
    await user.type(nsInput, "custom-ns");

    const addBtn = await screen.findByRole("button", { name: /Add/i });
    await user.click(addBtn);

    await waitFor(() =>
      expect(addBinding).toHaveBeenCalledWith(
        2,
        expect.objectContaining({ roleName: "viewer", namespace: "custom-ns" }),
      ),
    );
  });

  it("disables add button when namespace is empty", async () => {
    const user = userEvent.setup();
    bindings.mockResolvedValue([]);
    renderPage();
    await user.click(await screen.findByLabelText("Actions for alice"));
    await user.click(await screen.findByText("Edit user"));

    const addBtn = await screen.findByRole("button", { name: /Add/i });
    expect(addBtn).toBeDisabled();
  });

  it("displays error when adding binding fails", async () => {
    const user = userEvent.setup();
    bindings.mockResolvedValue([]);
    addBinding.mockRejectedValue(new Error("Namespace not found"));
    renderPage();
    await user.click(await screen.findByLabelText("Actions for alice"));
    await user.click(await screen.findByText("Edit user"));

    const nsInput = await screen.findByPlaceholderText("namespace");
    await user.type(nsInput, "invalid-ns");

    const addBtn = screen.getByRole("button", { name: /Add/i });
    await user.click(addBtn);

    expect(await screen.findByText("Namespace not found")).toBeInTheDocument();
  });
});
