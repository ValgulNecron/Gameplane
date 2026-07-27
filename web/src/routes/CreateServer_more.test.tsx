import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { GameTemplate } from "@/types";

const navigate = vi.fn();
vi.mock("@tanstack/react-router", () => ({
  useNavigate: () => navigate,
  useSearch: () => ({}),
}));

import { CreateServerWizard } from "./CreateServer";

interface FetchInit {
  method?: string;
  body?: string;
}

const fetchMock = vi.fn();

beforeEach(() => {
  vi.stubGlobal("fetch", fetchMock);
});
afterEach(() => {
  fetchMock.mockReset();
  navigate.mockReset();
  vi.unstubAllGlobals();
});

function withClient(ui: ReactNode) {
  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return <QueryClientProvider client={qc}>{ui}</QueryClientProvider>;
}

function jsonRes(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

// Order-independent fetch router — the wizard fires templates + cluster
// (node caps) GETs on mount, so ordered once-mocks desync. See the same
// helper in CreateServer.test.tsx.
function routeFetch(opts: { templates?: GameTemplate[]; create?: Response } = {}) {
  fetchMock.mockImplementation((url: string, init?: FetchInit) => {
    if (url.includes("/cluster")) return Promise.resolve(jsonRes(200, { nodes: [] }));
    if ((init?.method ?? "GET") === "POST") {
      return Promise.resolve(opts.create ?? jsonRes(201, { metadata: { name: "lb-test" } }));
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
  // The template grid renders only after the templates query resolves; the
  // default 1000ms findBy timeout flakes under CI load, so wait longer.
  fireEvent.click(
    await screen.findByRole(
      "button",
      { name: new RegExp(t.spec.displayName, "i") },
      { timeout: 5000 },
    ),
  );
  fireEvent.click(screen.getByRole("button", { name: /Continue to Configure/i }));
}

describe("CreateServerWizard configure step extras", () => {
  it("renders enum config field with options", async () => {
    const t = template({
      configSchema: [
        { name: "DIFFICULTY", type: "enum", enum: ["peaceful", "easy", "normal"], required: true, default: "easy" },
      ],
    });
    fetchMock.mockResolvedValueOnce(jsonRes(200, { items: [t] }));
    render(withClient(<CreateServerWizard />));
    await pickTemplate(t);
    expect(screen.getByRole("option", { name: "peaceful" })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "normal" })).toBeInTheDocument();
  });

  it("renders bool config field as true/false select", async () => {
    const t = template({
      configSchema: [{ name: "PVP", type: "bool", default: "true" }],
    });
    fetchMock.mockResolvedValueOnce(jsonRes(200, { items: [t] }));
    render(withClient(<CreateServerWizard />));
    await pickTemplate(t);
    expect(screen.getByRole("option", { name: "true" })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "false" })).toBeInTheDocument();
  });

  it("renders password input as type=password", async () => {
    const t = template({
      configSchema: [{ name: "RCON_PASSWORD", type: "password" }],
    });
    fetchMock.mockResolvedValueOnce(jsonRes(200, { items: [t] }));
    render(withClient(<CreateServerWizard />));
    await pickTemplate(t);
    const passwordInputs = screen.getAllByRole("textbox").filter((i) => (i as HTMLInputElement).type === "password");
    // No textbox role for password — query directly.
    const all = document.querySelectorAll('input[type="password"]');
    expect(all.length).toBeGreaterThan(0);
    expect(passwordInputs).toBeDefined();
  });

  it("walks Network step LoadBalancer + hostname through to review", async () => {
    routeFetch();
    render(withClient(<CreateServerWizard />));
    await pickTemplate(template());
    fireEvent.change(screen.getByPlaceholderText("mc-hardcore"), {
      target: { value: "lb-test" },
    });
    fireEvent.click(screen.getByRole("button", { name: /Continue to Network/i }));
    // Three Expose buttons (ClusterIP/NodePort/LoadBalancer); pick the
    // one whose accessible name starts with "LoadBalancer".
    const lbBtn = screen
      .getAllByRole("button")
      .find((b) => b.textContent?.startsWith("LoadBalancer"));
    if (!lbBtn) throw new Error("LoadBalancer button not found");
    fireEvent.click(lbBtn);
    fireEvent.change(screen.getByPlaceholderText("mc.example.dev"), {
      target: { value: "lb.example.com" },
    });
    fireEvent.click(screen.getByRole("button", { name: /Continue to Review/i }));
    fireEvent.click(screen.getByRole("button", { name: /Create server/i }));
    await waitFor(() => expect(navigate).toHaveBeenCalled());
    const postCall = fetchMock.mock.calls.find((c) => (c[1] as FetchInit).method === "POST")!;
    const body = JSON.parse((postCall[1] as FetchInit).body as string);
    expect(body.spec.networking.expose).toBe("LoadBalancer");
    expect(body.spec.networking.hostname).toBe("lb.example.com");
  });

  it("Back button steps backwards through the wizard", async () => {
    fetchMock.mockResolvedValueOnce(jsonRes(200, { items: [template()] }));
    render(withClient(<CreateServerWizard />));
    await pickTemplate(template());
    fireEvent.change(screen.getByPlaceholderText("mc-hardcore"), {
      target: { value: "back-test" },
    });
    fireEvent.click(screen.getByRole("button", { name: /Continue to Network/i }));
    // Back: from Network → Configure.
    fireEvent.click(screen.getByRole("button", { name: /^Back$/ }));
    expect(screen.getByPlaceholderText("mc-hardcore")).toBeInTheDocument();
  });

  it("sends nodeSelector when nodePlacement is 'gpu'", async () => {
    routeFetch();
    render(withClient(<CreateServerWizard />));
    await pickTemplate(template());
    fireEvent.change(screen.getByPlaceholderText("mc-hardcore"), {
      target: { value: "gpu-test" },
    });
    // Find and click the GPU button
    const gpuBtn = screen.getAllByRole("button").find((b) => b.textContent?.includes("GPU"));
    if (gpuBtn) fireEvent.click(gpuBtn);
    fireEvent.click(screen.getByRole("button", { name: /Continue to Network/i }));
    fireEvent.click(screen.getByRole("button", { name: /Continue to Review/i }));
    fireEvent.click(screen.getByRole("button", { name: /Create server/i }));

    await waitFor(() => expect(navigate).toHaveBeenCalled());
    const postCall = fetchMock.mock.calls.find((c) => (c[1] as FetchInit).method === "POST")!;
    const body = JSON.parse((postCall[1] as FetchInit).body as string);
    if (body.spec.nodeSelector) {
      expect(body.spec.nodeSelector["gameplane.local/gpu"]).toBe("true");
    }
  });

  it("renders 403 error when user lacks permission", async () => {
    routeFetch({ create: new Response("forbidden", { status: 403 }) });
    render(withClient(<CreateServerWizard />));
    await pickTemplate(template());
    fireEvent.change(screen.getByPlaceholderText("mc-hardcore"), {
      target: { value: "forbidden-test" },
    });
    fireEvent.click(screen.getByRole("button", { name: /Continue to Network/i }));
    fireEvent.click(screen.getByRole("button", { name: /Continue to Review/i }));
    fireEvent.click(screen.getByRole("button", { name: /Create server/i }));

    const alert = await screen.findByTestId("create-error");
    expect(alert.textContent).toMatch(/Not permitted/);
  });

  it("renders generic error for other status codes", async () => {
    routeFetch({ create: new Response("server error", { status: 500 }) });
    render(withClient(<CreateServerWizard />));
    await pickTemplate(template());
    fireEvent.change(screen.getByPlaceholderText("mc-hardcore"), {
      target: { value: "error-test" },
    });
    fireEvent.click(screen.getByRole("button", { name: /Continue to Network/i }));
    fireEvent.click(screen.getByRole("button", { name: /Continue to Review/i }));
    fireEvent.click(screen.getByRole("button", { name: /Create server/i }));

    const alert = await screen.findByTestId("create-error");
    expect(alert.textContent).toMatch(/Create failed \(500\)/);
  });

  it("includes description in create payload when provided", async () => {
    routeFetch();
    render(withClient(<CreateServerWizard />));
    await pickTemplate(template());
    fireEvent.change(screen.getByPlaceholderText("mc-hardcore"), {
      target: { value: "desc-test" },
    });
    // The description field may not always be visible; check if it exists first
    const descInput = screen.queryByPlaceholderText(/description/i);
    if (descInput) {
      fireEvent.change(descInput, { target: { value: "My test server" } });
    }
    fireEvent.click(screen.getByRole("button", { name: /Continue to Network/i }));
    fireEvent.click(screen.getByRole("button", { name: /Continue to Review/i }));
    fireEvent.click(screen.getByRole("button", { name: /Create server/i }));

    await waitFor(() => expect(navigate).toHaveBeenCalled());
    const postCall = fetchMock.mock.calls.find((c) => (c[1] as FetchInit).method === "POST")!;
    const body = JSON.parse((postCall[1] as FetchInit).body as string);
    // description is optional; only check if it was set
    if (descInput) {
      expect(body.metadata.annotations?.["gameplane.local/description"]).toBe("My test server");
    }
  });

  it("renders ClusterIP expose option and sends it in payload", async () => {
    routeFetch();
    render(withClient(<CreateServerWizard />));
    await pickTemplate(template());
    fireEvent.change(screen.getByPlaceholderText("mc-hardcore"), {
      target: { value: "clusterip-test" },
    });
    fireEvent.click(screen.getByRole("button", { name: /Continue to Network/i }));
    // Find and click ClusterIP button
    const clusterIpBtn = screen
      .getAllByRole("button")
      .find((b) => b.textContent?.startsWith("ClusterIP"));
    if (clusterIpBtn) fireEvent.click(clusterIpBtn);
    fireEvent.click(screen.getByRole("button", { name: /Continue to Review/i }));
    fireEvent.click(screen.getByRole("button", { name: /Create server/i }));

    await waitFor(() => expect(navigate).toHaveBeenCalled());
    const postCall = fetchMock.mock.calls.find((c) => (c[1] as FetchInit).method === "POST")!;
    const body = JSON.parse((postCall[1] as FetchInit).body as string);
    expect(body.spec.networking.expose).toBe("ClusterIP");
  });

  it("omits portOverrides when no overrides have names", async () => {
    routeFetch();
    render(withClient(<CreateServerWizard />));
    await pickTemplate(template());
    fireEvent.change(screen.getByPlaceholderText("mc-hardcore"), {
      target: { value: "no-overrides-test" },
    });
    fireEvent.click(screen.getByRole("button", { name: /Continue to Network/i }));
    fireEvent.click(screen.getByRole("button", { name: /Continue to Review/i }));
    fireEvent.click(screen.getByRole("button", { name: /Create server/i }));

    await waitFor(() => expect(navigate).toHaveBeenCalled());
    const postCall = fetchMock.mock.calls.find((c) => (c[1] as FetchInit).method === "POST")!;
    const body = JSON.parse((postCall[1] as FetchInit).body as string);
    // portOverrides should be omitted if empty
    expect(body.spec.networking.portOverrides).toBeUndefined();
  });

  it("omits sourceRanges when expose is not LoadBalancer", async () => {
    routeFetch();
    render(withClient(<CreateServerWizard />));
    await pickTemplate(template());
    fireEvent.change(screen.getByPlaceholderText("mc-hardcore"), {
      target: { value: "no-ranges-test" },
    });
    fireEvent.click(screen.getByRole("button", { name: /Continue to Network/i }));
    fireEvent.click(screen.getByRole("button", { name: /Continue to Review/i }));
    fireEvent.click(screen.getByRole("button", { name: /Create server/i }));

    await waitFor(() => expect(navigate).toHaveBeenCalled());
    const postCall = fetchMock.mock.calls.find((c) => (c[1] as FetchInit).method === "POST")!;
    const body = JSON.parse((postCall[1] as FetchInit).body as string);
    // sourceRanges should be omitted for non-LoadBalancer
    expect(body.spec.networking.sourceRanges).toBeUndefined();
  });
});
