import React from "react";
import { Chip } from "@heroui/react";
import { Edit2, Package, Minus } from "lucide-react";

// ProvenanceBadge renders a data provenance indicator showing how a configuration value was sourced.
// Three variants: "overridden" (manual edit), "fromHelm" (Helm values), "notConfigured" (not set).
// Used in admin settings and configuration screens to track configuration origin.

export type ProvenanceType = "overridden" | "fromHelm" | "notConfigured";

export interface ProvenanceBadgeProps {
  type: ProvenanceType;
  size?: "sm" | "md" | "lg";
  className?: string;
}

const provenanceConfig: Record<ProvenanceType, {
  icon: React.ReactNode;
  label: string;
}> = {
  overridden: {
    icon: <Edit2 className="h-3 w-3" />,
    label: "Overridden in dashboard",
  },
  fromHelm: {
    icon: <Package className="h-3 w-3" />,
    label: "From Helm values",
  },
  notConfigured: {
    icon: <Minus className="h-3 w-3" />,
    label: "Not configured",
  },
};

export function ProvenanceBadge({
  type,
  size = "sm",
  className,
}: ProvenanceBadgeProps) {
  const config = provenanceConfig[type];

  return (
    <Chip
      variant="secondary"
      color="default"
      size={size}
      data-type={type}
      className={className}
    >
      <div className="flex items-center gap-1">
        {config.icon}
        <span className="text-xs font-medium">{config.label}</span>
      </div>
    </Chip>
  );
}
