import { Alert } from "@heroui/react";
import { AlertCircle } from "lucide-react";
import { APIError } from "@/lib/api";

export interface ErrorBannerProps {
  err: unknown;
  onDismiss?: () => void;
}

export function ErrorBanner({ err, onDismiss }: ErrorBannerProps) {
  const msg =
    err instanceof APIError
      ? err.body || err.message
      : String(err);

  return (
    <Alert status="danger" className="flex items-center gap-3">
      <Alert.Indicator>
        <AlertCircle className="h-5 w-5 shrink-0 text-danger" />
      </Alert.Indicator>
      <Alert.Content className="flex flex-1 items-center justify-between gap-3">
        <span className="text-sm">{msg}</span>
        {onDismiss ? (
          <button
            onClick={onDismiss}
            className="ml-2 inline-flex shrink-0 items-center justify-center rounded px-2 py-1 hover:bg-danger/20 focus:outline-none focus:ring-1 focus:ring-danger"
            aria-label="Dismiss error"
          >
            <span className="text-xs font-bold text-danger">×</span>
          </button>
        ) : null}
      </Alert.Content>
    </Alert>
  );
}
