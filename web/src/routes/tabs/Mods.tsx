import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import * as Dialog from "@radix-ui/react-dialog";
import { Package, Plus, RotateCw, Trash2 } from "lucide-react";

import type { GameServer, GameTemplate, InstalledMod } from "@/types";
import { Servers } from "@/lib/endpoints";
import { APIError } from "@/lib/api";
import { resolveModVolume } from "@/lib/capabilities";
import { useMe, can } from "@/lib/auth";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { cn, formatBytes, formatRelative } from "@/lib/utils";

type Banner = { kind: "ok" | "err"; text: string };

// ModsTab is the generic mod/plugin manager. It's rendered only when the
// template declares spec.capabilities.mods; everything game-specific
// (the mods directory, what installs are allowed) lives server-side. The
// dashboard just lists, installs by URL, and removes by name.
export function ModsTab({ name, tmpl, gs }: { name: string; tmpl?: GameTemplate; gs?: GameServer }) {
  const qc = useQueryClient();
  const { data: me } = useMe();
  const canManage = can(me, "servers:write");

  const caps = tmpl?.spec.capabilities?.mods;
  const canInstall = !!caps?.install;

  // The active version+loader fully implies which mod volume this server
  // manages — surface it as a header label (no selector). For the legacy
  // single-path model this is just the path.
  const active = resolveModVolume(tmpl, gs);
  const activeLabel = active
    ? [active.versionLabel ?? active.loader, active.path].filter(Boolean).join(" · ")
    : null;

  const [installOpen, setInstallOpen] = useState(false);
  const [confirmRemove, setConfirmRemove] = useState<InstalledMod | null>(null);
  const [banner, setBanner] = useState<Banner | null>(null);

  const { data: mods, isFetching, isError, error: listError, refetch } = useQuery({
    queryKey: ["mods", name],
    queryFn: () => Servers.mods(name),
  });

  const install = useMutation({
    mutationFn: (body: { url: string; name?: string }) => Servers.installMod(name, body),
    onSuccess: (mod) => {
      setInstallOpen(false);
      setBanner({ kind: "ok", text: `Installed ${mod.name}` });
      return qc.invalidateQueries({ queryKey: ["mods", name] });
    },
    onError: (err) => setBanner({ kind: "err", text: errMsg(err) }),
  });

  const remove = useMutation({
    mutationFn: (mod: string) => Servers.removeMod(name, mod),
    onSuccess: (_void, mod) => {
      setConfirmRemove(null);
      setBanner({ kind: "ok", text: `Removed ${mod}` });
      return qc.invalidateQueries({ queryKey: ["mods", name] });
    },
    onError: (err) => {
      setConfirmRemove(null);
      setBanner({ kind: "err", text: errMsg(err) });
    },
  });

  return (
    <div className="space-y-6 p-6">
      <header className="flex items-center justify-between gap-3">
        <div className="space-y-0.5">
          <h2 className="text-sm text-muted">
            {mods ? `${mods.length} installed` : isError ? "Couldn’t load mods" : "Loading…"}
          </h2>
          {activeLabel && (
            <p className="text-[11px] text-muted" data-testid="mods-active">{activeLabel}</p>
          )}
        </div>
        <div className="flex items-center gap-2">
          <Button variant="ghost" size="sm" onClick={() => refetch()} disabled={isFetching} title="Refresh">
            <RotateCw className={cn("h-3 w-3", isFetching && "animate-spin")} />
          </Button>
          {canInstall && (
            <Button
              size="sm"
              onClick={() => setInstallOpen(true)}
              disabled={!canManage}
              title={canManage ? undefined : "Requires operator role"}
            >
              <Plus className="h-4 w-4" /> Install mod
            </Button>
          )}
        </div>
      </header>

      {banner && (
        <div
          className={cn(
            "rounded border px-3 py-2 text-sm",
            banner.kind === "ok"
              ? "border-border bg-surface/40 text-fg"
              : "border-danger/40 bg-danger/10 text-danger",
          )}
        >
          <div className="flex items-start justify-between gap-3">
            <span className="font-mono break-all">{banner.text}</span>
            <button onClick={() => setBanner(null)} className="shrink-0 text-xs text-muted hover:text-fg">
              dismiss
            </button>
          </div>
        </div>
      )}

      {isError && !mods && (
        <div className="rounded border border-danger/40 bg-danger/10 px-3 py-2 text-sm text-danger">
          {errMsg(listError)} ·{" "}
          <button onClick={() => refetch()} className="underline hover:no-underline">
            retry
          </button>
        </div>
      )}

      {mods && mods.length === 0 ? (
        <div className="flex flex-col items-center gap-2 rounded-lg border border-dashed border-border py-12 text-center">
          <Package className="h-6 w-6 text-muted" />
          <p className="text-sm text-muted">No mods installed.</p>
          {canInstall && canManage && (
            <p className="text-xs text-muted">Use “Install mod” to add one from a URL.</p>
          )}
        </div>
      ) : (
        <ul className="space-y-1">
          {mods?.map((m) => (
            <li
              key={m.name}
              className="flex items-center justify-between gap-3 rounded border border-border bg-surface/30 px-3 py-2"
            >
              <div className="flex min-w-0 items-center gap-3">
                <Package className="h-4 w-4 shrink-0 text-muted" />
                <div className="min-w-0">
                  <div className="truncate font-mono text-sm">{m.name}</div>
                  <div className="text-xs text-muted">
                    {formatBytes(m.size)}
                    {m.modTime ? ` · ${formatRelative(m.modTime)}` : ""}
                  </div>
                </div>
              </div>
              {canManage && (
                <Button
                  variant="ghost"
                  size="sm"
                  title={`Remove ${m.name}`}
                  onClick={() => setConfirmRemove(m)}
                  disabled={remove.isPending}
                >
                  <Trash2 className="h-3 w-3" />
                </Button>
              )}
            </li>
          ))}
        </ul>
      )}

      {installOpen && (
        <InstallDialog
          allowedHosts={caps?.install?.allowedHosts ?? []}
          pending={install.isPending}
          onCancel={() => setInstallOpen(false)}
          onInstall={(body) => install.mutate(body)}
        />
      )}

      <ConfirmDialog
        open={confirmRemove !== null}
        onOpenChange={(o) => !o && setConfirmRemove(null)}
        title="Remove mod"
        description={
          <>
            Remove <span className="font-mono text-fg">{confirmRemove?.name}</span> from the server? It will be gone on
            the next restart.
          </>
        }
        confirmLabel="Remove"
        destructive
        busy={remove.isPending}
        onConfirm={() => confirmRemove && remove.mutate(confirmRemove.name)}
      />
    </div>
  );
}

