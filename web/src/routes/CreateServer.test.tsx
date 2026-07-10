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

import {
  CreateServerWizard,
  nodeCaps,
  nonEmptyPortOverrides,
  parseSourceRanges,
} from "./CreateServer";
import { gameCategory } from "@/lib/games";

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

// The wizard fires several GETs on mount (templates + cluster node caps),
// so ordered once-mocks are fragile — an extra GET shifts the queue. This
// routes by URL/method instead, resolving each call correctly regardless
// of order: /cluster → node caps, POST → the create response, else the
// template list.
function routeFetch(opts: { templates?: GameTemplate[]; create?: Response } = {}) {
  fetchMock.mockImplementation((url: string, init?: FetchInit) => {
    if (url.includes("/cluster")) return Promise.resolve(jsonRes(200, { nodes: [] }));
    if ((init?.method ?? "GET") === "POST") {
      return Promise.resolve(opts.create ?? jsonRes(201, { metadata: { name: "mc-test" } }));
    }
    return Promise.resolve(jsonRes(200, { items: opts.templates ?? [template()] }));
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

function versionedTemplate(): GameTemplate {
  return template({
    versions: [
      { id: "1.21.4-paper", displayName: "1.21.4 Paper", loader: "paper", default: true },
      { id: "1.21.4-forge", displayName: "1.21.4 Forge", loader: "forge" },
    ],
  });
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
    routeFetch({ create: new Response("name taken", { status: 409 }) });

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
    routeFetch();

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
    expect(body.spec.nodeSelector).toEqual({ "gameplane.local/pinned": "true" });
  });

  it("sends requests equal to limits (Guaranteed QoS)", async () => {
    routeFetch();

    render(withClient(<CreateServerWizard />));
    await pickTemplate(template());

    fireEvent.change(screen.getByPlaceholderText("mc-hardcore"), {
      target: { value: "mc-test" },
    });
    fireEvent.click(screen.getByRole("button", { name: /Continue to Network/i }));
    fireEvent.click(screen.getByRole("button", { name: /Continue to Review/i }));
    fireEvent.click(screen.getByRole("button", { name: /Create server/i }));

    await waitFor(() => expect(navigate).toHaveBeenCalled());
    const postCall = fetchMock.mock.calls.find((c) => (c[1] as FetchInit).method === "POST")!;
    const body = JSON.parse((postCall[1] as FetchInit).body as string);
    // Defaults from the wizard state: 2 cores / 4Gi.
    expect(body.spec.resources.requests).toEqual({ cpu: "2", memory: "4Gi" });
    expect(body.spec.resources.limits).toEqual({ cpu: "2", memory: "4Gi" });
  });

  it("keeps the 4-step flow when the template declares no versions", async () => {
    fetchMock.mockResolvedValueOnce(jsonRes(200, { items: [template()] }));
    render(withClient(<CreateServerWizard />));
    fireEvent.click(await screen.findByRole("button", { name: /Minecraft Java/i }));
    expect(screen.getByRole("button", { name: /Continue to Configure/i })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Continue to Version/i })).not.toBeInTheDocument();
  });

  it("inserts a Version step and pre-selects the default for versioned templates", async () => {
    fetchMock.mockResolvedValueOnce(jsonRes(200, { items: [versionedTemplate()] }));
    render(withClient(<CreateServerWizard />));
    fireEvent.click(await screen.findByRole("button", { name: /Minecraft Java/i }));
    // The step after Template is Version, not Configure.
    fireEvent.click(screen.getByRole("button", { name: /Continue to Version/i }));
    expect(screen.getByText("1.21.4 Paper")).toBeInTheDocument();
    expect(screen.getByText("Default")).toBeInTheDocument();
    // The default seeds a valid selection, so Configure is reachable.
    expect(screen.getByRole("button", { name: /Continue to Configure/i })).toBeEnabled();
  });

  it("includes the selected version in the create body", async () => {
    routeFetch({ templates: [versionedTemplate()] });

    render(withClient(<CreateServerWizard />));
    fireEvent.click(await screen.findByRole("button", { name: /Minecraft Java/i }));
    fireEvent.click(screen.getByRole("button", { name: /Continue to Version/i }));
    // Switch from the default (paper) to forge.
    fireEvent.click(screen.getByRole("button", { name: /1\.21\.4 Forge/i }));
    fireEvent.click(screen.getByRole("button", { name: /Continue to Configure/i }));
    fireEvent.change(screen.getByPlaceholderText("mc-hardcore"), { target: { value: "mc-test" } });
    fireEvent.click(screen.getByRole("button", { name: /Continue to Network/i }));
    fireEvent.click(screen.getByRole("button", { name: /Continue to Review/i }));
    fireEvent.click(screen.getByRole("button", { name: /Create server/i }));

    await waitFor(() => expect(navigate).toHaveBeenCalled());
    const postCall = fetchMock.mock.calls.find((c) => (c[1] as FetchInit).method === "POST")!;
    const body = JSON.parse((postCall[1] as FetchInit).body as string);
    expect(body.spec.version).toBe("1.21.4-forge");
  });

  it("filters templates by search text and category", async () => {
    const minecraftTemplate: GameTemplate = {
      metadata: { name: "minecraft-java" },
      spec: {
        displayName: "Minecraft Java",
        game: "minecraft",
        category: "Sandbox",
        version: "1.21",
        image: "itzg/minecraft-server",
      },
    };
    const valheimTemplate: GameTemplate = {
      metadata: { name: "valheim" },
      spec: {
        displayName: "Valheim",
        game: "valheim",
        category: "Survival",
        version: "1.0",
        image: "x",
      },
    };
    fetchMock.mockResolvedValue(jsonRes(200, { items: [minecraftTemplate, valheimTemplate] }));
    render(withClient(<CreateServerWizard />));

    // Both templates visible by default.
    expect(await screen.findByRole("button", { name: /Minecraft Java/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Valheim/i })).toBeInTheDocument();

    // Search narrows to matching templates.
    fireEvent.change(screen.getByLabelText("Search templates"), { target: { value: "valheim" } });
    expect(screen.queryByRole("button", { name: /Minecraft Java/i })).toBeNull();
    expect(screen.getByRole("button", { name: /Valheim/i })).toBeInTheDocument();

    // Category pill filters by the declared category (Sandbox).
    fireEvent.change(screen.getByLabelText("Search templates"), { target: { value: "" } });
    fireEvent.click(screen.getByRole("button", { name: "Sandbox" }));
    expect(screen.getByRole("button", { name: /Minecraft Java/i })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Valheim/i })).toBeNull();
  });
});

