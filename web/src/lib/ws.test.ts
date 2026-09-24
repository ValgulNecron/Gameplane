import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { openWS, type WSStatus, type WSStatusInfo } from "./ws";

// Minimal in-process WebSocket double used for unit tests. Records
// every instance so the test can drive open/message/close from the
// outside.
class FakeSocket {
  static instances: FakeSocket[] = [];
  url: string;
  readyState = 0;
  onopen?: (ev: Event) => void;
  onmessage?: (ev: MessageEvent) => void;
  onclose?: (ev: CloseEvent) => void;
  onerror?: (ev: Event) => void;
  sent: unknown[] = [];

  constructor(url: string) {
    this.url = url;
    FakeSocket.instances.push(this);
  }

  send(data: unknown) {
    if (this.readyState === 0) {
      throw new DOMException("Still in CONNECTING state.", "InvalidStateError");
    }
    if (this.readyState === 2 || this.readyState === 3) {
      throw new Error("send() called while CLOSING/CLOSED — should have been guarded");
    }
    this.sent.push(data);
  }
  close() { this.readyState = 3; this.onclose?.(new CloseEvent("close")); }
  triggerOpen() { this.readyState = 1; this.onopen?.(new Event("open")); }
  triggerMessage(data: string) { this.onmessage?.(new MessageEvent("message", { data })); }
  triggerClose() { this.readyState = 3; this.onclose?.(new CloseEvent("close")); }
}

