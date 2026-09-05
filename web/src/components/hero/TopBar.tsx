import { Menu } from "lucide-react";
import type { ReactNode } from "react";
import {
  Button,
  Avatar,
  Dropdown,
  DropdownTrigger,
  DropdownMenu,
  DropdownItem,
} from "@heroui/react";
import { Auth } from "@/lib/endpoints";
import type { User } from "@/types";
import type { JSX } from "react";

export interface TopBarProps {
  /** Breadcrumb navigation slot — typically <Breadcrumbs items={...} /> */
  breadcrumbs: ReactNode;
  /** Cluster selector slot — typically <ClusterSelector /> */
  clusterSelector: ReactNode;
  /** Search slot — typically <GlobalSearch /> */
  search: ReactNode;
  /** Notifications panel slot — typically <NotificationsPanel /> */
  notifications: ReactNode;
  /** Current user for display in the avatar menu */
  user?: User;
  /** Called when hamburger button is clicked (mobile navigation trigger) */
  onMenuClick: () => void;
}

/**
 * TopBar is a pure layout component that composes the application header.
 * It accepts four `ReactNode` slots for its major sections (breadcrumbs, cluster
 * selector, search, notifications) and renders them alongside the mobile hamburger
 * button and user avatar dropdown menu. Each slot component owns its own data fetching.
 */
export function TopBar({
  breadcrumbs,
  clusterSelector,
  search,
  notifications,
  user,
  onMenuClick,
}: TopBarProps): JSX.Element {
  const name = user?.displayName || user?.username || "guest";
  const initials = name.slice(0, 2).toUpperCase();

  const handleLogout = async () => {
    try {
      await Auth.logout().catch(() => {});
    } finally {
      location.assign("/login");
    }
  };

  return (
    <header className="flex h-14 items-center justify-between gap-4 border-b border-border bg-background px-3 sm:px-6">
      {/* Left: hamburger + breadcrumbs */}
      <div className="flex min-w-0 items-center gap-2">
        <Button
          isIconOnly
          variant="ghost"
          size="sm"
          aria-label="Open navigation"
          onClick={onMenuClick}
          className="lg:hidden"
        >
          <Menu className="h-5 w-5" />
        </Button>
        <div className="min-w-0 text-sm text-muted">{breadcrumbs}</div>
      </div>

      {/* Right: cluster selector, search, notifications, user menu */}
      <div className="flex shrink-0 items-center gap-3">
        {clusterSelector}
        {search}
        {notifications}

        {/* User avatar dropdown menu */}
        <Dropdown>
          <DropdownTrigger>
            <Avatar
              size="sm"
              className="cursor-pointer"
              color="default"
              aria-label="User menu"
            >
              <Avatar.Fallback>{initials}</Avatar.Fallback>
            </Avatar>
          </DropdownTrigger>
          <DropdownMenu disabledKeys={["profile"]} className="w-48">
            <DropdownItem key="profile" className="py-2">
              <div className="flex flex-col">
                <span className="font-semibold text-foreground">{name}</span>
                <span className="text-xs text-muted">{user?.role ?? "—"}</span>
              </div>
            </DropdownItem>
            <DropdownItem
              key="logout"
              onClick={handleLogout}
              className="text-danger"
            >
              Sign out
            </DropdownItem>
          </DropdownMenu>
        </Dropdown>
      </div>
    </header>
  );
}
