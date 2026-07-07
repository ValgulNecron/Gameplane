import {
  useEffect,
  useRef,
  useState,
  type ChangeEvent,
  type ReactNode,
} from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import * as Dialog from "@radix-ui/react-dialog";
import Editor from "@monaco-editor/react";
import {
  Download,
  File as FileIcon,
  FilePlus,
  Folder,
  FolderPlus,
  Loader2,
  RefreshCw,
  Save,
  Trash2,
  Upload,
} from "lucide-react";

import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { Input } from "@/components/ui/input";
import { Files, type FileEntry } from "@/lib/endpoints";
import { cn, formatBytes } from "@/lib/utils";

const ROOT = "/";

export function FilesTab({ name, ns }: { name: string; ns?: string }) {
  const qc = useQueryClient();
  const listKey = (cwd: string) => ["files", name, cwd, ns] as const;

  const [cwd, setCwd] = useState(ROOT);
  const [selected, setSelected] = useState<FileEntry | null>(null);
  const [serverContent, setServerContent] = useState<string | null>(null);
  const [editorValue, setEditorValue] = useState<string>("");
  const [loadError, setLoadError] = useState<string | null>(null);
  const [opError, setOpError] = useState<string | null>(null);

  const [confirmDelete, setConfirmDelete] = useState<FileEntry | null>(null);
  const [mkdirOpen, setMkdirOpen] = useState(false);
  const [newFileOpen, setNewFileOpen] = useState(false);

  const uploadInputRef = useRef<HTMLInputElement | null>(null);

  const dirty = serverContent !== null && editorValue !== serverContent;

  const { data: entries, isFetching, refetch } = useQuery({
    queryKey: listKey(cwd),
    queryFn: () => Files.list(name, cwd, ns),
  });

  // Load file contents when a file is selected (and only then). Folder
  // navigation is handled in onEntryClick to avoid an effect cascade.
  useEffect(() => {
    if (!selected || selected.dir) return;
    let aborted = false;
    setLoadError(null);
    Files.read(name, selected.path, ns)
      .then((text) => {
        if (aborted) return;
        setServerContent(text);
        setEditorValue(text);
      })
      .catch((err: Error) => {
        if (aborted) return;
        setServerContent(null);
        setEditorValue("");
        setLoadError(err.message);
      });
    return () => {
      aborted = true;
    };
  }, [selected, name, ns]);

  function navigateTo(path: string) {
    if (dirty && !confirmDiscard()) return;
    setSelected(null);
    setServerContent(null);
    setEditorValue("");
    setCwd(path);
  }

  function onEntryClick(e: FileEntry) {
    if (e.dir) {
      navigateTo(e.path);
      return;
    }
    if (selected?.path === e.path) return;
    if (dirty && !confirmDiscard()) return;
    setSelected(e);
  }

  const saveMutation = useMutation({
    mutationFn: async (body: string) => {
      if (!selected) throw new Error("no file selected");
      await Files.write(name, selected.path, body, ns);
    },
    onSuccess: async (_data, body) => {
      setServerContent(body);
      await qc.invalidateQueries({ queryKey: listKey(cwd) });
      setOpError(null);
    },
    onError: (err: Error) => setOpError(err.message),
  });

  const deleteMutation = useMutation({
    mutationFn: async (entry: FileEntry) => {
      await Files.remove(name, entry.path, entry.dir, ns);
      return entry;
    },
    onSuccess: async (entry) => {
      if (selected?.path === entry.path) {
        setSelected(null);
        setServerContent(null);
        setEditorValue("");
      }
      setConfirmDelete(null);
      await qc.invalidateQueries({ queryKey: listKey(cwd) });
      setOpError(null);
    },
    onError: (err: Error) => setOpError(err.message),
  });

  const mkdirMutation = useMutation({
    mutationFn: (folderName: string) =>
      Files.mkdir(name, joinPath(cwd, folderName), ns),
    onSuccess: async () => {
      setMkdirOpen(false);
      await qc.invalidateQueries({ queryKey: listKey(cwd) });
      setOpError(null);
    },
    onError: (err: Error) => setOpError(err.message),
  });

  const newFileMutation = useMutation({
    mutationFn: async (fileName: string) => {
      const path = joinPath(cwd, fileName);
      await Files.write(name, path, "", ns);
      return path;
    },
    onSuccess: async (path) => {
      setNewFileOpen(false);
      await qc.invalidateQueries({ queryKey: listKey(cwd) });
      setSelected({
        name: path.slice(path.lastIndexOf("/") + 1),
        path,
        size: 0,
        dir: false,
      });
      setOpError(null);
    },
    onError: (err: Error) => setOpError(err.message),
  });

  const uploadMutation = useMutation({
    mutationFn: (files: FileList) => Files.upload(name, cwd, files, ns),
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: listKey(cwd) });
      setOpError(null);
    },
    onError: (err: Error) => setOpError(err.message),
  });

  function onUploadPicked(e: ChangeEvent<HTMLInputElement>) {
    const files = e.target.files;
    if (files && files.length > 0) uploadMutation.mutate(files);
    e.target.value = "";
  }

  function downloadSelected() {
    if (!selected || selected.dir) return;
    window.location.href = Files.downloadURL(name, selected.path, ns);
  }

  return (
    <div className="flex h-full flex-col gap-3">
      <div className="flex items-center justify-between gap-3">
        <Breadcrumbs cwd={cwd} onNavigate={navigateTo} />
        <div className="flex items-center gap-2">
          <Button variant="outline" size="sm" onClick={() => setNewFileOpen(true)}>
            <FilePlus className="h-4 w-4" /> New file
          </Button>
          <Button variant="outline" size="sm" onClick={() => setMkdirOpen(true)}>
            <FolderPlus className="h-4 w-4" /> New folder
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={() => uploadInputRef.current?.click()}
            disabled={uploadMutation.isPending}
          >
            {uploadMutation.isPending ? (
              <Loader2 className="h-4 w-4 animate-spin" />
            ) : (
              <Upload className="h-4 w-4" />
            )}{" "}
            Upload
          </Button>
          <input
            ref={uploadInputRef}
            type="file"
            multiple
            className="hidden"
            onChange={onUploadPicked}
            data-testid="files-upload-input"
          />
          <Button
            variant="outline"
            size="icon"
            aria-label="Refresh"
            onClick={() => refetch()}
          >
            <RefreshCw className={cn("h-4 w-4", isFetching && "animate-spin")} />
          </Button>
        </div>
      </div>

      {opError && (
        <div className="rounded border border-danger/40 bg-danger/10 px-3 py-2 text-xs text-danger">
          {opError}
        </div>
      )}

      <div className="flex min-h-0 flex-1 overflow-hidden rounded border border-border bg-card">
        <aside className="flex w-72 shrink-0 flex-col border-r border-border">
          <div className="border-b border-border px-3 py-2 font-mono text-xs text-muted">
            {cwd}
          </div>
          <ul className="flex-1 overflow-auto">
            {cwd !== ROOT && (
              <li
                className="flex cursor-pointer items-center gap-2 px-3 py-1 text-sm text-muted hover:bg-surface"
                onClick={() => navigateTo(parentOf(cwd))}
              >
                <Folder className="h-3 w-3" /> ..
              </li>
            )}
            {entries?.length === 0 && (
              <li className="px-3 py-4 text-center text-xs text-muted">
                Empty folder
              </li>
            )}
            {entries?.map((e) => (
              <li
                key={e.path}
                onClick={() => onEntryClick(e)}
                className={cn(
                  "flex cursor-pointer items-center gap-2 px-3 py-1 text-sm hover:bg-surface",
                  selected?.path === e.path && "bg-primary/10",
                )}
              >
                {e.dir ? (
                  <Folder className="h-3 w-3 text-muted" />
                ) : (
                  <FileIcon className="h-3 w-3 text-muted" />
                )}
                <span className="truncate">{e.name}</span>
                {!e.dir && (
                  <span className="ml-auto text-xs text-muted">
                    {formatBytes(e.size)}
                  </span>
                )}
              </li>
            ))}
          </ul>
        </aside>

        <main className="flex min-w-0 flex-1 flex-col">
          {selected && !selected.dir ? (
            <>
              <div className="flex items-center justify-between border-b border-border px-4 py-2">
                <div className="flex items-center gap-2 text-sm">
                  <FileIcon className="h-4 w-4 text-primary" />
                  <span className="font-medium text-fg">{selected.name}</span>
                  {dirty && (
                    <span className="text-xs text-warning">· modified</span>
                  )}
                </div>
                <div className="flex items-center gap-2">
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={downloadSelected}
                  >
                    <Download className="h-3 w-3" /> Download
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => setConfirmDelete(selected)}
                  >
                    <Trash2 className="h-3 w-3 text-danger" />
                    <span className="text-danger">Delete</span>
                  </Button>
                  <Button
                    size="sm"
                    disabled={!dirty || saveMutation.isPending}
                    onClick={() => saveMutation.mutate(editorValue)}
                  >
                    {saveMutation.isPending ? (
                      <Loader2 className="h-3 w-3 animate-spin" />
                    ) : (
                      <Save className="h-3 w-3" />
                    )}{" "}
                    Save
                  </Button>
                </div>
              </div>
              <div className="min-h-0 flex-1">
                {loadError ? (
                  <div className="grid h-full place-items-center text-sm text-danger">
                    {loadError}
                  </div>
                ) : serverContent === null ? (
                  <div className="grid h-full place-items-center text-sm text-muted">
                    <Loader2 className="h-4 w-4 animate-spin" />
                  </div>
                ) : (
                  <Editor
                    theme="vs-dark"
                    language={guessLang(selected.name)}
                    value={editorValue}
                    onChange={(v) => setEditorValue(v ?? "")}
                    options={{
                      minimap: { enabled: false },
                      fontFamily: "JetBrains Mono",
                    }}
                  />
                )}
              </div>
            </>
          ) : (
            <div className="grid h-full place-items-center text-sm text-muted">
              Select a file to edit.
            </div>
          )}
        </main>
      </div>

      {confirmDelete && (
        <ConfirmDialog
          open
          onOpenChange={(open) => !open && setConfirmDelete(null)}
          title={`Delete ${confirmDelete.name}?`}
          description={
            confirmDelete.dir
              ? `This permanently removes the folder and everything inside it from the server's data volume. This action cannot be undone.`
              : `This permanently removes the file from the server's data volume. This action cannot be undone.`
          }
          destructive
          confirmLabel="Delete"
          busy={deleteMutation.isPending}
          onConfirm={() => deleteMutation.mutate(confirmDelete)}
        />
      )}

      <NamePromptDialog
        open={mkdirOpen}
        onOpenChange={setMkdirOpen}
        title="New folder"
        label="Folder name"
        placeholder="my-folder"
        confirmLabel="Create"
        busy={mkdirMutation.isPending}
        onSubmit={(value) => mkdirMutation.mutate(value)}
      />
      <NamePromptDialog
        open={newFileOpen}
        onOpenChange={setNewFileOpen}
        title="New file"
        label="File name"
        placeholder="config.yaml"
        confirmLabel="Create"
        busy={newFileMutation.isPending}
        onSubmit={(value) => newFileMutation.mutate(value)}
      />
    </div>
  );
}

