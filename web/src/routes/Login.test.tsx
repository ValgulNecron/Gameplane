import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { LoginPage } from "./Login";

vi.mock("@tanstack/react-router", async () => {
  const actual = await vi.importActual("@tanstack/react-router");
  return {
    ...actual,
    useNavigate: vi.fn(),
  };
});

const fetchMock = vi.fn();
const assignMock = vi.fn();
const navigateMock = vi.fn().mockResolvedValue(undefined);

// The login page fetches /auth/providers on mount; route fetches by URL
// so a single mock serves both the providers probe and the login POST.
let providers: Array<{ name?: string; kind: string; label: string }>;
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
  loginResponse = () =>
    new Response(
      JSON.stringify({
        user: { id: 1, username: "admin", displayName: "Admin", role: "admin", permissions: {} },
        csrf: "token123",
      }),
      { status: 200 },
    );
  fetchMock.mockImplementation(router);
  vi.stubGlobal("fetch", fetchMock);
  vi.stubGlobal("location", { assign: assignMock });
  vi.mocked(useNavigate).mockReturnValue(navigateMock);
  navigateMock.mockResolvedValue(undefined);
});
afterEach(() => {
  fetchMock.mockReset();
  assignMock.mockReset();
  navigateMock.mockReset();
  vi.unstubAllGlobals();
});

describe("LoginPage", () => {
  it("submits credentials, seeds cache, and navigates to / on success", async () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={queryClient}>
        <LoginPage />
      </QueryClientProvider>,
    );
    fireEvent.change(screen.getByLabelText(/username/i), { target: { value: "admin" } });
    // The show/hide toggle also carries "password" in its aria-label, so
    // scope the query to the input element.
    fireEvent.change(screen.getByLabelText(/password/i, { selector: "input" }), {
      target: { value: "hunter2" },
    });
    fireEvent.click(screen.getByRole("button", { name: /sign in/i }));

    await waitFor(() => expect(navigateMock).toHaveBeenCalledWith({ to: "/" }));
    // Verify no hard reload triggered
    expect(assignMock).not.toHaveBeenCalled();
    // Verify the ["me"] cache was seeded with the user from the response
    expect(queryClient.getQueryData(["me"])).toEqual({
      id: 1,
      username: "admin",
      displayName: "Admin",
      role: "admin",
      permissions: {},
    });
    const login = fetchMock.mock.calls.find((c) => String(c[0]).includes("/auth/login"));
    expect(login).toBeTruthy();
    const init = login![1] as RequestInit;
    expect(init.method).toBe("POST");
    expect(init.body).toBe(JSON.stringify({ username: "admin", password: "hunter2" }));
  });

  it("shows an error message on 401", async () => {
    loginResponse = () => new Response("nope", { status: 401 });
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={queryClient}>
        <LoginPage />
      </QueryClientProvider>,
    );
    fireEvent.change(screen.getByLabelText(/username/i), { target: { value: "x" } });
    fireEvent.change(screen.getByLabelText(/password/i, { selector: "input" }), {
      target: { value: "y" },
    });
    fireEvent.click(screen.getByRole("button", { name: /sign in/i }));

    expect(await screen.findByText("Invalid credentials")).toBeInTheDocument();
    expect(assignMock).not.toHaveBeenCalled();
    expect(navigateMock).not.toHaveBeenCalled();
  });

  it("reveals a reset hint when Forgot is clicked", async () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={queryClient}>
        <LoginPage />
      </QueryClientProvider>,
    );
    expect(screen.queryByText(/contact your administrator/i)).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Forgot?" }));
    expect(screen.getByText(/contact your administrator to reset/i)).toBeInTheDocument();
  });

  it("shows the OIDC button only when configured, using its label", async () => {
    providers = [
      { kind: "local", label: "Local account" },
      { kind: "oidc", label: "Acme SSO" },
    ];
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={queryClient}>
        <LoginPage />
      </QueryClientProvider>,
    );
    const btn = await screen.findByRole("button", { name: /Continue with Acme SSO/i });
    fireEvent.click(btn);
    expect(assignMock).toHaveBeenCalledWith("/auth/oidc/start");
  });

  it("hides the OIDC button when only local auth is enabled", async () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={queryClient}>
        <LoginPage />
      </QueryClientProvider>,
    );
    await screen.findByRole("button", { name: /sign in/i });
    await waitFor(() =>
      expect(screen.queryByRole("button", { name: /Continue with/i })).not.toBeInTheDocument(),
    );
  });

  it("renders one button per SSO provider, routed by name", async () => {
    providers = [
      { name: "local", kind: "local", label: "Local account" },
      { name: "corp", kind: "oidc", label: "Acme SSO" },
      { name: "helm", kind: "oidc", label: "Helm SSO" },
    ];
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={queryClient}>
        <LoginPage />
      </QueryClientProvider>,
    );
    const corp = await screen.findByRole("button", { name: /Continue with Acme SSO/i });
    fireEvent.click(corp);
    expect(assignMock).toHaveBeenCalledWith("/auth/oidc/corp/start");
    // The Helm-flag provider keeps the legacy start path — its cookies
    // and IdP-registered callback live there.
    fireEvent.click(screen.getByRole("button", { name: /Continue with Helm SSO/i }));
    expect(assignMock).toHaveBeenCalledWith("/auth/oidc/start");
  });

  it("hides the password form when local login is disabled", async () => {
    providers = [{ name: "corp", kind: "oidc", label: "Acme SSO" }];
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={queryClient}>
        <LoginPage />
      </QueryClientProvider>,
    );
    await screen.findByRole("button", { name: /Continue with Acme SSO/i });
    expect(screen.queryByLabelText(/username/i)).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /sign in/i })).not.toBeInTheDocument();
  });

  it("keeps the password form when the providers fetch fails", async () => {
    fetchMock.mockImplementation((url: RequestInfo | URL) => {
      if (String(url).includes("/auth/providers")) {
        return Promise.reject(new Error("network down"));
      }
      return router(url);
    });
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={queryClient}>
        <LoginPage />
      </QueryClientProvider>,
    );
    expect(await screen.findByRole("button", { name: /sign in/i })).toBeInTheDocument();
  });

  it("reveals the typed password when the eye toggle is clicked", async () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={queryClient}>
        <LoginPage />
      </QueryClientProvider>,
    );
    const pw = screen.getByLabelText(/password/i, { selector: "input" }) as HTMLInputElement;
    fireEvent.change(pw, { target: { value: "hunter2" } });
    expect(pw.type).toBe("password");
    fireEvent.click(screen.getByRole("button", { name: "Show password" }));
    expect(pw.type).toBe("text");
    fireEvent.click(screen.getByRole("button", { name: "Hide password" }));
    expect(pw.type).toBe("password");
  });

  it("renders no version string on the pre-auth page", () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const { container } = render(
      <QueryClientProvider client={queryClient}>
        <LoginPage />
      </QueryClientProvider>,
    );
    expect(container.textContent).not.toMatch(/v\d+\.\d+\.\d+/);
    expect(container.textContent).not.toMatch(/alpha/i);
  });
});
