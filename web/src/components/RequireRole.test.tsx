import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { RequireRole, RequirePermission } from "./RequireRole";
import { APIError } from "@/lib/api";

const useMeMock = vi.fn();
// Keep the real can(); only stub useMe.
vi.mock("@/lib/auth", async (orig) => ({
  ...(await orig<typeof import("@/lib/auth")>()),
  useMe: () => useMeMock(),
}));

vi.mock("@tanstack/react-router", () => ({
  Link: ({ children, ...rest }: { children: ReactNode } & Record<string, unknown>) => (
    <a {...rest}>{children}</a>
  ),
}));

const assignMock = vi.fn();
const refetchMock = vi.fn();
beforeEach(() => {
  vi.stubGlobal("location", { assign: assignMock });
});
afterEach(() => {
  useMeMock.mockReset();
  assignMock.mockReset();
  refetchMock.mockReset();
  vi.unstubAllGlobals();
});

function renderGuard(roles: ("admin" | "operator" | "viewer")[]) {
  return render(
    <RequireRole roles={roles}>
      <div>secret</div>
    </RequireRole>,
  );
}

describe("RequireRole", () => {
  it("renders a skeleton while loading", () => {
    useMeMock.mockReturnValue({ data: undefined, error: null, isLoading: true });
    const { container } = renderGuard(["admin"]);
    expect(container.querySelector(".animate-pulse")).not.toBeNull();
  });

  it("renders Forbidden when role doesn't match", () => {
    useMeMock.mockReturnValue({
      data: { id: 1, username: "v", displayName: "V", email: "", role: "viewer" },
      error: null,
      isLoading: false,
    });
    renderGuard(["admin"]);
    expect(screen.getByText("Access denied")).toBeInTheDocument();
    expect(screen.queryByText("secret")).toBeNull();
  });

  it("renders children when role matches", () => {
    useMeMock.mockReturnValue({
      data: { id: 1, username: "a", displayName: "A", email: "", role: "admin" },
      error: null,
      isLoading: false,
    });
    renderGuard(["admin"]);
    expect(screen.getByText("secret")).toBeInTheDocument();
  });

  it("redirects to /login on 401", () => {
    useMeMock.mockReturnValue({
      data: undefined,
      error: new APIError(401, "unauth"),
      isLoading: false,
    });
    renderGuard(["admin"]);
    expect(assignMock).toHaveBeenCalledWith("/login");
  });

  it("offers a retry — not Access denied — when the identity fetch fails", () => {
    useMeMock.mockReturnValue({
      data: undefined,
      error: new APIError(500, "boom"),
      isLoading: false,
      refetch: refetchMock,
    });
    renderGuard(["admin"]);
    expect(screen.getByText("Can't reach the control plane")).toBeInTheDocument();
    expect(screen.queryByText("Access denied")).toBeNull();
    expect(assignMock).not.toHaveBeenCalled();
  });

  it("refetches when the retry button is pressed", async () => {
    useMeMock.mockReturnValue({
      data: undefined,
      error: new APIError(503, "unavailable"),
      isLoading: false,
      refetch: refetchMock,
    });
    renderGuard(["admin"]);
    await userEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(refetchMock).toHaveBeenCalledOnce();
  });
});

describe("RequirePermission", () => {
  function renderPerm(perm: string) {
    return render(
      <RequirePermission perm={perm}>
        <div>secret</div>
      </RequirePermission>,
    );
  }

  it("renders children when the permission is held", () => {
    useMeMock.mockReturnValue({
      data: { id: 1, username: "a", role: "admin", permissions: { "*": ["*"] } },
      error: null,
      isLoading: false,
    });
    renderPerm("users:manage");
    expect(screen.getByText("secret")).toBeInTheDocument();
  });

  it("renders Forbidden when the permission is missing", () => {
    useMeMock.mockReturnValue({
      data: { id: 1, username: "v", role: "viewer", permissions: { "*": ["servers:read"] } },
      error: null,
      isLoading: false,
    });
    renderPerm("users:manage");
    expect(screen.getByText("Access denied")).toBeInTheDocument();
    expect(screen.queryByText("secret")).toBeNull();
  });

  // Regression: a 500 on /users/me used to fall through to Forbidden, so a
  // transient API restart told an admin they had lost their permissions.
  it("offers a retry — not Access denied — when the identity fetch fails", () => {
    useMeMock.mockReturnValue({
      data: undefined,
      error: new APIError(500, "boom"),
      isLoading: false,
      refetch: refetchMock,
    });
    renderPerm("users:manage");
    expect(screen.getByText("Can't reach the control plane")).toBeInTheDocument();
    expect(screen.queryByText("Access denied")).toBeNull();
  });
});
