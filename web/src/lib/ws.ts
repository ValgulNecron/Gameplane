// Thin reconnecting WebSocket helper. Used by the console and logs tabs.
//
// NOTE: WebSocket streams are currently local-cluster-scoped. The cluster
// selector threads ?cluster= through API fetches (REST), but WS paths
// (console/RCON/log streams) remain bound to the local cluster. Cross-cluster
// WebSocket support is a follow-up task. See docs/roadmap.md.

// WebSocket.OPEN state constant. Defined locally to avoid relying on static
// properties in test stubs that may not implement them.
const WS_OPEN = 1; // WebSocket.OPEN

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
  // The timer scheduled by onclose's backoff retry. Tracked so close()
  // can cancel a reconnect that hasn't fired yet (F-121): without this,
  // a tab switch during a backoff window leaves the timer armed, and it
  // later opens a socket into a component that already unmounted.
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null;

  // Queue for messages sent while the first connection is opening.
  // After the first successful open, messages sent while disconnected
  // are dropped (not queued), since replaying them after reconnect
  // (e.g., a "stop" command) is worse than losing them.
  const MAX_QUEUED = 500;
  let messageQueue: (string | Blob | BufferSource)[] = [];
  let hasEverOpened = false;

  function connect() {
    // A reconnect scheduled before close() was called must not open a
    // new socket once the caller has torn down (F-121).
    if (closedByUser) return;
    opts.onStatus?.("connecting", { attempt });
    const proto = location.protocol === "https:" ? "wss:" : "ws:";
    sock = new WebSocket(`${proto}//${location.host}${path}`);
    sock.onopen = () => {
      attempt = 0;

      // Flush queued messages only on the first successful open.
      // On reconnects, pending commands are dropped to avoid silent
      // replays that could have unintended side effects.
      if (!hasEverOpened && messageQueue.length > 0) {
        hasEverOpened = true;
        while (messageQueue.length > 0) {
          const msg = messageQueue.shift();
          if (sock && sock.readyState === WS_OPEN) {
            sock.send(msg!);
          }
        }
      } else if (!hasEverOpened) {
        hasEverOpened = true;
      }

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
      reconnectTimer = setTimeout(connect, delayMs);
    };
  }
  connect();

  return {
    send(data: string | Blob | BufferSource) {
      if (closedByUser) return;

      // Before the first open: queue the message (bounded by MAX_QUEUED).
      // Dropping oldest if the queue is full prevents unbounded memory growth.
      if (!hasEverOpened && sock && sock.readyState !== WS_OPEN) {
        if (messageQueue.length >= MAX_QUEUED) {
          messageQueue.shift(); // Drop oldest
        }
        messageQueue.push(data);
        return;
      }

      // After the first open: only send if actually open. Messages sent
      // while reconnecting are dropped (not queued or replayed).
      if (sock && sock.readyState === WS_OPEN) {
        sock.send(data);
      }
    },
    close() {
      closedByUser = true;
      messageQueue = [];
      if (reconnectTimer !== null) {
        clearTimeout(reconnectTimer);
        reconnectTimer = null;
      }
      sock?.close();
    },
  };
}
