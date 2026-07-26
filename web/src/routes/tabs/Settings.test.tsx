import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { SettingsTab } from "./Settings";
import type { GameServer } from "@/types";

type FetchInit = Parameters<typeof fetch>[1];

const navigateMock = vi.fn();
vi.mock("@tanstack/react-router", () => ({
  useNavigate: () => navigateMock,
}));

const fetchMock = vi.fn();

beforeEach(() => {
  vi.stubGlobal("fetch", fetchMock);
  navigateMock.mockReset();
});
afterEach(() => {
  fetchMock.mockReset();
  vi.unstubAllGlobals();
});

function withClient(ui: ReactNode) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={qc}>{ui}</QueryClientProvider>;
}

function gs(overrides: Partial<GameServer> = {}): GameServer {
  return {
    metadata: { name: "mc-survival", resourceVersion: "100" },
    spec: { templateRef: { name: "minecraft-java" } },
    status: { phase: "Running" },
    ...overrides,
  };
}

function jsonRes(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

// The Templates.get query fires whenever templateRef is set; default it.
function stubTemplate() {
  fetchMock.mockImplementation(async (url: string) => {
    if (url.startsWith("/templates/")) {
      return jsonRes({
        metadata: { name: "minecraft-java" },
        spec: { displayName: "Minecraft", game: "minecraft-java", version: "1.0", image: "x" },
      });
    }
    throw new Error(`unexpected fetch: ${url}`);
  });
}

describe("SettingsTab", () => {
  it("starts clean and reports dirty after editing description", async () => {
    stubTemplate();
    const onDirty = vi.fn();
    render(withClient(<SettingsTab gs={gs()} name="mc-survival" onDirtyChange={onDirty} />));

    await waitFor(() => expect(onDirty).toHaveBeenCalledWith(false));

    const textarea = screen.getByPlaceholderText(/Long-standing/);
    fireEvent.change(textarea, { target: { value: "Hello world" } });

    await waitFor(() => expect(onDirty).toHaveBeenLastCalledWith(true));
    expect(screen.getByText("Unsaved changes")).toBeInTheDocument();
  });

  it("adds, removes, and toggles env-var rows", async () => {
    stubTemplate();
    render(withClient(<SettingsTab gs={gs()} name="mc-survival" />));
    fireEvent.click(screen.getByRole("button", { name: /Environment/i }));

    fireEvent.click(screen.getByRole("button", { name: /Add variable/i }));
    const nameInput = screen.getByPlaceholderText("VAR_NAME");
    fireEvent.change(nameInput, { target: { value: "FOO" } });
    fireEvent.change(screen.getByPlaceholderText("value"), { target: { value: "bar" } });

    fireEvent.click(screen.getByRole("button", { name: /Add from secret/i }));
    expect(screen.getAllByPlaceholderText("VAR_NAME")).toHaveLength(2);

    // Remove the literal row.
    const removeButtons = screen.getAllByTitle("Remove");
    fireEvent.click(removeButtons[0]);
    expect(screen.getAllByPlaceholderText("VAR_NAME")).toHaveLength(1);
  });

  it("flags duplicate env names as invalid", async () => {
    stubTemplate();
    render(withClient(<SettingsTab gs={gs()} name="mc-survival" />));
    fireEvent.click(screen.getByRole("button", { name: /Environment/i }));

    fireEvent.click(screen.getByRole("button", { name: /Add variable/i }));
    fireEvent.click(screen.getByRole("button", { name: /Add variable/i }));
    const inputs = screen.getAllByPlaceholderText("VAR_NAME");
    fireEvent.change(inputs[0], { target: { value: "FOO" } });
    fireEvent.change(inputs[1], { target: { value: "FOO" } });

    expect(screen.getAllByText("Duplicate name")).toHaveLength(2);
  });

  it("PUTs merged object that preserves unknown spec fields and uses latest resourceVersion", async () => {
    let putBody: unknown = null;
    fetchMock.mockImplementation(async (url: string, init?: FetchInit) => {
      if (url.startsWith("/templates/")) {
        return jsonRes({
          metadata: { name: "minecraft-java" },
          spec: { displayName: "Minecraft", game: "minecraft-java", version: "1.0", image: "x" },
        });
      }
      if (url === "/servers/mc-survival" && (!init || init.method === "GET")) {
        // Latest server-side state, with a NEW resourceVersion and an
        // operator-added annotation the UI never saw.
        return jsonRes({
          metadata: {
            name: "mc-survival",
            resourceVersion: "200",
            annotations: { "gameplane.local/managed-by-operator": "true" },
          },
          spec: { templateRef: { name: "minecraft-java" } },
          status: { phase: "Running" },
        });
      }
      if (url === "/servers/mc-survival" && init?.method === "PUT") {
        putBody = JSON.parse(init.body as string);
        return jsonRes(putBody);
      }
      throw new Error(`unexpected fetch: ${url} ${init?.method ?? "GET"}`);
    });

    render(withClient(<SettingsTab gs={gs()} name="mc-survival" />));

    fireEvent.change(screen.getByPlaceholderText(/Long-standing/), {
      target: { value: "test desc" },
    });

    fireEvent.click(screen.getByRole("button", { name: /Save changes/i }));

    await waitFor(() => expect(putBody).not.toBeNull());

    const body = putBody as GameServer;
    // Latest resourceVersion preserved.
    expect(body.metadata.resourceVersion).toBe("200");
    // Operator-managed annotation preserved.
    expect(body.metadata.annotations?.["gameplane.local/managed-by-operator"]).toBe("true");
    // User-edited annotation applied.
    expect(body.metadata.annotations?.["gameplane.local/description"]).toBe("test desc");
  });

  it("surfaces a reload prompt on 409 conflict", async () => {
    fetchMock.mockImplementation(async (url: string, init?: FetchInit) => {
      if (url.startsWith("/templates/")) {
        return jsonRes({
          metadata: { name: "minecraft-java" },
          spec: { displayName: "Minecraft", game: "minecraft-java", version: "1.0", image: "x" },
        });
      }
      if (url === "/servers/mc-survival" && (!init || init.method === "GET")) {
        return jsonRes(gs());
      }
      if (init?.method === "PUT") {
        return jsonRes({ message: "conflict" }, 409);
      }
      throw new Error(`unexpected fetch: ${url}`);
    });

    render(withClient(<SettingsTab gs={gs()} name="mc-survival" />));

    fireEvent.change(screen.getByPlaceholderText(/Long-standing/), {
      target: { value: "x" },
    });
    fireEvent.click(screen.getByRole("button", { name: /Save changes/i }));

    await waitFor(() =>
      expect(screen.getByText(/changed since you opened this page/i)).toBeInTheDocument(),
    );
  });

  it("requires typing the server name to delete from the danger zone", async () => {
    stubTemplate();
    render(withClient(<SettingsTab gs={gs()} name="mc-survival" />));
    fireEvent.click(screen.getByRole("button", { name: /Danger zone/i }));

    fireEvent.click(screen.getByRole("button", { name: /Delete server…/i }));
    const confirmBtn = screen.getByRole("button", { name: /^Delete server$/i });
    expect(confirmBtn).toBeDisabled();

    const phraseInput = screen.getByRole("textbox");
    fireEvent.change(phraseInput, { target: { value: "wrong" } });
    expect(confirmBtn).toBeDisabled();

    fireEvent.change(phraseInput, { target: { value: "mc-survival" } });
    expect(confirmBtn).toBeEnabled();

    fetchMock.mockImplementationOnce(async () => new Response(null, { status: 204 }));
    await act(async () => {
      fireEvent.click(confirmBtn);
    });
    await waitFor(() => expect(navigateMock).toHaveBeenCalledWith({ to: "/servers" }));
  });

  it("clears dirty flag after successful save", async () => {
    const onDirty = vi.fn();
    stubTemplate();
    let putCalled = false;
    fetchMock.mockImplementation(async (url: string, init?: FetchInit) => {
      if (url.startsWith("/templates/")) {
        return jsonRes({
          metadata: { name: "minecraft-java" },
          spec: { displayName: "Minecraft", game: "minecraft-java", version: "1.0", image: "x" },
        });
      }
      if (url === "/servers/mc-survival" && (!init || init.method === "GET")) {
        return jsonRes(gs());
      }
      if (url === "/servers/mc-survival" && init?.method === "PUT") {
        putCalled = true;
        const body = JSON.parse(init.body as string);
        return jsonRes(body);
      }
      throw new Error(`unexpected fetch: ${url}`);
    });

    render(withClient(<SettingsTab gs={gs()} name="mc-survival" onDirtyChange={onDirty} />));
    await waitFor(() => expect(onDirty).toHaveBeenCalledWith(false));
    onDirty.mockClear();

    fireEvent.change(screen.getByPlaceholderText(/Long-standing/), {
      target: { value: "test" },
    });
    await waitFor(() => expect(onDirty).toHaveBeenCalledWith(true));

    fireEvent.click(screen.getByRole("button", { name: /Save changes/i }));
    await waitFor(() => {
      expect(putCalled).toBe(true);
      expect(onDirty).toHaveBeenCalledWith(false);
    });
    expect(screen.getByText(/Saved/i)).toBeInTheDocument();
  });

  it("shows error message on save failure (non-409)", async () => {
    stubTemplate();
    fetchMock.mockImplementation(async (url: string, init?: FetchInit) => {
      if (url.startsWith("/templates/")) {
        return jsonRes({
          metadata: { name: "minecraft-java" },
          spec: { displayName: "Minecraft", game: "minecraft-java", version: "1.0", image: "x" },
        });
      }
      if (url === "/servers/mc-survival" && (!init || init.method === "GET")) {
        return jsonRes(gs());
      }
      if (init?.method === "PUT") {
        return jsonRes({ error: "internal server error" }, 500);
      }
      throw new Error(`unexpected fetch: ${url}`);
    });

    render(withClient(<SettingsTab gs={gs()} name="mc-survival" />));

    fireEvent.change(screen.getByPlaceholderText(/Long-standing/), {
      target: { value: "x" },
    });
    fireEvent.click(screen.getByRole("button", { name: /Save changes/i }));

    await waitFor(() =>
      expect(screen.getByText(/internal server error/i)).toBeInTheDocument(),
    );
  });

  it("disables save button when section is invalid", async () => {
    stubTemplate();
    render(withClient(<SettingsTab gs={gs()} name="mc-survival" />));

    // General section is safe, but let's switch to Lifecycle and make it invalid
    fireEvent.click(screen.getByRole("button", { name: /Lifecycle/i }));

    // Make a change first
    fireEvent.change(screen.getAllByRole("textbox")[0], {
      target: { value: "30" },
    });

    // Now create invalid grace period state
    render(
      withClient(
        <SettingsTab
          gs={{
            ...gs(),
            spec: { ...gs().spec, stopGracePeriodSeconds: 999 },
          }}
          name="mc-survival"
        />,
      ),
    );

    fireEvent.click(screen.getByRole("button", { name: /Lifecycle/i }));
    const saveBtn = screen.getByRole("button", { name: /Save changes/i });
    // Section is invalid (grace period > 600), so save should be disabled
    expect(saveBtn).toBeDisabled();
  });

  it("clears error when editing after error", async () => {
    stubTemplate();
    fetchMock.mockImplementation(async (url: string, init?: FetchInit) => {
      if (url.startsWith("/templates/")) {
        return jsonRes({
          metadata: { name: "minecraft-java" },
          spec: { displayName: "Minecraft", game: "minecraft-java", version: "1.0", image: "x" },
        });
      }
      if (url === "/servers/mc-survival" && (!init || init.method === "GET")) {
        return jsonRes(gs());
      }
      if (init?.method === "PUT") {
        return jsonRes({ error: "something failed" }, 500);
      }
      throw new Error(`unexpected fetch: ${url}`);
    });

    render(withClient(<SettingsTab gs={gs()} name="mc-survival" />));

    fireEvent.change(screen.getByPlaceholderText(/Long-standing/), {
      target: { value: "x" },
    });
    fireEvent.click(screen.getByRole("button", { name: /Save changes/i }));

    await waitFor(() =>
      expect(screen.getByText(/something failed/i)).toBeInTheDocument(),
    );

    fireEvent.change(screen.getByPlaceholderText(/Long-standing/), {
      target: { value: "y" },
    });

    // Error should remain until successful save or reset
    expect(screen.getByText(/something failed/i)).toBeInTheDocument();
  });

  it("discard button is disabled when not dirty", () => {
    stubTemplate();
    render(withClient(<SettingsTab gs={gs()} name="mc-survival" />));

    const discardBtn = screen.getByRole("button", { name: /Discard/i });
    expect(discardBtn).toBeDisabled();
  });

  it("discard button resets draft to baseline", async () => {
    const onDirty = vi.fn();
    stubTemplate();
    render(
      withClient(
        <SettingsTab
          gs={gs({ metadata: { name: "mc-survival", annotations: { "gameplane.local/description": "old" } } })}
          name="mc-survival"
          onDirtyChange={onDirty}
        />,
      ),
    );

    await waitFor(() => expect(onDirty).toHaveBeenCalledWith(false));

    fireEvent.change(screen.getByPlaceholderText(/Long-standing/), {
      target: { value: "new" },
    });
    await waitFor(() => expect(onDirty).toHaveBeenCalledWith(true));

    const discardBtn = screen.getByRole("button", { name: /Discard/i });
    fireEvent.click(discardBtn);

    await waitFor(() => expect(onDirty).toHaveBeenCalledWith(false));
    // Verify the field is reset back to old
    const textarea = screen.getByPlaceholderText(/Long-standing/) as HTMLTextAreaElement;
    expect(textarea.value).toBe("old");
  });

  it("reload button fetches fresh server and resets local state", async () => {
    stubTemplate();
    let fetchCount = 0;
    fetchMock.mockImplementation(async (url: string, init?: FetchInit) => {
      if (url.startsWith("/templates/")) {
        return jsonRes({
          metadata: { name: "minecraft-java" },
          spec: { displayName: "Minecraft", game: "minecraft-java", version: "1.0", image: "x" },
        });
      }
      if (url === "/servers/mc-survival" && (!init || init.method === "GET")) {
        fetchCount++;
        if (fetchCount === 1) {
          return jsonRes(gs({ metadata: { name: "mc-survival", annotations: {} } }));
        }
        return jsonRes(gs({ metadata: { name: "mc-survival", annotations: { "gameplane.local/description": "reloaded" } } }));
      }
      throw new Error(`unexpected fetch: ${url}`);
    });

    const onDirty = vi.fn();
    render(withClient(<SettingsTab gs={gs()} name="mc-survival" onDirtyChange={onDirty} />));

    await waitFor(() => expect(onDirty).toHaveBeenCalledWith(false));
    onDirty.mockClear();

    fireEvent.change(screen.getByPlaceholderText(/Long-standing/), {
      target: { value: "edited" },
    });
    await waitFor(() => expect(onDirty).toHaveBeenCalledWith(true));

    // Simulate 409 conflict
    fetchMock.mockImplementationOnce(async (url: string, init?: FetchInit) => {
      if (url.startsWith("/templates/")) {
        return jsonRes({
          metadata: { name: "minecraft-java" },
          spec: { displayName: "Minecraft", game: "minecraft-java", version: "1.0", image: "x" },
        });
      }
      if (url === "/servers/mc-survival" && init?.method === "PUT") {
        return jsonRes({ error: "conflict" }, 409);
      }
      if (url === "/servers/mc-survival" && (!init || init.method === "GET")) {
        return jsonRes(gs({ metadata: { name: "mc-survival", annotations: { "gameplane.local/description": "reloaded" } } }));
      }
      throw new Error(`unexpected fetch: ${url}`);
    });

    fireEvent.click(screen.getByRole("button", { name: /Save changes/i }));
    await waitFor(() =>
      expect(screen.getByText(/changed since you opened this page/i)).toBeInTheDocument(),
    );

    const reloadBtn = screen.getByRole("button", { name: /Reload/i });
    fireEvent.click(reloadBtn);

    await waitFor(() => {
      expect(screen.queryByText(/changed since you opened this page/i)).not.toBeInTheDocument();
      expect(onDirty).toHaveBeenCalledWith(false);
    });
  });

  it("filters out version section when template has no versions", async () => {
    fetchMock.mockImplementation(async (url: string) => {
      if (url.startsWith("/templates/")) {
        return jsonRes({
          metadata: { name: "minecraft-java" },
          spec: { displayName: "Minecraft", game: "minecraft-java", version: "1.0", image: "x" },
          // No versions array
        });
      }
      throw new Error(`unexpected fetch: ${url}`);
    });

    render(withClient(<SettingsTab gs={gs()} name="mc-survival" />));

    // Version button should not be in the nav
    expect(screen.queryByRole("button", { name: /^Version$/i })).not.toBeInTheDocument();
  });

  it("includes version section when template has versions", async () => {
    fetchMock.mockImplementation(async (url: string) => {
      if (url.startsWith("/templates/")) {
        return jsonRes({
          metadata: { name: "minecraft-java" },
          spec: {
            displayName: "Minecraft",
            game: "minecraft-java",
            version: "1.0",
            image: "x",
            versions: [
              { id: "1.19", label: "1.19" },
              { id: "1.20", label: "1.20" },
            ],
          },
        });
      }
      throw new Error(`unexpected fetch: ${url}`);
    });

    render(withClient(<SettingsTab gs={gs()} name="mc-survival" />));

    // Version button should be in the nav
    await waitFor(() => {
      expect(screen.getByRole("button", { name: /^Version$/i })).toBeInTheDocument();
    });
  });
});
