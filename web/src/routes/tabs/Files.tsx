import {
  useEffect,
  useRef,
  useState,
  type ChangeEvent,
  type ReactNode,
} from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
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
} from "@heroui/react";
import Editor from "@monaco-editor/react";
import {
  ChevronLeft,
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

import { ConfirmDialog } from "@/components/hero/ConfirmDialog";
import { ErrorBanner } from "@/components/hero/ErrorBanner";
import { Files, type FileEntry } from "@/lib/endpoints";
import { cn, formatBytes } from "@/lib/utils";

const ROOT = "/";

export function FilesTab({ name, ns }: { name: string; ns?: string }) {
  const qc = useQueryClient();
  const listKey = (cwd: string) => ["files", name, cwd, ns] as const;

  const [cwd, setCwd] = useState(ROOT);
  const [selected, setSelected] = useState<FileEntry | null>(null);
  // Below `md` the tree and the file view can't sit side by side — show one
  // pane at a time, switched by tapping an entry (→ view) or the "Files"
  // back-button (→ tree). At `md`+ both panes are always visible regardless
  // of this state (see the `md:flex` overrides below).
  const [pane, setPane] = useState<"tree" | "view">("tree");
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
    const load = async () => {
      // Clear any stale error as the first step of a fresh load.
      setLoadError(null);
      try {
        const text = await Files.read(name, selected.path, ns);
        if (aborted) return;
        setServerContent(text);
        setEditorValue(text);
      } catch (err) {
        if (aborted) return;
        setServerContent(null);
        setEditorValue("");
        setLoadError((err as Error).message);
      }
    };
    void load();
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
    // Navigating (breadcrumbs, "..") is a browsing action — show the tree
    // on mobile rather than stranding the user on the now-stale editor pane.
    setPane("tree");
  }

  function onEntryClick(e: FileEntry) {
    if (e.dir) {
      navigateTo(e.path);
      return;
    }
    if (selected?.path === e.path) {
      setPane("view");
      return;
    }
    if (dirty && !confirmDiscard()) return;
    setSelected(e);
    setPane("view");
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
        setPane("tree");
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
        <BreadcrumbsNav cwd={cwd} onNavigate={navigateTo} />
        <div className="flex items-center gap-2">
          <Button
            variant="secondary"
            size="sm"
            onPress={() => setNewFileOpen(true)}
          >
            <FilePlus className="h-4 w-4" /> New file
          </Button>
          <Button
            variant="secondary"
            size="sm"
            onPress={() => setMkdirOpen(true)}
          >
            <FolderPlus className="h-4 w-4" /> New folder
          </Button>
          <Button
            variant="secondary"
            size="sm"
            onPress={() => uploadInputRef.current?.click()}
            isDisabled={uploadMutation.isPending}
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
            isIconOnly
            variant="ghost"
            size="sm"
            aria-label="Refresh"
            onPress={() => refetch()}
          >
            <RefreshCw className={cn("h-4 w-4", isFetching && "animate-spin")} />
          </Button>
        </div>
      </div>

      {opError && <ErrorBanner err={opError} onDismiss={() => setOpError(null)} />}

      <div className="flex min-h-0 flex-1 overflow-hidden rounded-lg border border-divider bg-surface">
        <aside
          className={cn(
            "w-full shrink-0 flex-col border-r border-divider md:flex md:w-72",
            pane === "tree" ? "flex" : "hidden",
          )}
        >
          <div className="border-b border-divider bg-background px-3 py-2 font-mono text-xs text-default-500">
            {cwd}
          </div>
          <ul className="flex-1 overflow-auto">
            {cwd !== ROOT && (
              <li
                className="flex cursor-pointer items-center gap-2 px-3 py-1.5 text-sm text-default-500 hover:bg-default-100"
                onClick={() => navigateTo(parentOf(cwd))}
              >
                <Folder className="h-3 w-3" /> ..
              </li>
            )}
            {entries?.length === 0 && (
              <li className="px-3 py-4 text-center text-xs text-default-500">
                Empty folder
              </li>
            )}
            {entries?.map((e) => (
              <li
                key={e.path}
                onClick={() => onEntryClick(e)}
                className={cn(
                  "flex cursor-pointer items-center gap-2 px-3 py-1.5 text-sm hover:bg-default-100",
                  selected?.path === e.path && "bg-primary/10 text-foreground",
                )}
              >
                {e.dir ? (
                  <Folder className="h-3 w-3 text-default-500" />
                ) : (
                  <FileIcon className="h-3 w-3 text-default-500" />
                )}
                <span className="truncate">{e.name}</span>
                {!e.dir && (
                  <span className="ml-auto text-xs text-default-500">
                    {formatBytes(e.size)}
                  </span>
                )}
              </li>
            ))}
          </ul>
        </aside>

        <main
          className={cn(
            "min-w-0 flex-1 flex-col md:flex",
            pane === "view" ? "flex" : "hidden",
          )}
        >
          <div className="flex items-center border-b border-divider bg-background px-2 py-1.5 md:hidden">
            <Button
              isIconOnly
              variant="ghost"
              size="sm"
              onPress={() => setPane("tree")}
              aria-label="Back to files"
              className="text-default-500"
            >
              <ChevronLeft className="h-4 w-4" />
            </Button>
            <span className="text-xs text-default-500 ml-1">Files</span>
          </div>
          {selected && !selected.dir ? (
            <>
              <div className="flex items-center justify-between border-b border-divider bg-background px-4 py-2">
                <div className="flex items-center gap-2 text-sm">
                  <FileIcon className="h-4 w-4 text-primary" />
                  <span className="font-medium text-foreground">{selected.name}</span>
                  {dirty && (
                    <span className="text-xs text-warning">· modified</span>
                  )}
                </div>
                <div className="flex items-center gap-2">
                  <Button
                    variant="secondary"
                    size="sm"
                    onPress={downloadSelected}
                  >
                    <Download className="h-3 w-3" /> Download
                  </Button>
                  <Button
                    variant="danger"
                    size="sm"
                    onPress={() => setConfirmDelete(selected)}
                  >
                    <Trash2 className="h-3 w-3" /> Delete
                  </Button>
                  <Button
                    size="sm"
                    isDisabled={!dirty || saveMutation.isPending}
                    onPress={() => saveMutation.mutate(editorValue)}
                    variant="primary"
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
              <div className="min-h-0 flex-1 bg-background">
                {loadError ? (
                  <div className="grid h-full place-items-center text-sm text-danger">
                    {loadError}
                  </div>
                ) : serverContent === null ? (
                  <div className="grid h-full place-items-center text-sm text-default-500">
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
            <div className="grid h-full place-items-center text-sm text-default-500">
              Select a file to edit.
            </div>
          )}
        </main>
      </div>

      {confirmDelete && (
        <ConfirmDialog
          open={!!confirmDelete}
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

function BreadcrumbsNav({
  cwd,
  onNavigate,
}: {
  cwd: string;
  onNavigate: (p: string) => void;
}): ReactNode {
  const segments = cwd === ROOT ? [] : cwd.split("/").filter(Boolean);
  return (
    <div className="flex items-center gap-0.5 rounded-lg border border-divider bg-background px-3 py-1.5">
      <button
        type="button"
        className="text-default-500 hover:text-foreground text-xs font-mono"
        onClick={() => onNavigate(ROOT)}
      >
        /
      </button>
      {segments.map((seg, i) => {
        const path = "/" + segments.slice(0, i + 1).join("/");
        const isLast = i === segments.length - 1;
        return (
          <span key={path} className="flex items-center gap-0.5">
            <button
              type="button"
              className={cn(
                "text-xs font-mono hover:text-foreground",
                isLast
                  ? "font-medium text-foreground"
                  : "text-default-500",
              )}
              onClick={() => onNavigate(path)}
            >
              {seg}
            </button>
            {!isLast && <span className="text-default-500">/</span>}
          </span>
        );
      })}
    </div>
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
  // Clear the input whenever the dialog transitions to open. Adjusted
  // directly during render (not in an effect), gated on the previous
  // `open` value tracked in state.
  const [prevOpen, setPrevOpen] = useState(open);
  if (open !== prevOpen) {
    setPrevOpen(open);
    if (open) setValue("");
  }
  const trimmed = value.trim();
  // Reject empty, slashes, and dot-only names — keeps the prompt aligned with
  // what the agent's resolve() will accept anyway, so users get instant feedback.
  const valid = trimmed.length > 0 && !trimmed.includes("/") && trimmed !== "." && trimmed !== "..";
  return (
    <Modal isOpen={open} onOpenChange={onOpenChange}>
      <ModalBackdrop isDismissable={!busy}>
        <ModalContainer>
          <ModalDialog>
            <ModalHeader className="flex flex-col gap-1">{title}</ModalHeader>
            <ModalBody>
              <form
                onSubmit={(e) => {
                  e.preventDefault();
                  if (valid && !busy) onSubmit(trimmed);
                }}
                className="space-y-4"
              >
                <div>
                  <label className="block pb-2 text-xs text-default-500">{label}</label>
                  <Input
                    autoFocus
                    value={value}
                    placeholder={placeholder}
                    onChange={(e) => setValue(e.target.value)}
                    spellCheck={false}
                  />
                </div>
              </form>
            </ModalBody>
            <ModalFooter>
              <Button
                variant="secondary"
                onPress={() => onOpenChange(false)}
                isDisabled={busy}
              >
                Cancel
              </Button>
              <Button
                variant="primary"
                isDisabled={!valid || busy}
                onPress={() => {
                  if (valid && !busy) onSubmit(trimmed);
                }}
              >
                {busy ? "Working…" : confirmLabel}
              </Button>
            </ModalFooter>
          </ModalDialog>
        </ModalContainer>
      </ModalBackdrop>
    </Modal>
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
