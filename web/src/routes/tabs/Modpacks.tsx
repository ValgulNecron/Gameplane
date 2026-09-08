import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Package, X } from "lucide-react";
import { Button, Card, Alert } from "@heroui/react";

import type { GameTemplate, RegistryProject } from "@/types";
import { Servers } from "@/lib/endpoints";
import { APIError } from "@/lib/api";
import { errorText } from "@/lib/errors";
import { useMe, can } from "@/lib/auth";
import { RegistryBrowser, RegistryIcon, compactNum } from "@/components/registry-browser";

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
  ns,
}: {
  name: string;
  tmpl?: GameTemplate;
  ns?: string;
}) {
  const qc = useQueryClient();
  const { data: me } = useMe();
  const canManage = can(me, "servers:write");

  const providers = tmpl?.spec.capabilities?.mods?.registry?.providers ?? [];
  const declFor = (p: string) => providers.find((x) => x.provider === p);

  const [banner, setBanner] = useState<Banner | null>(null);
  const [busy, setBusy] = useState<string | null>(null); // project id being installed

  const installEnv = useMutation({
    mutationFn: (v: { p: RegistryProject; provider: string }) =>
      Servers.installModpack(name, { ref: v.p.slug || v.p.id }, v.provider, ns),
    onMutate: (v) => setBusy(v.p.id),
    onSuccess: (_r, v) => {
      setBanner({ kind: "ok", text: `Set modpack ${v.p.title}. The server is restarting to install it.` });
      return qc.invalidateQueries({ queryKey: ["server", name, ns] });
    },
    onError: (err) => setBanner({ kind: "err", text: errMsg(err) }),
    onSettled: () => setBusy(null),
  });

  const installDeps = useMutation({
    mutationFn: async (v: { p: RegistryProject; provider: string }) => {
      const files = await Servers.modpackDeps(name, v.p.id, v.provider, ns);
      for (const f of files) {
        await Servers.installMod(name, { url: f.downloadUrl, name: f.filename }, ns);
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


      {banner && (
        <Alert status={banner.kind === "ok" ? "success" : "danger"} className="flex items-start justify-between gap-3">
          <Alert.Content className="flex flex-1 flex-col gap-0.5">
            <Alert.Description className="text-sm">{banner.text}</Alert.Description>
          </Alert.Content>
          <Button variant="ghost" size="sm" onPress={() => setBanner(null)} isIconOnly aria-label="dismiss">
            <X className="h-3 w-3" />
          </Button>
        </Alert>
      )}

      <div className="min-h-0 flex-1">
        <RegistryBrowser
          name={name}
          type="modpack"
          categories={MODPACK_CATEGORIES}
          renderItem={(p, provider) => (
            <Card className="flex flex-row items-center gap-3 p-4">
              <div className="shrink-0">
                <RegistryIcon url={p.iconUrl} fallback={<Package className="h-9 w-9 rounded p-2 text-muted" />} />
              </div>
              <div className="min-w-0 flex-1">
                <div className="truncate text-sm font-medium">{p.title}</div>
                <div className="truncate text-xs text-muted">
                  {[p.author, p.downloads != null ? `${compactNum(p.downloads)} downloads` : null]
                    .filter(Boolean)
                    .join(" · ")}
                </div>
              </div>
              {/* The disabled reason lives on a wrapping element's title
                  (not the Button's aria-label) so the button's accessible
                  name stays its visible text ("Install"/"Installing…") for
                  role queries and assistive tech alike. */}
              <span title={canManage ? undefined : "Requires operator role"} tabIndex={0}>
                <Button
                  size="sm"
                  variant="primary"
                  isDisabled={!canManage || busy !== null}
                  onPress={() => install(p, provider)}
                >
                  {busy === p.id ? "Installing…" : "Install"}
                </Button>
              </span>
            </Card>
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
  return errorText(err, "request failed");
}
