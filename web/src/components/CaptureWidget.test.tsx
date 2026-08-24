import { afterEach, describe, it, expect, onTestFinished, vi } from "vitest";
import type { ReactNode } from "react";
import { http, HttpResponse } from "msw";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { server } from "@/test/server";
import { renderWithQuery } from "@/test/render";
import { makeServer, makeCapture } from "@/test/factories";
import type { CaptureStartBody } from "@/lib/api";
import { CaptureWidget } from "./CaptureWidget";

// Mock TanStack Router to avoid import errors in tests
vi.mock("@tanstack/react-router", () => ({
  Link: ({ children, to, ...rest }: { children: ReactNode; to: string } & Record<string, unknown>) => (
    <a href={to} {...rest}>{children}</a>
  ),
}));

afterEach(() => {
  server.resetHandlers();
});

describe("CaptureWidget", () => {
  describe("disabled state", () => {
    it("shows the disabled banner when capture is not enabled on the server", () => {
      const gs = makeServer({
        spec: { capture: { enabled: false } },
      });
      renderWithQuery(<CaptureWidget name="alpha" ns="gameplane-games" gs={gs} />);

      expect(screen.getByText(/Capture is not enabled on this server/i)).toBeInTheDocument();
      expect(screen.getByText(/Network packet capture is an optional feature/i)).toBeInTheDocument();
    });

    it("renders the Enable Capture button in disabled state", () => {
      const gs = makeServer({
        spec: { capture: { enabled: false } },
      });
      renderWithQuery(<CaptureWidget name="alpha" ns="gameplane-games" gs={gs} />);

      const button = screen.getByRole("button", { name: /Enable Capture/i });
      expect(button).toBeInTheDocument();
      expect(button).not.toBeDisabled();
    });

    it("calls enable mutation when Enable Capture is clicked", async () => {
      const gs = makeServer({
        spec: { capture: { enabled: false } },
      });
      // The response is gated on the test, not on a timer. An
      // instantly-resolving handler can settle inside the act() that
      // userEvent.click awaits, so the pending render is already gone when
      // the assertion runs; a fixed delay only narrows that window (a
      // loaded CI worker can still overrun it). Holding the response until
      // the test releases it makes the pending state deterministic, and
      // releasing it afterwards still exercises the success path.
      let releaseEnable!: () => void;
      const enableGate = new Promise<void>((resolve) => {
        releaseEnable = resolve;
      });
      server.use(
        http.post(/servers\/alpha:capture-enable(\?.*)?$/, async () => {
          await enableGate;
          return HttpResponse.json({ name: "alpha", status: { capture: { enabled: true } } });
        }),
      );

      renderWithQuery(<CaptureWidget name="alpha" ns="gameplane-games" gs={gs} />);

      const button = screen.getByRole("button", { name: /Enable Capture/i });
      await userEvent.click(button);

      // Button shows the loading state while the gated request is in
      // flight. Safe to assert synchronously: mutate() flips isPending
      // inside the act() that userEvent.click awaits, and nothing can
      // settle it back before the gate opens.
      expect(button).toHaveTextContent(/Enabling/i);

      // Release, and confirm the mutation settles back out of its pending
      // state — which also runs the mutation's onSuccess.
      releaseEnable();
      await waitFor(() => expect(button).toHaveTextContent(/Enable Capture/i));
    });

    it("displays error banner when enable mutation fails", async () => {
      const gs = makeServer({
        spec: { capture: { enabled: false } },
      });
      server.use(
        http.post(/servers\/alpha:capture-enable(\?.*)?$/, () =>
          new HttpResponse("permission denied", { status: 403 }),
        ),
      );

      renderWithQuery(<CaptureWidget name="alpha" ns="gameplane-games" gs={gs} />);

      const button = screen.getByRole("button", { name: /Enable Capture/i });
      await userEvent.click(button);

      await waitFor(() => {
        expect(screen.getByText(/permission denied/i)).toBeInTheDocument();
      });
    });
  });

  describe("enabled state", () => {
    it("renders the caution banner by default", () => {
      const gs = makeServer({
        spec: { capture: { enabled: true } },
      });
      renderWithQuery(<CaptureWidget name="alpha" ns="gameplane-games" gs={gs} />);

      expect(screen.getByText(/Caution: network packet captures contain real player data/i)).toBeInTheDocument();
      expect(screen.getByText(/Player IP addresses and port numbers/i)).toBeInTheDocument();
    });

    it("allows dismissing the caution banner", async () => {
      const gs = makeServer({
        spec: { capture: { enabled: true } },
      });
      renderWithQuery(<CaptureWidget name="alpha" ns="gameplane-games" gs={gs} />);

      const dismissBtn = screen.getByRole("button", { name: /Dismiss/i });
      await userEvent.click(dismissBtn);

      expect(screen.queryByText(/Caution: network packet captures contain real player data/i)).not.toBeInTheDocument();
    });

    it("shows Ready status when no active capture", () => {
      const gs = makeServer({
        spec: { capture: { enabled: true, retentionSeconds: 86400 } },
      });
      server.use(
        http.get(/servers\/alpha:captures(\?.*)?$/, () =>
          HttpResponse.json({ captures: [], total: 0, limit: 100, offset: 0 }),
        ),
      );

      renderWithQuery(<CaptureWidget name="alpha" ns="gameplane-games" gs={gs} />);

      expect(screen.getByText(/Ready/i)).toBeInTheDocument();
      expect(screen.getByText(/Captures will auto-delete after 24 hours/i)).toBeInTheDocument();
    });

    it("shows empty state when no captures exist", async () => {
      const gs = makeServer({
        spec: { capture: { enabled: true } },
      });
      server.use(
        http.get(/servers\/alpha:captures(\?.*)?$/, () =>
          HttpResponse.json({ captures: [], total: 0, limit: 100, offset: 0 }),
        ),
      );

      renderWithQuery(<CaptureWidget name="alpha" ns="gameplane-games" gs={gs} />);

      expect(await screen.findByText(/No captures yet/i)).toBeInTheDocument();
    });

    it("renders Disable Capture and Start Capture buttons when no active capture", async () => {
      const gs = makeServer({
        spec: { capture: { enabled: true } },
      });
      server.use(
        http.get(/servers\/alpha:captures(\?.*)?$/, () =>
          HttpResponse.json({ captures: [], total: 0, limit: 100, offset: 0 }),
        ),
      );

      renderWithQuery(<CaptureWidget name="alpha" ns="gameplane-games" gs={gs} />);

      expect(await screen.findByRole("button", { name: /Disable Capture/i })).toBeInTheDocument();
      expect(screen.getByRole("button", { name: /Start Capture/i })).toBeInTheDocument();
    });

    it("disables the Disable Capture button when disable mutation is pending", async () => {
      const gs = makeServer({
        spec: { capture: { enabled: true } },
      });
      // Gated on the test rather than on a timer, for the same reason as
      // the enable test above.
      let releaseDisable!: () => void;
      const disableGate = new Promise<void>((resolve) => {
        releaseDisable = resolve;
      });
      server.use(
        http.get(/servers\/alpha:captures(\?.*)?$/, () => new Promise(() => {})), // Never resolves
        http.post(/servers\/alpha:capture-disable(\?.*)?$/, async () => {
          await disableGate;
          return HttpResponse.json({ name: "alpha", status: { capture: { enabled: false } } });
        }),
      );

      renderWithQuery(<CaptureWidget name="alpha" ns="gameplane-games" gs={gs} />);

      const disableBtn = await screen.findByRole("button", { name: /Disable Capture/i });
      await userEvent.click(disableBtn);

      expect(disableBtn).toBeDisabled();

      releaseDisable();
      await waitFor(() => expect(disableBtn).not.toBeDisabled());
    });
  });

  describe("active capture state", () => {
    it("shows Capturing… status when there is an active Running capture", async () => {
      const activeCap = makeCapture({ phase: "Running", captureId: "cap-active", startedAt: new Date(Date.now() - 5000).toISOString() });
      const gs = makeServer({
        spec: { capture: { enabled: true } },
      });
      server.use(
        http.get(/servers\/alpha:captures(\?.*)?$/, () =>
          HttpResponse.json({ captures: [activeCap], total: 1, limit: 100, offset: 0 }),
        ),
        http.get(/servers\/alpha:capture\?/, () =>
          HttpResponse.json(activeCap),
        ),
      );

      renderWithQuery(<CaptureWidget name="alpha" ns="gameplane-games" gs={gs} />);

      expect(await screen.findByText(/Capturing/i)).toBeInTheDocument();
    });

    it("displays active capture details with max duration and size meters", async () => {
      const activeCap = makeCapture({
        phase: "Running",
        captureId: "cap-active",
        maxDurationSeconds: 3600,
        maxSizeBytes: 104857600,
        bytesWritten: 10485760, // 10MB
        packetsWritten: 500,
        startedAt: new Date(Date.now() - 600000).toISOString(),
      });
      const gs = makeServer({
        spec: { capture: { enabled: true } },
      });
      server.use(
        http.get(/servers\/alpha:captures(\?.*)?$/, () =>
          HttpResponse.json({ captures: [activeCap], total: 1, limit: 100, offset: 0 }),
        ),
        http.get(/servers\/alpha:capture\?/, () =>
          HttpResponse.json(activeCap),
        ),
      );

      renderWithQuery(<CaptureWidget name="alpha" ns="gameplane-games" gs={gs} />);

      expect(await screen.findByText(/Max duration/i)).toBeInTheDocument();
      expect(screen.getByText(/Max size/i)).toBeInTheDocument();
      expect(screen.getByText(/500 packets captured/i)).toBeInTheDocument();
    });

    it("renders Stop Capture button for active captures", async () => {
      const activeCap = makeCapture({
        phase: "Running",
        captureId: "cap-active",
      });
      const gs = makeServer({
        spec: { capture: { enabled: true } },
      });
      server.use(
        http.get(/servers\/alpha:captures(\?.*)?$/, () =>
          HttpResponse.json({ captures: [activeCap], total: 1, limit: 100, offset: 0 }),
        ),
        http.get(/servers\/alpha:capture\?/, () =>
          HttpResponse.json(activeCap),
        ),
      );

      renderWithQuery(<CaptureWidget name="alpha" ns="gameplane-games" gs={gs} />);

      expect(await screen.findByRole("button", { name: /Stop Capture/i })).toBeInTheDocument();
    });

    it("calls stop mutation when Stop Capture is clicked", async () => {
      const activeCap = makeCapture({
        phase: "Running",
        captureId: "cap-active",
      });
      const gs = makeServer({
        spec: { capture: { enabled: true } },
      });
      // Gated on the test rather than on a timer, for the same reason as
      // the enable/disable tests above.
      let releaseStop!: () => void;
      const stopGate = new Promise<void>((resolve) => {
        releaseStop = resolve;
      });
      server.use(
        http.get(/servers\/alpha:captures(\?.*)?$/, () =>
          HttpResponse.json({ captures: [activeCap], total: 1, limit: 100, offset: 0 }),
        ),
        http.get(/servers\/alpha:capture\?/, () =>
          HttpResponse.json(activeCap),
        ),
        http.post(/servers\/alpha:capture-stop(\?.*)?$/, async () => {
          await stopGate;
          return HttpResponse.json({ ...activeCap, phase: "Completed", completedAt: new Date().toISOString() });
        }),
      );

      renderWithQuery(<CaptureWidget name="alpha" ns="gameplane-games" gs={gs} />);

      const stopBtn = await screen.findByRole("button", { name: /Stop Capture/i });
      await userEvent.click(stopBtn);

      expect(stopBtn).toBeDisabled();

      releaseStop();
      await waitFor(() => expect(stopBtn).not.toBeDisabled());
    });
  });

  describe("capture list", () => {
    it("renders completed captures in a table", async () => {
      const completed1 = makeCapture({
        captureId: "cap-1",
        phase: "Completed",
        serverName: "alpha",
        filter: "tcp port 25565",
        bytesWritten: 2048,
        packetsWritten: 150,
      });
      const completed2 = makeCapture({
        captureId: "cap-2",
        phase: "Failed",
        serverName: "alpha",
        bytesWritten: 512,
        packetsWritten: 20,
      });
      const gs = makeServer({
        spec: { capture: { enabled: true } },
      });
      server.use(
        http.get(/servers\/alpha:captures(\?.*)?$/, () =>
          HttpResponse.json({ captures: [completed1, completed2], total: 2, limit: 100, offset: 0 }),
        ),
      );

      renderWithQuery(<CaptureWidget name="alpha" ns="gameplane-games" gs={gs} />);

      expect(await screen.findByText("cap-1")).toBeInTheDocument();
      expect(screen.getByText("cap-2")).toBeInTheDocument();
    });

    it("renders capture phase badge with correct styling", async () => {
      const completed = makeCapture({
        captureId: "cap-1",
        phase: "Completed",
      });
      const gs = makeServer({
        spec: { capture: { enabled: true } },
      });
      server.use(
        http.get(/servers\/alpha:captures(\?.*)?$/, () =>
          HttpResponse.json({ captures: [completed], total: 1, limit: 100, offset: 0 }),
        ),
      );

      renderWithQuery(<CaptureWidget name="alpha" ns="gameplane-games" gs={gs} />);

      expect(await screen.findByText("Completed")).toBeInTheDocument();
    });

    it("displays capture size and packet count formatted", async () => {
      const completed = makeCapture({
        captureId: "cap-1",
        bytesWritten: 1048576, // 1 MB
        packetsWritten: 1000,
      });
      const gs = makeServer({
        spec: { capture: { enabled: true } },
      });
      server.use(
        http.get(/servers\/alpha:captures(\?.*)?$/, () =>
          HttpResponse.json({ captures: [completed], total: 1, limit: 100, offset: 0 }),
        ),
      );

      renderWithQuery(<CaptureWidget name="alpha" ns="gameplane-games" gs={gs} />);

      expect(await screen.findByText(/1(,)?000/)).toBeInTheDocument(); // Formatted packet count
    });

    it("enables download button only for Completed captures", async () => {
      const completed = makeCapture({
        captureId: "cap-1",
        phase: "Completed",
      });
      const failed = makeCapture({
        captureId: "cap-2",
        phase: "Failed",
      });
      const gs = makeServer({
        spec: { capture: { enabled: true } },
      });
      server.use(
        http.get(/servers\/alpha:captures(\?.*)?$/, () =>
          HttpResponse.json({ captures: [completed, failed], total: 2, limit: 100, offset: 0 }),
        ),
      );

      renderWithQuery(<CaptureWidget name="alpha" ns="gameplane-games" gs={gs} />);

      const buttons = await screen.findAllByLabelText(/Download capture/i);
      expect(buttons.length).toBe(2);
      // First button (Completed) should be enabled
      expect(buttons[0]).not.toBeDisabled();
      // Second button (Failed) should be disabled
      expect(buttons[1]).toBeDisabled();
    });

    it("downloads capture file on button click", async () => {
      const completed = makeCapture({
        captureId: "cap-1",
        phase: "Completed",
      });
      const gs = makeServer({
        spec: { capture: { enabled: true } },
      });
      // The response is gated on the test rather than on a timer: this test
      // asserts the in-flight (disabled) state AND the completed download,
      // so it needs the request to stay pending until the first assertion
      // has run and then complete on demand. A fixed delay would only make
      // the "still pending" window probable; the gate makes it certain.
      let releaseFile!: () => void;
      const fileGate = new Promise<void>((resolve) => {
        releaseFile = resolve;
      });
      server.use(
        http.get(/servers\/alpha:captures(\?.*)?$/, () =>
          HttpResponse.json({ captures: [completed], total: 1, limit: 100, offset: 0 }),
        ),
        http.get(/servers\/alpha:capture-file\?/, async () => {
          await fileGate;
          return new HttpResponse(new Blob(["mock pcapng data"]), {
            headers: { "Content-Type": "application/octet-stream" },
          });
        }),
      );

      // jsdom doesn't implement URL.createObjectURL/revokeObjectURL, and
      // the mutation's onSuccess (CaptureWidget.tsx) calls both to build the
      // download link — stub them so that handler doesn't throw once the
      // mocked fetch resolves (same pattern as AuditLog.test.tsx's export).
      const createURL = vi.fn((_blob: Blob) => "blob:test");
      const revokeURL = vi.fn();
      Object.defineProperty(URL, "createObjectURL", { configurable: true, writable: true, value: createURL });
      Object.defineProperty(URL, "revokeObjectURL", { configurable: true, writable: true, value: revokeURL });
      const clickSpy = vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(() => undefined);
      // Registered immediately, and via onTestFinished rather than at the end
      // of the body, so the globals are restored even when an assertion below
      // throws — a leaked anchor-click spy or URL stub would otherwise poison
      // every later test in this file. Neither property exists on jsdom's URL
      // before this test defines it, so remove them outright rather than
      // restoring a prior descriptor.
      onTestFinished(() => {
        clickSpy.mockRestore();
        delete (URL as { createObjectURL?: unknown }).createObjectURL;
        delete (URL as { revokeObjectURL?: unknown }).revokeObjectURL;
      });

      renderWithQuery(<CaptureWidget name="alpha" ns="gameplane-games" gs={gs} />);

      const downloadBtn = await screen.findByLabelText(/Download capture cap-1/i);
      await userEvent.click(downloadBtn);

      // Button is disabled while the (still-gated) download is in flight.
      // Safe to assert synchronously: mutate() flips isPending inside the
      // act() that userEvent.click awaits, and the gate guarantees nothing
      // has settled it back.
      expect(downloadBtn).toBeDisabled();

      // Let the response through, then wait for the mutation's onSuccess to
      // run the full download sequence.
      releaseFile();
      await waitFor(() => expect(clickSpy).toHaveBeenCalledOnce());
      expect(createURL).toHaveBeenCalledOnce();
      expect(revokeURL).toHaveBeenCalledWith("blob:test");
      // Button re-enables once the mutation settles, so the test doesn't
      // leave a pending mutation behind.
      await waitFor(() => expect(downloadBtn).not.toBeDisabled());
    });

    it("expands row to show details on eye button click", async () => {
      const completed = makeCapture({
        captureId: "cap-1",
        createdAt: "2026-08-23T12:00:00Z",
        startedAt: "2026-08-23T12:05:00Z",
        serverName: "alpha",
      });
      const gs = makeServer({
        spec: { capture: { enabled: true } },
      });
      server.use(
        http.get(/servers\/alpha:captures(\?.*)?$/, () =>
          HttpResponse.json({ captures: [completed], total: 1, limit: 100, offset: 0 }),
        ),
      );

      renderWithQuery(<CaptureWidget name="alpha" ns="gameplane-games" gs={gs} />);

      const eyeBtn = await screen.findByLabelText(/View capture cap-1/i);
      await userEvent.click(eyeBtn);

      // Details row should appear
      const detailsRow = await screen.findByText(/Created/i);
      expect(detailsRow).toBeInTheDocument();
      expect(screen.getByText(/Started/i)).toBeInTheDocument();
      // "Server" appears in multiple places, so we check for the server name "alpha"
      expect(screen.getByText("alpha")).toBeInTheDocument();
    });

    it("collapses row when eye button is clicked again", async () => {
      const completed = makeCapture({
        captureId: "cap-1",
      });
      const gs = makeServer({
        spec: { capture: { enabled: true } },
      });
      server.use(
        http.get(/servers\/alpha:captures(\?.*)?$/, () =>
          HttpResponse.json({ captures: [completed], total: 1, limit: 100, offset: 0 }),
        ),
      );

      renderWithQuery(<CaptureWidget name="alpha" ns="gameplane-games" gs={gs} />);

      const eyeBtn = await screen.findByLabelText(/View capture cap-1/i);

      // Expand
      await userEvent.click(eyeBtn);
      await screen.findByText(/Created/i);

      // Collapse
      await userEvent.click(eyeBtn);
      await waitFor(() => {
        expect(screen.queryByText(/Created/i)).not.toBeInTheDocument();
      });
    });

    it("shows delete button for each capture", async () => {
      const completed = makeCapture({
        captureId: "cap-1",
      });
      const gs = makeServer({
        spec: { capture: { enabled: true } },
      });
      server.use(
        http.get(/servers\/alpha:captures(\?.*)?$/, () =>
          HttpResponse.json({ captures: [completed], total: 1, limit: 100, offset: 0 }),
        ),
      );

      renderWithQuery(<CaptureWidget name="alpha" ns="gameplane-games" gs={gs} />);

      expect(await screen.findByLabelText(/Delete capture cap-1/i)).toBeInTheDocument();
    });

    it("opens confirm dialog when delete button is clicked", async () => {
      const completed = makeCapture({
        captureId: "cap-1",
      });
      const gs = makeServer({
        spec: { capture: { enabled: true } },
      });
      server.use(
        http.get(/servers\/alpha:captures(\?.*)?$/, () =>
          HttpResponse.json({ captures: [completed], total: 1, limit: 100, offset: 0 }),
        ),
      );

      renderWithQuery(<CaptureWidget name="alpha" ns="gameplane-games" gs={gs} />);

      const deleteBtn = await screen.findByLabelText(/Delete capture cap-1/i);
      await userEvent.click(deleteBtn);

      // Exact match on the dialog title text ("Delete capture?"): a loose
      // /Delete capture/i also matches the dialog's own confirm button
      // ("Delete capture", no "?"), so findByText resolves two elements
      // and never settles — the same ambiguous-regex class documented on
      // "deletes capture when confirmed" below, just not yet called out
      // here.
      expect(await screen.findByText("Delete capture?")).toBeInTheDocument();
      expect(screen.getByText(/permanently deletes capture/i)).toBeInTheDocument();
    });

    it("deletes capture when confirmed", async () => {
      const completed = makeCapture({
        captureId: "cap-1",
      });
      const gs = makeServer({
        spec: { capture: { enabled: true } },
      });
      // Gated on the test rather than on a timer. Besides the usual
      // pending-window race, an instant response here is actively
      // misleading: deleteMut's onSuccess closes the dialog, so a
      // too-fast response leaves `confirmBtn` a detached node whose stale
      // `disabled` attribute the assertion could read either way.
      let releaseDelete!: () => void;
      const deleteGate = new Promise<void>((resolve) => {
        releaseDelete = resolve;
      });
      server.use(
        http.get(/servers\/alpha:captures(\?.*)?$/, () =>
          HttpResponse.json({ captures: [completed], total: 1, limit: 100, offset: 0 }),
        ),
        http.delete(/servers\/alpha:capture\?/, async () => {
          await deleteGate;
          return HttpResponse.json({ deleted: true, captureId: "cap-1" });
        }),
      );

      renderWithQuery(<CaptureWidget name="alpha" ns="gameplane-games" gs={gs} />);

      const deleteBtn = await screen.findByLabelText(/Delete capture cap-1/i);
      await userEvent.click(deleteBtn);

      // Exact match: the row's icon button has aria-label "Delete capture
      // cap-1", which also satisfies a loose /Delete capture/i match — only
      // the dialog's confirm button has the accessible name "Delete capture".
      const confirmBtn = await screen.findByRole("button", { name: "Delete capture" });
      await userEvent.click(confirmBtn);

      expect(confirmBtn).toBeDisabled();

      // Release, and confirm the delete actually went through: onSuccess
      // clears deleteTarget, which closes the dialog. That is the only
      // observable proof the DELETE was issued and succeeded — the
      // captures list is a fixed mock, so the row itself never disappears.
      releaseDelete();
      await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    });

    it("shows expiry time with correct tone badge", async () => {
      const completed = makeCapture({
        captureId: "cap-1",
        // +30s past the exact hour: expiryLabel's Date.now() call happens
        // after an async render/fetch round trip, so an exact-multiple
        // offset (3600000) has zero margin — any elapsed wall-clock time
        // floors secondsLeft below the hour and flips the label to
        // "59m", missing the "1h" query entirely. The 30s pad tolerates
        // realistic CI overhead without changing the asserted "1h" text.
        expiresAt: new Date(Date.now() + 3630000).toISOString(), // ~1 hour
      });
      const gs = makeServer({
        spec: { capture: { enabled: true } },
      });
      server.use(
        http.get(/servers\/alpha:captures(\?.*)?$/, () =>
          HttpResponse.json({ captures: [completed], total: 1, limit: 100, offset: 0 }),
        ),
      );

      renderWithQuery(<CaptureWidget name="alpha" ns="gameplane-games" gs={gs} />);

      // Exact "1h", not /1h/: makeCapture's default startedAt→completedAt
      // span is exactly one hour, so the row's Duration cell renders
      // "1h 0m" and a loose /1h/ matches BOTH cells — findByText then
      // rejects with "found multiple elements", surfacing as a bare 5s
      // timeout (asyncUtilTimeout == testTimeout). Exact matching pins the
      // query to the expiry badge, whose whole text is "1h".
      const expiryCell = await screen.findByText("1h");
      expect(expiryCell.className).toContain("text-warning");
    });

    it("displays capture count at the bottom", async () => {
      const cap1 = makeCapture({ captureId: "cap-1" });
      const cap2 = makeCapture({ captureId: "cap-2" });
      const gs = makeServer({
        spec: { capture: { enabled: true } },
      });
      server.use(
        http.get(/servers\/alpha:captures(\?.*)?$/, () =>
          HttpResponse.json({ captures: [cap1, cap2], total: 5, limit: 100, offset: 0 }),
        ),
      );

      renderWithQuery(<CaptureWidget name="alpha" ns="gameplane-games" gs={gs} />);

      expect(await screen.findByText(/Showing 2 of 5 captures/i)).toBeInTheDocument();
    });
  });

  describe("StartCaptureModal", () => {
    it("opens modal when Start Capture button is clicked", async () => {
      const gs = makeServer({
        spec: { capture: { enabled: true } },
      });
      server.use(
        http.get(/servers\/alpha:captures(\?.*)?$/, () =>
          HttpResponse.json({ captures: [], total: 0, limit: 100, offset: 0 }),
        ),
      );

      renderWithQuery(<CaptureWidget name="alpha" ns="gameplane-games" gs={gs} />);

      const startBtn = await screen.findByRole("button", { name: /Start Capture/i });
      await userEvent.click(startBtn);

      // getByText would also match the still-mounted trigger button and the
      // submit button, both of which share the same "Start Capture" text —
      // the dialog title is uniquely queryable by its heading role.
      expect(await screen.findByRole("heading", { name: /Start Capture/i })).toBeInTheDocument();
      expect(screen.getByText(/Records raw network traffic/i)).toBeInTheDocument();
    });

    it("renders all form fields with correct labels", async () => {
      const gs = makeServer({
        spec: { capture: { enabled: true } },
      });
      server.use(
        http.get(/servers\/alpha:captures(\?.*)?$/, () =>
          HttpResponse.json({ captures: [], total: 0, limit: 100, offset: 0 }),
        ),
      );

      renderWithQuery(<CaptureWidget name="alpha" ns="gameplane-games" gs={gs} />);

      const startBtn = await screen.findByRole("button", { name: /Start Capture/i });
      await userEvent.click(startBtn);

      expect(await screen.findByLabelText(/Packet Filter/i)).toBeInTheDocument();
      expect(screen.getByLabelText(/Max duration value/i)).toBeInTheDocument();
      expect(screen.getByLabelText(/Max duration unit/i)).toBeInTheDocument();
      expect(screen.getByLabelText(/Max size value/i)).toBeInTheDocument();
      expect(screen.getByLabelText(/Max size unit/i)).toBeInTheDocument();
      expect(screen.getByLabelText(/Retention value/i)).toBeInTheDocument();
      expect(screen.getByLabelText(/Retention unit/i)).toBeInTheDocument();
    });

    it("starts with default values", async () => {
      const gs = makeServer({
        spec: { capture: { enabled: true } },
      });
      server.use(
        http.get(/servers\/alpha:captures(\?.*)?$/, () =>
          HttpResponse.json({ captures: [], total: 0, limit: 100, offset: 0 }),
        ),
      );

      renderWithQuery(<CaptureWidget name="alpha" ns="gameplane-games" gs={gs} />);

      const startBtn = await screen.findByRole("button", { name: /Start Capture/i });
      await userEvent.click(startBtn);

      const filterInput = (await screen.findByLabelText(/Packet Filter/i)) as HTMLInputElement;
      const durationInput = screen.getByLabelText(/Max duration value/i) as HTMLInputElement;
      const sizeInput = screen.getByLabelText(/Max size value/i) as HTMLInputElement;
      const retentionInput = screen.getByLabelText(/Retention value/i) as HTMLInputElement;

      expect(filterInput.value).toBe("");
      expect(durationInput.value).toBe("300");
      expect(sizeInput.value).toBe("5120");
      expect(retentionInput.value).toBe("24");
    });

    it("validates BPF filter and shows error when API returns filter error", async () => {
      const gs = makeServer({
        spec: { capture: { enabled: true } },
      });
      server.use(
        http.get(/servers\/alpha:captures(\?.*)?$/, () =>
          HttpResponse.json({ captures: [], total: 0, limit: 100, offset: 0 }),
        ),
        http.post(/servers\/alpha:capture-start(\?.*)?$/, () =>
          new HttpResponse("invalid filter syntax: syntax error\n", { status: 400 }),
        ),
      );

      renderWithQuery(<CaptureWidget name="alpha" ns="gameplane-games" gs={gs} />);

      const startBtn = await screen.findByRole("button", { name: /Start Capture/i });
      await userEvent.click(startBtn);

      const filterInput = await screen.findByLabelText(/Packet Filter/i);
      await userEvent.type(filterInput, "invalid [[ syntax");

      const submitBtn = screen.getByRole("button", { name: /Start Capture/ });
      await userEvent.click(submitBtn);

      // Wait for error to appear
      await waitFor(() => {
        expect(screen.getByText(/invalid filter syntax/i)).toBeInTheDocument();
      });
    });

    it("shows error indicator when filter has validation error", async () => {
      const gs = makeServer({
        spec: { capture: { enabled: true } },
      });
      server.use(
        http.get(/servers\/alpha:captures(\?.*)?$/, () =>
          HttpResponse.json({ captures: [], total: 0, limit: 100, offset: 0 }),
        ),
        http.post(/servers\/alpha:capture-start(\?.*)?$/, () =>
          new HttpResponse("invalid filter\n", { status: 400 }),
        ),
      );

      renderWithQuery(<CaptureWidget name="alpha" ns="gameplane-games" gs={gs} />);

      const startBtn = await screen.findByRole("button", { name: /Start Capture/i });
      await userEvent.click(startBtn);

      const filterInput = await screen.findByLabelText(/Packet Filter/i);
      await userEvent.type(filterInput, "bad filter");

      const submitBtn = screen.getByRole("button", { name: /Start Capture/ });
      await userEvent.click(submitBtn);

      // Wait for error and check that error icon appears
      await waitFor(() => {
        expect(screen.getByText(/invalid filter/i)).toBeInTheDocument();
      });
    });

    it("clears filter error when user edits the filter field", async () => {
      const gs = makeServer({
        spec: { capture: { enabled: true } },
      });
      server.use(
        http.get(/servers\/alpha:captures(\?.*)?$/, () =>
          HttpResponse.json({ captures: [], total: 0, limit: 100, offset: 0 }),
        ),
        http.post(/servers\/alpha:capture-start(\?.*)?$/, () =>
          new HttpResponse("invalid filter\n", { status: 400 }),
        ),
      );

      renderWithQuery(<CaptureWidget name="alpha" ns="gameplane-games" gs={gs} />);

      const startBtn = await screen.findByRole("button", { name: /Start Capture/i });
      await userEvent.click(startBtn);

      const filterInput = await screen.findByLabelText(/Packet Filter/i);
      await userEvent.type(filterInput, "bad");

      const submitBtn = screen.getByRole("button", { name: /Start Capture/ });
      await userEvent.click(submitBtn);

      // Wait for error
      await waitFor(() => {
        expect(screen.getByText(/invalid filter/i)).toBeInTheDocument();
      });

      // Type more — error should clear
      await userEvent.type(filterInput, " filter");
      expect(screen.queryByText(/invalid filter/i)).not.toBeInTheDocument();
    });

    it("allows changing duration unit between seconds and minutes", async () => {
      const gs = makeServer({
        spec: { capture: { enabled: true } },
      });
      server.use(
        http.get(/servers\/alpha:captures(\?.*)?$/, () =>
          HttpResponse.json({ captures: [], total: 0, limit: 100, offset: 0 }),
        ),
      );

      renderWithQuery(<CaptureWidget name="alpha" ns="gameplane-games" gs={gs} />);

      const startBtn = await screen.findByRole("button", { name: /Start Capture/i });
      await userEvent.click(startBtn);

      const durationUnit = screen.getByLabelText(/Max duration unit/i);
      // Assuming the Select component uses a native select or accessible select
      await userEvent.selectOptions(durationUnit, "minutes");

      expect((durationUnit as HTMLSelectElement).value).toBe("minutes");
    });

    it("allows changing size unit between MB and GB", async () => {
      const gs = makeServer({
        spec: { capture: { enabled: true } },
      });
      server.use(
        http.get(/servers\/alpha:captures(\?.*)?$/, () =>
          HttpResponse.json({ captures: [], total: 0, limit: 100, offset: 0 }),
        ),
      );

      renderWithQuery(<CaptureWidget name="alpha" ns="gameplane-games" gs={gs} />);

      const startBtn = await screen.findByRole("button", { name: /Start Capture/i });
      await userEvent.click(startBtn);

      const sizeUnit = screen.getByLabelText(/Max size unit/i);
      await userEvent.selectOptions(sizeUnit, "GB");

      expect((sizeUnit as HTMLSelectElement).value).toBe("GB");
    });

    it("allows changing retention unit between hours and days", async () => {
      const gs = makeServer({
        spec: { capture: { enabled: true } },
      });
      server.use(
        http.get(/servers\/alpha:captures(\?.*)?$/, () =>
          HttpResponse.json({ captures: [], total: 0, limit: 100, offset: 0 }),
        ),
      );

      renderWithQuery(<CaptureWidget name="alpha" ns="gameplane-games" gs={gs} />);

      const startBtn = await screen.findByRole("button", { name: /Start Capture/i });
      await userEvent.click(startBtn);

      const retentionUnit = screen.getByLabelText(/Retention unit/i);
      await userEvent.selectOptions(retentionUnit, "days");

      expect((retentionUnit as HTMLSelectElement).value).toBe("days");
    });

    it("disables Start button when duration value is below 1", async () => {
      const gs = makeServer({
        spec: { capture: { enabled: true } },
      });
      server.use(
        http.get(/servers\/alpha:captures(\?.*)?$/, () =>
          HttpResponse.json({ captures: [], total: 0, limit: 100, offset: 0 }),
        ),
      );

      renderWithQuery(<CaptureWidget name="alpha" ns="gameplane-games" gs={gs} />);

      const startBtn = await screen.findByRole("button", { name: /Start Capture/i });
      await userEvent.click(startBtn);

      const durationInput = screen.getByLabelText(/Max duration value/i);
      await userEvent.clear(durationInput);
      await userEvent.type(durationInput, "0");

      const submitBtn = screen.getByRole("button", { name: /Start Capture/ });
      expect(submitBtn).toBeDisabled();
    });

    it("disables Start button when size value is below 1", async () => {
      const gs = makeServer({
        spec: { capture: { enabled: true } },
      });
      server.use(
        http.get(/servers\/alpha:captures(\?.*)?$/, () =>
          HttpResponse.json({ captures: [], total: 0, limit: 100, offset: 0 }),
        ),
      );

      renderWithQuery(<CaptureWidget name="alpha" ns="gameplane-games" gs={gs} />);

      const startBtn = await screen.findByRole("button", { name: /Start Capture/i });
      await userEvent.click(startBtn);

      const sizeInput = screen.getByLabelText(/Max size value/i);
      await userEvent.clear(sizeInput);
      await userEvent.type(sizeInput, "0");

      const submitBtn = screen.getByRole("button", { name: /Start Capture/ });
      expect(submitBtn).toBeDisabled();
    });

    it("disables Start button when filter has validation error", async () => {
      const gs = makeServer({
        spec: { capture: { enabled: true } },
      });
      server.use(
        http.get(/servers\/alpha:captures(\?.*)?$/, () =>
          HttpResponse.json({ captures: [], total: 0, limit: 100, offset: 0 }),
        ),
        http.post(/servers\/alpha:capture-start(\?.*)?$/, () =>
          new HttpResponse("invalid filter\n", { status: 400 }),
        ),
      );

      renderWithQuery(<CaptureWidget name="alpha" ns="gameplane-games" gs={gs} />);

      const startBtn = await screen.findByRole("button", { name: /Start Capture/i });
      await userEvent.click(startBtn);

      const filterInput = await screen.findByLabelText(/Packet Filter/i);
      await userEvent.type(filterInput, "bad");

      const submitBtn = screen.getByRole("button", { name: /Start Capture/ });
      await userEvent.click(submitBtn);

      // Wait for error
      await waitFor(() => {
        expect(screen.getByText(/invalid filter/i)).toBeInTheDocument();
      });

      expect(submitBtn).toBeDisabled();
    });

    it("submits capture start with correct body values in seconds and bytes", async () => {
      const gs = makeServer({
        spec: { capture: { enabled: true } },
      });
      let captureStartBody: Partial<CaptureStartBody> | null = null;
      server.use(
        http.get(/servers\/alpha:captures(\?.*)?$/, () =>
          HttpResponse.json({ captures: [], total: 0, limit: 100, offset: 0 }),
        ),
        http.post(/servers\/alpha:capture-start(\?.*)?$/, async ({ request }) => {
          captureStartBody = (await request.json()) as Partial<CaptureStartBody>;
          return HttpResponse.json(
            {
              captureId: "cap-new",
              phase: "Pending",
              serverName: "alpha",
              filter: "tcp port 8080",
              createdAt: new Date().toISOString(),
              startedAt: null,
              completedAt: null,
              bytesWritten: 0,
              packetsWritten: 0,
              expiresAt: new Date(Date.now() + 86400000).toISOString(),
            },
            { status: 202 },
          );
        }),
      );

      renderWithQuery(<CaptureWidget name="alpha" ns="gameplane-games" gs={gs} />);

      const startBtn = await screen.findByRole("button", { name: /Start Capture/i });
      await userEvent.click(startBtn);

      const filterInput = await screen.findByLabelText(/Packet Filter/i);
      await userEvent.type(filterInput, "tcp port 8080");

      const durationInput = screen.getByLabelText(/Max duration value/i);
      await userEvent.clear(durationInput);
      await userEvent.type(durationInput, "60");

      const sizeInput = screen.getByLabelText(/Max size value/i);
      await userEvent.clear(sizeInput);
      await userEvent.type(sizeInput, "1");

      const submitBtn = screen.getByRole("button", { name: /Start Capture/ });
      await userEvent.click(submitBtn);

      await waitFor(() => {
        expect(captureStartBody).toBeDefined();
        expect(captureStartBody!.filter).toBe("tcp port 8080");
        expect(captureStartBody!.maxDurationSeconds).toBe(60);
        expect(captureStartBody!.maxSizeBytes).toBe(1048576); // 1 MB in bytes
      });
    });

    it("converts duration from minutes to seconds on submit", async () => {
      const gs = makeServer({
        spec: { capture: { enabled: true } },
      });
      let captureStartBody: Partial<CaptureStartBody> | null = null;
      server.use(
        http.get(/servers\/alpha:captures(\?.*)?$/, () =>
          HttpResponse.json({ captures: [], total: 0, limit: 100, offset: 0 }),
        ),
        http.post(/servers\/alpha:capture-start(\?.*)?$/, async ({ request }) => {
          captureStartBody = (await request.json()) as Partial<CaptureStartBody>;
          return HttpResponse.json(
            { captureId: "cap-new", phase: "Pending", serverName: "alpha", createdAt: new Date().toISOString(), bytesWritten: 0, packetsWritten: 0 },
            { status: 202 },
          );
        }),
      );

      renderWithQuery(<CaptureWidget name="alpha" ns="gameplane-games" gs={gs} />);

      const startBtn = await screen.findByRole("button", { name: /Start Capture/i });
      await userEvent.click(startBtn);

      const durationInput = screen.getByLabelText(/Max duration value/i);
      const durationUnit = screen.getByLabelText(/Max duration unit/i);

      await userEvent.clear(durationInput);
      await userEvent.type(durationInput, "5");
      await userEvent.selectOptions(durationUnit, "minutes");

      const submitBtn = screen.getByRole("button", { name: /Start Capture/ });
      await userEvent.click(submitBtn);

      await waitFor(() => {
        expect(captureStartBody!.maxDurationSeconds).toBe(300); // 5 minutes * 60
      });
    });

    it("converts size from GB to bytes on submit", async () => {
      const gs = makeServer({
        spec: { capture: { enabled: true } },
      });
      let captureStartBody: Partial<CaptureStartBody> | null = null;
      server.use(
        http.get(/servers\/alpha:captures(\?.*)?$/, () =>
          HttpResponse.json({ captures: [], total: 0, limit: 100, offset: 0 }),
        ),
        http.post(/servers\/alpha:capture-start(\?.*)?$/, async ({ request }) => {
          captureStartBody = (await request.json()) as Partial<CaptureStartBody>;
          return HttpResponse.json(
            { captureId: "cap-new", phase: "Pending", serverName: "alpha", createdAt: new Date().toISOString(), bytesWritten: 0, packetsWritten: 0 },
            { status: 202 },
          );
        }),
      );

      renderWithQuery(<CaptureWidget name="alpha" ns="gameplane-games" gs={gs} />);

      const startBtn = await screen.findByRole("button", { name: /Start Capture/i });
      await userEvent.click(startBtn);

      const sizeInput = screen.getByLabelText(/Max size value/i);
      const sizeUnit = screen.getByLabelText(/Max size unit/i);

      await userEvent.clear(sizeInput);
      await userEvent.type(sizeInput, "1");
      await userEvent.selectOptions(sizeUnit, "GB");

      const submitBtn = screen.getByRole("button", { name: /Start Capture/ });
      await userEvent.click(submitBtn);

      await waitFor(() => {
        expect(captureStartBody!.maxSizeBytes).toBe(1073741824); // 1 GB in bytes
      });
    });

    it("converts retention from days to seconds on submit", async () => {
      const gs = makeServer({
        spec: { capture: { enabled: true } },
      });
      let captureStartBody: Partial<CaptureStartBody> | null = null;
      server.use(
        http.get(/servers\/alpha:captures(\?.*)?$/, () =>
          HttpResponse.json({ captures: [], total: 0, limit: 100, offset: 0 }),
        ),
        http.post(/servers\/alpha:capture-start(\?.*)?$/, async ({ request }) => {
          captureStartBody = (await request.json()) as Partial<CaptureStartBody>;
          return HttpResponse.json(
            { captureId: "cap-new", phase: "Pending", serverName: "alpha", createdAt: new Date().toISOString(), bytesWritten: 0, packetsWritten: 0 },
            { status: 202 },
          );
        }),
      );

      renderWithQuery(<CaptureWidget name="alpha" ns="gameplane-games" gs={gs} />);

      const startBtn = await screen.findByRole("button", { name: /Start Capture/i });
      await userEvent.click(startBtn);

      const retentionInput = screen.getByLabelText(/Retention value/i);
      const retentionUnit = screen.getByLabelText(/Retention unit/i);

      await userEvent.clear(retentionInput);
      await userEvent.type(retentionInput, "7");
      await userEvent.selectOptions(retentionUnit, "days");

      const submitBtn = screen.getByRole("button", { name: /Start Capture/ });
      await userEvent.click(submitBtn);

      await waitFor(() => {
        expect(captureStartBody!.ttlSecondsAfterFinished).toBe(604800); // 7 days * 86400
      });
    });

    it("shows loading state during capture start", async () => {
      const gs = makeServer({
        spec: { capture: { enabled: true } },
      });
      server.use(
        http.get(/servers\/alpha:captures(\?.*)?$/, () =>
          HttpResponse.json({ captures: [], total: 0, limit: 100, offset: 0 }),
        ),
        http.post(/servers\/alpha:capture-start(\?.*)?$/, async () => {
          await new Promise((r) => setTimeout(r, 100));
          return HttpResponse.json(
            { captureId: "cap-new", phase: "Pending", serverName: "alpha", createdAt: new Date().toISOString(), bytesWritten: 0, packetsWritten: 0 },
            { status: 202 },
          );
        }),
      );

      renderWithQuery(<CaptureWidget name="alpha" ns="gameplane-games" gs={gs} />);

      const startBtn = await screen.findByRole("button", { name: /Start Capture/i });
      await userEvent.click(startBtn);

      const filterInput = await screen.findByLabelText(/Packet Filter/i);
      await userEvent.type(filterInput, "tcp port 8080");

      const submitBtn = screen.getByRole("button", { name: /Start Capture/ });
      await userEvent.click(submitBtn);

      expect(submitBtn).toHaveTextContent(/Starting/i);
    });

    it("displays non-filter API errors in an error banner", async () => {
      const gs = makeServer({
        spec: { capture: { enabled: true } },
      });
      server.use(
        http.get(/servers\/alpha:captures(\?.*)?$/, () =>
          HttpResponse.json({ captures: [], total: 0, limit: 100, offset: 0 }),
        ),
        http.post(/servers\/alpha:capture-start(\?.*)?$/, () =>
          new HttpResponse("server error: pod not running\n", { status: 500 }),
        ),
      );

      renderWithQuery(<CaptureWidget name="alpha" ns="gameplane-games" gs={gs} />);

      const startBtn = await screen.findByRole("button", { name: /Start Capture/i });
      await userEvent.click(startBtn);

      const submitBtn = screen.getByRole("button", { name: /Start Capture/ });
      await userEvent.click(submitBtn);

      await waitFor(() => {
        expect(screen.getByText(/server error/i)).toBeInTheDocument();
      });
    });

    it("closes modal when Cancel is clicked", async () => {
      const gs = makeServer({
        spec: { capture: { enabled: true } },
      });
      server.use(
        http.get(/servers\/alpha:captures(\?.*)?$/, () =>
          HttpResponse.json({ captures: [], total: 0, limit: 100, offset: 0 }),
        ),
      );

      renderWithQuery(<CaptureWidget name="alpha" ns="gameplane-games" gs={gs} />);

      const startBtn = await screen.findByRole("button", { name: /Start Capture/i });
      await userEvent.click(startBtn);

      expect(await screen.findByRole("heading", { name: /Start Capture/i })).toBeInTheDocument();

      const cancelBtn = screen.getByRole("button", { name: /Cancel/i });
      await userEvent.click(cancelBtn);

      // The page's own "Start Capture" trigger button keeps that text after
      // the modal closes, so assert on the dialog itself, not the text.
      await waitFor(() => {
        expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
      });
    });

    it("disables Cancel button during capture start", async () => {
      const gs = makeServer({
        spec: { capture: { enabled: true } },
      });
      server.use(
        http.get(/servers\/alpha:captures(\?.*)?$/, () =>
          HttpResponse.json({ captures: [], total: 0, limit: 100, offset: 0 }),
        ),
        http.post(/servers\/alpha:capture-start(\?.*)?$/, async () => {
          await new Promise((r) => setTimeout(r, 100));
          return HttpResponse.json(
            { captureId: "cap-new", phase: "Pending", serverName: "alpha", createdAt: new Date().toISOString(), bytesWritten: 0, packetsWritten: 0 },
            { status: 202 },
          );
        }),
      );

      renderWithQuery(<CaptureWidget name="alpha" ns="gameplane-games" gs={gs} />);

      const startBtn = await screen.findByRole("button", { name: /Start Capture/i });
      await userEvent.click(startBtn);

      const submitBtn = screen.getByRole("button", { name: /Start Capture/ });
      await userEvent.click(submitBtn);

      const cancelBtn = screen.getByRole("button", { name: /Cancel/i });
      expect(cancelBtn).toBeDisabled();
    });

    it("closes modal and invalidates queries on successful capture start", async () => {
      const gs = makeServer({
        spec: { capture: { enabled: true } },
      });
      server.use(
        http.get(/servers\/alpha:captures(\?.*)?$/, () =>
          HttpResponse.json({ captures: [], total: 0, limit: 100, offset: 0 }),
        ),
        http.post(/servers\/alpha:capture-start(\?.*)?$/, () =>
          HttpResponse.json(
            { captureId: "cap-new", phase: "Pending", serverName: "alpha", createdAt: new Date().toISOString(), bytesWritten: 0, packetsWritten: 0 },
            { status: 202 },
          ),
        ),
      );

      renderWithQuery(<CaptureWidget name="alpha" ns="gameplane-games" gs={gs} />);

      const startBtn = await screen.findByRole("button", { name: /Start Capture/i });
      await userEvent.click(startBtn);

      const submitBtn = screen.getByRole("button", { name: /Start Capture/ });
      await userEvent.click(submitBtn);

      // Modal should close
      await waitFor(() => {
        expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
      });
    });
  });

  describe("retention settings", () => {
    it("displays default retention hours when retentionSeconds is not set", () => {
      const gs = makeServer({
        spec: { capture: { enabled: true } },
      });
      renderWithQuery(<CaptureWidget name="alpha" ns="gameplane-games" gs={gs} />);

      // Default is 86400 seconds = 24 hours
      expect(screen.getByText(/Captures will auto-delete after 24 hours/i)).toBeInTheDocument();
    });

    it("displays custom retention hours when retentionSeconds is set", () => {
      const gs = makeServer({
        spec: { capture: { enabled: true, retentionSeconds: 172800 } }, // 2 days
      });
      renderWithQuery(<CaptureWidget name="alpha" ns="gameplane-games" gs={gs} />);

      expect(screen.getByText(/Captures will auto-delete after 48 hours/i)).toBeInTheDocument();
    });

    it("displays singular hour when retention is 1 hour", () => {
      const gs = makeServer({
        spec: { capture: { enabled: true, retentionSeconds: 3600 } },
      });
      renderWithQuery(<CaptureWidget name="alpha" ns="gameplane-games" gs={gs} />);

      expect(screen.getByText(/Captures will auto-delete after 1 hour$/i)).toBeInTheDocument();
    });
  });

  describe("helper functions coverage (through rendering)", () => {
    it("formats duration of 0 seconds correctly", async () => {
      // shouldAdvanceTime keeps the fake clock ticking in step with real
      // time so promise-based infra (msw, TanStack Query, userEvent/
      // testing-library's findBy* polling) still resolves; a plain
      // vi.useFakeTimers() freezes those timers and hangs the test.
      vi.useFakeTimers({ shouldAdvanceTime: true });
      try {
        const now = new Date("2026-08-24T12:00:00Z");
        vi.setSystemTime(now);

        const activeCap = makeCapture({
          phase: "Running",
          captureId: "cap-active",
          startedAt: now.toISOString(),
        });
        const gs = makeServer({
          spec: { capture: { enabled: true } },
        });
        server.use(
          http.get(/servers\/alpha:captures(\?.*)?$/, () =>
            HttpResponse.json({ captures: [activeCap], total: 1, limit: 100, offset: 0 }),
          ),
          http.get(/servers\/alpha:capture\?/, () =>
            HttpResponse.json(activeCap),
          ),
        );

        renderWithQuery(<CaptureWidget name="alpha" ns="gameplane-games" gs={gs} />);

        // formatDuration(0) should display as "0s" through the elapsed
        // message. The captures list comes from TanStack Query through
        // msw and resolves asynchronously, so this must be awaited rather
        // than asserted on the synchronous first render.
        expect(await screen.findByText(/Capture started 0s ago/i)).toBeInTheDocument();
      } finally {
        // Restored unconditionally so a failed assertion above can never
        // leak fake timers into every later test in this file.
        vi.useRealTimers();
      }
    });

    it("formats duration correctly with minutes and seconds", async () => {
      const activeCap = makeCapture({
        phase: "Running",
        captureId: "cap-active",
        startedAt: new Date(Date.now() - 125000).toISOString(), // ~2 minutes, 5 seconds
      });
      const gs = makeServer({
        spec: { capture: { enabled: true } },
      });
      server.use(
        http.get(/servers\/alpha:captures(\?.*)?$/, () =>
          HttpResponse.json({ captures: [activeCap], total: 1, limit: 100, offset: 0 }),
        ),
        http.get(/servers\/alpha:capture\?/, () =>
          HttpResponse.json(activeCap),
        ),
      );

      renderWithQuery(<CaptureWidget name="alpha" ns="gameplane-games" gs={gs} />);

      // Should show minutes and seconds. The seconds component is left
      // open (\d+) on purpose: elapsedSeconds() is recomputed at render
      // time, so any wall-clock time between building startedAt above and
      // the post-fetch render ticks "2m 5s" to "2m 6s" — a sub-second
      // margin this file has already been bitten by four times over
      // (see the padded expiry offsets below). The m/s shape is what this
      // test is about, and the regex still pins that.
      expect(await screen.findByText(/Capture started 2m \d+s ago/i)).toBeInTheDocument();
    });

    it("formats duration correctly with hours and minutes", async () => {
      const activeCap = makeCapture({
        phase: "Running",
        captureId: "cap-active",
        startedAt: new Date(Date.now() - 3725000).toISOString(), // ~1 hour, 2 minutes
      });
      const gs = makeServer({
        spec: { capture: { enabled: true } },
      });
      server.use(
        http.get(/servers\/alpha:captures(\?.*)?$/, () =>
          HttpResponse.json({ captures: [activeCap], total: 1, limit: 100, offset: 0 }),
        ),
        http.get(/servers\/alpha:capture\?/, () =>
          HttpResponse.json(activeCap),
        ),
      );

      renderWithQuery(<CaptureWidget name="alpha" ns="gameplane-games" gs={gs} />);

      // Should show hours and minutes
      expect(await screen.findByText(/Capture started 1h 2m ago/i)).toBeInTheDocument();
    });

    it("handles invalid startedAt date gracefully", async () => {
      const activeCap = makeCapture({
        phase: "Running",
        captureId: "cap-active",
        startedAt: "invalid-date",
      });
      const gs = makeServer({
        spec: { capture: { enabled: true } },
      });
      server.use(
        http.get(/servers\/alpha:captures(\?.*)?$/, () =>
          HttpResponse.json({ captures: [activeCap], total: 1, limit: 100, offset: 0 }),
        ),
        http.get(/servers\/alpha:capture\?/, () =>
          HttpResponse.json(activeCap),
        ),
      );

      renderWithQuery(<CaptureWidget name="alpha" ns="gameplane-games" gs={gs} />);

      // Should fall back to createdAt when startedAt is invalid
      expect(await screen.findByText(/Capture started/i)).toBeInTheDocument();
    });

    it("displays durationBetween with em-dash when dates are missing", async () => {
      const completed = makeCapture({
        captureId: "cap-1",
        startedAt: "",
        completedAt: "",
      });
      const gs = makeServer({
        spec: { capture: { enabled: true } },
      });
      server.use(
        http.get(/servers\/alpha:captures(\?.*)?$/, () =>
          HttpResponse.json({ captures: [completed], total: 1, limit: 100, offset: 0 }),
        ),
      );

      renderWithQuery(<CaptureWidget name="alpha" ns="gameplane-games" gs={gs} />);

      // Duration column should show em-dash for missing dates
      const durationCells = await screen.findAllByText(/—/);
      expect(durationCells.length).toBeGreaterThan(0);
    });

    it("displays expiry badge with danger tone when less than 1 hour remaining", async () => {
      const completed = makeCapture({
        captureId: "cap-1",
        // +30s past the exact 30 minutes: expiryLabel's Date.now() call
        // happens after an async render/fetch round trip, so an
        // exact-minute offset has zero margin against CI-realistic delay —
        // any elapsed time floors the minutes component down to 29m and
        // the /30m/ query never finds it (masked as a 5s timeout, not a
        // "not found" error, because asyncUtilTimeout == testTimeout).
        expiresAt: new Date(Date.now() + 1830000).toISOString(), // ~30 minutes
      });
      const gs = makeServer({
        spec: { capture: { enabled: true } },
      });
      server.use(
        http.get(/servers\/alpha:captures(\?.*)?$/, () =>
          HttpResponse.json({ captures: [completed], total: 1, limit: 100, offset: 0 }),
        ),
      );

      renderWithQuery(<CaptureWidget name="alpha" ns="gameplane-games" gs={gs} />);

      // Should show 30m with danger tone. Exact match (not /30m/) so the
      // query can only hit the expiry badge — the same row also renders a
      // Duration cell ("1h 0m" from makeCapture's defaults) and a relative
      // "Completed at" cell, either of which a loose regex could grow into.
      const expiryBadge = await screen.findByText("30m");
      expect(expiryBadge.className).toContain("text-danger");
    });

    it("displays expiry badge with warning tone when 1-6 hours remaining", async () => {
      const completed = makeCapture({
        captureId: "cap-1",
        // +30s past the exact 2 hours, for the same reason as the 30m and
        // 24h boundary tests in this file: 7200000 is an exact multiple of
        // an hour, so it has zero margin and any elapsed render/fetch time
        // flips the hour count down to "1h", missing /2h/.
        expiresAt: new Date(Date.now() + 7230000).toISOString(), // ~2 hours
      });
      const gs = makeServer({
        spec: { capture: { enabled: true } },
      });
      server.use(
        http.get(/servers\/alpha:captures(\?.*)?$/, () =>
          HttpResponse.json({ captures: [completed], total: 1, limit: 100, offset: 0 }),
        ),
      );

      renderWithQuery(<CaptureWidget name="alpha" ns="gameplane-games" gs={gs} />);

      // Should show 2h with warning tone; exact match for the same
      // single-element reason as the 30m case above.
      const expiryBadge = await screen.findByText("2h");
      expect(expiryBadge.className).toContain("text-warning");
    });

    it("displays expiry badge with muted tone when more than 6 hours remaining", async () => {
      const completed = makeCapture({
        captureId: "cap-1",
        // +30s past the exact 24 hours, for the same reason as the 30m and
        // 2h boundary tests above: 86400000 is an exact multiple of an
        // hour, so it has zero margin against render/fetch delay before
        // the hour count flips down to "23h", missing /24h/.
        expiresAt: new Date(Date.now() + 86430000).toISOString(), // ~24 hours
      });
      const gs = makeServer({
        spec: { capture: { enabled: true } },
      });
      server.use(
        http.get(/servers\/alpha:captures(\?.*)?$/, () =>
          HttpResponse.json({ captures: [completed], total: 1, limit: 100, offset: 0 }),
        ),
      );

      renderWithQuery(<CaptureWidget name="alpha" ns="gameplane-games" gs={gs} />);

      // Should show 24h with muted tone; exact match for the same
      // single-element reason as the 30m case above.
      const expiryBadge = await screen.findByText("24h");
      expect(expiryBadge.className).toContain("text-muted");
    });

    it("displays expiry badge with em-dash when expiresAt is missing", async () => {
      const completed = makeCapture({
        captureId: "cap-1",
        expiresAt: undefined,
      });
      const gs = makeServer({
        spec: { capture: { enabled: true } },
      });
      server.use(
        http.get(/servers\/alpha:captures(\?.*)?$/, () =>
          HttpResponse.json({ captures: [completed], total: 1, limit: 100, offset: 0 }),
        ),
      );

      renderWithQuery(<CaptureWidget name="alpha" ns="gameplane-games" gs={gs} />);

      // Expiry column should be empty or show nothing
      await screen.findByText("cap-1");
    });

    it("displays expiry badge with em-dash when expiresAt is invalid date", async () => {
      const completed = makeCapture({
        captureId: "cap-1",
        expiresAt: "invalid-date",
      });
      const gs = makeServer({
        spec: { capture: { enabled: true } },
      });
      server.use(
        http.get(/servers\/alpha:captures(\?.*)?$/, () =>
          HttpResponse.json({ captures: [completed], total: 1, limit: 100, offset: 0 }),
        ),
      );

      renderWithQuery(<CaptureWidget name="alpha" ns="gameplane-games" gs={gs} />);

      const emDash = await screen.findByText(/—/);
      expect(emDash).toBeInTheDocument();
    });

    it("omits optional filter field when empty string is submitted", async () => {
      const gs = makeServer({
        spec: { capture: { enabled: true } },
      });
      let captureStartBody: Partial<CaptureStartBody> | null = null;
      server.use(
        http.get(/servers\/alpha:captures(\?.*)?$/, () =>
          HttpResponse.json({ captures: [], total: 0, limit: 100, offset: 0 }),
        ),
        http.post(/servers\/alpha:capture-start(\?.*)?$/, async ({ request }) => {
          captureStartBody = (await request.json()) as Partial<CaptureStartBody>;
          return HttpResponse.json(
            { captureId: "cap-new", phase: "Pending", serverName: "alpha", createdAt: new Date().toISOString(), bytesWritten: 0, packetsWritten: 0 },
            { status: 202 },
          );
        }),
      );

      renderWithQuery(<CaptureWidget name="alpha" ns="gameplane-games" gs={gs} />);

      const startBtn = await screen.findByRole("button", { name: /Start Capture/i });
      await userEvent.click(startBtn);

      const submitBtn = screen.getByRole("button", { name: /Start Capture/ });
      await userEvent.click(submitBtn);

      await waitFor(() => {
        expect(captureStartBody!.filter).toBeUndefined();
      });
    });

    it("includes filter when provided", async () => {
      const gs = makeServer({
        spec: { capture: { enabled: true } },
      });
      let captureStartBody: Partial<CaptureStartBody> | null = null;
      server.use(
        http.get(/servers\/alpha:captures(\?.*)?$/, () =>
          HttpResponse.json({ captures: [], total: 0, limit: 100, offset: 0 }),
        ),
        http.post(/servers\/alpha:capture-start(\?.*)?$/, async ({ request }) => {
          captureStartBody = (await request.json()) as Partial<CaptureStartBody>;
          return HttpResponse.json(
            { captureId: "cap-new", phase: "Pending", serverName: "alpha", createdAt: new Date().toISOString(), bytesWritten: 0, packetsWritten: 0 },
            { status: 202 },
          );
        }),
      );

      renderWithQuery(<CaptureWidget name="alpha" ns="gameplane-games" gs={gs} />);

      const startBtn = await screen.findByRole("button", { name: /Start Capture/i });
      await userEvent.click(startBtn);

      const filterInput = await screen.findByLabelText(/Packet Filter/i);
      await userEvent.type(filterInput, "  tcp port 25565  ");

      const submitBtn = screen.getByRole("button", { name: /Start Capture/ });
      await userEvent.click(submitBtn);

      await waitFor(() => {
        expect(captureStartBody!.filter).toBe("tcp port 25565");
      });
    });
  });
});
