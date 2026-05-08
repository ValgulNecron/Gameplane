// Thin reconnecting WebSocket helper. Used by the console and logs tabs.

export interface WSOptions {
  onMessage: (data: string | ArrayBuffer) => void;
  onOpen?: () => void;
  onClose?: () => void;
  reconnect?: boolean;
}

export function openWS(path: string, opts: WSOptions) {
  let closedByUser = false;
  let sock: WebSocket | null = null;
  const reconnect = opts.reconnect ?? true;
  let attempt = 0;

  function connect() {
    const proto = location.protocol === "https:" ? "wss:" : "ws:";
    sock = new WebSocket(`${proto}//${location.host}${path}`);
    sock.onopen = () => { attempt = 0; opts.onOpen?.(); };
    sock.onmessage = (ev) => opts.onMessage(ev.data);
    sock.onclose = () => {
      opts.onClose?.();
      if (closedByUser || !reconnect) return;
      attempt++;
      const delayMs = Math.min(30_000, 500 * 2 ** Math.min(attempt, 6));
      setTimeout(connect, delayMs);
    };
  }
  connect();

  return {
    send(data: string | ArrayBufferLike | Blob | ArrayBufferView) { sock?.send(data); },
    close() { closedByUser = true; sock?.close(); },
  };
}
