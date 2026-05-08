import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { LoginPage } from "./Login";

const fetchMock = vi.fn();
const assignMock = vi.fn();

beforeEach(() => {
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
    fetchMock.mockResolvedValueOnce(
      new Response(JSON.stringify({ id: 1, role: "admin" }), { status: 200 }),
    );

    render(<LoginPage />);
    fireEvent.change(screen.getByLabelText(/username/i), {
      target: { value: "admin" },
    });
    fireEvent.change(screen.getByLabelText(/password/i), {
      target: { value: "hunter2" },
    });
    fireEvent.click(screen.getByRole("button", { name: /sign in/i }));

    await waitFor(() => expect(assignMock).toHaveBeenCalledWith("/"));
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe("/auth/login");
    expect(init.method).toBe("POST");
    expect(init.body).toBe(JSON.stringify({ username: "admin", password: "hunter2" }));
  });

  it("shows an error message on 401", async () => {
    fetchMock.mockResolvedValueOnce(
      new Response("nope", { status: 401 }),
    );

    render(<LoginPage />);
    fireEvent.change(screen.getByLabelText(/username/i), {
      target: { value: "x" },
    });
    fireEvent.change(screen.getByLabelText(/password/i), {
      target: { value: "y" },
    });
    fireEvent.click(screen.getByRole("button", { name: /sign in/i }));

    expect(await screen.findByText("Invalid credentials")).toBeInTheDocument();
    expect(assignMock).not.toHaveBeenCalled();
  });

  it("the OIDC button navigates to /auth/oidc/start", () => {
    render(<LoginPage />);
    fireEvent.click(screen.getByRole("button", { name: /Continue with OIDC/i }));
    expect(assignMock).toHaveBeenCalledWith("/auth/oidc/start");
  });
});
