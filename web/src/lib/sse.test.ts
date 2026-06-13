import { describe, it, expect, vi } from "vitest";
import { openEventStream, queryKeyForKind } from "./sse";

describe("queryKeyForKind", () => {
  it("maps known CRD kinds to query keys", () => {
    expect(queryKeyForKind("servers")).toEqual(["servers"]);
    expect(queryKeyForKind("backups")).toEqual(["backups"]);
    expect(queryKeyForKind("schedules")).toEqual(["schedules"]);
    expect(queryKeyForKind("restores")).toEqual(["restores"]);
    expect(queryKeyForKind("templates")).toEqual(["templates"]);
  });

  it("returns null for unknown kinds", () => {
    expect(queryKeyForKind("widgets")).toBeNull();
  });
});

describe("openEventStream", () => {
  it("is a safe no-op without EventSource (jsdom)", () => {
    // jsdom provides no EventSource; the client degrades to the pollers.
    expect(typeof EventSource).toBe("undefined");
    const onEvent = vi.fn();
    const dispose = openEventStream({ onEvent });
    expect(typeof dispose).toBe("function");
    dispose();
    expect(onEvent).not.toHaveBeenCalled();
  });
});