describe("parseSourceRanges", () => {
  it("splits on newlines and commas, trimming blanks", () => {
    expect(parseSourceRanges("203.0.113.0/24\n10.0.0.0/8")).toEqual([
      "203.0.113.0/24",
      "10.0.0.0/8",
    ]);
    expect(parseSourceRanges(" 1.2.3.0/24 , , 5.6.7.0/24 ")).toEqual([
      "1.2.3.0/24",
      "5.6.7.0/24",
    ]);
    expect(parseSourceRanges("")).toEqual([]);
  });
});

describe("nonEmptyPortOverrides", () => {
  it("drops rows with a blank or whitespace-only name", () => {
    expect(
      nonEmptyPortOverrides([
        { name: "game", nodePort: 30005 },
        { name: "" },
        { name: "  " },
      ]),
    ).toEqual([{ name: "game", nodePort: 30005 }]);
    expect(nonEmptyPortOverrides([])).toEqual([]);
  });
});

describe("nodeCaps", () => {
  it("returns the largest single node's CPU cores and memory GiB", () => {
    const caps = nodeCaps([
      { cpu: { capacity: 4 }, memory: { capacity: 14656061440 } }, // ~13.6 GiB
      { cpu: { capacity: 8 }, memory: { capacity: 8 * 1024 ** 3 } },
    ]);
    expect(caps.maxCpu).toBe(8);
    // Largest memory node is the ~13.6 GiB one → floor to 13 GiB.
    expect(caps.maxMemGi).toBe(13);
  });
  it("is {0,0} when node data is unavailable (no cap)", () => {
    expect(nodeCaps([])).toEqual({ maxCpu: 0, maxMemGi: 0 });
  });
});

