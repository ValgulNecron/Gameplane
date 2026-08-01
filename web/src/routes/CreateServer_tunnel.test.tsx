import { beforeEach, describe, it, expect, vi } from "vitest";
import type { ReactNode } from "react";
import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { server } from "@/test/server";
import { renderWithQuery } from "@/test/render";
import { makeTemplate } from "@/test/factories";

const navigate = vi.fn();
vi.mock("@tanstack/react-router", () => ({
  Link: ({ children, to, ...rest }: { children: ReactNode; to: string } & Record<string, unknown>) => (
    <a href={to} {...rest}>{children}</a>
  ),
  useSearch: () => ({}),
  useNavigate: () => navigate,
  useRouter: () => ({ state: { location: { pathname: "/servers/new" } } }),
}));

import { CreateServerWizard } from "./CreateServer";

describe("CreateServerWizard tunnel configuration", () => {
  beforeEach(() => {
    server.use(
      http.get("/templates", () =>
        HttpResponse.json({
          items: [makeTemplate({ metadata: { name: "minecraft" } })],
        }),
      ),
      http.get("/cluster", () =>
        HttpResponse.json({
          nodes: [
            {
              cpu: { capacity: 16 },
              memory: { capacity: 64 * 1024 ** 3 },
            },
          ],
        }),
      ),
    );
  });

  it("shows tunnel enable option in network step", async () => {
    const user = userEvent.setup();
    renderWithQuery(<CreateServerWizard />);

    // Pick a template
    const templateButton = await screen.findByRole("button", { name: /Minecraft/i });
    await user.click(templateButton);

    // Continue to Configure
    const continueBtn = screen.getByRole("button", { name: /Continue/i });
    await user.click(continueBtn);

    // Fill in required fields
    const nameInput = screen.getByPlaceholderText("mc-hardcore");
    await user.type(nameInput, "test-server");

    // Continue to Network
    await user.click(screen.getByRole("button", { name: /Continue/i }));

    // Check for tunnel toggle
    expect(
      screen.getByRole("checkbox", { name: /Enable tunnel/i }),
    ).toBeInTheDocument();
  });

  it("shows provider options when tunnel is enabled", async () => {
    const user = userEvent.setup();
    renderWithQuery(<CreateServerWizard />);

    // Pick a template and navigate to network step
    let templateButton = await screen.findByRole("button", { name: /Minecraft/i });
    await user.click(templateButton);
    await user.click(screen.getByRole("button", { name: /Continue/i }));

    const nameInput = screen.getByPlaceholderText("mc-hardcore");
    await user.type(nameInput, "test-server");
    await user.click(screen.getByRole("button", { name: /Continue/i }));

    // Enable tunnel
    const tunnelToggle = screen.getByRole("checkbox", { name: /Enable tunnel/i });
    await user.click(tunnelToggle);

    // Check for provider options
    expect(screen.getByRole("button", { name: "frp" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Tailscale" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "playit.gg" })).toBeInTheDocument();
  });

  it("shows tunnel info in review step when enabled", async () => {
    const user = userEvent.setup();
    renderWithQuery(<CreateServerWizard />);

    // Complete wizard with tunnel enabled
    let templateButton = await screen.findByRole("button", { name: /Minecraft/i });
    await user.click(templateButton);
    await user.click(screen.getByRole("button", { name: /Continue/i }));

    const nameInput = screen.getByPlaceholderText("mc-hardcore");
    await user.type(nameInput, "test-server");
    await user.click(screen.getByRole("button", { name: /Continue/i }));

    // Enable tunnel (defaults to frp) and fill the fields frp requires
    const tunnelToggle = screen.getByRole("checkbox", { name: /Enable tunnel/i });
    await user.click(tunnelToggle);

    const credsInput = screen.getByTestId("tunnel-credentials-input") as HTMLInputElement;
    await user.type(credsInput, "my-creds");

    const addrInput = screen.getByTestId("frp-server-addr") as HTMLInputElement;
    await user.type(addrInput, "relay.example.com");

    const addBtn = screen.getByTestId("tunnel-port-add");
    await user.click(addBtn);

    const nameField = screen.getByTestId("tunnel-port-name-0") as HTMLInputElement;
    await user.type(nameField, "game");

    const portField = screen.getByTestId("tunnel-port-number-0") as HTMLInputElement;
    await user.clear(portField);
    await user.type(portField, "25565");

    // Continue to Review
    await user.click(screen.getByRole("button", { name: /Continue to Review/i }));

    // Check review shows tunnel
    expect(screen.getByText("Tunnel provider")).toBeInTheDocument();
    expect(screen.getByText("frp")).toBeInTheDocument();
  });

  it("allows selecting different tunnel providers", async () => {
    const user = userEvent.setup();
    renderWithQuery(<CreateServerWizard />);

    // Navigate to network step
    let templateButton = await screen.findByRole("button", { name: /Minecraft/i });
    await user.click(templateButton);
    await user.click(screen.getByRole("button", { name: /Continue/i }));

    const nameInput = screen.getByPlaceholderText("mc-hardcore");
    await user.type(nameInput, "test-server");
    await user.click(screen.getByRole("button", { name: /Continue/i }));

    // Enable tunnel
    const tunnelToggle = screen.getByRole("checkbox", { name: /Enable tunnel/i });
    await user.click(tunnelToggle);

    // Select Tailscale
    const tailscaleButton = screen.getByRole("button", { name: "Tailscale" });
    await user.click(tailscaleButton);

    // Continue to Review
    await user.click(screen.getByRole("button", { name: /Continue/i }));

    // Check review shows Tailscale
    expect(screen.getByText("Tailscale")).toBeInTheDocument();
  });

  it("omits tunnel from review when not enabled", async () => {
    const user = userEvent.setup();
    renderWithQuery(<CreateServerWizard />);

    // Navigate to review without enabling tunnel
    let templateButton = await screen.findByRole("button", { name: /Minecraft/i });
    await user.click(templateButton);
    await user.click(screen.getByRole("button", { name: /Continue/i }));

    const nameInput = screen.getByPlaceholderText("mc-hardcore");
    await user.type(nameInput, "test-server");
    await user.click(screen.getByRole("button", { name: /Continue/i }));

    // Don't enable tunnel
    await user.click(screen.getByRole("button", { name: /Continue/i }));

    // Check review doesn't show tunnel
    const tunnelLabels = screen.queryAllByText("Tunnel");
    expect(tunnelLabels.length).toBe(0);
  });

  it("blocks create when tunnel enabled but credentials not set", async () => {
    const user = userEvent.setup();
    renderWithQuery(<CreateServerWizard />);

    // Navigate to network step
    const templateButton = await screen.findByRole("button", { name: /Minecraft/i });
    await user.click(templateButton);
    await user.click(screen.getByRole("button", { name: /Continue/i }));

    const nameInput = screen.getByPlaceholderText("mc-hardcore");
    await user.type(nameInput, "tunnel-test");
    await user.click(screen.getByRole("button", { name: /Continue/i }));

    // Enable tunnel without filling credentials
    const tunnelToggle = screen.getByRole("checkbox", { name: /Enable tunnel/i });
    await user.click(tunnelToggle);

    // Try to continue - should be blocked
    const continueBtn = screen.getByRole("button", { name: /Continue to Review/i });
    expect(continueBtn).toBeDisabled();
    expect(screen.getByText(/Tunnel requires credentials/)).toBeInTheDocument();
  });

  it("blocks create when frp enabled but server address not set", async () => {
    const user = userEvent.setup();
    renderWithQuery(<CreateServerWizard />);

    // Navigate to network step
    const templateButton = await screen.findByRole("button", { name: /Minecraft/i });
    await user.click(templateButton);
    await user.click(screen.getByRole("button", { name: /Continue/i }));

    const nameInput = screen.getByPlaceholderText("mc-hardcore");
    await user.type(nameInput, "tunnel-test");
    await user.click(screen.getByRole("button", { name: /Continue/i }));

    // Enable tunnel, set credentials but not server address
    const tunnelToggle = screen.getByRole("checkbox", { name: /Enable tunnel/i });
    await user.click(tunnelToggle);

    const credsInput = screen.getByTestId("tunnel-credentials-input") as HTMLInputElement;
    await user.type(credsInput, "my-creds");

    // Try to continue - should be blocked because no server address
    const continueBtn = screen.getByRole("button", { name: /Continue to Review/i });
    expect(continueBtn).toBeDisabled();
    expect(screen.getByText(/frp requires a server address/)).toBeInTheDocument();
  });

  it("blocks create when frp enabled but no port mappings added", async () => {
    const user = userEvent.setup();
    renderWithQuery(<CreateServerWizard />);

    // Navigate to network step
    const templateButton = await screen.findByRole("button", { name: /Minecraft/i });
    await user.click(templateButton);
    await user.click(screen.getByRole("button", { name: /Continue/i }));

    const nameInput = screen.getByPlaceholderText("mc-hardcore");
    await user.type(nameInput, "tunnel-test");
    await user.click(screen.getByRole("button", { name: /Continue/i }));

    // Enable tunnel, set credentials and server address but no port mappings
    const tunnelToggle = screen.getByRole("checkbox", { name: /Enable tunnel/i });
    await user.click(tunnelToggle);

    const credsInput = screen.getByTestId("tunnel-credentials-input") as HTMLInputElement;
    await user.type(credsInput, "my-creds");

    const addrInput = screen.getByTestId("frp-server-addr") as HTMLInputElement;
    await user.type(addrInput, "relay.example.com");

    // Try to continue - should be blocked because no port mappings
    const continueBtn = screen.getByRole("button", { name: /Continue to Review/i });
    expect(continueBtn).toBeDisabled();
    expect(screen.getByText(/frp requires at least one complete port mapping/)).toBeInTheDocument();
  });

  it("saves credentials via separate PUT call after server creation", async () => {
    const user = userEvent.setup();
    const apiCalls: Array<{ method: string; url: string; body?: unknown }> = [];
    vi.stubGlobal("fetch", async (url: string, init?: { method?: string; body?: string }) => {
      const method = init?.method ?? "GET";
      apiCalls.push({
        method,
        url: typeof url === "string" ? url : url.toString(),
        body: init?.body ? JSON.parse(init.body) : undefined,
      });

      const urlStr = typeof url === "string" ? url : url.toString();

      if (urlStr.includes("/cluster")) {
        return Promise.resolve(
          new Response(JSON.stringify({ nodes: [] }), {
            status: 200,
            headers: { "Content-Type": "application/json" },
          }),
        );
      }
      if (method === "POST" && urlStr.includes("/servers")) {
        return Promise.resolve(
          new Response(JSON.stringify({ metadata: { name: "tunnel-test" } }), {
            status: 201,
            headers: { "Content-Type": "application/json" },
          }),
        );
      }
      if (method === "PUT" && urlStr.includes(":tunnel-credentials")) {
        return Promise.resolve(new Response("", { status: 204 }));
      }
      return Promise.resolve(
        new Response(
          JSON.stringify({
            items: [makeTemplate({ metadata: { name: "minecraft" } })],
          }),
          {
            status: 200,
            headers: { "Content-Type": "application/json" },
          },
        ),
      );
    });

    try {
      renderWithQuery(<CreateServerWizard />);

      // Pick template
      const templateButton = await screen.findByRole("button", { name: /Minecraft/i });
      await user.click(templateButton);
      await user.click(screen.getByRole("button", { name: /Continue/i }));

      // Configure
      const nameInput = screen.getByPlaceholderText("mc-hardcore");
      await user.type(nameInput, "tunnel-test");
      await user.click(screen.getByRole("button", { name: /Continue/i }));

      // Network - enable tunnel and fill tailscale details (simpler than frp)
      const tunnelToggle = screen.getByRole("checkbox", { name: /Enable tunnel/i });
      await user.click(tunnelToggle);

      const tailscaleBtn = screen.getByRole("button", { name: "Tailscale" });
      await user.click(tailscaleBtn);

      const credsInput = screen.getByTestId("tunnel-credentials-input") as HTMLInputElement;
      await user.type(credsInput, "tskey_aabbcc");

      // Go to review and create
      await user.click(screen.getByRole("button", { name: /Continue to Review/i }));
      await user.click(screen.getByRole("button", { name: /Create server/i }));

      // Verify both POST and PUT calls were made
      const postCalls = apiCalls.filter((c) => c.method === "POST" && c.url.includes("/servers"));
      expect(postCalls.length).toBe(1);

      const putCalls = apiCalls.filter((c) => c.method === "PUT" && c.url.includes(":tunnel-credentials"));
      expect(putCalls.length).toBe(1);

      // Verify the PUT payload has the right structure
      if (putCalls[0]?.body) {
        expect(putCalls[0].body).toEqual({
          provider: "tailscale",
          values: { authKey: "tskey_aabbcc" },
        });
      }
    } finally {
      vi.unstubAllGlobals();
    }
  });

  it("includes frp config in create payload with credentialsSecretRef only when using existing secret", async () => {
    const user = userEvent.setup();
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);

    try {
      fetchMock.mockImplementation((url: string, init?: { method?: string; body?: string }) => {
        if (url.includes("/cluster")) {
          return Promise.resolve(
            new Response(JSON.stringify({ nodes: [] }), {
              status: 200,
              headers: { "Content-Type": "application/json" },
            }),
          );
        }
        if ((init?.method ?? "GET") === "POST") {
          return Promise.resolve(
            new Response(JSON.stringify({ metadata: { name: "tunnel-test" } }), {
              status: 201,
              headers: { "Content-Type": "application/json" },
            }),
          );
        }
        if ((init?.method ?? "GET") === "PUT") {
          return Promise.resolve(new Response("", { status: 204 }));
        }
        return Promise.resolve(
          new Response(
            JSON.stringify({
              items: [makeTemplate({ metadata: { name: "minecraft" } })],
            }),
            {
              status: 200,
              headers: { "Content-Type": "application/json" },
            },
          ),
        );
      });

      renderWithQuery(<CreateServerWizard />);

      // Pick template
      const templateButton = await screen.findByRole("button", { name: /Minecraft/i });
      await user.click(templateButton);
      await user.click(screen.getByRole("button", { name: /Continue/i }));

      // Configure
      const nameInput = screen.getByPlaceholderText("mc-hardcore");
      await user.type(nameInput, "tunnel-test");
      await user.click(screen.getByRole("button", { name: /Continue/i }));

      // Network - enable tunnel and fill frp details with EXISTING secret
      const tunnelToggle = screen.getByRole("checkbox", { name: /Enable tunnel/i });
      await user.click(tunnelToggle);

      // Use existing secret, not credential value
      const existingSecretInput = screen.getByTestId("tunnel-existing-secret") as HTMLInputElement;
      await user.type(existingSecretInput, "my-existing-secret");

      const addrInput = screen.getByTestId("frp-server-addr") as HTMLInputElement;
      await user.type(addrInput, "relay.example.com");

      const portInput = screen.getByTestId("frp-server-port") as HTMLInputElement;
      await user.clear(portInput);
      await user.type(portInput, "7001");

      // Add port mapping
      const addBtn = screen.getByTestId("tunnel-port-add");
      await user.click(addBtn);

      const nameField = screen.getByTestId("tunnel-port-name-0") as HTMLInputElement;
      await user.type(nameField, "game");

      const portField = screen.getByTestId("tunnel-port-number-0") as HTMLInputElement;
      await user.clear(portField);
      await user.type(portField, "25565");

      // Go to review and create
      await user.click(screen.getByRole("button", { name: /Continue to Review/i }));
      await user.click(screen.getByRole("button", { name: /Create server/i }));

      // Verify the POST call includes credentialsSecretRef (because we provided existing secret name)
      expect(fetchMock).toHaveBeenCalled();
      const postCall = fetchMock.mock.calls.find((c) => {
        const init = c[1] as { method?: string; body?: string };
        return (init?.method ?? "GET") === "POST";
      });
      expect(postCall).toBeDefined();

      const body = JSON.parse((postCall![1] as { body?: string }).body as string);
      expect(body.spec.networking.tunnel).toBeDefined();
      expect(body.spec.networking.tunnel.enabled).toBe(true);
      expect(body.spec.networking.tunnel.provider).toBe("frp");
      // When using existing secret, credentialsSecretRef should be in the payload
      expect(body.spec.networking.tunnel.credentialsSecretRef).toEqual({ name: "my-existing-secret" });
      expect(body.spec.networking.tunnel.frp).toBeDefined();
      expect(body.spec.networking.tunnel.frp.serverAddr).toBe("relay.example.com");
      expect(body.spec.networking.tunnel.frp.serverPort).toBe(7001);
      expect(body.spec.networking.tunnel.frp.remotePorts).toEqual([{ name: "game", remotePort: 25565 }]);
    } finally {
      vi.unstubAllGlobals();
    }
  });

  it("includes tailscale tunnel config in create payload with credentials but no tailscale block if optional fields empty", async () => {
    const user = userEvent.setup();
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);

    try {
      fetchMock.mockImplementation((url: string, init?: { method?: string; body?: string }) => {
        if (url.includes("/cluster")) {
          return Promise.resolve(
            new Response(JSON.stringify({ nodes: [] }), {
              status: 200,
              headers: { "Content-Type": "application/json" },
            }),
          );
        }
        if ((init?.method ?? "GET") === "POST") {
          return Promise.resolve(
            new Response(JSON.stringify({ metadata: { name: "tunnel-test" } }), {
              status: 201,
              headers: { "Content-Type": "application/json" },
            }),
          );
        }
        return Promise.resolve(
          new Response(
            JSON.stringify({
              items: [makeTemplate({ metadata: { name: "minecraft" } })],
            }),
            {
              status: 200,
              headers: { "Content-Type": "application/json" },
            },
          ),
        );
      });

      renderWithQuery(<CreateServerWizard />);

      // Pick template
      const templateButton = await screen.findByRole("button", { name: /Minecraft/i });
      await user.click(templateButton);
      await user.click(screen.getByRole("button", { name: /Continue/i }));

      // Configure
      const nameInput = screen.getByPlaceholderText("mc-hardcore");
      await user.type(nameInput, "tunnel-test");
      await user.click(screen.getByRole("button", { name: /Continue/i }));

      // Network - enable tunnel, select Tailscale
      const tunnelToggle = screen.getByRole("checkbox", { name: /Enable tunnel/i });
      await user.click(tunnelToggle);

      const tailscaleBtn = screen.getByRole("button", { name: "Tailscale" });
      await user.click(tailscaleBtn);

      const credsInput = screen.getByTestId("tunnel-credentials-input") as HTMLInputElement;
      await user.type(credsInput, "tailscale-secret");

      // Go to review and create without filling optional fields
      await user.click(screen.getByRole("button", { name: /Continue to Review/i }));
      await user.click(screen.getByRole("button", { name: /Create server/i }));

      // Verify the POST call
      const postCall = fetchMock.mock.calls.find((c) => {
        const init = c[1] as { method?: string; body?: string };
        return (init?.method ?? "GET") === "POST";
      });

      const body = JSON.parse((postCall![1] as { body?: string }).body as string);
      expect(body.spec.networking.tunnel.provider).toBe("tailscale");
      // credentialsSecretRef.name should be <serverName>-tunnel-auth, not the credential value
      expect(body.spec.networking.tunnel.credentialsSecretRef).toEqual({ name: "tunnel-test-tunnel-auth" });
      // Tailscale block may be present but empty, or omitted entirely - either is valid
      if (body.spec.networking.tunnel.tailscale) {
        expect(body.spec.networking.tunnel.tailscale.hostname).toBeUndefined();
        expect(body.spec.networking.tunnel.tailscale.tags).toBeUndefined();
      }
    } finally {
      vi.unstubAllGlobals();
    }
  });

  it("includes credentialsSecretRef in create payload when credential value is entered (not existing secret)", async () => {
    const user = userEvent.setup();
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);

    try {
      fetchMock.mockImplementation((url: string, init?: { method?: string; body?: string }) => {
        if (url.includes("/cluster")) {
          return Promise.resolve(
            new Response(JSON.stringify({ nodes: [] }), {
              status: 200,
              headers: { "Content-Type": "application/json" },
            }),
          );
        }
        if ((init?.method ?? "GET") === "POST") {
          return Promise.resolve(
            new Response(JSON.stringify({ metadata: { name: "frp-test" } }), {
              status: 201,
              headers: { "Content-Type": "application/json" },
            }),
          );
        }
        if ((init?.method ?? "GET") === "PUT") {
          return Promise.resolve(new Response("", { status: 204 }));
        }
        return Promise.resolve(
          new Response(
            JSON.stringify({
              items: [makeTemplate({ metadata: { name: "minecraft" } })],
            }),
            {
              status: 200,
              headers: { "Content-Type": "application/json" },
            },
          ),
        );
      });

      renderWithQuery(<CreateServerWizard />);

      // Pick template
      const templateButton = await screen.findByRole("button", { name: /Minecraft/i });
      await user.click(templateButton);
      await user.click(screen.getByRole("button", { name: /Continue/i }));

      // Configure
      const nameInput = screen.getByPlaceholderText("mc-hardcore");
      await user.type(nameInput, "frp-test");
      await user.click(screen.getByRole("button", { name: /Continue/i }));

      // Network - enable tunnel with frp, provide credential value (NOT existing secret)
      const tunnelToggle = screen.getByRole("checkbox", { name: /Enable tunnel/i });
      await user.click(tunnelToggle);

      // Make sure we're NOT using an existing secret
      const existingSecretInput = screen.getByTestId("tunnel-existing-secret") as HTMLInputElement;
      expect(existingSecretInput.value).toBe("");

      const credsInput = screen.getByTestId("tunnel-credentials-input") as HTMLInputElement;
      await user.type(credsInput, "my-frp-token");

      const addrInput = screen.getByTestId("frp-server-addr") as HTMLInputElement;
      await user.type(addrInput, "relay.example.com");

      const addBtn = screen.getByTestId("tunnel-port-add");
      await user.click(addBtn);

      const nameField = screen.getByTestId("tunnel-port-name-0") as HTMLInputElement;
      await user.type(nameField, "game");

      const portField = screen.getByTestId("tunnel-port-number-0") as HTMLInputElement;
      await user.clear(portField);
      await user.type(portField, "25565");

      // Go to review and create
      await user.click(screen.getByRole("button", { name: /Continue to Review/i }));
      await user.click(screen.getByRole("button", { name: /Create server/i }));

      // Verify the POST call includes credentialsSecretRef with the derived secret name
      const postCall = fetchMock.mock.calls.find((c) => {
        const init = c[1] as { method?: string; body?: string };
        return (init?.method ?? "GET") === "POST";
      });
      expect(postCall).toBeDefined();

      const body = JSON.parse((postCall![1] as { body?: string }).body as string);
      expect(body.spec.networking.tunnel).toBeDefined();
      expect(body.spec.networking.tunnel.enabled).toBe(true);
      expect(body.spec.networking.tunnel.provider).toBe("frp");
      // credentialsSecretRef.name should be <serverName>-tunnel-auth
      expect(body.spec.networking.tunnel.credentialsSecretRef).toEqual({ name: "frp-test-tunnel-auth" });
      expect(body.spec.networking.tunnel.frp).toBeDefined();
    } finally {
      vi.unstubAllGlobals();
    }
  });

  it("shows error message with recovery steps when credential PUT fails after successful create", async () => {
    const user = userEvent.setup();
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);

    try {
      let callCount = 0;
      fetchMock.mockImplementation((url: string, init?: { method?: string; body?: string }) => {
        if (url.includes("/cluster")) {
          return Promise.resolve(
            new Response(JSON.stringify({ nodes: [] }), {
              status: 200,
              headers: { "Content-Type": "application/json" },
            }),
          );
        }
        if ((init?.method ?? "GET") === "POST" && url.includes("/servers")) {
          callCount++;
          return Promise.resolve(
            new Response(JSON.stringify({ metadata: { name: "partial-tunnel" } }), {
              status: 201,
              headers: { "Content-Type": "application/json" },
            }),
          );
        }
        // PUT credentials fails on purpose
        if ((init?.method ?? "GET") === "PUT" && url.includes(":tunnel-credentials")) {
          return Promise.reject(new Error("Secret quota exceeded"));
        }
        return Promise.resolve(
          new Response(
            JSON.stringify({
              items: [makeTemplate({ metadata: { name: "minecraft" } })],
            }),
            {
              status: 200,
              headers: { "Content-Type": "application/json" },
            },
          ),
        );
      });

      renderWithQuery(<CreateServerWizard />);

      // Pick template
      const templateButton = await screen.findByRole("button", { name: /Minecraft/i });
      await user.click(templateButton);
      await user.click(screen.getByRole("button", { name: /Continue/i }));

      // Configure
      const nameInput = screen.getByPlaceholderText("mc-hardcore");
      await user.type(nameInput, "partial-tunnel");
      await user.click(screen.getByRole("button", { name: /Continue/i }));

      // Network - enable tunnel with Tailscale (simpler than frp)
      const tunnelToggle = screen.getByRole("checkbox", { name: /Enable tunnel/i });
      await user.click(tunnelToggle);

      const tailscaleBtn = screen.getByRole("button", { name: "Tailscale" });
      await user.click(tailscaleBtn);

      const credsInput = screen.getByTestId("tunnel-credentials-input") as HTMLInputElement;
      await user.type(credsInput, "tskey_credential");

      // Go to review and create
      await user.click(screen.getByRole("button", { name: /Continue to Review/i }));
      await user.click(screen.getByRole("button", { name: /Create server/i }));

      // Wait for error to appear
      const errorAlert = await screen.findByTestId("create-error");
      expect(errorAlert).toBeInTheDocument();

      // Error should mention that server was created and provide recovery steps
      const errorText = errorAlert.textContent;
      expect(errorText).toContain("Server created");
      expect(errorText).toContain("credential save failed");
      expect(errorText).toContain("Networking settings");

      // Verify that the POST call succeeded (server creation)
      const postCalls = fetchMock.mock.calls.filter((c) => {
        const init = c[1] as { method?: string; body?: string };
        return (init?.method ?? "GET") === "POST" && c[0].toString().includes("/servers");
      });
      expect(postCalls.length).toBe(1);

      // Verify that the PUT call was attempted (and failed)
      const putCalls = fetchMock.mock.calls.filter((c) => {
        const init = c[1] as { method?: string; body?: string };
        return (init?.method ?? "GET") === "PUT" && c[0].toString().includes(":tunnel-credentials");
      });
      expect(putCalls.length).toBe(1);
    } finally {
      vi.unstubAllGlobals();
    }
  });

  it("omits tunnel key entirely when tunnel is disabled", async () => {
    const user = userEvent.setup();
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);

    try {
      fetchMock.mockImplementation((url: string, init?: { method?: string; body?: string }) => {
        if (url.includes("/cluster")) {
          return Promise.resolve(
            new Response(JSON.stringify({ nodes: [] }), {
              status: 200,
              headers: { "Content-Type": "application/json" },
            }),
          );
        }
        if ((init?.method ?? "GET") === "POST") {
          return Promise.resolve(
            new Response(JSON.stringify({ metadata: { name: "no-tunnel-test" } }), {
              status: 201,
              headers: { "Content-Type": "application/json" },
            }),
          );
        }
        return Promise.resolve(
          new Response(
            JSON.stringify({
              items: [makeTemplate({ metadata: { name: "minecraft" } })],
            }),
            {
              status: 200,
              headers: { "Content-Type": "application/json" },
            },
          ),
        );
      });

      renderWithQuery(<CreateServerWizard />);

      // Pick template
      const templateButton = await screen.findByRole("button", { name: /Minecraft/i });
      await user.click(templateButton);
      await user.click(screen.getByRole("button", { name: /Continue/i }));

      // Configure
      const nameInput = screen.getByPlaceholderText("mc-hardcore");
      await user.type(nameInput, "no-tunnel-test");
      await user.click(screen.getByRole("button", { name: /Continue/i }));

      // Network - don't enable tunnel
      await user.click(screen.getByRole("button", { name: /Continue to Review/i }));
      await user.click(screen.getByRole("button", { name: /Create server/i }));

      // Verify the POST call
      const postCall = fetchMock.mock.calls.find((c) => {
        const init = c[1] as { method?: string; body?: string };
        return (init?.method ?? "GET") === "POST";
      });

      const body = JSON.parse((postCall![1] as { body?: string }).body as string);
      expect(body.spec.networking.tunnel).toBeUndefined();
    } finally {
      vi.unstubAllGlobals();
    }
  });
});
