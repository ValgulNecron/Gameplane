import React from "react";
import { Alert } from "@heroui/react";
import { AlertCircle } from "lucide-react";
import { cn } from "@/lib/utils";

export interface ErrorCardProps {
  /** The error message to display */
  message: string;
  /** Optional title/heading for the error card */
  title?: string;
  /** Optional custom icon element; defaults to AlertCircle */
  icon?: React.ReactNode;
  /** Optional callback when dismiss button is clicked */
  onDismiss?: () => void;
  /** Optional callback when retry button is clicked */
  onRetry?: () => void;
  /** Optional CSS class name */
  className?: string;
}

export function ErrorCard({
  message,
  title,
  icon,
  onDismiss,
  onRetry,
  className,
}: ErrorCardProps) {
  const defaultIcon =
    icon || <AlertCircle className="h-5 w-5 shrink-0 text-danger" />;

  return (
    <Alert
      status="danger"
      className={cn("flex items-start gap-3", className)}
    >
      <Alert.Indicator className="mt-0.5">{defaultIcon}</Alert.Indicator>
      <Alert.Content className="flex flex-1 flex-col gap-2">
        {title && (
          <Alert.Title className="font-semibold text-sm">
            {title}
          </Alert.Title>
        )}
        <Alert.Description className="text-sm">{message}</Alert.Description>
      </Alert.Content>
      {(onDismiss || onRetry) && (
        <div className="ml-2 flex shrink-0 gap-1">
          {onRetry && (
            <button
              onClick={onRetry}
              className="inline-flex items-center justify-center rounded px-2 py-1 text-xs text-danger hover:bg-danger/20 focus:outline-none focus:ring-1 focus:ring-danger"
              aria-label="Retry"
            >
              Retry
            </button>
          )}
          {onDismiss && (
            <button
              onClick={onDismiss}
              className="inline-flex items-center justify-center rounded px-2 py-1 hover:bg-danger/20 focus:outline-none focus:ring-1 focus:ring-danger"
              aria-label="Dismiss error"
            >
              <span className="text-xs text-danger">×</span>
            </button>
          )}
        </div>
      )}
    </Alert>
  );
}
