import { Alert } from "@heroui/react";
import { AlertCircle } from "lucide-react";
import { APIError } from "@/lib/api";

export function ErrorBanner({ err }: { err: unknown }) {
  const msg = err instanceof APIError ? err.body || err.message : String(err);
  return (
    <Alert status="error" className="text-xs">
      <div className="flex gap-2">
        <AlertCircle className="h-4 w-4 flex-shrink-0" />
        <span>{msg}</span>
      </div>
    </Alert>
  );
}
