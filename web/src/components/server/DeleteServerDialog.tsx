import { useMutation } from "@tanstack/react-query";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { Servers } from "@/lib/endpoints";
import { errorText } from "@/lib/errors";

interface Props {
  name: string;
  ns?: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Called after the server is deleted — navigate away or invalidate lists. */
  onDeleted?: () => void;
}

// DeleteServerDialog is the confirm-to-delete flow, self-contained (owns its
// mutation + error) so it can be dropped into the Settings danger zone, the
// server-detail header menu, or a Servers-list row menu alike.
export function DeleteServerDialog({ name, ns, open, onOpenChange, onDeleted }: Props) {
  const del = useMutation({
    mutationFn: () => Servers.remove(name, ns),
    onSuccess: () => {
      onOpenChange(false);
      onDeleted?.();
    },
  });

  return (
    <ConfirmDialog
      open={open}
      onOpenChange={(v) => {
        if (!v) del.reset();
        onOpenChange(v);
      }}
      title={`Delete ${name}?`}
      description={
        <>
          This will permanently remove the GameServer resource and detach its
          persistent volume. Data will be reclaimed by the cluster according to
          the StorageClass reclaim policy.
          {del.isError && (
            <div className="pt-2 text-danger">{errorText(del.error, "delete failed")}</div>
          )}
        </>
      }
      confirmPhrase={name}
      confirmLabel="Delete server"
      destructive
      busy={del.isPending}
      onConfirm={() => del.mutate()}
    />
  );
}