// InstallDialog collects a download URL (and optional filename). The
// agent enforces the host allowlist and size cap; this dialog only does
// a light http(s) check and surfaces the allowed hosts as a hint.
function InstallDialog({
  allowedHosts,
  pending,
  onCancel,
  onInstall,
}: {
  allowedHosts: string[];
  pending: boolean;
  onCancel: () => void;
  onInstall: (body: { url: string; name?: string }) => void;
}) {
  const [url, setUrl] = useState("");
  const [fileName, setFileName] = useState("");

  const trimmed = url.trim();
  const validURL = /^https?:\/\/.+/i.test(trimmed);

  return (
    <Dialog.Root open onOpenChange={(o) => !o && onCancel()}>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 z-40 bg-black/60" />
        <Dialog.Content className="fixed left-1/2 top-1/2 z-50 w-[460px] max-w-[calc(100vw-2rem)] -translate-x-1/2 -translate-y-1/2 rounded-lg border border-border bg-card p-5 text-fg shadow-2xl">
          <Dialog.Title className="text-base font-semibold">Install mod</Dialog.Title>
          <Dialog.Description asChild>
            <p className="pt-2 text-sm text-muted">
              Download a mod from a URL into the server&apos;s mods directory.
            </p>
          </Dialog.Description>

          <div className="space-y-3 pt-4">
            <div>
              <label className="block pb-1 text-xs text-muted" htmlFor="mod-url">
                Download URL <span className="text-danger">*</span>
              </label>
              <Input
                id="mod-url"
                autoFocus
                placeholder="https://cdn.modrinth.com/…/mod.jar"
                value={url}
                onChange={(e) => setUrl(e.target.value)}
                spellCheck={false}
              />
              {trimmed !== "" && !validURL && (
                <p className="pt-1 text-xs text-danger">Must be an http(s) URL.</p>
              )}
            </div>
            <div>
              <label className="block pb-1 text-xs text-muted" htmlFor="mod-name">
                Filename
              </label>
              <Input
                id="mod-name"
                placeholder="optional — derived from the URL"
                value={fileName}
                onChange={(e) => setFileName(e.target.value)}
                spellCheck={false}
              />
            </div>
            {allowedHosts.length > 0 && (
              <p className="text-[11px] text-muted">
                Allowed hosts: <span className="font-mono">{allowedHosts.join(", ")}</span>
              </p>
            )}
          </div>

          <div className="flex items-center justify-end gap-2 pt-5">
            <Button variant="ghost" size="sm" onClick={onCancel} disabled={pending}>
              Cancel
            </Button>
            <Button
              size="sm"
              disabled={!validURL || pending}
              onClick={() =>
                onInstall({ url: trimmed, ...(fileName.trim() ? { name: fileName.trim() } : {}) })
              }
            >
              {pending ? "Installing…" : "Install"}
            </Button>
          </div>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
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
    if (err.status === 403) return "Your role does not allow managing mods.";
    return err.body || `request failed (${err.status})`;
  }
  return err instanceof Error ? err.message : "request failed";
}
