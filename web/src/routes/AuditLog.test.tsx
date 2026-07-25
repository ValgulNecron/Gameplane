import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AuditLogPage, auditAction } from "./AuditLog";
import type { AuditEvent, AuditVerifyResult } from "@/types";

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
  // Return a fresh Response each time so body stream can be read multiple times
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  });
}

describe("AuditLogPage", () => {
  it("renders rows from the first page", async () => {
    const verifyResult: AuditVerifyResult = { ok: true, checked: 100, message: "audit chain intact" };
    const events = [
      event(1, { actor: "alice", method: "POST", path: "/servers/mc:start" }),
      event(2, { actor: "bob",   method: "DELETE", path: "/servers/old", status: 500 }),
    ];

    fetchMock.mockImplementation((url) => {
      const u = url.toString();
      if (u.includes("/admin/audit/verify")) return Promise.resolve(jsonRes(verifyResult));
      return Promise.resolve(jsonRes(events));
    });

    render(withClient(<AuditLogPage />));
    // Raw HTTP is mapped to human-readable actions + allow/deny outcomes.
    expect(await screen.findByText("Started server")).toBeInTheDocument();
    expect(screen.getByText("Deleted server")).toBeInTheDocument();
    expect(screen.getByText("Error")).toBeInTheDocument(); // status 500
  });

  it("filters rows by status class", async () => {
    const verifyResult: AuditVerifyResult = { ok: true, checked: 100, message: "audit chain intact" };
    const events = [
      event(1, { status: 200, method: "POST", path: "/backups" }),
      event(2, { status: 500, method: "DELETE", path: "/servers/x" }),
    ];

    fetchMock.mockImplementation((url) => {
      const u = url.toString();
      if (u.includes("/admin/audit/verify")) return Promise.resolve(jsonRes(verifyResult));
      return Promise.resolve(jsonRes(events));
    });

    const user = userEvent.setup();
    render(withClient(<AuditLogPage />));
    expect(await screen.findByText("Created backup")).toBeInTheDocument();
    expect(screen.getByText("Deleted server")).toBeInTheDocument();

    await user.click(screen.getByText(/5xx · 1/));
    expect(screen.getByText("Deleted server")).toBeInTheDocument();
    expect(screen.queryByText("Created backup")).toBeNull();
  });

  it("shows an empty state when no events match", async () => {
    const verifyResult: AuditVerifyResult = { ok: true, checked: 100, message: "audit chain intact" };
    const events = [event(1, { status: 200 })];

    fetchMock.mockImplementation((url) => {
      const u = url.toString();
      if (u.includes("/admin/audit/verify")) return Promise.resolve(jsonRes(verifyResult));
      return Promise.resolve(jsonRes(events));
    });

    const user = userEvent.setup();
    render(withClient(<AuditLogPage />));
    await screen.findByText(/loaded/);
    await user.click(screen.getByText(/4xx · 0/));
    expect(screen.getByText("No events match the active filters.")).toBeInTheDocument();
  });

  it("exports CSV with applied filters and triggers a browser download", async () => {
    const csvText = "id,ts,actor\n1,2026-05-03T12:00:00Z,alice";

    const verifyResult: AuditVerifyResult = { ok: true, checked: 100, message: "audit chain intact" };
    const events = [event(1, { actor: "alice", method: "POST" })];

    fetchMock.mockImplementation((url) => {
      const u = url.toString();
      if (u.includes("/admin/audit/verify")) return Promise.resolve(jsonRes(verifyResult));
      if (u.includes("/admin/audit/export")) {
        // Body is a string, not a `Blob` instance: jsdom 29 ships its own
        // Blob implementation (previously jsdom deferred to Node's), and
        // it lacks `Blob.prototype.stream()`. msw's globalThis.Response
        // proxy (recordRawHeaders.ts) calls `.stream()` on a Blob body
        // while constructing the Response, so `new Response(aJsdomBlob,
        // ...)` throws "object.stream is not a function" in this
        // environment. A string body hits a different, working
        // construction path and still round-trips through `res.blob()`
        // exactly like a real network response would (Audit.exportCsv
        // calls `res.blob()` regardless of the original body shape).
        return Promise.resolve(new Response(csvText, { status: 200, headers: { "Content-Type": "text/csv" } }));
      }
      return Promise.resolve(jsonRes(events));
    });

    // The mutation's onSuccess (AuditLog.tsx) builds a real download: an
    // object URL for the fetched blob, an <a> pointed at it, a click, then
    // a revoke. Stub URL's two methods in place (rather than replacing the
    // whole URL global) so `new URL(...)` elsewhere keeps working, and spy
    // on createElement/click so we can inspect the anchor the component
    // actually built.
    const createURL = vi.fn((_blob: Blob) => "blob:test");
    const revokeURL = vi.fn();
    Object.defineProperty(URL, "createObjectURL", { configurable: true, writable: true, value: createURL });
    Object.defineProperty(URL, "revokeObjectURL", { configurable: true, writable: true, value: revokeURL });
    const createElementSpy = vi.spyOn(document, "createElement");
    const clickSpy = vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(() => undefined);

    const user = userEvent.setup();
    render(withClient(<AuditLogPage />));
    await screen.findByText("Created server");

    // Set a filter to verify it's passed to the export endpoint
    const methodSelect = screen.getByDisplayValue("All methods");
    await user.selectOptions(methodSelect, "POST");

    await user.click(screen.getByText("Export CSV"));

    // Wait for the export endpoint to be called with the filter
    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledWith(
        expect.stringContaining("/admin/audit/export?format=csv&method=POST"),
        expect.any(Object),
      );
    }, { timeout: 10000 });

    // Wait for the mutation's onSuccess to run the download sequence.
    await waitFor(() => expect(clickSpy).toHaveBeenCalledOnce());

    // Verify a real download was triggered, not just the fetch.
    expect(createElementSpy).toHaveBeenCalledWith("a");
    const anchorCallIndex = createElementSpy.mock.calls.findIndex(([tag]) => tag === "a");
    const anchor = createElementSpy.mock.results[anchorCallIndex]?.value as HTMLAnchorElement;
    // Not `expect.any(Blob)`: the object passed to createObjectURL comes
    // from the Fetch implementation's internal `res.blob()`, which is a
    // different Blob constructor/realm than jsdom's global `Blob` used
    // here — `instanceof Blob` reliably fails even though it is a
    // functioning Blob-like object. Assert on the structural properties
    // that matter instead (content-type carried through from the mocked
    // response) plus call-was-made, matching (and exceeding) what the
    // pre-migration test asserted.
    expect(createURL).toHaveBeenCalledOnce();
    expect(createURL.mock.calls[0][0]).toMatchObject({ type: "text/csv" });
    expect(anchor.href).toBe("blob:test");
    expect(anchor.download).toMatch(/^gameplane-audit-.*\.csv$/);
    expect(revokeURL).toHaveBeenCalledWith("blob:test");

    createElementSpy.mockRestore();
    clickSpy.mockRestore();
  });

  it("renders the integrity banner when chain is verified", async () => {
    const verifyResult: AuditVerifyResult = { ok: true, checked: 150, message: "audit chain intact" };
    const events = [event(1)];

    fetchMock.mockImplementation((url) => {
      const u = url.toString();
      if (u.includes("/admin/audit/verify")) return Promise.resolve(jsonRes(verifyResult));
      return Promise.resolve(jsonRes(events));
    });

    render(withClient(<AuditLogPage />));
    expect(await screen.findByText("Audit chain verified — no tampering detected")).toBeInTheDocument();
    expect(screen.getByText("Re-check")).toBeInTheDocument();
  });

  it("renders the failure banner when chain is broken", async () => {
    const verifyResult: AuditVerifyResult = {
      ok: false,
      firstBadId: 42,
      checked: 50,
      message: "row 42: prev_hash does not match the previous row's hash (a row was inserted, deleted, or reordered)",
    };
    const events = [event(1)];

    fetchMock.mockImplementation((url) => {
      const u = url.toString();
      if (u.includes("/admin/audit/verify")) return Promise.resolve(jsonRes(verifyResult));
      return Promise.resolve(jsonRes(events));
    });

    render(withClient(<AuditLogPage />));
    expect(await screen.findByText(verifyResult.message)).toBeInTheDocument();
    expect(screen.getByText("Re-check")).toBeInTheDocument();
  });

  it("refetches the verify query when Re-check is clicked", async () => {
    const verifyResult: AuditVerifyResult = { ok: true, checked: 100, message: "audit chain intact" };
    const events = [event(1)];

    fetchMock.mockImplementation((url) => {
      const u = url.toString();
      if (u.includes("/admin/audit/verify")) return Promise.resolve(jsonRes(verifyResult));
      return Promise.resolve(jsonRes(events));
    });

    const user = userEvent.setup();
    render(withClient(<AuditLogPage />));
    await screen.findByText("Audit chain verified — no tampering detected");

    await user.click(screen.getByText("Re-check"));

    // Verify the verify endpoint was called at least twice (initial mount + re-check click)
    await waitFor(() => {
      const verifyCalls = fetchMock.mock.calls.filter((call) =>
        call[0]?.toString().includes("/admin/audit/verify")
      );
      expect(verifyCalls.length).toBeGreaterThanOrEqual(2);
    });
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
