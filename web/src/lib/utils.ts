import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

// Upper-cases the first character. The operator's provisioning messages
// are lowercase sentence fragments ("pulling the game image"); this makes
// them presentable as a status line without mangling the rest.
export function capitalize(s: string): string {
  return s ? s.charAt(0).toUpperCase() + s.slice(1) : s;
}

export function formatBytes(n: number): string {
  if (!n) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB", "PB"];
  const i = Math.min(units.length - 1, Math.floor(Math.log(n) / Math.log(1024)));
  const v = n / Math.pow(1024, i);
  return `${v.toFixed(v >= 10 || i === 0 ? 0 : 1)} ${units[i]}`;
}

export function formatRelative(iso?: string): string {
  if (!iso) return "—";
  const t = new Date(iso).getTime();
  if (Number.isNaN(t)) return iso;
  const s = Math.max(0, Math.floor((Date.now() - t) / 1000));
  if (s < 60)       return `${s}s ago`;
  if (s < 3600)     return `${Math.floor(s / 60)}m ago`;
  if (s < 86400)    return `${Math.floor(s / 3600)}h ago`;
  return `${Math.floor(s / 86400)}d ago`;
}

// formatRelativeFuture renders a forward-looking interval ("in 45m") for a
// timestamp expected to be in the future (e.g. a schedule's next run). Past
// or now reads as "due".
export function formatRelativeFuture(iso?: string): string {
  if (!iso) return "—";
  const t = new Date(iso).getTime();
  if (Number.isNaN(t)) return iso;
  const s = Math.floor((t - Date.now()) / 1000);
  if (s <= 0)       return "due";
  if (s < 60)       return `in ${s}s`;
  if (s < 3600)     return `in ${Math.floor(s / 60)}m`;
  if (s < 86400)    return `in ${Math.floor(s / 3600)}h`;
  return `in ${Math.floor(s / 86400)}d`;
}

// parseQuantityToBytes parses a size string into bytes. Accepts plain byte
// counts ("5368709120"), Kubernetes binary-SI ("5Gi", "512Mi") and decimal-SI
// ("5G", "500M") suffixes, and human "GiB"/"GB" forms. Returns 0 for empty or
// unparseable input, so it's safe to sum across a list.
export function parseQuantityToBytes(s?: string): number {
  if (!s) return 0;
  const m = /^\s*([0-9.]+)\s*([A-Za-z]*)\s*$/.exec(s);
  if (!m) return 0;
  const n = parseFloat(m[1]);
  if (Number.isNaN(n)) return 0;
  let unit = m[2].toUpperCase();
  if (unit.endsWith("B")) unit = unit.slice(0, -1); // GiB->GI, GB->G, B->""
  if (unit === "") return n;
  const factors: Record<string, number> = {
    KI: 1024, MI: 1024 ** 2, GI: 1024 ** 3, TI: 1024 ** 4, PI: 1024 ** 5,
    K: 1000, M: 1000 ** 2, G: 1000 ** 3, T: 1000 ** 4, P: 1000 ** 5,
  };
  return (factors[unit] ?? 0) * n;
}

export function formatUptime(startedAt?: string): string {
  if (!startedAt) return "—";
  const t = new Date(startedAt).getTime();
  if (Number.isNaN(t)) return "—";
  const s = Math.max(0, Math.floor((Date.now() - t) / 1000));
  const d = Math.floor(s / 86400);
  const h = Math.floor((s % 86400) / 3600);
  const m = Math.floor((s % 3600) / 60);
  if (d) return `${d}d ${h}h`;
  if (h) return `${h}h ${m}m`;
  return `${m}m`;
}
