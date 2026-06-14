import { useEffect, type ReactNode } from "react";
import { ShieldAlert } from "lucide-react";
import { Link } from "@tanstack/react-router";
import { APIError } from "@/lib/api";
import { useMe, can, type Role } from "@/lib/auth";
import { Card } from "@/components/ui/card";

interface Props {
  roles: readonly Role[];
  children: ReactNode;
}

export function RequireRole({ roles, children }: Props) {
  const { data: me, error, isLoading } = useMe();

  useEffect(() => {
    if (error instanceof APIError && error.status === 401) {
      location.assign("/login");
    }
  }, [error]);

  if (isLoading) return <RoleSkeleton />;
  if (error instanceof APIError && error.status === 401) return <RoleSkeleton />;
  if (!me || !roles.includes(me.role)) return <Forbidden />;
  return <>{children}</>;
}

interface PermProps {
  perm: string;
  children: ReactNode;
}

/**
 * RequirePermission gates a page on a single permission instead of a fixed
 * role list, so custom roles that hold the permission get access. Cluster-
 * scoped pages (Users, Settings, Audit) are the natural fit.
 */
export function RequirePermission({ perm, children }: PermProps) {
  const { data: me, error, isLoading } = useMe();

  useEffect(() => {
    if (error instanceof APIError && error.status === 401) {
      location.assign("/login");
    }
  }, [error]);

  if (isLoading) return <RoleSkeleton />;
  if (error instanceof APIError && error.status === 401) return <RoleSkeleton />;
  if (!can(me, perm)) return <Forbidden />;
  return <>{children}</>;
}

function RoleSkeleton() {
  return (
    <div className="p-6">
      <Card className="h-32 animate-pulse" />
    </div>
  );
}

function Forbidden() {
  return (
    <div className="flex h-full items-center justify-center p-6">
      <Card className="max-w-md p-8 text-center">
        <div className="flex justify-center">
          <div className="flex h-12 w-12 items-center justify-center rounded-full bg-warning/15 text-warning">
            <ShieldAlert className="h-6 w-6" />
          </div>
        </div>
        <div className="pt-4 text-base font-medium text-fg">Access denied</div>
        <p className="pt-1 text-sm text-muted">
          Your role doesn&apos;t permit access to this page. Ask an
          administrator if you believe this is a mistake.
        </p>
        <Link
          to="/"
          className="mt-4 inline-block rounded-md bg-primary px-4 py-2 text-sm text-primary-fg hover:opacity-90"
        >
          Back to dashboard
        </Link>
      </Card>
    </div>
  );
}
