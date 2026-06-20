import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Package, PackageCheck } from "lucide-react";

import type { GameServer, GameTemplate, RegistryProject } from "@/types";
import { Servers } from "@/lib/endpoints";
import { APIError } from "@/lib/api";
import { useMe, can } from "@/lib/auth";
import { Button } from "@/components/ui/button";
import { RegistryBrowser, RegistryIcon, compactNum } from "@/components/registry-browser";
import { cn } from "@/lib/utils";

type Banner = { kind: "ok" | "err"; text: string };

// Curated Modrinth modpack categories for the filter chips.
const MODPACK_CATEGORIES: { value: string; label: string }[] = [
  { value: "adventure", label: "Adventure" },
  { value: "technology", label: "Tech" },
  { value: "magic", label: "Magic" },
  { value: "optimization", label: "Optimized" },
  { value: "multiplayer", label: "Multiplayer" },
];

// ModpacksTab browses whole modpacks and installs one. Install differs by
// game: env-mode (Minecraft/itzg) pins the pack via the server's env and
// restarts (one active pack); deps-mode (Valheim/Thunderstore) resolves the
// pack's dependencies and installs each into the mods directory.
export function ModpacksTab({
  name,
  tmpl,
  gs,
}: {
  name: string;
  tmpl?: GameTemplate;
  gs?: GameServer;
}) {
  const qc = useQueryClient();
  const { data: me } = useMe();
  const canManage = can(me, "servers:write");

  const mp = tmpl?.spec.capabilities?.mods?.registry;
  const provider = mp?.provider;
  const refEnv = mp?.modpacks?.refEnv;
  const envMode = !!refEnv;
  const active = envMode ? gs?.spec.env?.find((e) => e.name === refEnv)?.value : undefined;

  const [banner, setBanner] = useState<Banner | null>(null);
  const [busy, setBusy] = useState<string | null>(null); // project id being installed

  const installEnv = useMutation({
    mutationFn: (p: RegistryProject) => Servers.installModpack(name, { ref: p.slug || p.id }),
    onMutate: (p) => setBusy(p.id),
    onSuccess: (_r, p) => {
      setBanner({ kind: "ok", text: `Set modpack ${p.title}. The server is restarting to install it.` });
      return qc.invalidateQueries({ queryKey: ["server", name] });
    },
    onError: (err) => setBanner({ kind: "err", text: errMsg(err) }),
    onSettled: () => setBusy(null),
  });

  const installDeps = useMutation({
    mutationFn: async (p: RegistryProject) => {
      const files = await Servers.modpackDeps(name, p.id);
      for (const f of files) {
        await Servers.installMod(name, { url: f.downloadUrl, name: f.filename });
      }
      return files.length;
    },
    onMutate: (p) => setBusy(p.id),
    onSuccess: (count, p) => {
      setBanner({ kind: "ok", text: `Installed ${p.title} — ${count} mod${count === 1 ? "" : "s"}.` });
      return qc.invalidateQueries({ queryKey: ["mods", name] });
    },
    onError: (err) => setBanner({ kind: "err", text: errMsg(err) }),
    onSettled: () => setBusy(null),
  });

  const install = (p: RegistryProject) => (envMode ? installEnv : installDeps).mutate(p);

  return (
    <div className="flex h-full flex-col gap-4 p-6">
      <header className="space-y-0.5">
        <h2 className="text-sm text-muted">Browse modpacks</h2>
        <p className="text-[11px] text-muted">
          {envMode
            ? "Installing a modpack replaces this server's content and restarts it — one active pack."
            : "Installing a modpack adds all of its mods to this server."}
        </p>
      </header>

      {envMode && active && (
        <div className="flex items-center gap-2 rounded border border-border bg-surface/40 px-3 py-2 text-sm">
          <PackageCheck className="h-4 w-4 text-primary" />
          <span className="text-muted">Active modpack:</span>
          <span className="font-mono text-fg">{active}</span>
        </div>
      )}

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
            <span className="break-all">{banner.text}</span>
            <button onClick={() => setBanner(null)} className="shrink-0 text-xs text-muted hover:text-fg">
              dismiss
            </button>
          </div>
        </div>
      )}

      <div className="min-h-0 flex-1">
        <RegistryBrowser
          name={name}
          type="modpack"
          categories={provider === "modrinth" ? MODPACK_CATEGORIES : undefined}
          renderItem={(p) => (
            <div className="flex items-center gap-3 rounded border border-border bg-surface/30 p-2.5">
              <RegistryIcon url={p.iconUrl} fallback={<Package className="h-9 w-9 shrink-0 rounded p-2 text-muted" />} />
              <div className="min-w-0 flex-1">
                <div className="truncate text-sm font-medium">{p.title}</div>
                <div className="truncate text-xs text-muted">
                  {[p.author, p.downloads != null ? `${compactNum(p.downloads)} downloads` : null]
                    .filter(Boolean)
                    .join(" · ")}
                </div>
              </div>
              <Button
                size="sm"
                disabled={!canManage || busy !== null}
                title={canManage ? undefined : "Requires operator role"}
                onClick={() => install(p)}
              >
                {busy === p.id ? "Installing…" : "Install"}
              </Button>
            </div>
          )}
        />
      </div>
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
    if (err.status === 403) return "Your role does not allow managing this server.";
    return err.body || `request failed (${err.status})`;
  }
  return err instanceof Error ? err.message : "request failed";
}
