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

// Curated Modrinth modpack categories for the filter chips (shown only when
// the active provider is Modrinth).
const MODPACK_CATEGORIES: { value: string; label: string }[] = [
  { value: "adventure", label: "Adventure" },
  { value: "technology", label: "Tech" },
  { value: "magic", label: "Magic" },
  { value: "optimization", label: "Optimized" },
  { value: "multiplayer", label: "Multiplayer" },
];

// ModpacksTab browses whole modpacks and installs one. Install differs by
// the active provider: env-mode (e.g. Modrinth/itzg) pins the pack via the
// server's env and restarts (one active pack); deps-mode (e.g.
// Thunderstore) resolves the pack's dependencies and installs each.
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

  const providers = tmpl?.spec.capabilities?.mods?.registry?.providers ?? [];
  const declFor = (p: string) => providers.find((x) => x.provider === p);

  // Show whichever env-mode pack is currently pinned (provider-agnostic).
  const active = (() => {
    for (const p of providers) {
      const refEnv = p.modpacks?.refEnv;
      if (refEnv) {
        const v = gs?.spec.env?.find((e) => e.name === refEnv)?.value;
        if (v) return v;
      }
    }
    return undefined;
  })();

  const [banner, setBanner] = useState<Banner | null>(null);
  const [busy, setBusy] = useState<string | null>(null); // project id being installed

  const installEnv = useMutation({
    mutationFn: (v: { p: RegistryProject; provider: string }) =>
      Servers.installModpack(name, { ref: v.p.slug || v.p.id }, v.provider),
    onMutate: (v) => setBusy(v.p.id),
    onSuccess: (_r, v) => {
      setBanner({ kind: "ok", text: `Set modpack ${v.p.title}. The server is restarting to install it.` });
      return qc.invalidateQueries({ queryKey: ["server", name] });
    },
    onError: (err) => setBanner({ kind: "err", text: errMsg(err) }),
    onSettled: () => setBusy(null),
  });

  const installDeps = useMutation({
    mutationFn: async (v: { p: RegistryProject; provider: string }) => {
      const files = await Servers.modpackDeps(name, v.p.id, v.provider);
      for (const f of files) {
        await Servers.installMod(name, { url: f.downloadUrl, name: f.filename });
      }
      return files.length;
    },
    onMutate: (v) => setBusy(v.p.id),
    onSuccess: (count, v) => {
      setBanner({ kind: "ok", text: `Installed ${v.p.title} — ${count} mod${count === 1 ? "" : "s"}.` });
      return qc.invalidateQueries({ queryKey: ["mods", name] });
    },
    onError: (err) => setBanner({ kind: "err", text: errMsg(err) }),
    onSettled: () => setBusy(null),
  });

  const install = (p: RegistryProject, provider: string) => {
    const v = { p, provider };
    if (declFor(provider)?.modpacks?.refEnv) installEnv.mutate(v);
    else installDeps.mutate(v);
  };

  return (
    <div className="flex h-full flex-col gap-4 p-6">
      <header className="space-y-0.5">
        <h2 className="text-sm text-muted">Browse modpacks</h2>
        <p className="text-[11px] text-muted">
          Installing a modpack either pins it on the server (and restarts) or adds all of its mods,
          depending on the registry.
        </p>
      </header>

      {active && (
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
          categories={MODPACK_CATEGORIES}
          renderItem={(p, provider) => (
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
                onClick={() => install(p, provider)}
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
