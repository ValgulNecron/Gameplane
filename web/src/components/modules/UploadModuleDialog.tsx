import { useEffect, useRef, useState, type ReactNode } from "react";
import * as Dialog from "@radix-ui/react-dialog";
import { FileArchive, Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Select } from "@/components/ui/select";
import { APIError } from "@/lib/api";
import { ModuleSources, type UploadedModule } from "@/lib/endpoints";

interface UploadModuleDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  // Names of the upload-type ModuleSources that can receive bundles.
  sources: string[];
  onUploaded: () => Promise<void> | void;
}

// UploadModuleDialog lets an admin add a module by uploading a bundle
// archive (.tar.gz/.zip with module.yaml + template.yaml). The file is
// validated server-side via a dry run first, so the user sees the
// parsed metadata before committing.
export function UploadModuleDialog({ open, onOpenChange, sources, onUploaded }: UploadModuleDialogProps) {
  const fileInput = useRef<HTMLInputElement>(null);
  const [source, setSource] = useState("");
  const [file, setFile] = useState<File | null>(null);
  const [preview, setPreview] = useState<UploadedModule | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (!open) return;
    setSource(sources[0] ?? "");
    setFile(null);
    setPreview(null);
    setError(null);
  }, [open, sources]);

  async function pick(f: File | null) {
    setFile(f);
    setPreview(null);
    setError(null);
    if (!f || !source) return;
    setBusy(true);
    try {
      setPreview(await ModuleSources.upload(source, f, { dryRun: true }));
    } catch (err) {
      setError(err instanceof APIError ? err.body || err.message : (err as Error).message);
    } finally {
      setBusy(false);
    }
  }

  async function submit() {
    if (!file || !source) {
      setError("pick a bundle file first");
      return;
    }
    setBusy(true);
    setError(null);
    try {
      await ModuleSources.upload(source, file);
      await onUploaded();
      onOpenChange(false);
    } catch (err) {
      setError(err instanceof APIError ? err.body || err.message : (err as Error).message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 z-40 bg-black/60" />
        <Dialog.Content className="fixed left-1/2 top-1/2 z-50 w-[480px] max-w-[calc(100vw-2rem)] -translate-x-1/2 -translate-y-1/2 rounded-lg border border-border bg-card p-5 text-fg shadow-2xl">
          <Dialog.Title className="text-base font-semibold">Upload module</Dialog.Title>
          <Dialog.Description className="pt-1 text-xs text-muted">
            A .tar.gz or .zip holding one module directory (module.yaml +
            template.yaml). Stored in the cluster; the catalog indexes it
            immediately.
          </Dialog.Description>

          <div className="space-y-3 pt-4">
            {sources.length > 1 && (
              <Field label="Upload to">
                <Select
                  value={source}
                  onValueChange={setSource}
                  options={sources.map((s) => ({ value: s, label: s }))}
                />
              </Field>
            )}

            <input
              ref={fileInput}
              type="file"
              accept=".tar.gz,.tgz,.zip,application/gzip,application/zip"
              className="hidden"
              data-testid="bundle-file-input"
              onChange={(e) => void pick(e.target.files?.[0] ?? null)}
            />
            <button
              type="button"
              onClick={() => fileInput.current?.click()}
              className="flex w-full items-center justify-center gap-2 rounded-lg border border-dashed border-border bg-card/40 px-4 py-6 text-sm text-muted transition-colors hover:border-primary hover:text-fg"
            >
              <FileArchive className="h-4 w-4" />
              {file ? file.name : "Choose a bundle archive…"}
            </button>

            {preview && (
              <div className="rounded border border-border bg-card/40 px-3 py-2 text-xs">
                <div className="font-medium text-fg">
                  {preview.module.displayName}{" "}
                  <span className="font-mono text-muted">v{preview.module.version}</span>
                </div>
                <div className="pt-0.5 text-muted">
                  game: <span className="font-mono">{preview.module.game}</span>
                  {preview.module.summary && <> — {preview.module.summary}</>}
                </div>
              </div>
            )}
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
            <Button onClick={submit} disabled={busy || !preview}>
              {busy ? <Loader2 className="h-4 w-4 animate-spin" /> : null}
              Upload
            </Button>
          </div>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}

function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <label className="block">
      <div className="pb-1 text-xs text-muted">{label}</div>
      {children}
    </label>
  );
}
