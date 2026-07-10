import type { ServerEvent } from "@/types";
import { formatBytes } from "@/lib/utils";

// NormalizedServerEvent is the view model the dashboard renders a raw
// Kubernetes ServerEvent as — used by the server Overview card and the
// Events tab. Named to avoid shadowing the DOM `Event` global now that it's
// a shared symbol.
export interface NormalizedServerEvent {
  id: string;
  ts: string;
  kind: "info" | "warn" | "error";
  message: string;
  source?: string;
}

// humanizeBytes rewrites raw byte counts in kubelet event messages
// ("Image size: 333546371 bytes") into a readable form ("Image size:
// 318 MB"). Only 7+ digit runs followed by "bytes" are touched, so small
// numbers elsewhere in a message are left alone.
export function humanizeBytes(msg: string): string {
  return msg.replace(/\b(\d{7,})\s*bytes\b/g, (_m, n: string) => formatBytes(Number(n)));
}

// mapServerEvent maps a Kubernetes event onto the view model: Warning →
// warn, everything else → info; the reason prefixes the message ("Pulling:
// pulling image…") and the reporting component is the source.
export function mapServerEvent(e: ServerEvent): NormalizedServerEvent {
  return {
    id: e.id,
    ts: e.time,
    kind: e.type === "Warning" ? "warn" : "info",
    message: humanizeBytes(e.reason ? `${e.reason}: ${e.message}` : e.message),
    source: e.source || undefined,
  };
}