function Breadcrumbs({
  cwd,
  onNavigate,
}: {
  cwd: string;
  onNavigate: (p: string) => void;
}): ReactNode {
  const segments = cwd === ROOT ? [] : cwd.split("/").filter(Boolean);
  return (
    <nav className="flex items-center gap-1 rounded border border-border bg-card px-3 py-1.5 font-mono text-xs">
      <button
        type="button"
        className="text-muted hover:text-fg"
        onClick={() => onNavigate(ROOT)}
      >
        /
      </button>
      {segments.map((seg, i) => {
        const path = "/" + segments.slice(0, i + 1).join("/");
        const isLast = i === segments.length - 1;
        return (
          <span key={path} className="flex items-center gap-1">
            <button
              type="button"
              className={cn(
                "hover:text-fg",
                isLast ? "font-medium text-fg" : "text-muted",
              )}
              onClick={() => onNavigate(path)}
            >
              {seg}
            </button>
            {!isLast && <span className="text-muted">/</span>}
          </span>
        );
      })}
    </nav>
  );
}

interface NamePromptProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  label: string;
  placeholder?: string;
  confirmLabel: string;
  busy?: boolean;
  onSubmit: (value: string) => void;
}

function NamePromptDialog({
  open,
  onOpenChange,
  title,
  label,
  placeholder,
  confirmLabel,
  busy,
  onSubmit,
}: NamePromptProps) {
  const [value, setValue] = useState("");
  useEffect(() => {
    if (open) setValue("");
  }, [open]);
  const trimmed = value.trim();
  // Reject empty, slashes, and dot-only names — keeps the prompt aligned with
  // what the agent's resolve() will accept anyway, so users get instant feedback.
  const valid = trimmed.length > 0 && !trimmed.includes("/") && trimmed !== "." && trimmed !== "..";
  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 z-40 bg-black/60" />
        <Dialog.Content className="fixed left-1/2 top-1/2 z-50 w-[440px] max-w-[calc(100vw-2rem)] -translate-x-1/2 -translate-y-1/2 rounded-lg border border-border bg-card p-5 text-fg shadow-2xl">
          <Dialog.Title className="text-base font-semibold">{title}</Dialog.Title>
          <Dialog.Description className="sr-only">{label}</Dialog.Description>
          <form
            onSubmit={(e) => {
              e.preventDefault();
              if (valid && !busy) onSubmit(trimmed);
            }}
          >
            <label className="block pb-1 pt-3 text-xs text-muted">{label}</label>
            <Input
              autoFocus
              value={value}
              placeholder={placeholder}
              onChange={(e) => setValue(e.target.value)}
              spellCheck={false}
            />
            <div className="flex items-center justify-end gap-2 pt-5">
              <Button
                type="button"
                variant="ghost"
                size="sm"
                onClick={() => onOpenChange(false)}
                disabled={busy}
              >
                Cancel
              </Button>
              <Button type="submit" size="sm" disabled={!valid || busy}>
                {busy ? "Working…" : confirmLabel}
              </Button>
            </div>
          </form>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}

function joinPath(dir: string, name: string): string {
  if (dir === ROOT) return ROOT + name;
  return dir.replace(/\/$/, "") + "/" + name;
}

function parentOf(p: string): string {
  if (p === ROOT || p === "") return ROOT;
  const trimmed = p.replace(/\/$/, "");
  const idx = trimmed.lastIndexOf("/");
  return idx <= 0 ? ROOT : trimmed.slice(0, idx);
}

function confirmDiscard(): boolean {
  return window.confirm(
    "You have unsaved changes. Discard them and continue?",
  );
}

function guessLang(name: string): string {
  if (name.endsWith(".yaml") || name.endsWith(".yml")) return "yaml";
  if (name.endsWith(".json")) return "json";
  if (name.endsWith(".properties")) return "ini";
  if (name.endsWith(".toml")) return "toml";
  if (name.endsWith(".sh")) return "shell";
  if (name.endsWith(".js") || name.endsWith(".ts")) return "typescript";
  if (name.endsWith(".md")) return "markdown";
  return "plaintext";
}
