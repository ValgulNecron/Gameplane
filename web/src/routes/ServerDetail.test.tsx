import { describe, it, expect, vi } from "vitest";
import type { ReactNode } from "react";
import { http, HttpResponse } from "msw";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { server } from "@/test/server";
import { renderWithQuery } from "@/test/render";
import { makeServer } from "@/test/factories";

// Router APIs the route reaches into.
vi.mock("@tanstack/react-router", () => ({
  Link: ({ children, to, ...rest }: { children: ReactNode; to: string } & Record<string, unknown>) => (
    <a href={to} {...rest}>{children}</a>
  ),
  useParams: () => ({ name: "alpha" }),
}));

// Heavy lazy tabs (xterm / Monaco) — replace with stubs so jsdom doesn't
// have to evaluate them.
vi.mock("./tabs/Console", () => ({ ConsoleTab: () => <div>console-tab</div> }));
vi.mock("./tabs/Files", () => ({ FilesTab: () => <div>files-tab</div> }));
vi.mock("./tabs/Logs", () => ({ LogsTab: () => <div>logs-tab</div> }));
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
