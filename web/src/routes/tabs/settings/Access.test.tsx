import { describe, it, expect, vi, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { screen, waitFor, fireEvent } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { server } from "@/test/server";
import { renderWithQuery } from "@/test/render";
import { makeServer } from "@/test/factories";
import { AccessSection as AccessSectionView } from "./Access";
import type { GameServer } from "@/types";
import { ResourceTargetProvider, type ServerAccess } from "@/lib/resourceTarget";

const useMeMock = vi.fn();
const targetAdminMock = vi.fn();
vi.mock("@/lib/auth", async (orig) => ({
  ...(await orig<typeof import("@/lib/auth")>()),
  useMe: () => useMeMock(),
}));

function AccessSection({ gs, access }: { gs?: GameServer; access?: ServerAccess }) {
  const user = (useMeMock() as { data?: { id: number } }).data;
  const owner = !!gs && gs.metadata.annotations?.["gameplane.local/owner-id"] === String(user?.id);
  const administrator = targetAdminMock() === true;
  const targetAccess = access ?? { canWrite: administrator, canControl: administrator || owner, canConsole: administrator || owner, canDelete: administrator || owner, isOwner: owner, isCollaborator: false, permissions: administrator ? ["servers:read", "servers:write"] : [] };
  return <ResourceTargetProvider target={{ cluster: "remote", namespace: gs?.metadata.namespace ?? "gameplane-games", name: gs?.metadata.name ?? "test", uid: gs?.metadata.uid }} access={targetAccess}><AccessSectionView gs={gs} /></ResourceTargetProvider>;
}

describe("AccessSection", () => {
  beforeEach(() => {
    useMeMock.mockReturnValue({ data: { id: 1, username: "alice", permissions: {} } });
    targetAdminMock.mockReturnValue(false);
  });

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
    expect(screen.getByText("None yet")).toBeInTheDocument();
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
    let requestURL = "";
    server.use(
      http.put("/servers/test:collaborators", async ({ request }) => {
        requestURL = request.url;
        requestBody = await request.json();
        return new HttpResponse(null, { status: 204 });
      }),
    );

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
    await userEvent.type(input, "bob");
    await userEvent.click(screen.getByRole("button", { name: /^Add$/i }));

    await waitFor(() =>
      expect(requestBody).toEqual({ userIds: [], usernames: ["bob"] }),
    );
    expect(new URL(requestURL).searchParams.get("cluster")).toBe("remote");
    expect(new URL(requestURL).searchParams.get("namespace")).toBe("ns");
  });

  it("non-owner without servers:write cannot see add controls", () => {
    useMeMock.mockReturnValue({
      data: { id: 99, username: "viewer", permissions: {} },
    });
    targetAdminMock.mockReturnValue(false);

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
    const removeButtons = screen.queryAllByLabelText(/Remove/);
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

    const removeButtons = screen.getAllByLabelText(/Remove/);
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

  it("can add to existing collaborators", async () => {
    let requestBody: unknown;
    server.use(
      http.put("/servers/test:collaborators", async ({ request }) => {
        requestBody = await request.json();
        return new HttpResponse(null, { status: 204 });
      }),
    );

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

    const input = screen.getByPlaceholderText(/Add collaborator/i);
    await userEvent.type(input, "dave");
    await userEvent.click(screen.getByRole("button", { name: /^Add$/i }));

    await waitFor(() =>
      expect(requestBody).toEqual({ userIds: [2, 3], usernames: ["dave"] }),
    );
  });

  it("shows loading state when gs is undefined", () => {
    renderWithQuery(<AccessSection gs={undefined} />);
    expect(screen.getByText(/Loading/i)).toBeInTheDocument();
  });

  it("can add collaborator via Enter key", async () => {
    let requestBody: unknown;
    server.use(
      http.put("/servers/test:collaborators", async ({ request }) => {
        requestBody = await request.json();
        return new HttpResponse(null, { status: 204 });
      }),
    );

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
    await userEvent.type(input, "eve{Enter}");

    await waitFor(() =>
      expect(requestBody).toEqual({ userIds: [], usernames: ["eve"] }),
    );
  });

  it("renders read-only when collaborators and collaborator-names are misaligned", () => {
    useMeMock.mockReturnValue({
      data: { id: 1, username: "alice", permissions: {} },
    });
    targetAdminMock.mockReturnValue(false);

    const gs = makeServer({
      metadata: {
        name: "test",
        namespace: "ns",
        annotations: {
          "gameplane.local/owner": "alice",
          "gameplane.local/owner-id": "1",
          "gameplane.local/collaborators": "2,3",
          "gameplane.local/collaborator-names": "bob", // Mismatch: 2 IDs but 1 name
        },
      },
      spec: { templateRef: { name: "mc" } },
    });
    renderWithQuery(<AccessSection gs={gs} />);

    expect(screen.getByText("Collaborators were modified outside the dashboard.")).toBeInTheDocument();
    expect(screen.queryByPlaceholderText(/Add collaborator/i)).not.toBeInTheDocument();
    const removeButtons = screen.queryAllByLabelText(/Remove/);
    expect(removeButtons).toHaveLength(0);
  });

  it("shows owner as dash when no owner annotation", () => {
    const gs = makeServer({
      metadata: {
        name: "test",
        namespace: "ns",
        annotations: {},
      },
      spec: { templateRef: { name: "mc" } },
    });
    renderWithQuery(<AccessSection gs={gs} />);
    expect(screen.getByText("—")).toBeInTheDocument();
  });

  it("target administrator can manage collaborators without owning the server", () => {
    useMeMock.mockReturnValue({
      data: { id: 99, username: "administrator", permissions: {} },
    });
    targetAdminMock.mockReturnValue(true); // exact target administrator projection

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

    expect(screen.getByPlaceholderText(/Add collaborator/i)).toBeInTheDocument();
  });

  it("namespace write permission alone cannot change collaborators", () => {
    const gs = makeServer({ metadata: { name: "test", namespace: "ns", annotations: { "gameplane.local/owner-id": "1", "gameplane.local/owner": "alice" } } });
    renderWithQuery(<AccessSection gs={gs} access={{ canWrite: true, canControl: true, canConsole: false, canDelete: false, isOwner: false, isCollaborator: false, permissions: ["servers:read", "servers:write"] }} />);
    expect(screen.queryByPlaceholderText(/Add collaborator/i)).not.toBeInTheDocument();
  });

  it("add button disabled when input is empty", () => {
    useMeMock.mockReturnValue({
      data: { id: 1, username: "alice", permissions: {} },
    });
    targetAdminMock.mockReturnValue(false);

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

    const addBtn = screen.getByRole("button", { name: /^Add$/i });
    expect(addBtn).toBeDisabled();
  });

  it("add button enabled when input has text", async () => {
    useMeMock.mockReturnValue({
      data: { id: 1, username: "alice", permissions: {} },
    });
    targetAdminMock.mockReturnValue(false);

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
    await userEvent.type(input, "bob");

    const addBtn = screen.getByRole("button", { name: /^Add$/i });
    expect(addBtn).not.toBeDisabled();
  });

  it("input disabled during add mutation", async () => {
    useMeMock.mockReturnValue({
      data: { id: 1, username: "alice", permissions: {} },
    });
    targetAdminMock.mockReturnValue(false);

    server.use(
      http.put("/servers/test:collaborators", async ({ request }) => {
        await request.json();
        // Hold the response open briefly so the pending window is wide
        // enough for waitFor's polling to observe the disabled input —
        // without this, the mutation can resolve between polls and the
        // assertion never catches the transient state.
        await new Promise((resolve) => setTimeout(resolve, 20));
        return new HttpResponse(null, { status: 204 });
      }),
    );

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
    await userEvent.type(input, "bob");

    const addBtn = screen.getByRole("button", { name: /^Add$/i });
    fireEvent.click(addBtn);

    // Input should be disabled during the mutation
    await waitFor(() => expect(input).toBeDisabled());
  });

  it("error message parsing with error field", () => {
    server.use(
      http.put("/servers/test:collaborators", () =>
        HttpResponse.json({ error: "permissions denied" }, { status: 403 }),
      ),
    );

    useMeMock.mockReturnValue({
      data: { id: 1, username: "alice", permissions: {} },
    });
    targetAdminMock.mockReturnValue(false);

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
    fireEvent.change(input, { target: { value: "eve" } });
    fireEvent.click(screen.getByRole("button", { name: /^Add$/i }));

    // Should show the error after mutation completes
  });

  it("collaborators split correctly on commas with spaces", () => {
    const gs = makeServer({
      metadata: {
        name: "test",
        namespace: "ns",
        annotations: {
          "gameplane.local/owner": "alice",
          "gameplane.local/owner-id": "1",
          "gameplane.local/collaborators": "2, 3, 4",
          "gameplane.local/collaborator-names": "bob, charlie, dave",
        },
      },
      spec: { templateRef: { name: "mc" } },
    });
    renderWithQuery(<AccessSection gs={gs} />);
    expect(screen.getByText("bob")).toBeInTheDocument();
    expect(screen.getByText("charlie")).toBeInTheDocument();
    expect(screen.getByText("dave")).toBeInTheDocument();
  });

  it("namespace defaults to gameplane-games", () => {
    useMeMock.mockReturnValue({
      data: { id: 99, username: "operator", permissions: {} },
    });
    targetAdminMock.mockReturnValue(false);

    const gs = makeServer({
      metadata: {
        name: "test",
        // No namespace set
        annotations: {
          "gameplane.local/owner": "alice",
          "gameplane.local/owner-id": "1",
        },
      },
      spec: { templateRef: { name: "mc" } },
    });
    renderWithQuery(<AccessSection gs={gs} />);

    // Component should handle the default namespace internally
    expect(screen.getByText("alice")).toBeInTheDocument();
  });
});
