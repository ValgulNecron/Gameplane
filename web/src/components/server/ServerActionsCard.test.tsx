import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, screen, waitFor } from "@testing-library/react";
import { renderWithQuery } from "@/test/render";
import { ServerActionsCard } from "./ServerActionsCard";
import type { GameTemplate, ServerActionDecl } from "@/types";

const fetchMock = vi.fn();

beforeEach(() => vi.stubGlobal("fetch", fetchMock));
afterEach(() => {
  fetchMock.mockReset();
  vi.unstubAllGlobals();
});

function jsonRes(body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  });
}

interface RunCall {
  id: string;
  params?: Record<string, string>;
}

// routeFetch answers /users/me with the given role and records POSTs to
// the action-run endpoint.
function routeFetch(role: "operator" | "viewer", runs: RunCall[]) {
  // Running module actions is gated on servers:write; mirror that here.
  const permissions =
    role === "operator" ? { "*": ["servers:read", "servers:write"] } : { "*": ["servers:read"] };
  fetchMock.mockImplementation((url: string, opts?: { method?: string; body?: string }) => {
    if (url.endsWith("/users/me")) {
      return Promise.resolve(
        jsonRes({ id: 1, username: "u", displayName: "U", email: "", role, permissions }),
      );
    }
    if (url.endsWith("/actions/run")) {
      runs.push(JSON.parse(opts?.body ?? "{}") as RunCall);
      return Promise.resolve(jsonRes({ ok: true, raw: "done" }));
    }
    return Promise.resolve(jsonRes({}));
  });
}

function tmpl(actions: ServerActionDecl[]): GameTemplate {
  return {
    metadata: { name: "minecraft-java" },
    spec: {
      displayName: "Minecraft",
      game: "minecraft-java",
      version: "1",
      image: "img",
      rcon: { protocol: "source" },
      capabilities: { actions },
    },
  };
}

describe("ServerActionsCard", () => {
  it("renders nothing when no actions are declared", () => {
    const { container } = renderWithQuery(<ServerActionsCard name="s1" tmpl={tmpl([])} />);
    expect(container).toBeEmptyDOMElement();
  });

  it("runs a no-parameter action immediately", async () => {
    const runs: RunCall[] = [];
    routeFetch("operator", runs);
    renderWithQuery(
      <ServerActionsCard name="s1" tmpl={tmpl([{ id: "save-world", displayName: "Save world" }])} />,
    );
    const btn = await screen.findByRole("button", { name: /save world/i });
    await waitFor(() => expect(btn).not.toBeDisabled());
    fireEvent.click(btn);
    await waitFor(() => expect(runs).toEqual([{ id: "save-world" }]));
    expect(await screen.findByText("done")).toBeInTheDocument();
  });

  it("collects parameters in a dialog before running", async () => {
    const runs: RunCall[] = [];
    routeFetch("operator", runs);
    renderWithQuery(
      <ServerActionsCard
        name="s1"
        tmpl={tmpl([
          {
            id: "broadcast",
            displayName: "Broadcast message",
            params: [{ name: "message", type: "string", required: true }],
          },
        ])}
      />,
    );
    const open = await screen.findByRole("button", { name: /broadcast message/i });
    await waitFor(() => expect(open).not.toBeDisabled());
    fireEvent.click(open);

    const input = await screen.findByRole("textbox");
    fireEvent.change(input, { target: { value: "hello world" } });
    fireEvent.click(screen.getByRole("button", { name: "Run" }));

    await waitFor(() =>
      expect(runs).toEqual([{ id: "broadcast", params: { message: "hello world" } }]),
    );
  });

  it("disables actions for a viewer", async () => {
    routeFetch("viewer", []);
    renderWithQuery(
      <ServerActionsCard name="s1" tmpl={tmpl([{ id: "save-world", displayName: "Save world" }])} />,
    );
    const btn = await screen.findByRole("button", { name: /save world/i });
    // The /users/me query resolves to viewer; the button stays disabled.
    await waitFor(() => expect(btn).toBeDisabled());
  });
});
