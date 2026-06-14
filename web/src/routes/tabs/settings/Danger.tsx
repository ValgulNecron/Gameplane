import { useState, type ReactNode } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import * as Dialog from "@radix-ui/react-dialog";
import { Button } from "@/components/ui/button";
import { Select } from "@/components/ui/select";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { Servers, Users } from "@/lib/endpoints";
import { APIError } from "@/lib/api";

const OWNER_ANNOTATION = "kestrel.gg/owner";

interface Props {
  name: string;
}

export function DangerSection({ name }: Props) {
  const navigate = useNavigate();
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [wipeOpen, setWipeOpen] = useState(false);
  const [transferOpen, setTransferOpen] = useState(false);
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

  const wipe = useMutation({
    mutationFn: () => Servers.wipeData(name, name),
    onSuccess: () => setWipeOpen(false),
    onError: (err) => {
      setWipeOpen(false);
      setError(errMsg(err));
    },
  });

  return (
    <div className="space-y-3">
      <Row
        title="Wipe world data"
        body="Suspends the server and deletes the contents of its persistent volume, then you can restart into a fresh install. Keeps the GameServer."
        action={
          <Button
            variant="outline"
            size="sm"
            onClick={() => {
              setError(null);
              setWipeOpen(true);
            }}
          >
            Wipe world…
          </Button>
        }
      />
      <Row
        title="Transfer ownership"
        body="Hands the server over to another user. Ownership is informational."
        action={
          <Button variant="outline" size="sm" onClick={() => setTransferOpen(true)}>
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
      <TransferDialog name={name} open={transferOpen} onOpenChange={setTransferOpen} />
      <ConfirmDialog
        open={wipeOpen}
        onOpenChange={setWipeOpen}
        title={`Wipe ${name}'s world data?`}
        description={
          <>
            The server will be suspended and the contents of its data volume
            permanently deleted. The GameServer itself is kept; re-enable
            auto-restart to start fresh. This cannot be undone.
          </>
        }
        confirmPhrase={name}
        confirmLabel="Wipe world data"
        destructive
        busy={wipe.isPending}
        onConfirm={() => wipe.mutate()}
      />
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

function TransferDialog({
  name,
  open,
  onOpenChange,
}: {
  name: string;
  open: boolean;
  onOpenChange: (v: boolean) => void;
}) {
  const qc = useQueryClient();
  const [userId, setUserId] = useState("");
  const { data: server } = useQuery({
    queryKey: ["server", name],
    queryFn: () => Servers.get(name),
    enabled: open,
  });
  const { data: users = [], error: usersError } = useQuery({
    queryKey: ["users"],
    queryFn: () => Users.list(),
    enabled: open,
  });
  const currentOwner = server?.metadata.annotations?.[OWNER_ANNOTATION];

  const transfer = useMutation({
    mutationFn: () => Servers.transfer(name, Number(userId)),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["server", name] });
      onOpenChange(false);
    },
  });

  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 z-40 bg-black/60" />
        <Dialog.Content className="fixed left-1/2 top-1/2 z-50 w-[440px] max-w-[calc(100vw-2rem)] -translate-x-1/2 -translate-y-1/2 rounded-lg border border-border bg-card p-5 text-fg shadow-2xl">
          <Dialog.Title className="text-base font-semibold">
            Transfer {name}
          </Dialog.Title>
          <Dialog.Description asChild>
            <div className="pt-1 text-sm text-muted">
              Current owner: {currentOwner || "unassigned"}.
            </div>
          </Dialog.Description>
          <div className="space-y-3 pt-4">
            {usersError ? (
              <div className="text-xs text-danger">
                You need permission to list users to pick a recipient.
              </div>
            ) : (
              <Select
                value={userId}
                onValueChange={setUserId}
                options={[
                  { value: "", label: "Select a user…" },
                  ...users.map((u) => ({ value: String(u.id), label: u.username })),
                ]}
              />
            )}
            {transfer.error instanceof APIError && (
              <div className="text-xs text-danger">
                {transfer.error.body || "Transfer failed."}
              </div>
            )}
            <div className="flex justify-end gap-2 pt-1">
              <Button variant="ghost" onClick={() => onOpenChange(false)}>
                Cancel
              </Button>
              <Button
                disabled={!userId || transfer.isPending}
                onClick={() => transfer.mutate()}
              >
                {transfer.isPending ? "Transferring…" : "Transfer"}
              </Button>
            </div>
          </div>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
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
