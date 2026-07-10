import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { render, screen, fireEvent } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AuditLogPage, auditAction } from "./AuditLog";
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
    // Raw HTTP is mapped to human-readable actions + allow/deny outcomes.
    expect(await screen.findByText("Started server")).toBeInTheDocument();
    expect(screen.getByText("Deleted server")).toBeInTheDocument();
    expect(screen.getByText("Error")).toBeInTheDocument(); // status 500
  });

  it("filters rows by status class", async () => {
    fetchMock.mockResolvedValue(jsonRes([
      event(1, { status: 200, method: "POST", path: "/backups" }),
      event(2, { status: 500, method: "DELETE", path: "/servers/x" }),
    ]));

    render(withClient(<AuditLogPage />));
    expect(await screen.findByText("Created backup")).toBeInTheDocument();
    expect(screen.getByText("Deleted server")).toBeInTheDocument();

    fireEvent.click(screen.getByText(/5xx · 1/));
    expect(screen.getByText("Deleted server")).toBeInTheDocument();
    expect(screen.queryByText("Created backup")).toBeNull();
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

  it("exports CSV with applied filters", async () => {
    const csvBlob = new Blob(["id,ts,actor\n1,2026-05-03T12:00:00Z,alice"], { type: "text/csv" });
    const urlCreateObjectURLMock = vi.fn(() => "blob:http://localhost/test");
    const urlRevokeObjectURLMock = vi.fn();
    vi.stubGlobal("URL", { createObjectURL: urlCreateObjectURLMock, revokeObjectURL: urlRevokeObjectURLMock });

    const createElementMock = vi.spyOn(document, "createElement");

    fetchMock
      .mockResolvedValueOnce(jsonRes([event(1, { actor: "alice", method: "POST" })]))
      .mockResolvedValueOnce(new Response(csvBlob)); // export endpoint

    render(withClient(<AuditLogPage />));
    await screen.findByText("Created server");

    // Set a filter to verify it's passed to the export endpoint
    const methodSelect = screen.getByDisplayValue("All methods");
    fireEvent.change(methodSelect, { target: { value: "POST" } });

    fireEvent.click(screen.getByText("Export CSV"));

    // Wait for mutation to complete
    await new Promise((r) => setTimeout(r, 0));

    // Verify the export endpoint was called with the filter
    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringContaining("/admin/audit/export?format=csv&method=POST"),
      expect.any(Object),
    );

    // Verify download was triggered
    expect(createElementMock).toHaveBeenCalledWith("a");
    expect(urlCreateObjectURLMock).toHaveBeenCalled();
    expect(urlRevokeObjectURLMock).toHaveBeenCalledWith("blob:http://localhost/test");
  });
});

describe("auditAction", () => {
  const cases: Array<[Partial<AuditEvent>, string]> = [
    [{ method: "POST", path: "/api/v1/servers", target: "alpha" }, "Created server alpha"],
    [{ method: "POST", path: "/servers/mc:start" }, "Started server"],
    [{ method: "POST", path: "/servers/mc:wipe-data" }, "Wiped data on server"],
    [{ method: "DELETE", path: "/servers/old", target: "old" }, "Deleted server old"],
    [{ method: "POST", path: "/restores" }, "Restored backup"],
    [{ method: "POST", path: "/backups" }, "Created backup"],
    [{ method: "PUT", path: "/admin/config" }, "Updated settings"],
    [{ method: "POST", path: "/auth/login" }, "Signed in"],
    [{ method: "DELETE", path: "/users/3", target: "bob" }, "Deleted user bob"],
    [{ method: "GET", path: "/widgets" }, "Viewed widgets"],
  ];
  it.each(cases)("maps %o -> %s", (partial, expected) => {
    expect(auditAction({ method: "GET", path: "/", ...partial })).toBe(expected);
  });
});
