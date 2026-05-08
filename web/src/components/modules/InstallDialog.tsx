import { useEffect, useState, type ReactNode } from "react";
import * as Dialog from "@radix-ui/react-dialog";
import { Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { APIError } from "@/lib/api";
import type { CatalogEntry } from "@/types";

interface InstallDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  entry: CatalogEntry | null;
  onConfirm: (args: { source: string; version: string; name: string }) => Promise<void> | void;
  busy?: boolean;
}

// InstallDialog asks the user which source + version to install from
// when a CatalogEntry is published by more than one ModuleSource or
// has multiple available versions. The "name" defaults to the module's
// canonical name but can be edited so the same bundle can be installed
// twice under different names (e.g. for staging/prod side-by-side).
export function InstallDialog({ open, onOpenChange, entry, onConfirm, busy }: InstallDialogProps) {
  const [source, setSource] = useState("");
  const [version, setVersion] = useState("");
  const [name, setName] = useState("");
  const [error, setError] = useState<string | null>(null);

  // When the entry changes (or the dialog re-opens), reset to its
  // defaults: first source, latest version, canonical name.
  useEffect(() => {
    if (!open || !entry) return;
    setError(null);
    setSource(entry.sources[0] ?? "");
    setVersion(entry.latestVersion ?? entry.versions?.[0] ?? "");
    setName(entry.name);
  }, [open, entry]);

  if (!entry) return null;

  const versions = entry.versions ?? (entry.latestVersion ? [entry.latestVersion] : []);

  async function submit() {
    if (!source || !version || !name) {
      setError("source, version, and name are all required");
      return;
    }
    setError(null);
    try {
      await onConfirm({ source, version, name });
    } catch (err) {
      setError(err instanceof APIError ? err.body || err.message : (err as Error).message);
    }
  }

  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 z-40 bg-black/60" />
        <Dialog.Content className="fixed left-1/2 top-1/2 z-50 w-[480px] max-w-[calc(100vw-2rem)] -translate-x-1/2 -translate-y-1/2 rounded-lg border border-border bg-card p-5 text-fg shadow-2xl">
          <Dialog.Title className="text-base font-semibold">
            Install {entry.displayName ?? entry.name}
          </Dialog.Title>
          <Dialog.Description className="pt-1 text-xs text-muted">
            Pulls the OCI bundle and creates a Module resource. The cluster
            operator materializes the GameTemplate in the background.
          </Dialog.Description>

          <div className="space-y-3 pt-4">
            <Field label="Source">
              <Select
                value={source}
                onValueChange={setSource}
                options={entry.sources.map((s) => ({ value: s, label: s }))}
                disabled={entry.sources.length <= 1}
              />
            </Field>
            <Field label="Version">
              <Select
                value={version}
                onValueChange={setVersion}
                options={versions.map((v) => ({ value: v, label: v }))}
                disabled={versions.length <= 1}
              />
            </Field>
            <Field
              label="Install as"
              hint="The Module + GameTemplate name. Must be a DNS label."
            >
              <Input
                value={name}
                onChange={(e) => setName(e.target.value.toLowerCase())}
                placeholder={entry.name}
              />
            </Field>
          </div>

          {error && (
            <div className="mt-3 rounded border border-danger/40 bg-danger/10 px-3 py-2 text-xs text-danger">
              {error}
            </div>
          )}

          <div className="mt-5 flex justify-end gap-2">
            <Button variant="outline" onClick={() => onOpenChange(false)} disabled={busy}>
              Cancel
            </Button>
            <Button onClick={submit} disabled={busy}>
              {busy ? <Loader2 className="h-4 w-4 animate-spin" /> : null}
              Install
            </Button>
          </div>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}

function Field({ label, hint, children }: { label: string; hint?: string; children: ReactNode }) {
  return (
    <label className="block">
      <div className="pb-1 text-xs text-muted">{label}</div>
      {children}
      {hint && <div className="pt-1 text-[11px] text-muted">{hint}</div>}
    </label>
  );
}
