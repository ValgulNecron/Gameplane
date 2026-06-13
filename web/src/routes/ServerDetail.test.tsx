import { describe, it, expect, vi } from "vitest";
import type { ReactNode } from "react";
import { http, HttpResponse } from "msw";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { server } from "@/test/server";
import { renderWithQuery } from "@/test/render";
import { makeServer, makeTemplate, makeUser } from "@/test/factories";

// Router APIs the route reaches into.
const navigate = vi.fn();
vi.mock("@tanstack/react-router", () => ({
  Link: ({ children, to, ...rest }: { children: ReactNode; to: string } & Record<string, unknown>) => (
    <a href={to} {...rest}>{children}</a>
  ),
  useParams: () => ({ name: "alpha" }),
  useNavigate: () => navigate,
}));

// Heavy lazy tabs (xterm / Monaco) — replace with stubs so jsdom doesn't
// have to evaluate them.
vi.mock("./tabs/Console", () => ({ ConsoleTab: () => <div>console-tab</div> }));
vi.mock("./tabs/Files", () => ({ FilesTab: () => <div>files-tab</div> }));
vi.mock("./tabs/Logs", () => ({ LogsTab: () => <div>logs-tab</div> }));
vi.mock("./tabs/Mods", () => ({ ModsTab: () => <div>mods-tab</div> }));
vi.mock("./tabs/Players", () => ({ PlayersTab: () => <div>players-tab</div> }));
vi.mock("./tabs/Backups", () => ({ BackupsTab: () => <div>backups-tab</div> }));
vi.mock("./tabs/Overview", () => ({ OverviewTab: () => <div>overview-tab</div> }));
vi.mock("./tabs/Settings", () => ({ SettingsTab: () => <div>settings-tab</div> }));

import { ServerDetailPage } from "./ServerDetail";

describe("ServerDetailPage", () => {
  it("renders the server name and overview tab by default", async () => {
    server.use(
      http.get("/servers/alpha", () => HttpResponse.json(makeServer({ metadata: { name: "alpha" } }))),
    );
    renderWithQuery(<ServerDetailPage />);
    expect(await screen.findByRole("heading", { level: 1, name: "alpha" })).toBeInTheDocument();
    expect(await screen.findByText("overview-tab")).toBeInTheDocument();
  });

  it("switches to Logs tab on click", async () => {
    server.use(
      http.get("/servers/alpha", () => HttpResponse.json(makeServer())),
    );
    renderWithQuery(<ServerDetailPage />);
    const logsTab = await screen.findByRole("button", { name: /Logs/i });
    await userEvent.click(logsTab);
    await waitFor(() => expect(screen.getByText("logs-tab")).toBeInTheDocument());
  });

  it("loads Console as a lazy chunk", async () => {
    server.use(
      http.get("/servers/alpha", () => HttpResponse.json(makeServer())),
    );
    renderWithQuery(<ServerDetailPage />);
    // Two "console"-named buttons exist: the "Open console" header
    // action and the "Console" tab. Match the tab via its exact label.
    const consoleTab = await screen.findByRole("button", { name: "Console" });
    await userEvent.click(consoleTab);
    await waitFor(() => expect(screen.getByText("console-tab")).toBeInTheDocument());
  });

  it("loads Files as a lazy chunk", async () => {
    server.use(
      http.get("/servers/alpha", () => HttpResponse.json(makeServer())),
    );
    renderWithQuery(<ServerDetailPage />);
    const filesTab = await screen.findByRole("button", { name: /Files/i });
    await userEvent.click(filesTab);
    await waitFor(() => expect(screen.getByText("files-tab")).toBeInTheDocument());
  });
});

