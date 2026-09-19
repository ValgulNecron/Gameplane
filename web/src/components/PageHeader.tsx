import type { ReactNode } from "react";
import { PageHeader as HeroPageHeader, type BreadcrumbItem } from "@/components/ui/PageHeader";

export { type BreadcrumbItem };

export function PageHeader({
  title,
  subtitle,
  actions,
  breadcrumbs,
}: {
  title: ReactNode;
  subtitle?: ReactNode;
  actions?: ReactNode;
  breadcrumbs?: BreadcrumbItem[];
}) {
  return (
    <HeroPageHeader
      title={title}
      description={subtitle}
      actions={actions}
      breadcrumbs={breadcrumbs}
    />
  );
}
