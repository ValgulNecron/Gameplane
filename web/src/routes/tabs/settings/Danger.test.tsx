import { describe, it, expect, vi } from "vitest";
import { http, HttpResponse } from "msw";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { server } from "@/test/server";
import { renderWithQuery } from "@/test/render";

const navigate = vi.fn();
vi.mock("@tanstack/react-router", () => ({
  useNavigate: () => navigate,
}));

import { DangerSection } from "./Danger";

describe("DangerSection", () => {
  it("renders the three rows", () => {
    renderWithQuery(<DangerSection name="alpha" />);
    expect(screen.getByText(/Wipe world data/i)).toBeInTheDocument();
    expect(screen.getByText(/Transfer ownership/i)).toBeInTheDocument();
    // "Delete server" matches both the row title and the button label.
    expect(screen.getAllByText(/Delete server/i).length).toBeGreaterThan(0);
  });

  it("Delete button opens the confirm dialog", async () => {
    renderWithQuery(<DangerSection name="alpha" />);
    await userEvent.click(screen.getByRole("button", { name: /Delete server…/i }));
    expect(await screen.findByText(/Delete alpha\?/)).toBeInTheDocument();
  });

  it("happy-path delete navigates back to /servers", async () => {
    server.use(
      http.delete("/servers/alpha", () => HttpResponse.json({})),
    );
    navigate.mockClear();
    renderWithQuery(<DangerSection name="alpha" />);
    await userEvent.click(screen.getByRole("button", { name: /Delete server…/i }));
    // Need to type the confirm phrase first.
    const inp = await screen.findByRole("textbox");
    await userEvent.type(inp, "alpha");
    await userEvent.click(await screen.findByRole("button", { name: /Confirm|Delete/i }));
    await waitFor(() => expect(navigate).toHaveBeenCalledWith({ to: "/servers" }));
  });

  it("Wipe button confirms and posts the wipe request", async () => {
    let body: unknown;
    server.use(
      http.post("/servers/alpha:wipe-data", async ({ request }) => {
        body = await request.json();
        return new HttpResponse(null, { status: 202 });
      }),
    );
    renderWithQuery(<DangerSection name="alpha" />);
    await userEvent.click(screen.getByRole("button", { name: /Wipe world…/i }));
    expect(await screen.findByText(/Wipe alpha.s world data/i)).toBeInTheDocument();
    await userEvent.type(await screen.findByRole("textbox"), "alpha");
    await userEvent.click(await screen.findByRole("button", { name: /Wipe world data/i }));
    await waitFor(() => expect(body).toEqual({ confirm: "alpha" }));
  });

  it("delete failure surfaces the error", async () => {
    server.use(
      http.delete("/servers/alpha", () =>
        HttpResponse.text("nope", { status: 500 }),
      ),
    );
    renderWithQuery(<DangerSection name="alpha" />);
    await userEvent.click(screen.getByRole("button", { name: /Delete server…/i }));
    const inp = await screen.findByRole("textbox");
    await userEvent.type(inp, "alpha");
    await userEvent.click(await screen.findByRole("button", { name: /Confirm|Delete/i }));
    await waitFor(() => expect(screen.getByText(/nope/)).toBeInTheDocument());
  });
});
