import { describe, it, expect } from "vitest";
import { makeCapture } from "@/test/factories";
import type { NetworkCaptureList } from "@/types";
import {
  CAPTURE_LIST_POLL_MS,
  CAPTURE_STOPPING_POLL_MS,
  captureListRefetchMs,
  isCaptureActive,
} from "./capturePolling";

function list(...captures: ReturnType<typeof makeCapture>[]): NetworkCaptureList {
  return { captures, total: captures.length, limit: 100, offset: 0 };
}

describe("isCaptureActive", () => {
  it("is true for Pending and Running only", () => {
    expect(isCaptureActive({ phase: "Pending" })).toBe(true);
    expect(isCaptureActive({ phase: "Running" })).toBe(true);
    expect(isCaptureActive({ phase: "Completed" })).toBe(false);
    expect(isCaptureActive({ phase: "Failed" })).toBe(false);
    expect(isCaptureActive({ phase: "Expired" })).toBe(false);
  });
});

describe("captureListRefetchMs", () => {
  it("uses the base interval when no stop is pending", () => {
    expect(captureListRefetchMs(list(makeCapture({ captureId: "a", phase: "Running" })), null)).toBe(
      CAPTURE_LIST_POLL_MS,
    );
  });

  it("uses the base interval before the list has loaded", () => {
    expect(captureListRefetchMs(undefined, "a")).toBe(CAPTURE_LIST_POLL_MS);
  });

  it("polls faster while the stopped capture is still Running or Pending", () => {
    expect(captureListRefetchMs(list(makeCapture({ captureId: "a", phase: "Running" })), "a")).toBe(
      CAPTURE_STOPPING_POLL_MS,
    );
    expect(captureListRefetchMs(list(makeCapture({ captureId: "a", phase: "Pending" })), "a")).toBe(
      CAPTURE_STOPPING_POLL_MS,
    );
  });

  it("falls back to the base interval once the operator has completed it", () => {
    expect(captureListRefetchMs(list(makeCapture({ captureId: "a", phase: "Completed" })), "a")).toBe(
      CAPTURE_LIST_POLL_MS,
    );
  });

  it("ignores other captures that are still active", () => {
    expect(
      captureListRefetchMs(
        list(
          makeCapture({ captureId: "a", phase: "Completed" }),
          makeCapture({ captureId: "b", phase: "Running" }),
        ),
        "a",
      ),
    ).toBe(CAPTURE_LIST_POLL_MS);
  });
});
