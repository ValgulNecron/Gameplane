import { useState, type ReactNode } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Archive,
  Bell,
  Boxes,
  Cog,
  Info,
  Key,
  Plus,
  RefreshCcw,
  ShieldCheck,
  Activity,
  Trash2,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Card } from "@/components/ui/card";
import { PageHeader } from "@/components/PageHeader";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { Switch } from "@/components/ui/switch";
import { cn, formatRelative } from "@/lib/utils";
import { BackupDestinations, Cluster } from "@/lib/endpoints";
import type { ClusterInfo } from "@/types";
import {
  useConfig,
  useUpdateConfigSection,
  type AuthCfg,
  type GeneralCfg,
  type NotificationsCfg,
  type TelemetryCfg,
  type UpdateChannel,
  type UpdatesCfg,
} from "@/lib/config";
import { ErrorBanner } from "@/components/backups/ErrorBanner";
import { FieldLabel } from "@/components/ui/field";
import { ModuleSourcesPanel } from "@/components/modules/ModuleSourcesPanel";

type Section =
  | "general" | "auth" | "backups" | "modules" | "notifications"
  | "telemetry" | "updates" | "about";

const sections: Array<{ key: Section; label: string; icon: typeof Cog }> = [
  { key: "general",       label: "General",             icon: Cog },
  { key: "auth",          label: "Authentication",      icon: ShieldCheck },
  { key: "backups",       label: "Backup destinations", icon: Archive },
  { key: "modules",       label: "Module sources",      icon: Boxes },
  { key: "notifications", label: "Notifications",       icon: Bell },
  { key: "telemetry",     label: "Telemetry",           icon: Activity },
  { key: "updates",       label: "Updates",             icon: RefreshCcw },
  { key: "about",         label: "About",               icon: Info },
];

export function AdminSettingsPage() {
  const [section, setSection] = useState<Section>("general");
  const cfg = useConfig();

  return (
    <div className="space-y-6 p-6">
      <PageHeader
        title="Admin settings"
        subtitle="Platform-wide configuration for this Gameplane instance."
      />

      <div className="grid gap-6 lg:grid-cols-[220px_1fr]">
        <nav className="space-y-0.5">
          {sections.map(({ key, label, icon: Icon }) => (
            <button
              key={key}
              onClick={() => setSection(key)}
              className={cn(
                "flex w-full items-center gap-2 rounded-md px-3 py-2 text-sm transition-colors",
                "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary",
                section === key
                  ? "bg-surface text-fg"
                  : "text-muted hover:bg-surface/60 hover:text-fg",
              )}
            >
              <Icon className="h-4 w-4" />
              <span>{label}</span>
            </button>
          ))}
        </nav>

        <div className="space-y-6">
          {cfg.isLoading && (
            <Card className="p-5 text-sm text-muted">Loading configuration…</Card>
          )}
          {cfg.isError && (
            <Card className="p-5 text-sm text-danger">
              Failed to load configuration. Refresh to retry.
            </Card>
          )}
          {cfg.data && section === "general"       && <GeneralSection       initial={cfg.data.general} />}
          {cfg.data && section === "auth"          && <AuthSection          initial={cfg.data.auth} />}
          {section === "backups"                   && <BackupDestSection />}
          {section === "modules"                   && <ModuleSourcesPanel />}
          {cfg.data && section === "notifications" && <NotificationsSection initial={cfg.data.notifications} />}
          {cfg.data && section === "telemetry"     && <TelemetrySection     initial={cfg.data.telemetry} />}
          {cfg.data && section === "updates"       && <UpdatesSection       initial={cfg.data.updates} />}
          {section === "about"                     && <AboutSection />}
        </div>
      </div>
    </div>
  );
}

function SectionCard({
  title,
  subtitle,
  footer,
  children,
}: {
  title: string;
  subtitle?: string;
  footer?: ReactNode;
  children: ReactNode;
}) {
  return (
    <Card className="p-5 space-y-4">
      <div>
        <div className="font-medium">{title}</div>
        {subtitle && <div className="pt-0.5 text-xs text-muted">{subtitle}</div>}
      </div>
      {children}
      {footer && <div className="flex items-center justify-end gap-3 pt-2">{footer}</div>}
    </Card>
  );
}

function Field({ label, children, hint }: { label: string; children: ReactNode; hint?: string }) {
  return (
    <label className="block space-y-1.5">
      <span className="text-xs text-muted">{label}</span>
      {children}
      {hint && <span className="text-[11px] text-muted">{hint}</span>}
    </label>
  );
}

function SaveStatus({
  pending,
  error,
  saved,
}: {
  pending: boolean;
  error: string | null;
  saved: boolean;
}) {
  if (pending) return <span className="text-xs text-muted">Saving…</span>;
  if (error)   return <span className="text-xs text-danger">{error}</span>;
  if (saved)   return <span className="text-xs text-success">Saved</span>;
  return null;
}

