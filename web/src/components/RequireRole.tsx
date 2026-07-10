import { useEffect, type ReactNode } from "react";
import { ShieldAlert, PlugZap } from "lucide-react";
import { Link } from "@tanstack/react-router";
import { APIError } from "@/lib/api";
import { useMe, can, type Role } from "@/lib/auth";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";

function isUnauthorized(error: unknown): boolean {
  return error instanceof APIError && error.status === 401;
}

interface Props {
  roles: readonly Role[];
  children: ReactNode;
}

export function RequireRole({ roles, children }: Props) {
  const { data: me, error, isLoading, refetch } = useMe();

  useEffect(() => {
    if (isUnauthorized(error)) location.assign("/login");
  }, [error]);

  if (isLoading) return <RoleSkeleton />;
  if (isUnauthorized(error)) return <RoleSkeleton />;
  if (!me) return <IdentityUnavailable error={error} onRetry={() => void refetch()} />;
  if (!roles.includes(me.role)) return <Forbidden />;
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
  const { data: me, error, isLoading, refetch } = useMe();

  useEffect(() => {
    if (isUnauthorized(error)) location.assign("/login");
  }, [error]);

  if (isLoading) return <RoleSkeleton />;
  if (isUnauthorized(error)) return <RoleSkeleton />;
  if (!me) return <IdentityUnavailable error={error} onRetry={() => void refetch()} />;
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

/**
 * IdentityUnavailable covers the case where /users/me could not be loaded for
 * a reason other than 401 — an API rollout, a proxy error, a stalled query.
 * The user's permissions are unknown, not absent, so claiming "access denied"
 * would be a lie that only a manual reload could undo.
 */
function IdentityUnavailable({ error, onRetry }: { error: unknown; onRetry: () => void }) {
  const detail = error instanceof APIError ? `HTTP ${error.status}` : null;
  return (
    <div className="flex h-full items-center justify-center p-6">
      <Card className="max-w-md p-8 text-center">
        <div className="flex justify-center">
          <div className="flex h-12 w-12 items-center justify-center rounded-full bg-muted/15 text-muted">
            <PlugZap className="h-6 w-6" />
          </div>
        </div>
        <div className="pt-4 text-base font-medium text-fg">
          Can&apos;t reach the control plane
        </div>
        <p className="pt-1 text-sm text-muted">
          We couldn&apos;t confirm who you are{detail ? ` (${detail})` : ""}. This is usually
          temporary — the API may be restarting.
        </p>
        <Button className="mt-4" onClick={onRetry}>
          Try again
        </Button>
      </Card>
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
