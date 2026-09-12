import { Chip } from "@heroui/react";
import { X } from "lucide-react";

// RemovableGroupChip renders a removable chip for group/role membership display.
// Used in role editors and user management to show assigned groups with inline remove capability.
// Three color variants: secondary (default), orange (warning), violet (accent).

export type RemovableGroupChipVariant = "secondary" | "orange" | "violet";

export interface RemovableGroupChipProps {
  label: string;
  variant?: RemovableGroupChipVariant;
  onRemove: () => void;
  size?: "sm" | "md" | "lg";
  className?: string;
}

const variantMap: Record<RemovableGroupChipVariant, {
  color: "default" | "warning" | "accent";
  variant: "primary" | "secondary" | "soft" | "tertiary";
}> = {
  secondary: {
    color: "default",
    variant: "secondary",
  },
  orange: {
    color: "warning",
    variant: "soft",
  },
  violet: {
    color: "accent",
    variant: "soft",
  },
};

export function RemovableGroupChip({
  label,
  variant = "secondary",
  onRemove,
  size = "sm",
  className,
}: RemovableGroupChipProps) {
  const config = variantMap[variant];

  return (
    <Chip
      color={config.color}
      variant={config.variant}
      size={size}
      data-variant={variant}
      className={className}
    >
      <div className="flex items-center gap-1">
        <span className="text-xs font-medium">{label}</span>
        <button
          onClick={(e) => {
            e.stopPropagation();
            onRemove();
          }}
          className="inline-flex items-center justify-center hover:opacity-75 transition-opacity"
          aria-label={`Remove ${label}`}
          type="button"
        >
          <X size={12} />
        </button>
      </div>
    </Chip>
  );
}
