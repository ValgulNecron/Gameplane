import { Alert } from "@heroui/react";
import { TriangleAlert } from "lucide-react";

interface CaptureWarningBannerProps {
  retentionHours?: number;
  onDismiss?: () => void;
}

export function CaptureWarningBanner({ retentionHours = 24, onDismiss }: CaptureWarningBannerProps) {
  return (
    <Alert status="warning" className="flex items-start gap-3">
      <Alert.Indicator className="mt-0.5 shrink-0">
        <TriangleAlert className="h-5 w-5 text-warning" />
      </Alert.Indicator>
      <Alert.Content className="flex flex-1 flex-col gap-2">
        <Alert.Title className="text-sm font-medium">
          Caution: network packet captures contain real player data
        </Alert.Title>
        <ul className="space-y-1 text-xs text-warning-foreground/80">
          <li>• Player IP addresses and port numbers</li>
          <li>• Network timing and game protocol messages</li>
          <li>• For some games, in-band credentials (passwords, tokens, session keys)</li>
        </ul>
        <Alert.Description className="text-xs">
          Captures are not redacted or sanitized. Access is restricted to administrators only.
          Captures are automatically deleted after the configured retention window (currently{" "}
          {retentionHours} hour{retentionHours === 1 ? "" : "s"}).
        </Alert.Description>
        {onDismiss && (
          <button
            type="button"
            onClick={onDismiss}
            className="mt-2 inline-flex text-xs font-medium text-warning hover:underline"
          >
            Dismiss
          </button>
        )}
      </Alert.Content>
    </Alert>
  );
}