// useSectionForm wires the common pattern: hold a draft of the section
// payload, submit on Save, surface server validation errors as the
// rendered SaveStatus, and reset the "Saved" indicator on next edit.
function useSectionForm<T>(initial: T, section: Parameters<typeof useUpdateConfigSection>[0]) {
  const [draft, setDraft] = useState<T>(initial);
  const [error, setError] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);
  const mut = useUpdateConfigSection(section);

  const update = (patch: Partial<T>) => {
    setDraft((d) => ({ ...d, ...patch }));
    setError(null);
    setSaved(false);
  };

  const replace = (next: T) => {
    setDraft(next);
    setError(null);
    setSaved(false);
  };

  const save = () => {
    setSaved(false);
    setError(null);
    mut.mutate(draft as never, {
      onSuccess: () => setSaved(true),
      onError: (e) => setError(e instanceof Error ? e.message : "Save failed"),
    });
  };

  return { draft, update, replace, save, pending: mut.isPending, error, saved };
}

const defaultGeneral: GeneralCfg = {
  instanceName: "",
  externalURL: "",
  defaultNamespace: "gameplane-games",
};

function GeneralSection({ initial }: { initial?: GeneralCfg }) {
  const f = useSectionForm<GeneralCfg>(initial ?? defaultGeneral, "general");
  return (
    <SectionCard
      title="General"
      subtitle="Basic identity for this Gameplane instance."
      footer={
        <>
          <SaveStatus pending={f.pending} error={f.error} saved={f.saved} />
          <Button onClick={f.save} disabled={f.pending}>Save changes</Button>
        </>
      }
    >
      <Field label="Instance name" hint="Shown in the UI and OIDC replies.">
        <Input
          value={f.draft.instanceName}
          onChange={(e) => f.update({ instanceName: e.target.value })}
        />
      </Field>
      <Field label="External URL" hint="Canonical base URL, used in emails, webhooks, OIDC callbacks.">
        <Input
          value={f.draft.externalURL}
          onChange={(e) => f.update({ externalURL: e.target.value })}
        />
      </Field>
      <Field label="Default namespace" hint="Where new GameServers land by default.">
        <Input
          value={f.draft.defaultNamespace}
          onChange={(e) => f.update({ defaultNamespace: e.target.value })}
        />
      </Field>
    </SectionCard>
  );
}

const defaultAuth: AuthCfg = {
  providers: [{ name: "Local accounts", kind: "local", enabled: true }],
};

function AuthSection({ initial }: { initial?: AuthCfg }) {
  const f = useSectionForm<AuthCfg>(initial ?? defaultAuth, "auth");
  const togglerFor = (idx: number) => () => {
    const next = f.draft.providers.map((p, i) =>
      i === idx ? { ...p, enabled: !p.enabled } : p,
    );
    f.replace({ ...f.draft, providers: next });
  };
  return (
    <SectionCard
      title="Authentication"
      subtitle="Built-in local accounts plus any federated identity providers. Configuration of the credentials themselves lives outside this screen."
      footer={
        <>
          <SaveStatus pending={f.pending} error={f.error} saved={f.saved} />
          <Button onClick={f.save} disabled={f.pending}>Save changes</Button>
        </>
      }
    >
      {f.draft.providers.length === 0 && (
        <div className="text-sm text-muted">No identity providers configured yet.</div>
      )}
      <ul className="divide-y divide-border">
        {f.draft.providers.map((p, idx) => (
          <li key={p.name} className="flex items-center gap-3 py-3">
            <div className="flex h-8 w-8 items-center justify-center rounded-md bg-surface">
              <Key className="h-4 w-4 text-muted" />
            </div>
            <div className="min-w-0 flex-1">
              <div className="text-sm">{p.name}</div>
              <div className="truncate text-xs text-muted">
                {p.kind}{p.configRef ? ` · ${p.configRef}` : ""}
              </div>
            </div>
            <button
              type="button"
              onClick={togglerFor(idx)}
              className={cn(
                "rounded px-2 py-0.5 text-[10px] font-mono uppercase",
                p.enabled ? "bg-success/15 text-success" : "bg-muted/15 text-muted",
              )}
            >
              {p.enabled ? "Enabled" : "Disabled"}
            </button>
          </li>
        ))}
      </ul>
    </SectionCard>
  );
}

