import type { JSX } from "react";
import { Link } from "@tanstack/react-router";

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
 * Renders the provided crumbs array as navigable links
 * (except the last item, which is marked as the current page via aria-current).
 * Uses TanStack Router's Link for navigation.
 */
export function Breadcrumbs({ items }: { items: Crumb[] }): JSX.Element {
  return (
    <nav aria-label="Breadcrumb" className="flex items-center gap-2 text-sm">
      <ol className="flex items-center gap-1">
        {items.map((crumb, index) => {
          const isLast = index === items.length - 1;
          return (
            <li
              key={crumb.to ?? crumb.label}
              className="flex items-center gap-1"
              aria-current={isLast ? "page" : undefined}
            >
              {crumb.to && !isLast ? (
                <Link
                  to={crumb.to}
                  className="hover:text-fg text-muted transition-colors"
                >
                  {crumb.label}
                </Link>
              ) : (
                <span className={isLast ? "text-fg" : "text-muted"}>
                  {crumb.label}
                </span>
              )}
              {!isLast && (
                <span className="text-muted" aria-hidden="true">
                  /
                </span>
              )}
            </li>
          );
        })}
      </ol>
    </nav>
  );
}
