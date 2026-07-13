import { useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  ArrowLeft,
  ArrowUpCircle,
  Compass,
  Download,
  Info,
  Package,
  Plus,
  RotateCcw,
  RotateCw,
  TriangleAlert,
  Trash2,
  Upload,
  X,
} from "lucide-react";

import type {
  GameServer,
  GameTemplate,
  InstalledMod,
  ModID,
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
import { RegistryBrowser, RegistryIcon, compactNum, providerLabel } from "@/components/registry-browser";
import { cn, formatBytes, formatRelative } from "@/lib/utils";

type Banner = { kind: "ok" | "err"; text: string };

// InstallBody is the proxied agent install request: plain URL installs,
// registry installs (meta records provenance in the agent's manifest), and
// in-place upgrades (replaces swaps out the old file).
type InstallBody = { url: string; name?: string; replaces?: string; meta?: ModMeta };

// ModsTab is the Mods tab's entry point. It's rendered only when the
// template declares spec.capabilities.mods (see serverHasMods). Games
// split into two entirely different UIs here: a template that declares
// idList (its server downloads its own mods given a list of ids — ARK's
// CurseForge ids, Project Zomboid's MOD_IDS, Steam Workshop lists) gets
// the id-managed batch editor; every other game keeps the existing
// file-based list/install/upload flow untouched. The branch happens
// before either side calls a hook, so switching modes cleanly unmounts
// one component and mounts the other rather than reordering hooks within
// one.
export function ModsTab({ name, tmpl, gs, ns }: { name: string; tmpl?: GameTemplate; gs?: GameServer; ns?: string }) {
  if (tmpl?.spec.capabilities?.mods?.idList) {
    return <ModsByIdTab name={name} tmpl={tmpl} ns={ns} />;
  }
  return <FileModsTab name={name} tmpl={tmpl} gs={gs} ns={ns} />;
}

// FileModsTab is the generic file-based mod/plugin manager (the original
// ModsTab, unchanged): lists, installs by URL, browses a registry, and
// uploads files into the resolved mods directory.
function FileModsTab({ name, tmpl, gs, ns }: { name: string; tmpl?: GameTemplate; gs?: GameServer; ns?: string }) {
  const qc = useQueryClient();
  const { data: me } = useMe();
  const canManage = can(me, "servers:write");

  const caps = tmpl?.spec.capabilities?.mods;
  // URL installs need the module's install (allowlist) block; uploads only
  // need a mods directory, so the install page is reachable whenever the
  // template declares mods at all.
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
    queryKey: ["mods", name, ns],
    queryFn: () => Servers.mods(name, ns),
  });

  // Update check is on demand (button), not on mount — it fans out to
  // external registries server-side, so the tab shouldn't trigger it on
  // every visit. Results stay cached for the session.
  const updates = useQuery({
    queryKey: ["mod-updates", name, ns],
    queryFn: () => Servers.modUpdates(name, ns),
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

  const upload = useMutation({
    mutationFn: (file: File) => Servers.uploadMod(name, file),
    onSuccess: (mod) => {
      setBanner({ kind: "ok", text: `Uploaded ${mod.name}` });
      return qc.invalidateQueries({ queryKey: ["mods", name] });
    },
    onError: (err) => setBanner({ kind: "err", text: errMsg(err) }),
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
  const extensions =
    (active?.loader ? caps?.loaders?.[active.loader]?.extensions : undefined) ??
    caps?.extensions ??
    [];

  if (browsing) {
    return (
      <InstallPage
        name={name}
        canInstall={canInstall}
        canBrowse={canBrowse}
        allowedHosts={caps?.install?.allowedHosts ?? []}
        extensions={extensions}
        pending={install.isPending}
        uploadPending={upload.isPending}
        banner={banner}
        onDismiss={() => setBanner(null)}
        onBack={() => {
          setBrowsing(false);
          setBanner(null);
        }}
        onInstall={(body) => install.mutate(body)}
        onUpload={(file) => upload.mutate(file)}
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
          <Button
            size="sm"
            onClick={() => setBrowsing(true)}
            disabled={!canManage}
            title={canManage ? undefined : "Requires operator role"}
          >
            <Plus className="h-4 w-4" /> Install mod
          </Button>
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
          {canManage && (
            <p className="text-xs text-muted">
              {canBrowse
                ? "Use “Install mod” to search a registry, add one from a URL, or upload a file."
                : canInstall
                  ? "Use “Install mod” to add one from a URL or upload a file."
                  : "Use “Install mod” to upload a file."}
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

// modIDPattern mirrors the API's modIDPattern (api/internal/handlers/
// mod_ids.go) and the CRD's ModRef.ID validation — client-side so the Add
// button/input give immediate feedback instead of round-tripping to a 400.
const modIDPattern = /^[A-Za-z0-9._-]{1,64}$/;

// A row's local, unsaved status relative to the last-saved list. "kept"
// mirrors the server; "added" is a new selection not yet saved; "removed"
// is marked for removal but still rendered (dimmed, with Undo) until Save.
type ModIDRowState = "kept" | "added" | "removed";
interface ModIDRow extends ModID {
  state: ModIDRowState;
}

// ModsByIdTab is the mods editor for games whose server downloads its own
// mods given a list of ids (spec.capabilities.mods.idList) — ARK: Survival
// Ascended's CurseForge ids, Project Zomboid's MOD_IDS, generic Steam
// Workshop lists. There is no mods directory and no per-mod file, so the
// file-based list/install/upload UI (FileModsTab) is meaningless here.
//
// Saving changes the game container's env and restarts the server (see
// GameTemplate.spec.capabilities.mods.idList and the operator's env
// projection), so this is a batch-then-save editor: adds/removes are held
// in local `rows` state and only reach the API as a single PUT — never a
// per-row write, which would cost one restart per mod.
function ModsByIdTab({
  name,
  tmpl,
  ns,
}: {
  name: string;
  tmpl?: GameTemplate;
  ns?: string;
}) {
  const qc = useQueryClient();
  const { data: me } = useMe();
  const canManage = can(me, "servers:write");

  const caps = tmpl?.spec.capabilities?.mods;
  // A registry provider (e.g. ARK declares curseforge) enables in-app
  // browse; the RegistryBrowser itself falls back to a message when the
  // provider is declared but unavailable (e.g. no CurseForge API key) — in
  // either case Add-by-ID below keeps working on its own.
  const provider = caps?.registry?.providers?.[0]?.provider;
  const canBrowse = !!caps?.registry;
  const providerName = provider ? providerLabel(provider) : undefined;

  const [browsing, setBrowsing] = useState(false);
  const [rows, setRows] = useState<ModIDRow[]>([]);
  const [idInput, setIdInput] = useState("");
  const [banner, setBanner] = useState<Banner | null>(null);

  // savedRef holds the last-known server truth (for Discard); lastSeenRef
  // dedupes re-syncs to the same query result (mirrors the Settings tab's
  // draft/baseline pattern for the same reason: a background refetch must
  // not clobber edits in progress).
  const savedRef = useRef<ModID[]>([]);
  const lastSeenRef = useRef<ModID[] | undefined>(undefined);

  const { data: saved, isFetching, isError, error: listError, refetch } = useQuery({
    queryKey: ["mod-ids", name, ns],
    queryFn: () => Servers.modIDs(name, ns),
  });

  const dirty = useMemo(() => rows.some((r) => r.state !== "kept"), [rows]);

  useEffect(() => {
    if (!saved || saved === lastSeenRef.current) return;
    lastSeenRef.current = saved;
    if (dirty) return; // local edits in flight — don't clobber
    savedRef.current = saved;
    setRows(saved.map((m) => ({ ...m, state: "kept" as const })));
  }, [saved, dirty]);

  const save = useMutation({
    mutationFn: () =>
      Servers.setModIDs(
        name,
        rows
          .filter((r) => r.state !== "removed")
          .map(({ id, name: label }): ModID => (label ? { id, name: label } : { id })),
        ns,
      ),
    onSuccess: (updated) => {
      savedRef.current = updated;
      lastSeenRef.current = updated;
      setRows(updated.map((m) => ({ ...m, state: "kept" as const })));
      setBanner({
        kind: "ok",
        text: `Saved — the server will restart to apply ${updated.length} mod${updated.length === 1 ? "" : "s"}.`,
      });
      return qc.invalidateQueries({ queryKey: ["mod-ids", name] });
    },
    onError: (err) => setBanner({ kind: "err", text: errMsg(err) }),
  });

  const discard = () => {
    setRows(savedRef.current.map((m) => ({ ...m, state: "kept" as const })));
    setBanner(null);
  };

  // addModID adds an id to the pending list. Re-adding an id that's
  // currently marked for removal just undoes the removal (the intuitive
  // outcome); adding an id already selected is a no-op.
  const addModID = (id: string, label?: string) => {
    const trimmed = id.trim();
    if (!modIDPattern.test(trimmed)) return;
    setRows((prev) => {
      const existing = prev.find((r) => r.id === trimmed);
      if (existing) {
        if (existing.state === "removed") {
          return prev.map((r) => (r.id === trimmed ? { ...r, state: "kept" as const } : r));
        }
        return prev;
      }
      return [...prev, { id: trimmed, name: label, state: "added" as const }];
    });
  };

  // markRemoved drops a never-saved "added" row outright (nothing to
  // undo — it never reached the server); a "kept" row is marked
  // "removed" so it stays visible (dimmed, with Undo) until Save.
  const markRemoved = (id: string) => {
    setRows((prev) =>
      prev.flatMap((r) => {
        if (r.id !== id) return [r];
        if (r.state === "added") return [];
        return [{ ...r, state: "removed" as const }];
      }),
    );
  };

  const undoRemove = (id: string) => {
    setRows((prev) => prev.map((r) => (r.id === id ? { ...r, state: "kept" as const } : r)));
  };

  // Every row renders (a "removed" row stays visible, dimmed, until Save
  // actually drops it); the header count excludes rows marked for removal.
  const selectedCount = rows.filter((r) => r.state !== "removed").length;

  if (browsing) {
    return (
      <div className="flex h-full flex-col gap-4 p-6">
        <header className="space-y-2">
          <button
            type="button"
            onClick={() => setBrowsing(false)}
            className="flex items-center gap-1 text-xs text-muted hover:text-fg"
          >
            <ArrowLeft className="h-3.5 w-3.5" /> Selected mods
          </button>
          <h2 className="text-base font-semibold">Browse {providerName ?? "registry"}</h2>
        </header>
        <div className="min-h-0 flex-1">
          <RegistryBrowser
            name={name}
            type="mod"
            renderItem={(p) => (
              <IdModCard
                project={p}
                added={rows.some((r) => r.id === p.id && r.state !== "removed")}
                onAdd={() => addModID(p.id, p.title)}
              />
            )}
          />
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6 p-6">
      <header className="flex items-center justify-between gap-3">
        <div className="space-y-0.5">
          <h2 className="text-sm text-muted">{selectedCount} selected</h2>
          <p className="text-[11px] text-muted">
            {[tmpl?.spec.displayName, providerName ? `${providerName} mod IDs` : null].filter(Boolean).join(" · ")}
          </p>
        </div>
        <Button variant="ghost" size="sm" onClick={() => refetch()} disabled={isFetching} title="Refresh">
          <RotateCw className={cn("h-3 w-3", isFetching && "animate-spin")} />
        </Button>
      </header>

      <div className="flex items-start gap-2 rounded-lg border border-border bg-surface/40 p-3 text-xs text-fg">
        <Info className="h-4 w-4 shrink-0 text-muted" />
        <p>
          This game&rsquo;s server downloads its own mods. Select them here — Gameplane passes the list to the
          server at launch.
        </p>
      </div>

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

      {isError && !saved && (
        <div className="rounded border border-danger/40 bg-danger/10 px-3 py-2 text-sm text-danger">
          {errMsg(listError)} ·{" "}
          <button onClick={() => refetch()} className="underline hover:no-underline">
            retry
          </button>
        </div>
      )}

      <div className="flex flex-wrap items-center gap-3">
        {canBrowse && canManage && (
          <>
            <Button variant="outline" size="sm" onClick={() => setBrowsing(true)}>
              <Compass className="h-4 w-4" /> Browse {providerName ?? "registry"}
            </Button>
            <div className="h-6 w-px bg-border" />
          </>
        )}
        <div className="flex items-center gap-2">
          <Input
            placeholder={`Paste a${providerName ? ` ${providerName}` : ""} mod ID…`}
            value={idInput}
            onChange={(e) => setIdInput(e.target.value)}
            disabled={!canManage}
            className="w-56"
            spellCheck={false}
          />
          <Button
            size="sm"
            disabled={!canManage || !modIDPattern.test(idInput.trim())}
            onClick={() => {
              addModID(idInput.trim());
              setIdInput("");
            }}
          >
            <Plus className="h-4 w-4" /> Add
          </Button>
        </div>
      </div>

      <div className="space-y-2">
        <h3 className="text-sm font-semibold">Selected mods</h3>
        {rows.length === 0 ? (
          <div className="flex flex-col items-center gap-2 rounded-lg border border-dashed border-border py-12 text-center">
            <Package className="h-6 w-6 text-muted" />
            <p className="text-sm text-muted">No mods selected.</p>
            {canManage && <p className="text-xs text-muted">Browse {providerName ?? "a registry"} or add a mod ID.</p>}
          </div>
        ) : (
          <ul className="space-y-2">
            {rows.map((r) => (
              <li
                key={r.id}
                className={cn(
                  "flex items-center justify-between gap-3 rounded-lg border border-border bg-card px-3.5 py-3.5",
                  r.state === "removed" && "opacity-60",
                )}
              >
                <div className="flex min-w-0 items-center gap-3">
                  <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded bg-surface">
                    <Package className="h-4 w-4 text-muted" />
                  </div>
                  <div className="min-w-0">
                    <div className={cn("truncate text-sm font-semibold", r.state === "removed" && "line-through")}>
                      {r.name || r.id}
                    </div>
                    {r.name && <div className="truncate text-xs text-muted">ID {r.id}</div>}
                  </div>
                </div>
                <div className="flex shrink-0 items-center gap-2">
                  {r.state === "added" && <ModIDChip kind="added">Added</ModIDChip>}
                  {r.state === "removed" && <ModIDChip kind="removed">Marked for removal</ModIDChip>}
                  {canManage &&
                    (r.state === "removed" ? (
                      <Button variant="ghost" size="sm" onClick={() => undoRemove(r.id)}>
                        <RotateCcw className="h-3 w-3" /> Undo
                      </Button>
                    ) : (
                      <Button variant="ghost" size="sm" onClick={() => markRemoved(r.id)}>
                        <X className="h-3 w-3" /> Remove
                      </Button>
                    ))}
                </div>
              </li>
            ))}
          </ul>
        )}
      </div>

      <div className="flex flex-wrap items-center justify-between gap-3 border-t border-border pt-4">
        <div className="flex items-center gap-2 text-xs text-warning">
          <TriangleAlert className="h-4 w-4" /> Saving restarts the server to apply the new mod list.
        </div>
        <div className="flex items-center gap-2">
          <Button variant="ghost" size="sm" onClick={discard} disabled={!dirty || save.isPending}>
            Discard
          </Button>
          <Button size="sm" onClick={() => save.mutate()} disabled={!dirty || save.isPending || !canManage}>
            {save.isPending ? "Saving…" : "Save changes"}
          </Button>
        </div>
      </div>
    </div>
  );
}

// ModIDChip is the pending-state pill for a row: "Added" (success) for a
// new not-yet-saved selection, "Marked for removal" (warning) for a kept
// row queued for removal. Mirrors the existing update-available badge's
// pill styling (rounded-full border + tinted bg/text) rather than
// introducing a new one.
function ModIDChip({ kind, children }: { kind: "added" | "removed"; children: ReactNode }) {
  return (
    <span
      className={cn(
        "shrink-0 rounded-full border px-2 py-0.5 text-[10px] font-medium",
        kind === "added" ? "border-success/40 bg-success/10 text-success" : "border-warning/40 bg-warning/10 text-warning",
      )}
    >
      {children}
    </span>
  );
}

// IdModCard is one registry-browse result in id-managed mode: no
// version/file picker (unlike FileModsTab's ModCard) — the operator only
// ever needs the provider-native project id, not a specific downloadable
// file, since the *game server itself* fetches the mod at launch.
function IdModCard({
  project,
  added,
  onAdd,
}: {
  project: RegistryProject;
  added: boolean;
  onAdd: () => void;
}) {
  return (
    <div className="flex items-center gap-3 rounded border border-border bg-surface/30 p-2.5">
      <RegistryIcon url={project.iconUrl} fallback={<Package className="h-9 w-9 shrink-0 rounded p-2 text-muted" />} />
      <div className="min-w-0 flex-1">
        <div className="truncate text-sm font-medium">{project.title}</div>
        <div className="truncate text-xs text-muted">
          {[project.author, project.downloads != null ? `${compactNum(project.downloads)} downloads` : null]
            .filter(Boolean)
            .join(" · ")}
        </div>
      </div>
      <Button size="sm" variant={added ? "outline" : "default"} disabled={added} onClick={onAdd}>
        {added ? "Added" : "Add"}
      </Button>
    </div>
  );
}

type InstallMode = "search" | "url" | "upload";

// InstallPage is the dedicated in-tab install view (replaces the old
// popup): browse a registry (when the template declares one), add a mod
// from a URL (when the module allows downloads), or upload a local file
// (always — uploads carry no SSRF risk, so they need no allowlist). All
// paths converge on agent endpoints that enforce the same checks.
function InstallPage({
  name,
  canInstall,
  canBrowse,
  allowedHosts,
  extensions,
  pending,
  uploadPending,
  banner,
  onDismiss,
  onBack,
  onInstall,
  onUpload,
}: {
  name: string;
  canInstall: boolean;
  canBrowse: boolean;
  allowedHosts: string[];
  extensions: string[];
  pending: boolean;
  uploadPending: boolean;
  banner: Banner | null;
  onDismiss: () => void;
  onBack: () => void;
  onInstall: (body: InstallBody) => void;
  onUpload: (file: File) => void;
}) {
  const modes: { key: InstallMode; label: string }[] = [
    ...(canBrowse ? [{ key: "search" as const, label: "Browse registry" }] : []),
    ...(canInstall ? [{ key: "url" as const, label: "From URL" }] : []),
    { key: "upload", label: "Upload file" },
  ];
  const [mode, setMode] = useState<InstallMode>(modes[0].key);
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
                : mode === "upload"
                  ? "Upload a mod file from your machine into the server’s mods directory."
                  : "Download a mod from a URL into the server’s mods directory."}
            </p>
          </div>
          {modes.length > 1 && (
            <div className="flex w-fit rounded border border-border text-xs">
              {modes.map((m, i) => (
                <button
                  key={m.key}
                  type="button"
                  onClick={() => setMode(m.key)}
                  aria-pressed={mode === m.key}
                  className={cn(
                    "h-8 px-3",
                    i === 0 && "rounded-l",
                    i === modes.length - 1 && "rounded-r",
                    i > 0 && "border-l border-border",
                    mode === m.key
                      ? m.key === "search"
                        ? "bg-primary font-medium text-primary-foreground"
                        : "bg-surface font-medium"
                      : "text-muted",
                  )}
                >
                  {m.label}
                </button>
              ))}
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
      ) : mode === "upload" ? (
        <UploadForm
          extensions={extensions}
          pending={uploadPending}
          onCancel={onBack}
          onUpload={onUpload}
        />
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

// UploadForm sends a local file to the agent (multipart). The agent applies
// the same name/extension/size checks as URL installs; extract-mode loaders
// unpack archives the same way. Uploads are recorded as provider "upload"
// in the manifest and are never update-checked.
function UploadForm({
  extensions,
  pending,
  onCancel,
  onUpload,
}: {
  extensions: string[];
  pending: boolean;
  onCancel: () => void;
  onUpload: (file: File) => void;
}) {
  const [file, setFile] = useState<File | null>(null);

  return (
    <>
      <div className="space-y-3 pt-4">
        <label
          htmlFor="mod-file"
          className="flex cursor-pointer flex-col items-center gap-1.5 rounded-lg border border-dashed border-border px-6 py-10 text-center hover:bg-surface/40"
          onDragOver={(e) => e.preventDefault()}
          onDrop={(e) => {
            e.preventDefault();
            const dropped = e.dataTransfer.files?.[0];
            if (dropped) setFile(dropped);
          }}
        >
          <Upload className="h-6 w-6 text-muted" />
          <span className="text-sm font-medium">
            {file ? file.name : "Drop a mod file here, or click to browse"}
          </span>
          <span className="text-[11px] text-muted">
            {extensions.length > 0
              ? `Accepted: ${extensions.join(", ")} · lands in the active loader’s mod volume`
              : "Lands in the active loader’s mod volume"}
          </span>
          <input
            id="mod-file"
            type="file"
            className="sr-only"
            accept={extensions.join(",") || undefined}
            onChange={(e) => setFile(e.target.files?.[0] ?? null)}
          />
        </label>
        <p className="text-[11px] text-muted">
          Uploads are recorded as provider “upload” in the manifest — they’re never update-checked.
        </p>
      </div>

      <div className="flex items-center justify-end gap-2 pt-5">
        <Button variant="ghost" size="sm" onClick={onCancel} disabled={pending}>
          Cancel
        </Button>
        <Button size="sm" disabled={!file || pending} onClick={() => file && onUpload(file)}>
          {pending ? "Uploading…" : "Upload"}
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