function BackupDestSection() {
  const qc = useQueryClient();
  const [adding, setAdding] = useState(false);
  const [deleting, setDeleting] = useState<string | null>(null);

  const list = useQuery({
    queryKey: ["backup-destinations"],
    queryFn: () => BackupDestinations.list(),
  });
  const remove = useMutation({
    mutationFn: (name: string) => BackupDestinations.remove(name),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["backup-destinations"] });
      setDeleting(null);
    },
  });

  const items = list.data?.items ?? [];

  return (
    <SectionCard
      title="Backup destinations"
      subtitle="Restic repositories for snapshots. Stored as labelled Kubernetes Secrets in the configured namespace."
      footer={
        <Button onClick={() => setAdding(true)} disabled={adding}>
          <Plus className="h-4 w-4" />
          Add destination
        </Button>
      }
    >
      {list.isLoading && <div className="text-sm text-muted">Loading destinations…</div>}
      {list.isError && <ErrorBanner err={list.error} />}
      {!list.isLoading && items.length === 0 && !adding && (
        <div className="text-sm text-muted">
          No backup destinations configured. Add one to enable snapshots.
        </div>
      )}
      {adding && <NewDestinationForm onClose={() => setAdding(false)} />}
      <ul className="divide-y divide-border">
        {items.map((d) => (
          <li key={d.name} className="flex items-center gap-3 py-3">
            <div className="flex h-8 w-8 items-center justify-center rounded-md bg-surface">
              <Archive className="h-4 w-4 text-muted" />
            </div>
            <div className="min-w-0 flex-1">
              <div className="text-sm">{d.name}</div>
              <div className="truncate font-mono text-xs text-muted">{d.url}</div>
            </div>
            <span className="rounded bg-success/15 px-2 py-0.5 text-[10px] font-mono uppercase text-success">
              {d.hasPassword ? "configured" : "no-password"}
            </span>
            {d.createdAt && (
              <span className="text-[11px] text-muted">{formatRelative(d.createdAt)}</span>
            )}
            <button
              type="button"
              aria-label={`Delete ${d.name}`}
              onClick={() => setDeleting(d.name)}
              className="rounded p-1 text-muted hover:bg-surface/60 hover:text-danger"
            >
              <Trash2 className="h-4 w-4" />
            </button>
          </li>
        ))}
      </ul>
      {remove.error && <ErrorBanner err={remove.error} />}
      <ConfirmDialog
        open={deleting !== null}
        onOpenChange={(open) => { if (!open) setDeleting(null); }}
        title="Delete backup destination?"
        description={
          <>
            <p>
              The repository at this destination will <strong>not</strong> be erased — only the
              credentials Gameplane uses to reach it. Existing backups remain intact;
              new backups against this destination will fail until it&apos;s re-added.
            </p>
            {deleting && (
              <p className="pt-2">
                Type <span className="font-mono">{deleting}</span> to confirm.
              </p>
            )}
          </>
        }
        confirmPhrase={deleting ?? undefined}
        confirmLabel="Delete"
        destructive
        busy={remove.isPending}
        onConfirm={() => deleting && remove.mutate(deleting)}
      />
    </SectionCard>
  );
}

function NewDestinationForm({ onClose }: { onClose: () => void }) {
  const qc = useQueryClient();
  const [form, setForm] = useState({ name: "", url: "", password: "" });
  const create = useMutation({
    mutationFn: () => BackupDestinations.upsert(form),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["backup-destinations"] });
      onClose();
    },
  });
  const valid =
    /^[a-z0-9]([-a-z0-9]{0,61}[a-z0-9])?$/.test(form.name) &&
    form.url.length > 0 &&
    form.password.length > 0;
  return (
    <div className="space-y-3 rounded-md border border-border bg-surface/30 p-4">
      <div className="text-sm font-medium">Add destination</div>
      <p className="text-xs text-muted">
        Stored as a Secret labelled <span className="font-mono">gameplane.gg/backup-destination=true</span>.
        Restic URL formats: <span className="font-mono">s3:host/bucket</span>, <span className="font-mono">b2:bucket</span>, <span className="font-mono">azure:account/container</span>, etc.
      </p>
      <div className="grid gap-3 md:grid-cols-2">
        <FieldLabel label="Name (DNS label)">
          <Input
            placeholder="gameplane-backup-repo"
            value={form.name}
            onChange={(e) => setForm({ ...form, name: e.target.value })}
          />
        </FieldLabel>
        <FieldLabel label="Restic URL">
          <Input
            placeholder="s3:s3.example.com/gameplane-bucket"
            value={form.url}
            onChange={(e) => setForm({ ...form, url: e.target.value })}
          />
        </FieldLabel>
        <FieldLabel label="Repository password">
          <Input
            type="password"
            placeholder="Strong, unique passphrase"
            value={form.password}
            onChange={(e) => setForm({ ...form, password: e.target.value })}
          />
        </FieldLabel>
      </div>
      {create.error && <ErrorBanner err={create.error} />}
      <div className="flex justify-end gap-2 pt-1">
        <Button variant="ghost" onClick={onClose}>Cancel</Button>
        <Button
          onClick={() => create.mutate()}
          disabled={!valid || create.isPending}
        >
          {create.isPending ? "Saving…" : "Save destination"}
        </Button>
      </div>
    </div>
  );
}

