import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { GameTemplate } from "@/types";

const navigate = vi.fn();
let search: { template?: string } = {};
vi.mock("@tanstack/react-router", () => ({
  useNavigate: () => navigate,
  useSearch: () => search,
}));

import { CreateServerWizard } from "./CreateServer";

interface FetchInit {
  method?: string;
  headers?: Record<string, string>;
  body?: string;
}

const fetchMock = vi.fn();

beforeEach(() => {
  vi.stubGlobal("fetch", fetchMock);
});
afterEach(() => {
  fetchMock.mockReset();
  navigate.mockReset();
  search = {};
  vi.unstubAllGlobals();
});

function withClient(ui: ReactNode) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  return <QueryClientProvider client={qc}>{ui}</QueryClientProvider>;
}

function jsonRes(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function template(overrides: Partial<GameTemplate["spec"]> = {}): GameTemplate {
  return {
    metadata: { name: "minecraft-java" },
    spec: {
      displayName: "Minecraft Java",
      game: "minecraft",
      version: "1.21",
      image: "itzg/minecraft-server",
      ...overrides,
    },
  };
}

async function pickTemplate(t: GameTemplate) {
  fireEvent.click(await screen.findByRole("button", { name: new RegExp(t.spec.displayName, "i") }));
  fireEvent.click(screen.getByRole("button", { name: /Continue to Configure/i }));
}

describe("CreateServerWizard", () => {
  it("blocks Continue with a reason when the name is invalid", async () => {
    fetchMock.mockResolvedValueOnce(jsonRes(200, { items: [template()] }));
    render(withClient(<CreateServerWizard />));
    await pickTemplate(template());

    const nameInput = screen.getByPlaceholderText("mc-hardcore");
    fireEvent.change(nameInput, { target: { value: "MC TEST" } });

    const continueBtn = screen.getByRole("button", { name: /Continue to Network/i });
    expect(continueBtn).toBeDisabled();
    expect(screen.getByTestId("step-reason").textContent).toMatch(/lowercase letters/);
  });

  it("blocks Continue when a required template config field is missing", async () => {
    const t = template({
      configSchema: [
        { name: "VERSION", type: "string", required: true },
      ],
    });
    fetchMock.mockResolvedValueOnce(jsonRes(200, { items: [t] }));
    render(withClient(<CreateServerWizard />));
    await pickTemplate(t);

    fireEvent.change(screen.getByPlaceholderText("mc-hardcore"), {
      target: { value: "mc-test" },
    });

    const continueBtn = screen.getByRole("button", { name: /Continue to Network/i });
    expect(continueBtn).toBeDisabled();
    expect(screen.getByTestId("step-reason").textContent).toMatch(/VERSION is required/);
  });

  it("renders an inline alert when the API returns 409", async () => {
    fetchMock
      .mockResolvedValueOnce(jsonRes(200, { items: [template()] }))
      .mockResolvedValueOnce(new Response("name taken", { status: 409 }));

    render(withClient(<CreateServerWizard />));
    await pickTemplate(template());

    fireEvent.change(screen.getByPlaceholderText("mc-hardcore"), {
      target: { value: "mc-test" },
    });
    fireEvent.click(screen.getByRole("button", { name: /Continue to Network/i }));
    fireEvent.click(screen.getByRole("button", { name: /Continue to Review/i }));
    fireEvent.click(screen.getByRole("button", { name: /Create server/i }));

    const alert = await screen.findByTestId("create-error");
    expect(alert.textContent).toMatch(/A server named mc-test already exists/);
    expect(navigate).not.toHaveBeenCalled();
  });

  it("pre-selects the template from the ?template= search param", async () => {
    search = { template: "minecraft-java" };
    fetchMock.mockResolvedValueOnce(jsonRes(200, { items: [template()] }));
    render(withClient(<CreateServerWizard />));

    // Without pre-selection, the name input (step 2) is never reachable; its
    // presence means step 1 auto-advanced past template selection isn't
    // needed — the template is already chosen, so Continue is enabled.
    const continueBtn = await screen.findByRole("button", { name: /Continue to Configure/i });
    await waitFor(() => expect(continueBtn).toBeEnabled());
    // The preview reflects the chosen template name.
    expect(screen.getAllByText(/Minecraft Java/i).length).toBeGreaterThan(0);
  });

  it("does not pre-select when no ?template= is present", async () => {
    fetchMock.mockResolvedValueOnce(jsonRes(200, { items: [template()] }));
    render(withClient(<CreateServerWizard />));

    await screen.findByRole("button", { name: new RegExp(template().spec.displayName, "i") });
    const continueBtn = screen.getByRole("button", { name: /Continue to Configure/i });
    expect(continueBtn).toBeDisabled();
  });

  it("sends nodeSelector when nodePlacement is 'pin'", async () => {
    fetchMock
      .mockResolvedValueOnce(jsonRes(200, { items: [template()] }))
      .mockResolvedValueOnce(jsonRes(201, { metadata: { name: "mc-test" } }));

    render(withClient(<CreateServerWizard />));
    await pickTemplate(template());

    fireEvent.change(screen.getByPlaceholderText("mc-hardcore"), {
      target: { value: "mc-test" },
    });
    fireEvent.click(screen.getByRole("button", { name: /Pin to node/i }));
    fireEvent.click(screen.getByRole("button", { name: /Continue to Network/i }));
    fireEvent.click(screen.getByRole("button", { name: /Continue to Review/i }));
    fireEvent.click(screen.getByRole("button", { name: /Create server/i }));

    await waitFor(() => expect(navigate).toHaveBeenCalled());
    const postCall = fetchMock.mock.calls.find((c) => (c[1] as FetchInit).method === "POST")!;
    const body = JSON.parse((postCall[1] as FetchInit).body as string);
    expect(body.kind).toBe("GameServer");
    expect(body.spec.nodeSelector).toEqual({ "kestrel.gg/pinned": "true" });
  });
});
