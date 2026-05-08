import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { FilesTab } from "./Files";

type FetchInit = Parameters<typeof fetch>[1];

// Monaco can't render in jsdom — replace with a controlled textarea so
// onChange fires and the editor value can be inspected/manipulated.
vi.mock("@monaco-editor/react", () => ({
  default: ({
    value,
    onChange,
  }: {
    value: string;
    onChange?: (v: string | undefined) => void;
  }) => (
    <textarea
      data-testid="monaco"
      value={value}
      onChange={(e) => onChange?.(e.target.value)}
    />
  ),
}));

const fetchMock = vi.fn();

beforeEach(() => {
  vi.stubGlobal("fetch", fetchMock);
});
afterEach(() => {
  fetchMock.mockReset();
  vi.unstubAllGlobals();
});

function withClient(ui: ReactNode) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={qc}>{ui}</QueryClientProvider>;
}

function jsonRes(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function textRes(body: string, status = 200): Response {
  return new Response(body, {
    status,
    headers: { "Content-Type": "text/plain" },
  });
}

const ROOT_ENTRIES = [
  { name: "config", path: "/config", size: 0, dir: true },
  { name: "server.properties", path: "/server.properties", size: 2100, dir: false },
];

describe("FilesTab", () => {
  it("renders entries from /files/list", async () => {
    fetchMock.mockImplementation(async (url: string) => {
      if (url === "/servers/mc-survival/files/list?path=%2F") {
        return jsonRes(ROOT_ENTRIES);
      }
      throw new Error(`unexpected fetch: ${url}`);
    });
    render(withClient(<FilesTab name="mc-survival" />));
    await waitFor(() => {
      expect(screen.getByText("config")).toBeInTheDocument();
      expect(screen.getByText("server.properties")).toBeInTheDocument();
    });
  });

  it("save button is disabled until the user edits, then POSTs to /files/write", async () => {
    let writeBody: string | null = null;
    fetchMock.mockImplementation(async (url: string, init?: FetchInit) => {
      if (url.startsWith("/servers/mc-survival/files/list")) return jsonRes(ROOT_ENTRIES);
      if (url.startsWith("/servers/mc-survival/files/read")) {
        return textRes("enable-jmx-monitoring=false\n");
      }
      if (
        url === "/servers/mc-survival/files/write?path=%2Fserver.properties" &&
        init?.method === "POST"
      ) {
        writeBody = init.body as string;
        return new Response(null, { status: 204 });
      }
      throw new Error(`unexpected fetch: ${url} ${init?.method ?? "GET"}`);
    });

    render(withClient(<FilesTab name="mc-survival" />));

    fireEvent.click(await screen.findByText("server.properties"));

    const monaco = await screen.findByTestId("monaco");
    const save = screen.getByRole("button", { name: /Save/ });
    expect(save).toBeDisabled();

    fireEvent.change(monaco, { target: { value: "enable-jmx-monitoring=true\n" } });
    expect(screen.getByText(/modified/)).toBeInTheDocument();
    expect(save).toBeEnabled();

    await act(async () => {
      fireEvent.click(save);
    });
    await waitFor(() => expect(writeBody).toBe("enable-jmx-monitoring=true\n"));
    // After a successful save the dirty marker should clear.
    await waitFor(() => expect(screen.queryByText(/modified/)).toBeNull());
  });

  it("opens the destructive confirm dialog when Delete is pressed", async () => {
    fetchMock.mockImplementation(async (url: string) => {
      if (url.startsWith("/servers/mc-survival/files/list")) return jsonRes(ROOT_ENTRIES);
      if (url.startsWith("/servers/mc-survival/files/read")) return textRes("hello");
      throw new Error(`unexpected fetch: ${url}`);
    });
    render(withClient(<FilesTab name="mc-survival" />));
    fireEvent.click(await screen.findByText("server.properties"));
    await screen.findByTestId("monaco");

    fireEvent.click(screen.getByRole("button", { name: /^Delete$/ }));
    expect(
      await screen.findByText(/Delete server\.properties\?/),
    ).toBeInTheDocument();
  });

  it("creates a new folder via /files/mkdir", async () => {
    let mkdirURL: string | null = null;
    fetchMock.mockImplementation(async (url: string, init?: FetchInit) => {
      if (url.startsWith("/servers/mc-survival/files/list")) return jsonRes(ROOT_ENTRIES);
      if (url.startsWith("/servers/mc-survival/files/mkdir") && init?.method === "POST") {
        mkdirURL = url;
        return new Response(null, { status: 204 });
      }
      throw new Error(`unexpected fetch: ${url} ${init?.method ?? "GET"}`);
    });
    render(withClient(<FilesTab name="mc-survival" />));
    await screen.findByText("server.properties");

    fireEvent.click(screen.getByRole("button", { name: /New folder/ }));
    const input = await screen.findByPlaceholderText("my-folder");
    fireEvent.change(input, { target: { value: "mods-disabled" } });
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: /^Create$/ }));
    });
    await waitFor(() =>
      expect(mkdirURL).toBe(
        "/servers/mc-survival/files/mkdir?path=%2Fmods-disabled",
      ),
    );
  });
});
