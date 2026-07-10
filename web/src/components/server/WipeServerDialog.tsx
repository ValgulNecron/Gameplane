import { useMutation } from "@tanstack/react-query";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { Servers } from "@/lib/endpoints";
import { errorText } from "@/lib/errors";

interface Props {
  name: string;
  ns?: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Called after the world data is wiped — e.g. to refresh server state. */
  onWiped?: () => void;
}

// WipeServerDialog suspends the server and asks the operator to wipe its data
// volume. Self-contained (owns its mutation + error) for reuse across the
// Settings danger zone and the server action menus.
export function WipeServerDialog({ name, ns, open, onOpenChange, onWiped }: Props) {
  const wipe = useMutation({
    // The API's :wipe-data body echoes the server name as a typed confirmation.
    mutationFn: () => Servers.wipeData(name, name, ns),
    onSuccess: () => {
      onOpenChange(false);
      onWiped?.();
    },
  });

  return (
    <ConfirmDialog
      open={open}
      onOpenChange={(v) => {
        if (!v) wipe.reset();
        onOpenChange(v);
      }}
      title={`Wipe ${name}'s world data?`}
      description={
        <>
          The server will be suspended and the contents of its data volume
          permanently deleted. The GameServer itself is kept; re-enable
          auto-restart to start fresh. This cannot be undone.
          {wipe.isError && (
            <div className="pt-2 text-danger">{errorText(wipe.error, "wipe failed")}</div>
          )}
        </>
      }
      confirmPhrase={name}
      confirmLabel="Wipe world data"
      destructive
      busy={wipe.isPending}
      onConfirm={() => wipe.mutate()}
    />
  );
}