describe("CreateServerWizard networking", () => {
  it("sends sourceRanges when LoadBalancer + CIDRs are set", async () => {
    routeFetch();
    render(withClient(<CreateServerWizard />));
    await pickTemplate(template());
    fireEvent.change(screen.getByPlaceholderText("mc-hardcore"), { target: { value: "mc-test" } });
    fireEvent.click(screen.getByRole("button", { name: /Continue to Network/i }));

    fireEvent.click(screen.getByRole("button", { name: /LoadBalancer/i }));
    fireEvent.change(screen.getByLabelText("IP allow-list"), {
      target: { value: "203.0.113.0/24\n10.0.0.0/8" },
    });
    fireEvent.click(screen.getByRole("button", { name: /Continue to Review/i }));
    fireEvent.click(screen.getByRole("button", { name: /Create server/i }));

    await waitFor(() => expect(navigate).toHaveBeenCalled());
    const postCall = fetchMock.mock.calls.find((c) => (c[1] as FetchInit).method === "POST")!;
    const body = JSON.parse((postCall[1] as FetchInit).body as string);
    expect(body.spec.networking.sourceRanges).toEqual(["203.0.113.0/24", "10.0.0.0/8"]);
  });

  it("sends portOverrides when a named override is added", async () => {
    routeFetch();
    render(withClient(<CreateServerWizard />));
    await pickTemplate(template());
    fireEvent.change(screen.getByPlaceholderText("mc-hardcore"), { target: { value: "mc-test" } });
    fireEvent.click(screen.getByRole("button", { name: /Continue to Network/i }));

    fireEvent.click(screen.getByRole("button", { name: /Add override/i }));
    fireEvent.change(screen.getByPlaceholderText("game"), { target: { value: "game" } });
    fireEvent.change(screen.getByPlaceholderText("30000-32767"), { target: { value: "30005" } });
    fireEvent.click(screen.getByRole("button", { name: /Continue to Review/i }));
    // The review summary lists the named override.
    expect(screen.getByText("Port overrides")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /Create server/i }));

    await waitFor(() => expect(navigate).toHaveBeenCalled());
    const postCall = fetchMock.mock.calls.find((c) => (c[1] as FetchInit).method === "POST")!;
    const body = JSON.parse((postCall[1] as FetchInit).body as string);
    expect(body.spec.networking.portOverrides).toEqual([{ name: "game", nodePort: 30005 }]);
  });

  it("omits portOverrides when the only row is left blank", async () => {
    routeFetch();
    render(withClient(<CreateServerWizard />));
    await pickTemplate(template());
    fireEvent.change(screen.getByPlaceholderText("mc-hardcore"), { target: { value: "mc-test" } });
    fireEvent.click(screen.getByRole("button", { name: /Continue to Network/i }));

    // An added-but-never-named row must not pollute the payload.
    fireEvent.click(screen.getByRole("button", { name: /Add override/i }));
    fireEvent.click(screen.getByRole("button", { name: /Continue to Review/i }));
    expect(screen.queryByText("Port overrides")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /Create server/i }));

    await waitFor(() => expect(navigate).toHaveBeenCalled());
    const postCall = fetchMock.mock.calls.find((c) => (c[1] as FetchInit).method === "POST")!;
    const body = JSON.parse((postCall[1] as FetchInit).body as string);
    expect(body.spec.networking.portOverrides).toBeUndefined();
  });
});

describe("gameCategory", () => {
  it.each([
    ["minecraft", "Sandbox"],
    ["terraria", "Sandbox"],
    ["valheim", "Survival"],
    ["palworld", "Survival"],
    ["cs2", "Shooter"],
    ["something-unknown", "Other"],
  ])("maps %s -> %s", (game, want) => {
    expect(gameCategory(game)).toBe(want);
  });
});

describe("CreateServerWizard review", () => {
  it("Edit links jump back to the matching step", async () => {
    fetchMock.mockResolvedValue(jsonRes(200, { items: [template()] }));
    render(withClient(<CreateServerWizard />));
    await pickTemplate(template());
    fireEvent.change(screen.getByPlaceholderText("mc-hardcore"), { target: { value: "mc-test" } });
    fireEvent.click(screen.getByRole("button", { name: /Continue to Network/i }));
    fireEvent.click(screen.getByRole("button", { name: /Continue to Review/i }));

    // Template / Configuration / Network sections each expose an Edit link.
    const edits = screen.getAllByRole("button", { name: "Edit" });
    expect(edits.length).toBe(3);
    // Configuration is the second section → jumps back to the Configure step.
    fireEvent.click(edits[1]);
    expect(screen.getByPlaceholderText("mc-hardcore")).toBeInTheDocument();
  });

  it("shows Cancel on step 1 and closes the wizard", async () => {
    fetchMock.mockResolvedValue(jsonRes(200, { items: [template()] }));
    render(withClient(<CreateServerWizard />));
    const cancel = await screen.findByRole("button", { name: "Cancel" });
    fireEvent.click(cancel);
    expect(navigate).toHaveBeenCalledWith({ to: "/" });
  });
});
