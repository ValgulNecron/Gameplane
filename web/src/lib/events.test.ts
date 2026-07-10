import { describe, expect, it } from "vitest";
import { humanizeBytes, mapServerEvent } from "./events";
import type { ServerEvent } from "@/types";

describe("humanizeBytes", () => {
  it("rewrites raw byte counts in event messages", () => {
    expect(humanizeBytes('Successfully pulled image in 12s. Image size: 333546371 bytes.')).toBe(
      "Successfully pulled image in 12s. Image size: 318 MB.",
    );
  });
  it("leaves small numbers and non-byte text alone", () => {
    expect(humanizeBytes("Started container in 27589 ms")).toBe("Started container in 27589 ms");
    expect(humanizeBytes("Back-off restarting failed container")).toBe(
      "Back-off restarting failed container",
    );
  });
});

describe("mapServerEvent", () => {
  const base: ServerEvent = {
    id: "e1",
    time: "2026-07-10T00:00:00Z",
    type: "Normal",
    reason: "Pulling",
    message: "pulling image",
    source: "kubelet",
    object: "pod/srv-0",
    count: 1,
  };

  it("prefixes the reason and keeps the source", () => {
    expect(mapServerEvent(base)).toEqual({
      id: "e1",
      ts: "2026-07-10T00:00:00Z",
      kind: "info",
      message: "Pulling: pulling image",
      source: "kubelet",
    });
  });

  it("maps Warning to warn", () => {
    expect(mapServerEvent({ ...base, type: "Warning" }).kind).toBe("warn");
  });

  it("omits an empty source", () => {
    expect(mapServerEvent({ ...base, source: "" }).source).toBeUndefined();
  });
});