describe("openWS", () => {
  beforeEach(() => {
    FakeSocket.instances = [];
    vi.stubGlobal("WebSocket", FakeSocket);
    vi.stubGlobal("location", { protocol: "https:", host: "example.com" });
    vi.useFakeTimers();
  });
  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it("uses wss when page is https", () => {
    openWS("/ws/foo", { onMessage: () => {} });
    expect(FakeSocket.instances[0].url).toBe("wss://example.com/ws/foo");
  });

  it("falls back to ws when page is http", () => {
    vi.stubGlobal("location", { protocol: "http:", host: "h" });
    openWS("/ws/foo", { onMessage: () => {} });
    expect(FakeSocket.instances[0].url).toBe("ws://h/ws/foo");
  });

  it("forwards messages and resets attempt counter on open", () => {
    const messages: unknown[] = [];
    openWS("/ws/x", {
      onMessage: (d) => messages.push(d),
      onOpen: () => messages.push("OPEN"),
    });
    const sock = FakeSocket.instances[0];
    sock.triggerOpen();
    sock.triggerMessage("hi");
    expect(messages).toEqual(["OPEN", "hi"]);
  });

  it("reconnects with exponential backoff after close", () => {
    openWS("/ws/x", { onMessage: () => {} });
    const first = FakeSocket.instances[0];
    first.triggerClose();
    // First attempt scheduled at min(30s, 500 * 2^1) = 1000ms.
    vi.advanceTimersByTime(1000);
    expect(FakeSocket.instances).toHaveLength(2);
    FakeSocket.instances[1].triggerClose();
    vi.advanceTimersByTime(2000);
    expect(FakeSocket.instances).toHaveLength(3);
  });

  it("close() suppresses reconnect", () => {
    const handle = openWS("/ws/x", { onMessage: () => {} });
    handle.close();
    // The fake's close() invokes onclose synchronously; after a tick we
    // should still have only one instance.
    vi.advanceTimersByTime(60_000);
    expect(FakeSocket.instances).toHaveLength(1);
  });

  it("close() during a pending backoff timer cancels the scheduled reconnect (F-121)", () => {
    const handle = openWS("/ws/x", { onMessage: () => {} });
    const first = FakeSocket.instances[0];
    first.triggerClose(); // schedules a reconnect at ~1000ms
    vi.advanceTimersByTime(100); // still inside the backoff window
    handle.close();
    vi.advanceTimersByTime(60_000);
    // No second socket should ever be constructed: the pending timer
    // must have been cancelled by close(), not merely raced.
    expect(FakeSocket.instances).toHaveLength(1);
  });

  it("reconnect:false skips reconnect after close", () => {
    openWS("/ws/x", { onMessage: () => {}, reconnect: false });
    FakeSocket.instances[0].triggerClose();
    vi.advanceTimersByTime(60_000);
    expect(FakeSocket.instances).toHaveLength(1);
  });

  it("send forwards to underlying socket", () => {
    const handle = openWS("/ws/x", { onMessage: () => {} });
    const sock = FakeSocket.instances[0];
    sock.triggerOpen();
    handle.send("ping");
    expect(sock.sent).toEqual(["ping"]);
  });

  it("queues input sent before the first open and flushes it in order on open", () => {
    const handle = openWS("/ws/x", { onMessage: () => {} });
    const sock = FakeSocket.instances[0];
    handle.send("msg1");
    handle.send("msg2");
    handle.send("msg3");
    expect(sock.sent).toEqual([]);
    sock.triggerOpen();
    expect(sock.sent).toEqual(["msg1", "msg2", "msg3"]);
  });

  it("drops input sent while reconnecting instead of replaying it", () => {
    const handle = openWS("/ws/x", { onMessage: () => {} });
    const sock1 = FakeSocket.instances[0];
    sock1.triggerOpen();
    sock1.triggerClose();
    vi.advanceTimersByTime(100); // Before reconnect
    handle.send("dropped");
    vi.advanceTimersByTime(900); // Complete the reconnect delay (1000 total)
    const sock2 = FakeSocket.instances[1];
    sock2.triggerOpen();
    // The new socket should not have received the message sent while reconnecting
    expect(sock2.sent).toEqual([]);
    // The first socket should not have received the message either
    expect(sock1.sent).not.toContain("dropped");
  });

  it("ignores sends after close()", () => {
    const handle = openWS("/ws/x", { onMessage: () => {} });
    const sock = FakeSocket.instances[0];
    sock.triggerOpen();
    handle.close();
    // Should not throw and should not record
    handle.send("ignored");
    expect(sock.sent).toEqual([]);
  });

  it("bounds the pre-open queue and drops the oldest frames", () => {
    const handle = openWS("/ws/x", { onMessage: () => {} });
    const sock = FakeSocket.instances[0];
    // Send 501 messages before open (exceeds MAX_QUEUED of 500)
    for (let i = 0; i < 501; i++) {
      handle.send(`f${i}`);
    }
    sock.triggerOpen();
    // After open, should have 500 messages and the first should be f1 (f0 was dropped)
    expect(sock.sent).toHaveLength(500);
    expect(sock.sent[0]).toBe("f1");
    expect(sock.sent[499]).toBe("f500");
  });

  it("invokes onClose hook", () => {
    const onClose = vi.fn();
    openWS("/ws/x", { onMessage: () => {}, onClose, reconnect: false });
    FakeSocket.instances[0].triggerClose();
    expect(onClose).toHaveBeenCalled();
  });

  it("emits connecting then open on a successful connect", () => {
    const seen: WSStatus[] = [];
    openWS("/ws/x", { onMessage: () => {}, onStatus: (s) => seen.push(s) });
    expect(seen).toEqual(["connecting"]);
    FakeSocket.instances[0].triggerOpen();
    expect(seen).toEqual(["connecting", "open"]);
  });

  it("emits reconnecting with attempt and backoff delay on a retryable close", () => {
    const events: Array<{ s: WSStatus; info: WSStatusInfo }> = [];
    openWS("/ws/x", { onMessage: () => {}, onStatus: (s, info) => events.push({ s, info }) });
    FakeSocket.instances[0].triggerClose();
    const last = events[events.length - 1];
    expect(last.s).toBe("reconnecting");
    expect(last.info).toEqual({ attempt: 1, nextRetryMs: 1000 });
    // The scheduled retry re-emits "connecting" for the next attempt.
    vi.advanceTimersByTime(1000);
    expect(events[events.length - 1].s).toBe("connecting");
    expect(events[events.length - 1].info.attempt).toBe(1);
  });

  it("emits closed (not reconnecting) when the user closes", () => {
    const seen: WSStatus[] = [];
    const handle = openWS("/ws/x", { onMessage: () => {}, onStatus: (s) => seen.push(s) });
    handle.close();
    expect(seen[seen.length - 1]).toBe("closed");
    vi.advanceTimersByTime(60_000);
    expect(seen).not.toContain("reconnecting");
  });

  it("emits closed when reconnect is disabled", () => {
    const seen: WSStatus[] = [];
    openWS("/ws/x", { onMessage: () => {}, reconnect: false, onStatus: (s) => seen.push(s) });
    FakeSocket.instances[0].triggerClose();
    expect(seen[seen.length - 1]).toBe("closed");
  });
});
