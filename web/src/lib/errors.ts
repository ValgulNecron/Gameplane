import { APIError } from "@/lib/api";

// errorText renders an unknown thrown value as a user-facing string,
// unwrapping the JSON {error|message} body of an APIError when present.
export function errorText(err: unknown, fallback = "request failed"): string {
  if (err instanceof APIError) {
    try {
      const parsed = JSON.parse(err.body) as { error?: string; message?: string };
      if (parsed.error) return parsed.error;
      if (parsed.message) return parsed.message;
    } catch {
      // body isn't JSON — fall through to the raw body
    }
    return err.body || `request failed (${err.status})`;
  }
  return err instanceof Error ? err.message : fallback;
}

// errorTextWithStatus is errorText plus an HTTP status prefix for APIError
// (e.g. "404: no pods found") — for diagnostic surfaces (like the admin log
// viewer) where the status code itself is useful context, not just the body.
export function errorTextWithStatus(err: unknown, fallback = "request failed"): string {
  if (err instanceof APIError) return `${err.status}: ${errorText(err, fallback)}`;
  return errorText(err, fallback);
}
