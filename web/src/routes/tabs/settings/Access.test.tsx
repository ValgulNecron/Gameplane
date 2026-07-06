import { describe, it, expect, vi } from "vitest";
import { http, HttpResponse } from "msw";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { server } from "@/test/server";
import { renderWithQuery } from "@/test/render";
import { makeServer } from "@/test/factories";
import { AccessSection } from "./Access";

vi.mock("@/lib/auth", () => ({
  useMe: () => ({ data: { id: 1, username: "alice", permissions: {} } }),
  can: () => false,
}));

describe("AccessSection", () => {
  it("renders owner and empty collaborators", () => {
    const gs = makeServer({
      metadata: {
        name: "test",
        namespace: "ns",
        annotations: {
          "gameplane.local/owner": "alice",
          "gameplane.local/owner-id": "1",
        },
      },
      spec: { templateRef: { name: "mc" } },
    });
    renderWithQuery(<AccessSection gs={gs} />);
    expect(screen.getByText("alice")).toBeInTheDocument();
    expect(screen.getByText(/None yet/i)).toBeInTheDocument();
  });

  it("renders owner and collaborators list", () => {
    const gs = makeServer({
      metadata: {
        name: "test",
        namespace: "ns",
        annotations: {
          "gameplane.local/owner": "alice",
          "gameplane.local/owner-id": "1",
          "gameplane.local/collaborators": "2,3",
          "gameplane.local/collaborator-names": "bob,charlie",
        },
      },
      spec: { templateRef: { name: "mc" } },
    });
    renderWithQuery(<AccessSection gs={gs} />);
    expect(screen.getByText("bob")).toBeInTheDocument();
    expect(screen.getByText("charlie")).toBeInTheDocument();
  });

  it("owner can add a collaborator", async () => {
    let requestBody: unknown;
    server.use(
      http.put("/servers/test:collaborators", async ({ request }) => {
        requestBody = await request.json();
        return new HttpResponse(null, { status: 204 });
      }),
    );

    vi.mocked(require("@/lib/auth").useMe).mockReturnValue({
      data: { id: 1, username: "alice", permissions: {} },
    });
    vi.mocked(require("@/lib/auth").can).mockReturnValue(false);

    const gs = makeServer({
      metadata: {
        name: "test",
        namespace: "ns",
        annotations: {
          "gameplane.local/owner": "alice",
          "gameplane.local/owner-id": "1",
        },
      },
      spec: { templateRef: { name: "mc" } },
    });
    renderWithQuery(<AccessSection gs={gs} />);

    // Mock useMe to return the owner
    vi.resetModules();
    vi.doMock("@/lib/auth", () => ({
      useMe: () => ({ data: { id: 1, username: "alice", permissions: {} } }),
      can: () => false,
    }));

    const input = screen.getByPlaceholderText(/Add collaborator/i);
    await userEvent.type(input, "bob");
    await userEvent.click(screen.getByRole("button", { name: /^Add$/i }));

    await waitFor(() =>
      expect(requestBody).toEqual({ userIds: [], usernames: ["bob"] }),
    );
  });

  it("non-owner without servers:write cannot see add controls", () => {
    vi.mocked(require("@/lib/auth").useMe).mockReturnValue({
      data: { id: 99, username: "viewer", permissions: {} },
    });
    vi.mocked(require("@/lib/auth").can).mockReturnValue(false);

    const gs = makeServer({
      metadata: {
        name: "test",
        namespace: "ns",
        annotations: {
          "gameplane.local/owner": "alice",
          "gameplane.local/owner-id": "1",
          "gameplane.local/collaborators": "2",
          "gameplane.local/collaborator-names": "bob",
        },
      },
      spec: { templateRef: { name: "mc" } },
    });
    renderWithQuery(<AccessSection gs={gs} />);

    expect(screen.queryByPlaceholderText(/Add collaborator/i)).not.toBeInTheDocument();
    // Remove buttons should not be visible
    const removeButtons = screen.queryAllByTitle("Remove");
    expect(removeButtons).toHaveLength(0);
  });

  it("owner can remove a collaborator", async () => {
    let requestBody: unknown;
    server.use(
      http.put("/servers/test:collaborators", async ({ request }) => {
        requestBody = await request.json();
        return new HttpResponse(null, { status: 204 });
      }),
    );

    vi.mocked(require("@/lib/auth").useMe).mockReturnValue({
      data: { id: 1, username: "alice", permissions: {} },
    });
    vi.mocked(require("@/lib/auth").can).mockReturnValue(false);

    const gs = makeServer({
      metadata: {
        name: "test",
        namespace: "ns",
        annotations: {
          "gameplane.local/owner": "alice",
          "gameplane.local/owner-id": "1",
          "gameplane.local/collaborators": "2,3",
          "gameplane.local/collaborator-names": "bob,charlie",
        },
      },
      spec: { templateRef: { name: "mc" } },
    });
    renderWithQuery(<AccessSection gs={gs} />);

    const removeButtons = screen.getAllByTitle("Remove");
    expect(removeButtons).toHaveLength(2);
    await userEvent.click(removeButtons[0]);

    await waitFor(() =>
      expect(requestBody).toEqual({ userIds: [3] }),
    );
  });

  it("displays error on add failure", async () => {
    server.use(
      http.put("/servers/test:collaborators", () =>
        HttpResponse.text("user not found: dave", { status: 400 }),
      ),
    );

    vi.mocked(require("@/lib/auth").useMe).mockReturnValue({
      data: { id: 1, username: "alice", permissions: {} },
    });
    vi.mocked(require("@/lib/auth").can).mockReturnValue(false);

    const gs = makeServer({
      metadata: {
        name: "test",
        namespace: "ns",
        annotations: {
          "gameplane.local/owner": "alice",
          "gameplane.local/owner-id": "1",
        },
      },
      spec: { templateRef: { name: "mc" } },
    });
    renderWithQuery(<AccessSection gs={gs} />);

    const input = screen.getByPlaceholderText(/Add collaborator/i);
    await userEvent.type(input, "dave");
    await userEvent.click(screen.getByRole("button", { name: /^Add$/i }));

    await waitFor(() =>
      expect(screen.getByText(/user not found: dave/i)).toBeInTheDocument(),
    );
  });
});
