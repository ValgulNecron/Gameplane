import { useState } from "react";
import {
  Modal,
  ModalBackdrop,
  ModalContainer,
  ModalDialog,
  ModalHeader,
  ModalBody,
  ModalFooter,
  Button,
  Input,
  Label,
  Description,
} from "@heroui/react";
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
  // defaults: first source, latest version, canonical name. Adjusted
  // directly during render (not in an effect), gated on the
  // previously-seen (open, entry) pair.
  const [resetFor, setResetFor] = useState<{ open: boolean; entry: CatalogEntry | null }>({
    open: false,
    entry,
  });
  if (open !== resetFor.open || entry !== resetFor.entry) {
    setResetFor({ open, entry });
    if (open && entry) {
      setError(null);
      setSource(entry.sources[0]?.name ?? "");
      setVersion(entry.latestVersion ?? entry.versions?.[0] ?? "");
      setName(entry.name);
    }
  }

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
    <Modal isOpen={open} onOpenChange={onOpenChange}>
      <ModalBackdrop isDismissable={!busy} isKeyboardDismissDisabled={busy} />
      <ModalContainer>
        <ModalDialog>
          <ModalHeader>
            <h2 className="text-base font-semibold">
              Install {entry.displayName ?? entry.name}
            </h2>
          </ModalHeader>

          <ModalBody className="gap-4">
            <Description className="text-sm text-muted">
              Pulls the module bundle and creates a Module resource. The cluster
              operator materializes the GameTemplate in the background.
            </Description>

            <div className="space-y-4">
              {/* Source */}
              {entry.sources.length > 1 ? (
                <div className="space-y-1">
                  <Label htmlFor="source-select" className="text-xs">
                    Source
                  </Label>
                  <select
                    id="source-select"
                    value={source}
                    onChange={(e) => setSource(e.target.value)}
                    className="rounded border border-border bg-surface px-3 py-2 text-sm"
                  >
                    {entry.sources.map((s) => (
                      <option key={s.name} value={s.name}>
                        {s.name} ({s.type})
                      </option>
                    ))}
                  </select>
                </div>
              ) : (
                <div className="space-y-1">
                  <Label className="text-xs">Source</Label>
                  <div className="flex h-10 w-full items-center rounded-md border border-border bg-surface/40 px-3 font-mono text-sm text-fg">
                    {entry.sources[0]
                      ? `${entry.sources[0].name} (${entry.sources[0].type})`
                      : "—"}
                  </div>
                </div>
              )}

              {/* Version */}
              {versions.length > 1 ? (
                <div className="space-y-1">
                  <Label htmlFor="version-select" className="text-xs">
                    Version
                  </Label>
                  <select
                    id="version-select"
                    value={version}
                    onChange={(e) => setVersion(e.target.value)}
                    className="rounded border border-border bg-surface px-3 py-2 text-sm"
                  >
                    {versions.map((v) => (
                      <option key={v} value={v}>
                        {v}
                      </option>
                    ))}
                  </select>
                </div>
              ) : (
                <div className="space-y-1">
                  <Label className="text-xs">Version</Label>
                  <div className="flex h-10 w-full items-center rounded-md border border-border bg-surface/40 px-3 font-mono text-sm text-fg">
                    {versions[0] ?? "—"}
                  </div>
                </div>
              )}

              {/* Install as */}
              <div className="space-y-1">
                <Label htmlFor="install-name" className="text-xs">
                  Install as
                </Label>
                <Input
                  id="install-name"
                  value={name}
                  onChange={(e) => setName(e.target.value.toLowerCase())}
                  placeholder={entry.name}
                  className="mt-0"
                />
                <Description className="text-xs text-muted">
                  The Module + GameTemplate name. Must be a DNS label.
                </Description>
              </div>
            </div>

            {error && (
              <div
                role="alert"
                className="rounded border border-danger/40 bg-danger/10 px-3 py-2 text-xs text-danger"
              >
                {error}
              </div>
            )}
          </ModalBody>

          <ModalFooter className="flex items-center justify-end gap-2">
            <Button
              variant="secondary"
              size="sm"
              onPress={() => onOpenChange(false)}
              isDisabled={busy}
            >
              Cancel
            </Button>
            <Button
              size="sm"
              variant="primary"
              isDisabled={busy}
              onPress={submit}
            >
              Install
            </Button>
          </ModalFooter>
        </ModalDialog>
      </ModalContainer>
    </Modal>
  );
}
