import { APIError } from "@/lib/api";

export function ErrorBanner({ err }: { err: unknown }) {
  const msg = err instanceof APIError ? err.body || err.message : String(err);
  return (
    <div className="rounded-md border border-danger/60 bg-danger/10 p-2 text-xs text-danger">
      {msg}
    </div>
  );
}
