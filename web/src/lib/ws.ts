// Thin reconnecting WebSocket helper. Used by the console and logs tabs.

// Connection lifecycle, surfaced to callers via onStatus so the UI can
// reflect "reconnecting…" in its chrome instead of writing a status line
// on every socket cycle. This is a dumb per-cycle emitter; consumers that
// only want to react to transitions (e.g. the console) de-dup themselves.
export type WSStatus = "connecting" | "open" | "reconnecting" | "closed";
export interface WSStatusInfo {
  // Failed connection attempts since the last successful open. 0 while open.
  attempt: number;
  // Backoff delay before the next attempt, set only on "reconnecting".
  nextRetryMs?: number;
}

export interface WSOptions {
  onMessage: (data: string | ArrayBuffer) => void;
  onOpen?: () => void;
  onClose?: () => void;
  onStatus?: (status: WSStatus, info: WSStatusInfo) => void;
  reconnect?: boolean;
}

export function openWS(path: string, opts: WSOptions) {
  let closedByUser = false;
  let sock: WebSocket | null = null;
  const reconnect = opts.reconnect ?? true;
  let attempt = 0;

  function connect() {
    opts.onStatus?.("connecting", { attempt });
    const proto = location.protocol === "https:" ? "wss:" : "ws:";
    sock = new WebSocket(`${proto}//${location.host}${path}`);
    sock.onopen = () => {
      attempt = 0;
      opts.onOpen?.();
      opts.onStatus?.("open", { attempt: 0 });
    };
    sock.onmessage = (ev) => opts.onMessage(ev.data);
    sock.onclose = () => {
      opts.onClose?.();
      if (closedByUser || !reconnect) {
        opts.onStatus?.("closed", { attempt });
        return;
      }
      attempt++;
      const delayMs = Math.min(30_000, 500 * 2 ** Math.min(attempt, 6));
      opts.onStatus?.("reconnecting", { attempt, nextRetryMs: delayMs });
      setTimeout(connect, delayMs);
    };
  }
  connect();

  return {
    send(data: string | ArrayBufferLike | Blob | ArrayBufferView) { sock?.send(data); },
    close() { closedByUser = true; sock?.close(); },
  };
}
