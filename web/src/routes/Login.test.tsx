import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { LoginPage } from "./Login";

const fetchMock = vi.fn();
const assignMock = vi.fn();

// The login page fetches /auth/providers on mount; route fetches by URL
// so a single mock serves both the providers probe and the login POST.
let providers: Array<{ kind: string; label: string }>;
let loginResponse: () => Response;

function router(url: RequestInfo | URL): Promise<Response> {
  const u = String(url);
  if (u.includes("/auth/providers")) {
    return Promise.resolve(new Response(JSON.stringify({ providers }), { status: 200 }));
  }
  if (u.includes("/auth/login")) {
    return Promise.resolve(loginResponse());
  }
  return Promise.resolve(new Response("{}", { status: 200 }));
}

beforeEach(() => {
  providers = [{ kind: "local", label: "Local account" }];
  loginResponse = () => new Response(JSON.stringify({ id: 1, role: "admin" }), { status: 200 });
  fetchMock.mockImplementation(router);
  vi.stubGlobal("fetch", fetchMock);
  vi.stubGlobal("location", { assign: assignMock });
});
afterEach(() => {
  fetchMock.mockReset();
  assignMock.mockReset();
  vi.unstubAllGlobals();
});

describe("LoginPage", () => {
  it("submits credentials and navigates to / on success", async () => {
    render(<LoginPage />);
    fireEvent.change(screen.getByLabelText(/username/i), { target: { value: "admin" } });
    fireEvent.change(screen.getByLabelText(/password/i), { target: { value: "hunter2" } });
    fireEvent.click(screen.getByRole("button", { name: /sign in/i }));

    await waitFor(() => expect(assignMock).toHaveBeenCalledWith("/"));
    const login = fetchMock.mock.calls.find((c) => String(c[0]).includes("/auth/login"));
    expect(login).toBeTruthy();
    const init = login![1] as RequestInit;
    expect(init.method).toBe("POST");
    expect(init.body).toBe(JSON.stringify({ username: "admin", password: "hunter2" }));
  });

  it("shows an error message on 401", async () => {
    loginResponse = () => new Response("nope", { status: 401 });
    render(<LoginPage />);
    fireEvent.change(screen.getByLabelText(/username/i), { target: { value: "x" } });
    fireEvent.change(screen.getByLabelText(/password/i), { target: { value: "y" } });
    fireEvent.click(screen.getByRole("button", { name: /sign in/i }));

    expect(await screen.findByText("Invalid credentials")).toBeInTheDocument();
    expect(assignMock).not.toHaveBeenCalled();
  });

  it("reveals a reset hint when Forgot is clicked", async () => {
    render(<LoginPage />);
    expect(screen.queryByText(/contact your administrator/i)).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Forgot?" }));
    expect(screen.getByText(/contact your administrator to reset/i)).toBeInTheDocument();
  });

  it("shows the OIDC button only when configured, using its label", async () => {
    providers = [
      { kind: "local", label: "Local account" },
      { kind: "oidc", label: "Acme SSO" },
    ];
    render(<LoginPage />);
    const btn = await screen.findByRole("button", { name: /Continue with Acme SSO/i });
    fireEvent.click(btn);
    expect(assignMock).toHaveBeenCalledWith("/auth/oidc/start");
  });

  it("hides the OIDC button when only local auth is enabled", async () => {
    render(<LoginPage />);
    await screen.findByRole("button", { name: /sign in/i });
    await waitFor(() =>
      expect(screen.queryByRole("button", { name: /Continue with/i })).not.toBeInTheDocument(),
    );
  });

  it("renders no version string on the pre-auth page", () => {
    const { container } = render(<LoginPage />);
    expect(container.textContent).not.toMatch(/v\d+\.\d+\.\d+/);
    expect(container.textContent).not.toMatch(/alpha/i);
  });
});
