import { useEffect, useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ArrowLeft, ArrowUpCircle, Download, Package, Plus, RotateCw, Trash2 } from "lucide-react";

import type {
  GameServer,
  GameTemplate,
  InstalledMod,
  ModMeta,
  ModUpdate,
  RegistryProject,
} from "@/types";
import { Servers } from "@/lib/endpoints";
import { APIError } from "@/lib/api";
import { resolveModVolume } from "@/lib/capabilities";
import { useMe, can } from "@/lib/auth";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { RegistryBrowser, RegistryIcon, compactNum } from "@/components/registry-browser";
import { cn, formatBytes, formatRelative } from "@/lib/utils";

type Banner = { kind: "ok" | "err"; text: string };

// InstallBody is the proxied agent install request: plain URL installs,
// registry installs (meta records provenance in the agent's manifest), and
// in-place upgrades (replaces swaps out the old file).
type InstallBody = { url: string; name?: string; replaces?: string; meta?: ModMeta };

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
  // Browse mode is offered when the template declares a registry. The
  // server is authoritative (it 501s when its game has no registry); the
  // dialog surfaces that by letting the user fall back to "From URL".
  const canBrowse = !!caps?.registry;

  // The active version+loader fully implies which mod volume this server
  // manages — surface it as a header label (no selector). For the legacy
  // single-path model this is just the path.
  const active = resolveModVolume(tmpl, gs);
  const activeLabel = active
    ? [active.versionLabel ?? active.loader, active.path].filter(Boolean).join(" · ")
    : null;

  const [browsing, setBrowsing] = useState(false);
  const [confirmRemove, setConfirmRemove] = useState<InstalledMod | null>(null);
  const [banner, setBanner] = useState<Banner | null>(null);

  const { data: mods, isFetching, isError, error: listError, refetch } = useQuery({
    queryKey: ["mods", name],
    queryFn: () => Servers.mods(name),
  });

  // Update check is on demand (button), not on mount — it fans out to
  // external registries server-side, so the tab shouldn't trigger it on
  // every visit. Results stay cached for the session.
  const updates = useQuery({
    queryKey: ["mod-updates", name],
    queryFn: () => Servers.modUpdates(name),
    enabled: false,
  });
  const updateByName = useMemo(() => {
    const out = new Map<string, ModUpdate>();
    for (const u of updates.data?.updates ?? []) out.set(u.name, u);
    return out;
  }, [updates.data]);

  // Install stays on the browse page (so you can add several); the banner
  // reports each result and the installed list refreshes underneath.
  const install = useMutation({
    mutationFn: (body: InstallBody) => Servers.installMod(name, body),
    onSuccess: (mod, body) => {
      setBanner({ kind: "ok", text: body.replaces ? `Updated to ${mod.name}` : `Installed ${mod.name}` });
      if (body.replaces && updates.data) void updates.refetch();
      return qc.invalidateQueries({ queryKey: ["mods", name] });
    },
    onError: (err) => setBanner({ kind: "err", text: errMsg(err) }),
  });

  // upgradeBody maps one available update to the install request that
  // applies it: the new file lands first, then the old one is removed.
  const upgradeBody = (u: ModUpdate): InstallBody => ({
    url: u.file.downloadUrl,
    name: u.file.filename,
    replaces: u.name,
    meta: {
      provider: u.provider,
      projectId: u.projectId,
      projectName: u.projectName,
      versionId: u.latestVersionId,
      versionNumber: u.latestVersionNumber,
      loader: active?.loader,
    },
  });

  const updateAll = useMutation({
    mutationFn: async () => {
      // Sequential on purpose: each install is a download on the agent;
      // parallel requests would just contend on the same volume.
      for (const u of updates.data?.updates ?? []) {
        await Servers.installMod(name, upgradeBody(u));
      }
      return updates.data?.updates.length ?? 0;
    },
    onSuccess: (n) => {
      setBanner({ kind: "ok", text: `Updated ${n} mod${n === 1 ? "" : "s"}` });
      void updates.refetch();
      return qc.invalidateQueries({ queryKey: ["mods", name] });
    },
    onError: (err) => {
      setBanner({ kind: "err", text: errMsg(err) });
      void updates.refetch();
      return qc.invalidateQueries({ queryKey: ["mods", name] });
    },
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

  // Dedicated install/browse page (replaces the old popup), matching the
  // Modpacks tab. Rendered after all hooks to keep hook order stable.
  if (browsing) {
    return (
      <InstallPage
        name={name}
        canBrowse={canBrowse}
        allowedHosts={caps?.install?.allowedHosts ?? []}
        pending={install.isPending}
        banner={banner}
        onDismiss={() => setBanner(null)}
        onBack={() => {
          setBrowsing(false);
          setBanner(null);
        }}
        onInstall={(body) => install.mutate(body)}
      />
    );
  }

  return (
    <div className="space-y-6 p-6">
      <header className="flex items-center justify-between gap-3">
        <div className="space-y-0.5">
          <h2 className="text-sm text-muted">
            {mods ? `${mods.length} installed` : isError ? "Couldn’t load mods" : "Loading…"}
            {updates.data && updateByName.size > 0 && (
              <span className="text-primary"> · {updateByName.size} update{updateByName.size === 1 ? "" : "s"} available</span>
            )}
          </h2>
          {(activeLabel || updates.data) && (
            <p className="text-[11px] text-muted" data-testid="mods-active">
              {[activeLabel, updates.data ? `checked ${formatRelative(updates.data.checkedAt)}` : null]
                .filter(Boolean)
                .join(" · ")}
            </p>
          )}
        </div>
        <div className="flex items-center gap-2">
          <Button variant="ghost" size="sm" onClick={() => refetch()} disabled={isFetching} title="Refresh">
            <RotateCw className={cn("h-3 w-3", isFetching && "animate-spin")} />
          </Button>
          {canInstall && (
            <Button
              variant="outline"
              size="sm"
              onClick={() => void updates.refetch()}
              disabled={updates.isFetching || !mods?.some((m) => m.meta && m.meta.provider !== "upload")}
              title="Check the registry for newer versions of managed mods"
            >
              <RotateCw className={cn("h-3 w-3", updates.isFetching && "animate-spin")} />{" "}
              {updates.isFetching ? "Checking…" : "Check updates"}
            </Button>
          )}
          {canInstall && canManage && updateByName.size > 0 && (
            <Button
              size="sm"
              variant="secondary"
              onClick={() => updateAll.mutate()}
              disabled={updateAll.isPending || install.isPending}
            >
              <ArrowUpCircle className="h-4 w-4" />{" "}
              {updateAll.isPending ? "Updating…" : `Update all (${updateByName.size})`}
            </Button>
          )}
          {canInstall && (
            <Button
              size="sm"
              onClick={() => setBrowsing(true)}
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
            <p className="text-xs text-muted">
              {canBrowse
                ? "Use “Install mod” to search a registry or add one from a URL."
                : "Use “Install mod” to add one from a URL."}
            </p>
          )}
        </div>
      ) : (
        <ul className="space-y-1">
          {mods?.map((m) => {
            const update = updateByName.get(m.name);
            return (
              <li
                key={m.name}
                className="flex items-center justify-between gap-3 rounded border border-border bg-surface/30 px-3 py-2"
              >
                <div className="flex min-w-0 items-center gap-3">
                  <Package className="h-4 w-4 shrink-0 text-muted" />
                  <div className="min-w-0">
                    <div className="flex min-w-0 items-center gap-2">
                      <span className="truncate font-mono text-sm">{m.name}</span>
                      {m.meta ? (
                        <span className="shrink-0 rounded-full border border-border px-2 py-0.5 text-[10px] capitalize text-muted">
                          {[m.meta.provider, m.meta.versionNumber].filter(Boolean).join(" · ")}
                        </span>
                      ) : (
                        <span
                          className="shrink-0 rounded-full bg-surface px-2 py-0.5 text-[10px] text-muted"
                          title="Placed outside the panel — no update checks"
                        >
                          unmanaged
                        </span>
                      )}
                    </div>
                    <div className="text-xs text-muted">
                      {formatBytes(m.size)}
                      {m.modTime ? ` · ${formatRelative(m.modTime)}` : ""}
                    </div>
                  </div>
                </div>
                <div className="flex shrink-0 items-center gap-2">
                  {update && (
                    <>
                      <span className="rounded-full border border-primary/40 bg-primary/10 px-2 py-0.5 text-[10px] font-medium text-primary">
                        {update.latestVersionNumber || "new version"} available
                      </span>
                      {canManage && (
                        <Button
                          size="sm"
                          title={`Update to ${update.latestVersionNumber ?? update.latestVersionId}`}
                          onClick={() => install.mutate(upgradeBody(update))}
                          disabled={install.isPending || updateAll.isPending}
                        >
                          <ArrowUpCircle className="h-3 w-3" /> Update
                        </Button>
                      )}
                    </>
                  )}
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
                </div>
              </li>
            );
          })}
        </ul>
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

type InstallMode = "search" | "url";

// InstallPage is the dedicated in-tab install view (replaces the old
// popup): browse a registry (when the template declares one) or add a mod
// from a URL. Both paths converge on the same install endpoint; the agent
// enforces the host allowlist and size cap.
function InstallPage({
  name,
  canBrowse,
  allowedHosts,
  pending,
  banner,
  onDismiss,
  onBack,
  onInstall,
}: {
  name: string;
  canBrowse: boolean;
  allowedHosts: string[];
  pending: boolean;
  banner: Banner | null;
  onDismiss: () => void;
  onBack: () => void;
  onInstall: (body: InstallBody) => void;
}) {
  const [mode, setMode] = useState<InstallMode>(canBrowse ? "search" : "url");
  // Prefilled by the browse view for registry files the panel can't
  // one-click install (requiresAuth, e.g. the Factorio portal) — the user
  // appends their own credentials in the URL form.
  const [urlPrefill, setUrlPrefill] = useState("");
  const browsing = mode === "search" && canBrowse;
  const useUrlForm = (url: string) => {
    setUrlPrefill(url);
    setMode("url");
  };

  return (
    <div className="flex h-full flex-col gap-4 p-6">
      <header className="space-y-2">
        <button
          type="button"
          onClick={onBack}
          className="flex items-center gap-1 text-xs text-muted hover:text-fg"
        >
          <ArrowLeft className="h-3.5 w-3.5" /> Installed mods
        </button>
        <div className="flex items-center justify-between gap-3">
          <div className="space-y-0.5">
            <h2 className="text-base font-semibold">Install mods</h2>
            <p className="text-[11px] text-muted">
              {browsing
                ? "Browse a registry and install for this server’s version + loader."
                : "Download a mod from a URL into the server’s mods directory."}
            </p>
          </div>
          {canBrowse && (
            <div className="flex w-fit rounded border border-border text-xs">
              <button
                type="button"
                onClick={() => setMode("search")}
                aria-pressed={browsing}
                className={cn(
                  "h-8 rounded-l px-3",
                  browsing ? "bg-primary font-medium text-primary-foreground" : "text-muted",
                )}
              >
                Browse registry
              </button>
              <button
                type="button"
                onClick={() => setMode("url")}
                aria-pressed={!browsing}
                className={cn(
                  "h-8 rounded-r border-l border-border px-3",
                  !browsing ? "bg-surface font-medium" : "text-muted",
                )}
              >
                From URL
              </button>
            </div>
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
            <button onClick={onDismiss} className="shrink-0 text-xs text-muted hover:text-fg">
              dismiss
            </button>
          </div>
        </div>
      )}

      {browsing ? (
        <div className="min-h-0 flex-1">
          <BrowseForm name={name} pending={pending} onInstall={onInstall} onUseUrl={useUrlForm} />
        </div>
      ) : (
        <UrlForm
          key={urlPrefill}
          initialUrl={urlPrefill}
          allowedHosts={allowedHosts}
          pending={pending}
          onCancel={onBack}
          onInstall={onInstall}
        />
      )}
    </div>
  );
}

// UrlForm collects a download URL (and optional filename); a light http(s)
// check only, with the allowed hosts shown as a hint.
function UrlForm({
  allowedHosts,
  pending,
  onCancel,
  onInstall,
  initialUrl = "",
}: {
  allowedHosts: string[];
  pending: boolean;
  onCancel: () => void;
  onInstall: (body: InstallBody) => void;
  initialUrl?: string;
}) {
  const [url, setUrl] = useState(initialUrl);
  const [fileName, setFileName] = useState("");
  const trimmed = url.trim();
  const validURL = /^https?:\/\/.+/i.test(trimmed);

  return (
    <>
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
    </>
  );
}

// BrowseForm searches the module's mod registry (debounced) and lets the
// user expand a result to pick a version/file. The API filters results to
// the server's active loader + game version, so the picker stays short.
// Curated Modrinth categories for the mod browser's filter chips.
const MOD_CATEGORIES: { value: string; label: string }[] = [
  { value: "optimization", label: "Performance" },
  { value: "utility", label: "Utility" },
  { value: "library", label: "Library" },
  { value: "worldgen", label: "Worldgen" },
  { value: "adventure", label: "Adventure" },
  { value: "storage", label: "Storage" },
  { value: "technology", label: "Tech" },
  { value: "magic", label: "Magic" },
];

// BrowseForm is the full mod browser inside the install dialog: the shared
// RegistryBrowser (popular by default, search, sort, category chips,
// load-more) with each result expandable to a version picker + Install.
function BrowseForm({
  name,
  pending,
  onInstall,
  onUseUrl,
}: {
  name: string;
  pending: boolean;
  onInstall: (body: InstallBody) => void;
  onUseUrl: (url: string) => void;
}) {
  return (
    <div className="flex min-h-0 flex-1 flex-col pt-3">
      <RegistryBrowser
        name={name}
        type="mod"
        categories={MOD_CATEGORIES}
        renderItem={(p, provider) => (
          <ModCard
            name={name}
            project={p}
            provider={provider}
            pending={pending}
            onInstall={onInstall}
            onUseUrl={onUseUrl}
          />
        )}
      />
    </div>
  );
}

// ModCard is one browser result; expanding it loads the project's versions
// (from the active provider) and offers a version/file picker + Install.
function ModCard({
  name,
  project,
  provider,
  pending,
  onInstall,
  onUseUrl,
}: {
  name: string;
  project: RegistryProject;
  provider: string;
  pending: boolean;
  onInstall: (body: InstallBody) => void;
  onUseUrl: (url: string) => void;
}) {
  const [open, setOpen] = useState(false);
  const [selId, setSelId] = useState("");
  const versions = useQuery({
    queryKey: ["mod-versions", name, provider, project.id],
    queryFn: () => Servers.modVersions(name, project.id, provider),
    enabled: open,
  });
  const list = versions.data ?? [];

  useEffect(() => {
    const first = versions.data?.[0];
    if (first && !selId) setSelId(first.id);
  }, [versions.data, selId]);

  const chosen = list.find((v) => v.id === selId) ?? list[0];
  const file = chosen?.files.find((f) => f.primary) ?? chosen?.files[0];

  return (
    <div className="rounded border border-border bg-surface/30">
      <button type="button" onClick={() => setOpen((o) => !o)} className="flex w-full items-center gap-3 p-2.5 text-left">
        <RegistryIcon url={project.iconUrl} fallback={<Package className="h-9 w-9 shrink-0 rounded p-2 text-muted" />} />
        <div className="min-w-0 flex-1">
          <div className="truncate text-sm font-medium">{project.title}</div>
          <div className="truncate text-xs text-muted">
            {[project.author, project.downloads != null ? `${compactNum(project.downloads)} downloads` : null]
              .filter(Boolean)
              .join(" · ")}
          </div>
        </div>
        <span className="shrink-0 rounded-full border border-border px-2 py-0.5 text-[10px] capitalize text-muted">
          {project.provider}
        </span>
      </button>
      {open && (
        <div className="flex items-center gap-2 border-t border-border p-2.5">
          {versions.isLoading ? (
            <span className="text-xs text-muted">Loading versions…</span>
          ) : versions.isError ? (
            <span className="text-xs text-danger">{errMsg(versions.error)}</span>
          ) : list.length === 0 ? (
            <span className="text-xs text-muted">No compatible files.</span>
          ) : (
            <>
              <select
                value={selId}
                onChange={(e) => setSelId(e.target.value)}
                className="h-8 min-w-0 flex-1 rounded border border-border bg-surface px-2 text-xs"
                aria-label="Version"
              >
                {list.map((v) => {
                  const f = v.files.find((x) => x.primary) ?? v.files[0];
                  return (
                    <option key={v.id} value={v.id}>
                      {(v.versionNumber ?? v.name ?? v.id) + (f ? ` · ${f.filename}` : "")}
                    </option>
                  );
                })}
              </select>
              {file?.requiresAuth ? (
                // Portal files (e.g. Factorio) download only with the
                // player's own credentials — hand off to the URL form so
                // the user can append them; never one-click install.
                <Button
                  size="sm"
                  variant="outline"
                  title="This registry's downloads need your own account credentials appended to the URL"
                  onClick={() => onUseUrl(file.downloadUrl)}
                >
                  Use URL form
                </Button>
              ) : (
                <Button
                  size="sm"
                  disabled={!file || pending}
                  onClick={() =>
                    file &&
                    onInstall({
                      url: file.downloadUrl,
                      name: file.filename,
                      meta: {
                        provider: project.provider,
                        projectId: project.id,
                        projectName: project.title,
                        versionId: chosen?.id,
                        versionNumber: chosen?.versionNumber,
                        loader: chosen?.loaders?.[0],
                        gameVersion: chosen?.gameVersions?.[0],
                      },
                    })
                  }
                >
                  <Download className="h-3 w-3" /> {pending ? "Installing…" : "Install"}
                </Button>
              )}
            </>
          )}
        </div>
      )}
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
    if (err.status === 403) return "Your role does not allow managing mods.";
    return err.body || `request failed (${err.status})`;
  }
  return err instanceof Error ? err.message : "request failed";
}
