import type { JSX } from "react";
import { Breadcrumbs as HeroBreadcrumbs } from "@heroui/react";
import { ChevronRight } from "lucide-react";

export interface Crumb {
  label: string;
  to?: string;
}

/**
 * Builds breadcrumb items from the current pathname.
 * Maps route parts to human-readable labels per the label map,
 * with "gameplane" as the home link and "Dashboard" as the leaf
 * when on the root path.
 */
export function buildCrumbs(pathname: string): Crumb[] {
  const crumbs: Crumb[] = [{ label: "gameplane", to: "/" }];
  const parts = pathname.split("/").filter(Boolean);
  const labels: Record<string, string> = {
    servers: "Servers",
    modules: "Modules",
    cluster: "Cluster",
    users: "Users & RBAC",
    admin: "Settings",
    audit: "Audit log",
    logs: "System logs",
    backups: "Backups",
    new: "New",
  };
  let acc = "";
  for (const p of parts) {
    acc += "/" + p;
    crumbs.push({ label: labels[p] ?? p, to: acc });
  }
  if (parts.length === 0) crumbs.push({ label: "Dashboard" });
  return crumbs;
}

/**
 * Route-hierarchy breadcrumb component.
 * Renders the provided crumbs array as navigable links (except the last item,
 * which is marked as the current page via aria-current).
 * Uses HeroUI Breadcrumbs with ChevronRight separator.
 */
export function Breadcrumbs({ items }: { items: Crumb[] }): JSX.Element {
  return (
    <nav aria-label="Breadcrumb">
      <HeroBreadcrumbs separator={<ChevronRight className="h-3.5 w-3.5 text-muted" />}>
        {items.map((crumb, index) => {
          const isLast = index === items.length - 1;
          return (
            <HeroBreadcrumbs.Item
              key={crumb.to ?? crumb.label}
              href={crumb.to && !isLast ? crumb.to : undefined}
              aria-current={isLast ? "page" : undefined}
              className="text-sm text-muted hover:text-foreground data-[current]:text-foreground data-[current]:hover:text-foreground"
            >
              {crumb.label}
            </HeroBreadcrumbs.Item>
          );
        })}
      </HeroBreadcrumbs>
    </nav>
  );
}