describe("ServerDetailPage dynamic tabs", () => {
  it("hides the Console tab and button when the template has no console", async () => {
    server.use(
      http.get("/servers/alpha", () => HttpResponse.json(makeServer())),
      http.get("/templates/:name", ({ params }) =>
        HttpResponse.json(
          makeTemplate({
            metadata: { name: String(params.name) },
            spec: { consoleMode: "none", rcon: { protocol: "none" } },
          }),
        ),
      ),
    );
    renderWithQuery(<ServerDetailPage />);
    await screen.findByRole("heading", { level: 1, name: "alpha" });
    await waitFor(() =>
      expect(screen.queryByRole("button", { name: "Console" })).not.toBeInTheDocument(),
    );
    expect(screen.queryByRole("button", { name: /Open console/i })).not.toBeInTheDocument();
  });

  it("hides the Mods tab unless the template declares the capability", async () => {
    server.use(
      http.get("/servers/alpha", () => HttpResponse.json(makeServer())),
      // default template has no capabilities.mods
    );
    renderWithQuery(<ServerDetailPage />);
    await screen.findByRole("button", { name: "Console" });
    await waitFor(() =>
      expect(screen.queryByRole("button", { name: "Mods" })).not.toBeInTheDocument(),
    );
  });

  it("shows the Mods tab when the template declares mods", async () => {
    server.use(
      http.get("/servers/alpha", () => HttpResponse.json(makeServer())),
      http.get("/templates/:name", ({ params }) =>
        HttpResponse.json(
          makeTemplate({
            metadata: { name: String(params.name) },
            spec: { capabilities: { mods: { path: "mods" } } },
          }),
        ),
      ),
    );
    renderWithQuery(<ServerDetailPage />);
    const modsTab = await screen.findByRole("button", { name: "Mods" });
    await userEvent.click(modsTab);
    await waitFor(() => expect(screen.getByText("mods-tab")).toBeInTheDocument());
  });

  it("hides the Logs tab when the template logs only to stdout", async () => {
    server.use(
      http.get("/servers/alpha", () => HttpResponse.json(makeServer())),
      http.get("/templates/:name", ({ params }) =>
        HttpResponse.json(
          makeTemplate({
            metadata: { name: String(params.name) },
            spec: { logPath: undefined },
          }),
        ),
      ),
    );
    renderWithQuery(<ServerDetailPage />);
    // Console stays (rcon), but Logs is gone without a logPath.
    await screen.findByRole("button", { name: "Console" });
    await waitFor(() =>
      expect(screen.queryByRole("button", { name: /^Logs$/i })).not.toBeInTheDocument(),
    );
  });
});

describe("ServerDetailPage clone action", () => {
  async function openMenu(user: ReturnType<typeof userEvent.setup>) {
    server.use(
      http.get("/servers/alpha", () => HttpResponse.json(makeServer({ metadata: { name: "alpha" } }))),
    );
    renderWithQuery(<ServerDetailPage />);
    await user.click(await screen.findByRole("button", { name: "More actions" }));
  }

  it("shows Clone server in the More menu", async () => {
    const user = userEvent.setup();
    await openMenu(user);
    expect(screen.getByText("Clone server")).toBeInTheDocument();
  });

  it("disables Clone server for viewers", async () => {
    server.use(
      http.get("/users/me", () => HttpResponse.json(makeUser({ role: "viewer" }))),
    );
    const user = userEvent.setup();
    await openMenu(user);
    const item = screen.getByText("Clone server").closest("[role='menuitem']");
    await waitFor(() => expect(item).toHaveAttribute("aria-disabled", "true"));
    expect(item).toHaveAttribute("title", "Requires operator role");
  });

  it("opens the dialog prefilled and validates the name", async () => {
    const user = userEvent.setup();
    await openMenu(user);
    await user.click(screen.getByText("Clone server"));
    const input = await screen.findByLabelText("New name");
    expect(input).toHaveValue("alpha-copy");

    await user.clear(input);
    await user.type(input, "Bad_Name");
    expect(
      screen.getByText("Name must be lowercase letters, digits, dashes (max 63)"),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Clone server" })).toBeDisabled();
  });

  it("clones and navigates to the new server", async () => {
    const user = userEvent.setup();
    await openMenu(user);
    await user.click(screen.getByText("Clone server"));
    await screen.findByLabelText("New name");
    await user.click(screen.getByRole("button", { name: "Clone server" }));
    await waitFor(() =>
      expect(navigate).toHaveBeenCalledWith({
        to: "/servers/$name",
        params: { name: "alpha-copy" },
      }),
    );
    expect(screen.queryByLabelText("New name")).not.toBeInTheDocument();
  });

  it("surfaces a 409 conflict and keeps the dialog open", async () => {
    server.use(
      http.post(/\/servers\/[^/]+:clone$/, () =>
        new HttpResponse("already exists", { status: 409 }),
      ),
    );
    const user = userEvent.setup();
    await openMenu(user);
    await user.click(screen.getByText("Clone server"));
    await screen.findByLabelText("New name");
    await user.click(screen.getByRole("button", { name: "Clone server" }));
    expect(
      await screen.findByText("A server named alpha-copy already exists."),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("New name")).toBeInTheDocument();
  });
});
