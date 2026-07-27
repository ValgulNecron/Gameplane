import { describe, it, expect, vi, afterEach } from "vitest";
import { openEventStream, queryKeyForKind, type GameplaneEvent } from "./sse";

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

// A controllable EventSource stand-in (jsdom has none) to drive the
// connect / onmessage / onerror / reconnect paths deterministically.
class FakeEventSource {
  static CLOSED = 2;
  static instances: FakeEventSource[] = [];
  readyState = 0;
  onmessage: ((e: { data: string }) => void) | null = null;
  onerror: (() => void) | null = null;
  closed = false;
  constructor(
    public url: string,
    public opts?: unknown,
  ) {
    FakeEventSource.instances.push(this);
  }
  close() {
    this.closed = true;
    this.readyState = FakeEventSource.CLOSED;
  }
}

describe("openEventStream with EventSource", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.useRealTimers();
    FakeEventSource.instances = [];
  });

  it("parses frames, ignores malformed ones, reconnects on error, and disposes", () => {
    vi.useFakeTimers();
    vi.stubGlobal("EventSource", FakeEventSource as unknown as typeof EventSource);

    const events: GameplaneEvent[] = [];
    let errored = 0;
    const dispose = openEventStream({
      onEvent: (e) => events.push(e),
      onError: () => {
        errored++;
      },
    });

    const es = FakeEventSource.instances[0];
    expect(es.url).toBe("/events");

    es.onmessage?.({ data: JSON.stringify({ kind: "servers", eventType: "ADDED", object: {} }) });
    es.onmessage?.({ data: "not json" }); // swallowed, no throw
    expect(events).toHaveLength(1);
    expect(events[0].kind).toBe("servers");

    // An error on a CLOSED stream schedules a reconnect after the backoff.
    es.readyState = FakeEventSource.CLOSED;
    es.onerror?.();
    expect(errored).toBe(1);
    vi.advanceTimersByTime(3000);
    expect(FakeEventSource.instances).toHaveLength(2);

    // Disposing clears any pending retry and closes the live stream.
    dispose();
    expect(FakeEventSource.instances[1].closed).toBe(true);
  });

  it("sends withCredentials flag for auth", () => {
    vi.stubGlobal("EventSource", FakeEventSource as unknown as typeof EventSource);
    vi.useFakeTimers();

    const dispose = openEventStream({ onEvent: vi.fn() });
    const es = FakeEventSource.instances[0];
    expect(es.opts).toEqual({ withCredentials: true });

    dispose();
    vi.unstubAllGlobals();
    vi.useRealTimers();
  });

  it("does not reconnect when the stream is already closed via dispose", () => {
    vi.useFakeTimers();
    vi.stubGlobal("EventSource", FakeEventSource as unknown as typeof EventSource);

    const dispose = openEventStream({ onEvent: vi.fn() });
    const es = FakeEventSource.instances[0];
    dispose();

    // Trigger error after dispose
    es.readyState = FakeEventSource.CLOSED;
    es.onerror?.();

    // Should not have created a second instance due to reconnect
    vi.advanceTimersByTime(3000);
    expect(FakeEventSource.instances).toHaveLength(1);

    vi.unstubAllGlobals();
    vi.useRealTimers();
  });

  it("does not reconnect when error fires on an open stream", () => {
    vi.useFakeTimers();
    vi.stubGlobal("EventSource", FakeEventSource as unknown as typeof EventSource);

    const dispose = openEventStream({ onEvent: vi.fn() });
    const es = FakeEventSource.instances[0];

    // readyState is CONNECTING (0) when open, so error doesn't trigger reconnect
    es.readyState = 0;
    es.onerror?.();

    vi.advanceTimersByTime(3000);
    expect(FakeEventSource.instances).toHaveLength(1);

    dispose();
    vi.unstubAllGlobals();
    vi.useRealTimers();
  });

  it("does not reconnect after dispose", () => {
    vi.useFakeTimers();
    vi.stubGlobal("EventSource", FakeEventSource as unknown as typeof EventSource);

    const dispose = openEventStream({ onEvent: vi.fn() });
    dispose();

    const es = FakeEventSource.instances[0];
    es.readyState = FakeEventSource.CLOSED;
    es.onerror?.();

    vi.advanceTimersByTime(3000);
    expect(FakeEventSource.instances).toHaveLength(1);

    vi.unstubAllGlobals();
    vi.useRealTimers();
  });

  it("handles events with metadata paths", () => {
    vi.useFakeTimers();
    vi.stubGlobal("EventSource", FakeEventSource as unknown as typeof EventSource);

    const events: GameplaneEvent[] = [];
    const dispose = openEventStream({
      onEvent: (e) => events.push(e),
    });

    const es = FakeEventSource.instances[0];
    es.onmessage?.({
      data: JSON.stringify({
        kind: "servers",
        eventType: "MODIFIED",
        object: {
          metadata: {
            name: "mc-server",
            namespace: "default",
          },
        },
      }),
    });

    expect(events).toHaveLength(1);
    expect(events[0].object.metadata?.name).toBe("mc-server");

    dispose();
    vi.unstubAllGlobals();
    vi.useRealTimers();
  });
});
