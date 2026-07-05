import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { AdminLogsPage, capBuffer } from "./AdminLogs";

const fetchMock = vi.fn();

beforeEach(() => {
  vi.stubGlobal("fetch", fetchMock);
});
afterEach(() => {
  fetchMock.mockReset();
  vi.unstubAllGlobals();
});

// Each call needs a fresh Response: a body stream can only be read once.
function logRes(body: string, pod: string): Response {
  return new Response(body, {
    status: 200,
    headers: {
      "Content-Type": "text/plain; charset=utf-8",
      "X-Gameplane-Pod": pod,
    },
  });
}

function calledURLs(): string[] {
  return fetchMock.mock.calls.map((c) => String(c[0]));
}

describe("AdminLogsPage", () => {
  it("renders controls and streams the api log by default", async () => {
    fetchMock.mockImplementation(() =>
      Promise.resolve(logRes("api line one\napi line two\n", "gameplane-api-0")),
    );
    render(<AdminLogsPage />);

    expect(screen.getByText("API server")).toBeInTheDocument();
    expect(screen.getByText("Operator")).toBeInTheDocument();
    expect(screen.getByRole("switch", { name: "Follow" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /download/i })).toBeInTheDocument();

    expect(await screen.findByText(/api line one/)).toBeInTheDocument();
    expect(screen.getByText("gameplane-api-0")).toBeInTheDocument();

    const url = calledURLs()[0];
    expect(url).toContain("/admin/system-logs/api?");
    expect(url).toContain("tailLines=500");
    expect(url).toContain("follow=false");
  });

  it("switching component starts a new stream", async () => {
    fetchMock.mockImplementation((input: RequestInfo | URL) =>
      Promise.resolve(
        String(input).includes("/operator")
          ? logRes("operator says hi\n", "gameplane-operator-0")
          : logRes("api says hi\n", "gameplane-api-0"),
      ),
    );
    render(<AdminLogsPage />);
    await screen.findByText(/api says hi/);

    fireEvent.click(screen.getByText("Operator"));

    expect(await screen.findByText(/operator says hi/)).toBeInTheDocument();
    expect(screen.getByText("gameplane-operator-0")).toBeInTheDocument();
    expect(calledURLs().some((u) => u.includes("/admin/system-logs/operator?"))).toBe(true);
    // The old scrollback is cleared on switch.
    expect(screen.queryByText(/api says hi/)).toBeNull();
  });

  it("changing tail refetches with the new tailLines", async () => {
    fetchMock.mockImplementation(() =>
      Promise.resolve(logRes("some output\n", "gameplane-api-0")),
    );
    render(<AdminLogsPage />);
    await screen.findByText(/some output/);

    fireEvent.change(screen.getByRole("combobox", { name: "Tail lines" }), {
      target: { value: "1000" },
    });

    await waitFor(() =>
      expect(calledURLs().some((u) => u.includes("tailLines=1000"))).toBe(true),
    );
  });

  it("toggling follow reconnects with follow=true", async () => {
    fetchMock.mockImplementation(() =>
      Promise.resolve(logRes("streamed line\n", "gameplane-api-0")),
    );
    render(<AdminLogsPage />);
    await screen.findByText(/streamed line/);

    fireEvent.click(screen.getByRole("switch", { name: "Follow" }));

    await waitFor(() =>
      expect(calledURLs().some((u) => u.includes("follow=true"))).toBe(true),
    );
  });

  it("shows an error when the stream fails", async () => {
    fetchMock.mockImplementation(() =>
      Promise.resolve(new Response("no pods found", { status: 404 })),
    );
    render(<AdminLogsPage />);
    expect(await screen.findByText(/404: no pods found/)).toBeInTheDocument();
    expect(screen.getByText("No output.")).toBeInTheDocument();
  });

  it("downloads the current tail as a .log file", async () => {
    const createURL = vi.fn(() => "blob:test");
    const revokeURL = vi.fn();
    Object.defineProperty(URL, "createObjectURL", {
      configurable: true, writable: true, value: createURL,
    });
    Object.defineProperty(URL, "revokeObjectURL", {
      configurable: true, writable: true, value: revokeURL,
    });
    const clickSpy = vi
      .spyOn(HTMLAnchorElement.prototype, "click")
      .mockImplementation(() => undefined);

    fetchMock.mockImplementation(() =>
      Promise.resolve(logRes("dump line\n", "gameplane-api-0")),
    );
    render(<AdminLogsPage />);
    await screen.findByText(/dump line/);

    fireEvent.click(screen.getByRole("button", { name: /download/i }));

    await waitFor(() => expect(createURL).toHaveBeenCalledOnce());
    expect(clickSpy).toHaveBeenCalledOnce();
    expect(revokeURL).toHaveBeenCalledWith("blob:test");
    // The download re-fetches without follow, pinned to the streamed pod.
    const url = calledURLs()[fetchMock.mock.calls.length - 1];
    expect(url).toContain("follow=false");
    expect(url).toContain("pod=gameplane-api-0");
    clickSpy.mockRestore();
  });
});

describe("capBuffer", () => {
  it("returns strings under the cap unchanged", () => {
    expect(capBuffer("abc", 10)).toBe("abc");
  });

  it("trims the head to the cap at a line boundary", () => {
    expect(capBuffer("aaa\nbbb\nccc", 8)).toBe("bbb\nccc");
  });

  it("falls back to a plain cut when no newline is in range", () => {
    expect(capBuffer("abcdefghij", 4)).toBe("ghij");
  });
});
