import type { ReactNode } from "react";
import { Breadcrumbs } from "@heroui/react";
import { ChevronRight } from "lucide-react";

export type BreadcrumbItem = {
  label: ReactNode;
  href?: string;
};

export function PageHeader({
  breadcrumbs,
  title,
  description,
  actions,
}: {
  breadcrumbs?: BreadcrumbItem[];
  title: ReactNode;
  description?: ReactNode;
  actions?: ReactNode;
}) {
  return (
    <div className="flex flex-col gap-4 pb-2">
      {/* Breadcrumbs row */}
      {breadcrumbs && breadcrumbs.length > 0 && (
        <div className="flex">
          <Breadcrumbs separator={<ChevronRight className="h-4 w-4 text-muted" />}>
            {breadcrumbs.map((crumb, idx, arr) => {
              const isLast = idx === arr.length - 1;
              return (
                <Breadcrumbs.Item
                  key={idx}
                  href={crumb.href && !isLast ? crumb.href : undefined}
                  aria-current={isLast ? "page" : undefined}
                  className="text-sm text-muted hover:text-foreground data-[current]:text-foreground data-[current]:hover:text-foreground"
                >
                  {crumb.label}
                </Breadcrumbs.Item>
              );
            })}
          </Breadcrumbs>
        </div>
      )}

      {/* Title and actions row */}
      <div className="flex items-start justify-between gap-6">
        <div className="min-w-0 flex-1">
          <h1 className="truncate text-2xl font-semibold text-foreground">{title}</h1>
          {description && <p className="pt-1 text-sm text-muted">{description}</p>}
        </div>
        {actions && <div className="flex shrink-0 items-center gap-2">{actions}</div>}
      </div>
    </div>
  );
}
