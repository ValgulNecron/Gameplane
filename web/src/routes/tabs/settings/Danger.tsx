import { useState, type ReactNode } from "react";
import { useMutation } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { Servers } from "@/lib/endpoints";
import { APIError } from "@/lib/api";

interface Props {
  name: string;
}

export function DangerSection({ name }: Props) {
  const navigate = useNavigate();
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const del = useMutation({
    mutationFn: () => Servers.remove(name),
    onSuccess: () => {
      setConfirmOpen(false);
      return navigate({ to: "/servers" });
    },
    onError: (err) => {
      setError(errMsg(err));
    },
  });

  return (
    <div className="space-y-3">
      <Row
        title="Wipe world data"
        body="Deletes the contents of the persistent volume but keeps the GameServer. Restarts back to a fresh install."
        action={
          <Button variant="outline" size="sm" disabled title="Coming soon">
            Wipe world…
          </Button>
        }
      />
      <Row
        title="Transfer ownership"
        body="Hands the server over to another user."
        action={
          <Button variant="outline" size="sm" disabled title="Coming soon">
            Transfer…
          </Button>
        }
      />
      <Row
        title="Delete server"
        body="Removes the GameServer resource and its persistent volume. This cannot be undone."
        action={
          <Button
            variant="danger"
            size="sm"
            onClick={() => {
              setError(null);
              setConfirmOpen(true);
            }}
          >
            Delete server…
          </Button>
        }
      />
      {error && (
        <div className="rounded border border-danger/40 bg-danger/10 px-3 py-2 text-sm text-danger">
          {error}
        </div>
      )}
      <ConfirmDialog
        open={confirmOpen}
        onOpenChange={setConfirmOpen}
        title={`Delete ${name}?`}
        description={
          <>
            This will permanently remove the GameServer resource and detach
            its persistent volume. Data will be reclaimed by the cluster
            according to the StorageClass reclaim policy.
          </>
        }
        confirmPhrase={name}
        confirmLabel="Delete server"
        destructive
        busy={del.isPending}
        onConfirm={() => del.mutate()}
      />
    </div>
  );
}

function Row({
  title,
  body,
  action,
}: {
  title: string;
  body: string;
  action: ReactNode;
}) {
  return (
    <div className="flex items-start justify-between gap-4 rounded border border-border bg-surface/30 p-4">
      <div className="min-w-0">
        <div className="text-sm font-medium text-fg">{title}</div>
        <div className="pt-1 text-xs text-muted">{body}</div>
      </div>
      {action}
    </div>
  );
}

function errMsg(err: unknown): string {
  if (err instanceof APIError) {
    try {
      const parsed = JSON.parse(err.body) as { error?: string };
      if (parsed.error) return parsed.error;
    } catch {
      // fall through
    }
    return err.body || `request failed (${err.status})`;
  }
  return err instanceof Error ? err.message : "delete failed";
}
