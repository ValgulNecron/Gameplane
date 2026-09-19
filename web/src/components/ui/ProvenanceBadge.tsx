import React from "react";
import { Chip } from "@heroui/react";
import { Pencil, Package, Minus } from "lucide-react";

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
  variant: "soft" | "tertiary";
  color: "default" | "danger" | "warning" | "success";
}> = {
  // Neutral/outlined per R65Xyx (no fill, bordered, muted text/icon) —
  // this design was moved off the pink chip--soft pill; see the scoped
  // [data-type="overridden"] rule in globals.css for the border/muted color.
  overridden: {
    icon: <Pencil className="h-2.5 w-2.5" />,
    label: "Overridden in dashboard",
    variant: "tertiary",
    color: "default",
  },
  fromHelm: {
    icon: <Package className="h-2.5 w-2.5" />,
    label: "From Helm values",
    variant: "soft",
    color: "default",
  },
  notConfigured: {
    icon: <Minus className="h-2.5 w-2.5" />,
    label: "Not configured",
    variant: "soft",
    color: "default",
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
      variant={config.variant}
      color={config.color}
      size={size}
      data-type={type}
      className={
        className
          ? `rounded-full px-2 py-0.5 ${className}`
          : "rounded-full px-2 py-0.5"
      }
    >
      <div className="flex items-center gap-[5px]">
        {config.icon}
        <span className="font-mono text-[10px] font-medium leading-[13px]">{config.label}</span>
      </div>
    </Chip>
  );
}
