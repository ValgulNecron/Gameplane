import type { ReactNode } from "react";
import * as Menu from "@radix-ui/react-dropdown-menu";
import { cn } from "@/lib/utils";

// Thin styled wrappers over Radix DropdownMenu. Root/Trigger/Portal pass
// through unchanged; Content and Separator carry the house styling so
// consumers don't restate it.

export const DropdownMenu = Menu.Root;
export const DropdownMenuTrigger = Menu.Trigger;

export function DropdownMenuContent({
  children,
  align = "end",
}: {
  children: ReactNode;
  align?: "start" | "center" | "end";
}) {
  return (
    <Menu.Portal>
      <Menu.Content
        align={align}
        sideOffset={4}
        className="z-50 min-w-[180px] rounded-md border border-border bg-card p-1 text-sm shadow-lg"
      >
        {children}
      </Menu.Content>
    </Menu.Portal>
  );
}

export function DropdownMenuSeparator() {
  return <Menu.Separator className="my-1 h-px bg-border" />;
}

export function DropdownMenuItem({
  icon,
  label,
  onSelect,
  destructive,
  disabled,
  hint,
}: {
  icon: ReactNode;
  label: string;
  onSelect: () => void;
  destructive?: boolean;
  disabled?: boolean;
  hint?: string;
}) {
  return (
    <Menu.Item
      onSelect={(e) => {
        if (disabled) {
          e.preventDefault();
          return;
        }
        onSelect();
      }}
      disabled={disabled}
      title={hint}
      className={cn(
        "flex cursor-pointer items-center gap-2 rounded px-2 py-1.5 outline-none",
        destructive ? "text-destructive" : "text-fg",
        disabled
          ? "opacity-50 cursor-not-allowed"
          : "data-[highlighted]:bg-surface/70",
      )}
    >
      {icon}
      <span>{label}</span>
    </Menu.Item>
  );
}
