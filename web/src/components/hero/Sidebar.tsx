import { useLocation } from "@tanstack/react-router";
import { Link } from "@tanstack/react-router";
import type { LucideIcon } from "lucide-react";
import { ShieldCheck, X, LogOut } from "lucide-react";
import {
  Drawer,
} from "@heroui/react";
import type { User } from "@/types";
import { cn } from "@/lib/utils";
import { AppearanceToggle, type AppearanceMode } from "./AppearanceToggle";

export interface SidebarNavItem {
  to: string;
  label: string;
  icon: LucideIcon;
}

export interface SidebarNavGroup {
  label: string;
  items: SidebarNavItem[];
}

export interface SidebarProps {
  navItems: SidebarNavGroup[];
  clusterName?: string;
  user?: User;
  variant: "fixed" | "drawer";
  isOpen?: boolean;
  onClose?: () => void;
  onNavigate?: () => void;
  onLogout: () => void | Promise<void>;
  theme?: AppearanceMode;
  onThemeChange?: (mode: AppearanceMode) => void;
}

export function Sidebar({
  navItems,
  clusterName,
  user,
  variant,
  isOpen,
  onClose,
  onNavigate,
  onLogout,
  theme = "system",
  onThemeChange,
}: SidebarProps) {
  const { pathname } = useLocation();

  // Compute which nav item is active (exact vs prefix matching per contract).
  const isActive = (to: string): boolean => {
    const exact = to === "/" || navItems.some((group) =>
      group.items.some((o) => o.to !== to && o.to.startsWith(to + "/"))
    );
    if (exact) return to === pathname;
    return pathname.startsWith(to);
  };

  const sidebarContent = (
    <aside className={cn(
      "flex flex-col border-r border-border bg-surface/60",
      variant === "fixed" ? "w-[260px] shrink-0" : "w-full h-full"
    )}>
      {/* Header */}
      <div className="flex items-center gap-2 px-5 py-4 border-b border-border">
        <div className="flex h-8 w-8 items-center justify-center rounded-md bg-primary/15">
          <ShieldCheck className="h-4 w-4 text-primary" />
        </div>
        <div className="min-w-0 flex-1 leading-tight">
          <div className="font-mono text-base font-semibold text-fg">gameplane</div>
          <div className="text-[11px] text-muted truncate">{clusterName || "—"}</div>
        </div>
        {variant === "drawer" && onClose && (
          <button
            type="button"
            aria-label="Close navigation"
            onClick={onClose}
            // Auto-focus so the drawer's own Escape-to-dismiss handling
            // (attached to the modal content, not a global listener) has
            // focus inside the overlay to bubble the keydown from.
            autoFocus
            className="rounded-md p-1.5 text-muted hover:bg-border/60 hover:text-fg"
          >
            <X className="h-4 w-4" />
          </button>
        )}
      </div>

      {/* Nav */}
      <nav className="flex-1 overflow-auto px-3 py-2 scrollbar-thin" aria-label="Primary">
        {navItems.map((group) => (
          group.items.length > 0 && (
            <div key={group.label}>
              <div className="px-3 pb-2 pt-3 text-[10px] font-semibold uppercase tracking-widest text-muted">
                {group.label}
              </div>
              <ul className="flex flex-col gap-0.5">
                {group.items.map(({ to, label, icon: Icon }) => (
                  <li key={to}>
                    <Link
                      to={to}
                      onClick={onNavigate}
                      className={cn(
                        "group flex items-center gap-3 rounded-md px-3 py-2 text-sm text-muted transition-colors",
                        "hover:bg-border/60 hover:text-fg",
                        isActive(to) && "bg-primary/10 text-primary"
                      )}
                    >
                      <Icon className="h-[18px] w-[18px] shrink-0" />
                      <span>{label}</span>
                    </Link>
                  </li>
                ))}
              </ul>
              {navItems.findIndex((g) => g === group) < navItems.length - 1 && group.items.length > 0 && (
                <div className="h-3" />
              )}
            </div>
          )
        ))}
      </nav>

      {/* Footer */}
      <div className="border-t border-border px-3 py-3 space-y-2">
        {/* Appearance toggle row */}
        <div className="flex justify-center">
          {onThemeChange && (
            <AppearanceToggle
              value={theme}
              onChange={onThemeChange}
            />
          )}
        </div>

        {/* User info + logout row */}
        <div className="flex items-center gap-3 rounded-md px-2 py-1.5">
          <div className="flex h-8 w-8 items-center justify-center rounded-full bg-primary/20 font-mono text-xs text-primary shrink-0">
            {(user?.displayName || user?.username || "G").slice(0, 2).toUpperCase()}
          </div>
          <div className="min-w-0 flex-1">
            <div className="truncate text-sm text-fg">
              {user?.displayName || user?.username || "guest"}
            </div>
            <div className="truncate text-[11px] text-muted">
              {user?.role ?? "—"}
            </div>
          </div>
          <button
            title="Sign out"
            className="rounded p-1 text-muted hover:bg-border/60 hover:text-fg transition-colors"
            onClick={onLogout}
          >
            <LogOut className="h-4 w-4" />
          </button>
        </div>
      </div>
    </aside>
  );

  // Render as drawer if variant is "drawer"
  if (variant === "drawer") {
    return (
      <Drawer.Root isOpen={isOpen ?? false} onOpenChange={(open) => { if (!open) onClose?.(); }}>
        <Drawer.Backdrop />
        <Drawer.Content placement="left">
          <Drawer.Dialog className="w-[280px] max-w-[85vw] p-0">
            <Drawer.Body className="p-0">
              {sidebarContent}
            </Drawer.Body>
          </Drawer.Dialog>
        </Drawer.Content>
      </Drawer.Root>
    );
  }

  // Render as fixed sidebar
  return sidebarContent;
}
