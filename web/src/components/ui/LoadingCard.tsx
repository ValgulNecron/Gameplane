// LoadingCard is a composed HeroUI Card + Spinner shown while async data loads.
// It renders a surface-colored card with a centered spinner and optional message.

import { Card, Spinner } from "@heroui/react";

export interface LoadingCardProps {
  /** Optional message displayed below the spinner. Defaults to "Loading…" */
  message?: string;
  /** Optional CSS class name for additional styling */
  className?: string;
}

export function LoadingCard({
  message = "Loading…",
  className,
}: LoadingCardProps) {
  return (
    <Card className={`flex items-center gap-3 ${className || ""}`}>
      <div className="flex items-center gap-3 p-5">
        <Spinner color="current" size="md" />
        <span className="text-sm text-muted">{message}</span>
      </div>
    </Card>
  );
}
