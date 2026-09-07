import { Alert, Button } from "@heroui/react";
import { Activity } from "lucide-react";

export interface AuditIntegrityBannerProps {
  message: string;
  onDismiss?: () => void;
  onRefetch?: () => void;
  isRefetching?: boolean;
}

export function AuditIntegrityBanner({
  message,
  onDismiss,
  onRefetch,
  isRefetching,
}: AuditIntegrityBannerProps) {
  return (
    <Alert status="danger" className="flex items-center gap-3">
      <Alert.Indicator>
        <Activity className="h-5 w-5 shrink-0 text-danger" />
      </Alert.Indicator>
      <Alert.Content className="flex flex-1 items-center justify-between gap-3">
        <span className="text-sm">{message}</span>
        <div className="flex items-center gap-2">
          {onRefetch ? (
            <Button
              size="sm"
              variant="ghost"
              onPress={onRefetch}
              isDisabled={isRefetching}
            >
              Re-check
            </Button>
          ) : null}
          {onDismiss ? (
            <button
              onClick={onDismiss}
              className="inline-flex shrink-0 items-center justify-center rounded px-2 py-1 hover:bg-danger/20 focus:outline-none focus:ring-1 focus:ring-danger"
              aria-label="Dismiss audit integrity alert"
            >
              <span className="text-xs font-bold text-danger">×</span>
            </button>
          ) : null}
        </div>
      </Alert.Content>
    </Alert>
  );
}
