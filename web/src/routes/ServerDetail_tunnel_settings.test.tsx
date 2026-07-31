import { beforeEach, describe, it, expect, vi } from "vitest";
import type { ReactNode } from "react";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { server } from "@/test/server";
import { renderWithQuery } from "@/test/render";
import { makeServer, makeTemplate } from "@/test/factories";

// Mock the tabs that won't be tested
vi.mock("./tabs/Console", () => ({ ConsoleTab: () => <div>console-tab</div> }));
vi.mock("./tabs/Files", () => ({ FilesTab: () => <div>files-tab</div> }));
vi.mock("./tabs/Logs", () => ({ LogsTab: () => <div>logs-tab</div> }));
vi.mock("./tabs/Mods", () => ({ ModsTab: () => <div>mods-tab</div> }));
vi.mock("./tabs/Players", () => ({ PlayersTab: () => <div>players-tab</div> }));
vi.mock("./tabs/Backups", () => ({ BackupsTab: () => <div>backups-tab</div> }));
vi.mock("./tabs/Overview", () => ({ OverviewTab: () => <div>overview-tab</div> }));

const navigate = vi.fn();
let searchParams = {};
vi.mock("@tanstack/react-router", () => ({
  Link: ({ children, to, ...rest }: { children: ReactNode; to: string } & Record<string, unknown>) => (
    <a href={to} {...rest}>{children}</a>
  ),
  useParams: () => ({ name: "tunnel-server" }),
  useSearch: () => searchParams,
  useNavigate: () => navigate,
}));

import { ServerDetailPage } from "./ServerDetail";

