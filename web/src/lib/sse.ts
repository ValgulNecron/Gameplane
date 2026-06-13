// Thin reconnecting Server-Sent Events client for the API's /events
// stream. The API frames each Kubernetes watch event as
// {kind, eventType, object}; the dashboard uses this to invalidate
// TanStack Query caches (so views refresh without waiting for the next
// poll) and to feed the notifications panel.

export interface KestrelEvent {
  // CRD path segment: "servers" | "templates" | "backups" | "schedules" | "restores"
  kind: string;
  // Kubernetes watch event type: "ADDED" | "MODIFIED" | "DELETED"
  eventType: string;
  object: { metadata?: { name?: string; namespace?: string } } & Record<string, unknown>;
}

export interface EventStreamOptions {
  onEvent: (ev: KestrelEvent) => void;
  onError?: () => void;
}

// openEventStream connects to /events and invokes onEvent for each parsed
// frame. Returns a disposer that closes the stream and stops reconnects.
// EventSource reconnects on transient drops on its own; we additionally
// re-open if the connection errors out and was closed.
export function openEventStream(opts: EventStreamOptions): () => void {
  // No EventSource (e.g. jsdom/test, or an ancient browser) → no-op; the
  // dashboard's refetchInterval pollers keep data fresh as a fallback.
  if (typeof EventSource === "undefined") return () => {};

  let es: EventSource | null = null;
  let closed = false;
  let retry: ReturnType<typeof setTimeout> | undefined;

  function connect() {
    if (closed) return;
    es = new EventSource("/events", { withCredentials: true });
    es.onmessage = (e) => {
      try {
        opts.onEvent(JSON.parse(e.data) as KestrelEvent);
      } catch {
        // Ignore malformed frames rather than tearing down the stream.
      }
    };
    es.onerror = () => {
      opts.onError?.();
      // EventSource auto-retries while open; if the browser closed it,
      // re-open after a short backoff.
      if (!closed && es && es.readyState === EventSource.CLOSED) {
        es.close();
        retry = setTimeout(connect, 3000);
      }
    };
  }
  connect();

  return () => {
    closed = true;
    if (retry) clearTimeout(retry);
    es?.close();
  };
}

// queryKeyForKind maps an event's CRD kind to the TanStack Query key the
// dashboard caches it under, so a watch event invalidates the right view.
export function queryKeyForKind(kind: string): string[] | null {
  switch (kind) {
    case "servers":
      return ["servers"];
    case "templates":
      return ["templates"];
    case "backups":
      return ["backups"];
    case "schedules":
      return ["schedules"];
    case "restores":
      return ["restores"];
    default:
      return null;
  }
}