const defaultNotif: NotificationsCfg = { sinks: [] };

function NotificationsSection({ initial }: { initial?: NotificationsCfg }) {
  const f = useSectionForm<NotificationsCfg>(initial ?? defaultNotif, "notifications");
  return (
    <SectionCard
      title="Notifications"
      subtitle="Outbound webhooks and alert routing. Sink credentials are managed as Secrets outside this screen."
      footer={
        <>
          <SaveStatus pending={f.pending} error={f.error} saved={f.saved} />
          <Button onClick={f.save} disabled={f.pending}>Save changes</Button>
        </>
      }
    >
      {f.draft.sinks.length === 0 && (
        <div className="text-sm text-muted">No notification sinks configured.</div>
      )}
      <ul className="divide-y divide-border">
        {f.draft.sinks.map((s, idx) => (
          <li key={s.name} className="flex items-center gap-3 py-3">
            <div className="flex h-8 w-8 items-center justify-center rounded-md bg-surface">
              <Bell className="h-4 w-4 text-muted" />
            </div>
            <div className="min-w-0 flex-1">
              <div className="text-sm">{s.name}</div>
              <div className="truncate text-xs text-muted">{s.kind}</div>
            </div>
            <Switch
              aria-label={s.enabled ? "Disable sink" : "Enable sink"}
              checked={s.enabled}
              onCheckedChange={(v) => {
                const next = f.draft.sinks.map((x, i) =>
                  i === idx ? { ...x, enabled: v } : x,
                );
                f.update({ sinks: next });
              }}
            />
          </li>
        ))}
      </ul>
    </SectionCard>
  );
}

const defaultTelemetry: TelemetryCfg = { sendMetrics: false };

function TelemetrySection({ initial }: { initial?: TelemetryCfg }) {
  const f = useSectionForm<TelemetryCfg>(initial ?? defaultTelemetry, "telemetry");
  return (
    <SectionCard
      title="Telemetry"
      subtitle="Anonymous usage metrics help us prioritize work."
      footer={
        <>
          <SaveStatus pending={f.pending} error={f.error} saved={f.saved} />
          <Button onClick={f.save} disabled={f.pending}>Save changes</Button>
        </>
      }
    >
      <div className="flex items-center justify-between">
        <div>
          <div className="text-sm">Send anonymous usage metrics</div>
          <div className="pt-0.5 text-xs text-muted">
            No server names, player counts, or identifying data.
          </div>
        </div>
        <Switch
          aria-label={f.draft.sendMetrics ? "Disable telemetry" : "Enable telemetry"}
          checked={f.draft.sendMetrics}
          onCheckedChange={(v) => f.update({ sendMetrics: v })}
        />
      </div>
    </SectionCard>
  );
}

const defaultUpdates: UpdatesCfg = { channel: "stable" };

function UpdatesSection({ initial }: { initial?: UpdatesCfg }) {
  const f = useSectionForm<UpdatesCfg>(initial ?? defaultUpdates, "updates");
  return (
    <SectionCard
      title="Updates"
      subtitle="Gameplane upgrade channel."
      footer={
        <>
          <SaveStatus pending={f.pending} error={f.error} saved={f.saved} />
          <Button onClick={f.save} disabled={f.pending}>Save changes</Button>
        </>
      }
    >
      <div className="flex items-center justify-between">
        <div>
          <div className="text-sm">Channel</div>
          <div className="pt-0.5 text-xs text-muted">Which releases to offer for updates.</div>
        </div>
        <select
          value={f.draft.channel}
          onChange={(e) => f.update({ channel: e.target.value as UpdateChannel })}
          className="h-9 rounded-md border border-border bg-surface px-3 text-sm"
        >
          <option value="stable">stable</option>
          <option value="beta">beta</option>
          <option value="nightly">nightly</option>
        </select>
      </div>
    </SectionCard>
  );
}

function AboutSection() {
  // Real versions from the API (the control plane is built and released as
  // one unit, so a single Gameplane version is honest); "—" until loaded.
  const { data } = useQuery({
    queryKey: ["cluster-info"],
    queryFn: () => Cluster.info().catch(() => ({} as ClusterInfo)),
    staleTime: 60_000,
  });
  return (
    <SectionCard title="About" subtitle="This Gameplane build.">
      <dl className="grid grid-cols-[auto_1fr] gap-x-6 gap-y-3 text-sm">
        <dt className="text-muted">Gameplane</dt><dd className="font-mono">{data?.gameplaneVersion || "—"}</dd>
        <dt className="text-muted">Kubernetes</dt><dd className="font-mono">{data?.version || "—"}</dd>
        <dt className="text-muted">License</dt><dd>AGPL-3.0</dd>
      </dl>
    </SectionCard>
  );
}

