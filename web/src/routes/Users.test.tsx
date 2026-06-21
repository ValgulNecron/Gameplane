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
