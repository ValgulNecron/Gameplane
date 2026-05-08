import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { render, screen, fireEvent } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AuditLogPage } from "./AuditLog";
import type { AuditEvent } from "@/types";

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

function event(id: number, partial: Partial<AuditEvent> = {}): AuditEvent {
  return {
    id,
    ts: "2026-05-03T12:00:00Z",
    actor: "alice",
    method: "POST",
    path: "/servers/mc",
    status: 200,
    ip: "10.0.0.1",
    ...partial,
  };
}

function jsonRes(body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  });
}

describe("AuditLogPage", () => {
  it("renders rows from the first page", async () => {
    fetchMock.mockResolvedValue(jsonRes([
      event(1, { actor: "alice", method: "POST", path: "/servers/mc:start" }),
      event(2, { actor: "bob",   method: "DELETE", path: "/servers/old", status: 500 }),
    ]));

    render(withClient(<AuditLogPage />));
    expect(await screen.findByText("/servers/mc:start")).toBeInTheDocument();
    expect(screen.getByText("/servers/old")).toBeInTheDocument();
    expect(screen.getByText("500")).toBeInTheDocument();
  });

  it("filters rows by status class", async () => {
    fetchMock.mockResolvedValue(jsonRes([
      event(1, { status: 200, path: "/ok" }),
      event(2, { status: 500, path: "/boom" }),
    ]));

    render(withClient(<AuditLogPage />));
    expect(await screen.findByText("/ok")).toBeInTheDocument();
    expect(screen.getByText("/boom")).toBeInTheDocument();

    fireEvent.click(screen.getByText(/5xx · 1/));
    expect(screen.getByText("/boom")).toBeInTheDocument();
    expect(screen.queryByText("/ok")).toBeNull();
  });

  it("shows an empty state when no events match", async () => {
    fetchMock.mockResolvedValue(jsonRes([
      event(1, { status: 200 }),
    ]));
    render(withClient(<AuditLogPage />));
    await screen.findByText(/loaded/);
    fireEvent.click(screen.getByText(/4xx · 0/));
    expect(screen.getByText("No events match the active filters.")).toBeInTheDocument();
  });
});
