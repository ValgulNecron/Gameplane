import { cn } from "@/lib/utils";

interface SwitchProps {
  isSelected: boolean;
  onChange: (value: boolean) => void;
  isDisabled?: boolean;
  "aria-label"?: string;
}

/**
 * Switch component with role="switch" for accessibility and testing.
 * Styled to match the HeroUI design language.
 */
export function Switch({
  isSelected,
  onChange,
  isDisabled,
  "aria-label": ariaLabel,
}: SwitchProps) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={isSelected}
      aria-label={ariaLabel}
      disabled={isDisabled}
      onClick={() => !isDisabled && onChange(!isSelected)}
      className={cn(
        "relative inline-flex h-6 w-11 shrink-0 items-center rounded-full transition-colors",
        "focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-2",
        isDisabled ? "opacity-50 cursor-not-allowed" : "cursor-pointer",
        isSelected ? "bg-primary" : "bg-default-300",
      )}
    >
      <span
        className={cn(
          "inline-block h-5 w-5 rounded-full bg-white transition-transform shadow-sm",
          isSelected ? "translate-x-5" : "translate-x-0.5",
        )}
      />
    </button>
  );
}
