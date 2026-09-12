import { type ReactNode } from "react";
import { ConfirmDialog } from "./ConfirmDialog";
import { AlertTriangle } from "lucide-react";

export interface ConfirmAdminMappingDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  adminGroups: string[];
  busy?: boolean;
  onConfirm: () => void;
}

export function ConfirmAdminMappingDialog({
  open,
  onOpenChange,
  adminGroups,
  busy,
  onConfirm,
}: ConfirmAdminMappingDialogProps) {
  const description: ReactNode = (
    <div className="space-y-4">
      <div className="rounded-md border border-warning/40 bg-warning/10 p-3 flex gap-3">
        <AlertTriangle className="h-4 w-4 text-warning flex-shrink-0 mt-0.5" />
        <div className="text-xs">
          <div className="font-medium text-warning mb-1">Full admin access</div>
          <p className="text-warning/80">
            Mapping users to the admin role grants full cluster control. Ensure the mapped group
            contains only authorized personnel. Anyone in these groups gets full admin access
            from their next login.
          </p>
        </div>
      </div>
      {adminGroups.length > 0 && (
        <div className="space-y-2">
          <p className="text-xs text-muted">Group(s) being mapped to admin:</p>
          <div className="flex flex-wrap gap-2">
            {adminGroups.map((group) => (
              <span
                key={group}
                className="px-2 py-1 rounded text-xs bg-muted/20 text-muted"
              >
                {group}
              </span>
            ))}
          </div>
        </div>
      )}
    </div>
  );

  return (
    <ConfirmDialog
      open={open}
      onOpenChange={onOpenChange}
      title="Confirm admin role mapping?"
      description={description}
      confirmLabel="Map to admin role"
      destructive
      busy={busy}
      onConfirm={onConfirm}
    />
  );
}
