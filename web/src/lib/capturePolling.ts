// Poll cadence for a server's capture list (CaptureWidget).
//
// F-259 (maintainer decision 2026-09-25): POST :capture-stop only asks the
// operator to stop a capture; the operator stops the sidecar and only then
// marks the capture Completed. The stop response therefore still reports
// Pending/Running, and the capture's Download action (offered for
// Completed captures only) must wait for a later list refresh. While a
// capture the user stopped is still Pending/Running, the list is polled
// faster so the UI picks up the Completed phase promptly instead of
// looking stale.
import type { NetworkCapture, NetworkCaptureList } from "@/types";

export const CAPTURE_LIST_POLL_MS = 5000;
export const CAPTURE_STOPPING_POLL_MS = 1000;

export function isCaptureActive(c: Pick<NetworkCapture, "phase">): boolean {
  return c.phase === "Pending" || c.phase === "Running";
}

// captureListRefetchMs returns the refetch interval for the capture list:
// CAPTURE_STOPPING_POLL_MS while the capture the user asked to stop
// (stoppingId) is still Pending/Running, otherwise CAPTURE_LIST_POLL_MS.
export function captureListRefetchMs(
  list: NetworkCaptureList | undefined,
  stoppingId: string | null,
): number {
  if (!stoppingId) return CAPTURE_LIST_POLL_MS;
  const stopping = (list?.captures ?? []).some(
    (c) => c.captureId === stoppingId && isCaptureActive(c),
  );
  return stopping ? CAPTURE_STOPPING_POLL_MS : CAPTURE_LIST_POLL_MS;
}