describe("NetworkingSection tunnel configuration", () => {
  beforeEach(() => {
    server.use(
      http.get("/servers/tunnel-server", () =>
        HttpResponse.json(makeServer({ metadata: { name: "tunnel-server" } })),
      ),
      http.get("/templates", () =>
        HttpResponse.json({ items: [makeTemplate()] }),
      ),
    );
  });

  it("shows tunnel enable toggle in networking settings", async () => {
    renderWithQuery(<ServerDetailPage />);
    const user = userEvent.setup();

    // Click Settings tab
    const settingsButton = await screen.findByRole("button", { name: /Settings/i });
    await user.click(settingsButton);

    // Click Networking tab
    const networkingButton = await screen.findByRole("button", { name: /Networking/i });
    await user.click(networkingButton);

    // Check for tunnel toggle
    expect(
      await screen.findByRole("checkbox", { name: /Enable tunnel/i }),
    ).toBeInTheDocument();
  });

  it("renders Tailscale warning when Tailscale provider is selected", async () => {
    renderWithQuery(<ServerDetailPage />);
    const user = userEvent.setup();

    // Navigate to networking settings
    const settingsButton = await screen.findByRole("button", { name: /Settings/i });
    await user.click(settingsButton);
    const networkingButton = await screen.findByRole("button", { name: /Networking/i });
    await user.click(networkingButton);

    // Enable tunnel
    const tunnelToggle = await screen.findByRole("checkbox", { name: /Enable tunnel/i });
    await user.click(tunnelToggle);

    // Select Tailscale provider (a native <select>, not a button)
    const providerSelect = (await screen.findByDisplayValue("frp")) as HTMLSelectElement;
    await user.selectOptions(providerSelect, "Tailscale");

    // Check for warning
    await waitFor(() => {
      expect(
        screen.getByText("Tailnet only — not public"),
      ).toBeInTheDocument();
      expect(
        screen.getByText(/becomes reachable only from your Tailscale network/),
      ).toBeInTheDocument();
    });
  });

  it("renders frp fields when frp provider is selected", async () => {
    renderWithQuery(<ServerDetailPage />);
    const user = userEvent.setup();

    // Navigate to networking settings
    const settingsButton = await screen.findByRole("button", { name: /Settings/i });
    await user.click(settingsButton);
    const networkingButton = await screen.findByRole("button", { name: /Networking/i });
    await user.click(networkingButton);

    // Enable tunnel (defaults to frp)
    const tunnelToggle = await screen.findByRole("checkbox", { name: /Enable tunnel/i });
    await user.click(tunnelToggle);

    // Check for frp-specific fields
    await waitFor(() => {
      expect(screen.getByPlaceholderText("relay.example.com")).toBeInTheDocument();
      expect(screen.getByDisplayValue("7000")).toBeInTheDocument();
      expect(screen.getByRole("button", { name: /Add port mapping/i })).toBeInTheDocument();
    });
  });

  it("renders playit.gg note when playit provider is selected", async () => {
    renderWithQuery(<ServerDetailPage />);
    const user = userEvent.setup();

    // Navigate to networking settings
    const settingsButton = await screen.findByRole("button", { name: /Settings/i });
    await user.click(settingsButton);
    const networkingButton = await screen.findByRole("button", { name: /Networking/i });
    await user.click(networkingButton);

    // Enable tunnel
    const tunnelToggle = await screen.findByRole("checkbox", { name: /Enable tunnel/i });
    await user.click(tunnelToggle);

    // Select playit provider (a native <select>, not a button)
    const providerSelect = (await screen.findByDisplayValue("frp")) as HTMLSelectElement;
    await user.selectOptions(providerSelect, "playit.gg");

    // Check for playit-specific note
    await waitFor(() => {
      expect(
        screen.getByText("Public address assigned at runtime"),
      ).toBeInTheDocument();
    });
  });

  it("shows credentials secret name input for all providers", async () => {
    renderWithQuery(<ServerDetailPage />);
    const user = userEvent.setup();

    // Navigate to networking settings
    const settingsButton = await screen.findByRole("button", { name: /Settings/i });
    await user.click(settingsButton);
    const networkingButton = await screen.findByRole("button", { name: /Networking/i });
    await user.click(networkingButton);

    // Enable tunnel
    const tunnelToggle = await screen.findByRole("checkbox", { name: /Enable tunnel/i });
    await user.click(tunnelToggle);

    // Check for credentials input
    await waitFor(() => {
      expect(screen.getByPlaceholderText("tunnel-secret")).toBeInTheDocument();
    });
  });

  it("blocks save when tunnel is enabled but credentials secret name is empty", async () => {
    renderWithQuery(<ServerDetailPage />);
    const user = userEvent.setup();

    // Navigate to networking settings
    const settingsButton = await screen.findByRole("button", { name: /Settings/i });
    await user.click(settingsButton);
    const networkingButton = await screen.findByRole("button", { name: /Networking/i });
    await user.click(networkingButton);

    // Enable tunnel
    const tunnelToggle = await screen.findByRole("checkbox", { name: /Enable tunnel/i });
    await user.click(tunnelToggle);

    // Select Tailscale so credentials are the only thing this test needs to
    // fill in — frp additionally requires a server address and a port
    // mapping, which would keep Save disabled for reasons unrelated to what
    // this test is checking.
    const providerSelect = (await screen.findByDisplayValue("frp")) as HTMLSelectElement;
    await user.selectOptions(providerSelect, "Tailscale");

    // Check that validation error is shown
    await waitFor(() => {
      expect(screen.getByText("Credentials Secret name is required")).toBeInTheDocument();
    });

    // Verify Save button is disabled
    const saveButton = await screen.findByRole("button", { name: /Save changes/i });
    expect(saveButton).toBeDisabled();

    // Fill in the credentials secret name
    const secretInput = screen.getByPlaceholderText("tunnel-secret");
    await user.type(secretInput, "my-secret");

    // Error should disappear and Save should be enabled
    await waitFor(() => {
      expect(screen.queryByText("Credentials Secret name is required")).not.toBeInTheDocument();
      expect(saveButton).not.toBeDisabled();
    });
  });

  it("blocks save for frp when no port mappings are configured", async () => {
    renderWithQuery(<ServerDetailPage />);
    const user = userEvent.setup();

    // Navigate to networking settings
    const settingsButton = await screen.findByRole("button", { name: /Settings/i });
    await user.click(settingsButton);
    const networkingButton = await screen.findByRole("button", { name: /Networking/i });
    await user.click(networkingButton);

    // Enable tunnel (defaults to frp)
    const tunnelToggle = await screen.findByRole("checkbox", { name: /Enable tunnel/i });
    await user.click(tunnelToggle);

    // Fill in credentials and server address
    const secretInput = screen.getByPlaceholderText("tunnel-secret");
    await user.type(secretInput, "my-secret");
    const serverAddrInput = screen.getByPlaceholderText("relay.example.com");
    await user.type(serverAddrInput, "relay.example.com");

    // Check that error about port mappings is shown
    await waitFor(() => {
      expect(
        screen.getByText("At least one port mapping is required for frp"),
      ).toBeInTheDocument();
    });

    // Verify Save button is disabled
    const saveButton = await screen.findByRole("button", { name: /Save changes/i });
    expect(saveButton).toBeDisabled();

    // Add a port mapping
    const addPortButton = screen.getByRole("button", { name: /Add port mapping/i });
    await user.click(addPortButton);

    // Fill in the port mapping
    const portInputs = screen.getAllByPlaceholderText("8080");
    const portInput = portInputs[portInputs.length - 1];
    await user.type(portInput, "8080");

    const portNameInputs = screen.getAllByPlaceholderText("game");
    const portNameInput = portNameInputs[portNameInputs.length - 1];
    await user.type(portNameInput, "game");

    // Error should disappear and Save should be enabled
    await waitFor(() => {
      expect(
        screen.queryByText("At least one port mapping is required for frp"),
      ).not.toBeInTheDocument();
      expect(saveButton).not.toBeDisabled();
    });
  });

  it("does not allow port 0 in port mappings", async () => {
    renderWithQuery(<ServerDetailPage />);
    const user = userEvent.setup();

    // Navigate to networking settings
    const settingsButton = await screen.findByRole("button", { name: /Settings/i });
    await user.click(settingsButton);
    const networkingButton = await screen.findByRole("button", { name: /Networking/i });
    await user.click(networkingButton);

    // Enable tunnel
    const tunnelToggle = await screen.findByRole("checkbox", { name: /Enable tunnel/i });
    await user.click(tunnelToggle);

    // Fill in credentials and server address
    const secretInput = screen.getByPlaceholderText("tunnel-secret");
    await user.type(secretInput, "my-secret");
    const serverAddrInput = screen.getByPlaceholderText("relay.example.com");
    await user.type(serverAddrInput, "relay.example.com");

    // Add a port mapping with port 0
    const addPortButton = screen.getByRole("button", { name: /Add port mapping/i });
    await user.click(addPortButton);

    // Fill in port name
    const portNameInputs = screen.getAllByPlaceholderText("game");
    const portNameInput = portNameInputs[portNameInputs.length - 1];
    await user.type(portNameInput, "game");

    // Try to enter 0 in the port — it should be treated as empty
    const portInputs = screen.getAllByPlaceholderText("8080");
    const portInput = portInputs[portInputs.length - 1] as HTMLInputElement;
    await user.type(portInput, "0");
    await user.clear(portInput);

    // Check that the validation error is shown
    await waitFor(() => {
      expect(
        screen.getByText(/At least one port mapping is required for frp/),
      ).toBeInTheDocument();
    });

    // Verify Save button is disabled
    const saveButton = await screen.findByRole("button", { name: /Save changes/i });
    expect(saveButton).toBeDisabled();

    // Now enter a valid port
    await user.type(portInput, "8080");

    // Error should disappear
    await waitFor(() => {
      expect(
        screen.queryByText(/At least one port mapping is required for frp/),
      ).not.toBeInTheDocument();
      expect(saveButton).not.toBeDisabled();
    });
  });

  it("allows save when tailscale tunnel is enabled with credentials secret only", async () => {
    renderWithQuery(<ServerDetailPage />);
    const user = userEvent.setup();

    // Navigate to networking settings
    const settingsButton = await screen.findByRole("button", { name: /Settings/i });
    await user.click(settingsButton);
    const networkingButton = await screen.findByRole("button", { name: /Networking/i });
    await user.click(networkingButton);

    // Enable tunnel
    const tunnelToggle = await screen.findByRole("checkbox", { name: /Enable tunnel/i });
    await user.click(tunnelToggle);

    // Select Tailscale provider (a native <select>, not a button)
    const providerSelect = (await screen.findByDisplayValue("frp")) as HTMLSelectElement;
    await user.selectOptions(providerSelect, "Tailscale");

    // Fill in credentials secret only
    const secretInput = screen.getByPlaceholderText("tunnel-secret");
    await user.type(secretInput, "my-secret");

    // Verify Save button is enabled (no error shown)
    const saveButton = await screen.findByRole("button", { name: /Save changes/i });
    await waitFor(() => {
      expect(saveButton).not.toBeDisabled();
    });
  });
});
