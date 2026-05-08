import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { UsersPage } from "./Users";

const list = vi.fn();
const create = vi.fn();
vi.mock("@/lib/endpoints", () => ({
  Users: {
    list: () => list(),
    create: (body: unknown) => create(body),
    update: vi.fn(),
    remove: vi.fn(),
    resetPassword: vi.fn(),
  },
}));
vi.mock("@/lib/auth", () => ({
  useMe: () => ({
    data: { id: 1, username: "root", displayName: "Root", email: "r@x", role: "admin" },
    error: null,
    isLoading: false,
  }),
}));
vi.mock("@tanstack/react-router", () => ({
  Link: ({ children, ...rest }: { children: ReactNode } & Record<string, unknown>) => (
    <a {...rest}>{children}</a>
  ),
}));

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <UsersPage />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  list.mockResolvedValue([]);
});

afterEach(() => {
  vi.resetAllMocks();
});

describe("UsersPage Invite dialog", () => {
  it("opens on Invite click and disables Create until username set", async () => {
    renderPage();
    await userEvent.click(screen.getByRole("button", { name: /Invite user/i }));
    expect(await screen.findByRole("heading", { name: /Invite user/i })).toBeInTheDocument();
    const submit = screen.getByRole("button", { name: /Create user/i });
    expect(submit).toBeDisabled();
  });

  it("validates password length", async () => {
    renderPage();
    await userEvent.click(screen.getByRole("button", { name: /Invite user/i }));
    await screen.findByRole("heading", { name: /Invite user/i });
    const usernameInput = screen.getByPlaceholderText("alice") as HTMLInputElement;
    await userEvent.type(usernameInput, "newuser");
    const pwInput = screen.getByPlaceholderText(/At least 12 characters/);
    await userEvent.type(pwInput, "short");
    expect(screen.getByText(/At least 12 characters\./)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Create user/i })).toBeDisabled();
  });

  it("submits a create with the form values", async () => {
    create.mockResolvedValue({ id: 99 });
    renderPage();
    await userEvent.click(screen.getByRole("button", { name: /Invite user/i }));
    await screen.findByRole("heading", { name: /Invite user/i });
    const usernameInput = screen.getByPlaceholderText("alice") as HTMLInputElement;
    await userEvent.type(usernameInput, "newuser");
    await userEvent.click(screen.getByRole("button", { name: /Create user/i }));
    await waitFor(() => expect(create).toHaveBeenCalled());
    const args = create.mock.calls[0][0];
    expect(args.username).toBe("newuser");
  });

  it("Cancel closes the dialog", async () => {
    renderPage();
    await userEvent.click(screen.getByRole("button", { name: /Invite user/i }));
    await screen.findByRole("heading", { name: /Invite user/i });
    await userEvent.click(screen.getByRole("button", { name: "Cancel" }));
    await waitFor(() =>
      expect(screen.queryByRole("heading", { name: /Invite user/i })).not.toBeInTheDocument(),
    );
  });
});
