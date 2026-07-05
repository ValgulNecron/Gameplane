import { useState, type ChangeEvent, type ReactNode } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Archive,
  Bell,
  BellRing,
  Boxes,
  Cog,
  Info,
  Key,
  Lock,
  Mail,
  MessagesSquare,
  Plus,
  RefreshCcw,
  ShieldCheck,
  Slack,
  Activity,
  Trash2,
  Webhook,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Card } from "@/components/ui/card";
import { PageHeader } from "@/components/PageHeader";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { Select } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { cn, formatRelative } from "@/lib/utils";
import { Auth, AuthProviders, BackupDestinations, Cluster, Notifications } from "@/lib/endpoints";
import type { ClusterInfo } from "@/types";
import {
  useConfig,
  useUpdateConfigSection,
  type AuthCfg,
  type AuthKind,
  type AuthProvider,
  type GeneralCfg,
  type NotifEventType,
  type NotifSink,
  type NotificationsCfg,
  type SinkKind,
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
          {cfg.data && section === "auth"          && <AuthSection          initial={cfg.data.auth} general={cfg.data.general} />}
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

// The managed-Secret name the API derives for a provider's clientSecret
// (mirrors providerSecretPrefix in api/internal/handlers).
const providerSecretPrefix = "gameplane-auth-";
const maxProviderName = 63 - providerSecretPrefix.length;

function AuthSection({ initial, general }: { initial?: AuthCfg; general?: GeneralCfg }) {
  const f = useSectionForm<AuthCfg>(initial ?? defaultAuth, "auth");
  const [adding, setAdding] = useState(false);
  // Runtime providers reveal whether a Helm-flag ("helm") provider exists;
  // it always counts as enabled and is managed in values.yaml, not here.
  const { data: runtime } = useQuery({
    queryKey: ["login-providers"],
    queryFn: () => Auth.providers().catch(() => null),
  });
  const helm = runtime?.providers.find((p) => p.name === "helm") ?? null;
  const enabledCount = f.draft.providers.filter((p) => p.enabled).length;
  // With a Helm provider present, login always stays possible, so the
  // last dashboard-managed toggle may be turned off.
  const lastToggleLocked = enabledCount === 1 && !helm;
  const togglerFor = (idx: number) => () => {
    const next = f.draft.providers.map((p, i) =>
      i === idx ? { ...p, enabled: !p.enabled } : p,
    );
    f.replace({ ...f.draft, providers: next });
  };
  const removeProvider = (idx: number) => {
    const p = f.draft.providers[idx];
    // Best-effort cleanup of the API-managed clientSecret Secret; the
    // server refuses Secrets it didn't create.
    if (!p.configRef || p.configRef === providerSecretPrefix + p.name) {
      void AuthProviders.deleteSecret(p.name).catch(() => undefined);
    }
    f.replace({ ...f.draft, providers: f.draft.providers.filter((_, i) => i !== idx) });
  };
  return (
    <SectionCard
      title="Authentication"
      subtitle="Built-in local accounts plus federated identity providers. Changes take effect on save — no restart needed."
      footer={
        <>
          <SaveStatus pending={f.pending} error={f.error} saved={f.saved} />
          <Button onClick={f.save} disabled={f.pending}>Save changes</Button>
        </>
      }
    >
      {f.draft.providers.length === 0 && !helm && (
        <div className="text-sm text-muted">No identity providers configured yet.</div>
      )}
      <ul className="divide-y divide-border">
        {f.draft.providers.map((p, idx) => (
          <li key={p.name} className="flex items-center gap-3 py-3">
            <div className="flex h-8 w-8 items-center justify-center rounded-md bg-surface">
              <Key className="h-4 w-4 text-muted" />
            </div>
            <div className="min-w-0 flex-1">
              <div className="text-sm">{p.displayName || p.name}</div>
              <div className="truncate text-xs text-muted">
                {p.kind}
                {p.issuer ? ` · ${p.issuer}` : p.configRef ? ` · ${p.configRef}` : ""}
              </div>
            </div>
            <button
              type="button"
              onClick={togglerFor(idx)}
              disabled={p.enabled && lastToggleLocked}
              title={
                p.enabled && lastToggleLocked
                  ? "At least one identity provider must stay enabled."
                  : undefined
              }
              className={cn(
                "rounded px-2 py-0.5 text-[10px] font-mono uppercase disabled:cursor-not-allowed disabled:opacity-60",
                p.enabled ? "bg-success/15 text-success" : "bg-muted/15 text-muted",
              )}
            >
              {p.enabled ? "Enabled" : "Disabled"}
            </button>
            {p.kind !== "local" && (
              <Button
                variant="ghost"
                size="icon"
                aria-label={`Delete provider ${p.name}`}
                onClick={() => removeProvider(idx)}
              >
                <Trash2 className="h-4 w-4" />
              </Button>
            )}
          </li>
        ))}
        {helm && (
          <li className="flex items-center gap-3 py-3">
            <div className="flex h-8 w-8 items-center justify-center rounded-md bg-surface">
              <Lock className="h-4 w-4 text-muted" />
            </div>
            <div className="min-w-0 flex-1">
              <div className="text-sm">{helm.label}</div>
              <div className="truncate text-xs text-muted">
                oidc · configured via Helm — manage in values.yaml
              </div>
            </div>
            <span className="rounded bg-success/15 px-2 py-0.5 text-[10px] font-mono uppercase text-success">
              Enabled
            </span>
          </li>
        )}
      </ul>
      {lastToggleLocked && (
        <p className="mt-2 text-xs text-muted">
          At least one identity provider must stay enabled — the last enabled
          provider can&apos;t be turned off.
        </p>
      )}
      {adding ? (
        <AddProviderForm
          existing={f.draft.providers.map((p) => p.name)}
          externalURLSet={Boolean(general?.externalURL)}
          onAdd={(p) => f.replace({ ...f.draft, providers: [...f.draft.providers, p] })}
          onClose={() => setAdding(false)}
        />
      ) : (
        <Button variant="outline" size="sm" onClick={() => setAdding(true)}>
          <Plus className="mr-1.5 h-4 w-4" />
          Add provider
        </Button>
      )}
    </SectionCard>
  );
}

function AddProviderForm({
  existing,
  externalURLSet,
  onAdd,
  onClose,
}: {
  existing: string[];
  externalURLSet: boolean;
  onAdd: (p: AuthProvider) => void;
  onClose: () => void;
}) {
  const [kind, setKind] = useState<AuthKind>("oidc");
  const [name, setName] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [issuer, setIssuer] = useState("");
  const [clientID, setClientID] = useState("");
  const [clientSecret, setClientSecret] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  // Presets prefill what they can: Google is a real OIDC issuer;
  // github.com publishes no OIDC discovery for user login, so that kind
  // needs a bridge (Dex or similar) whose issuer goes here.
  const applyPreset = (k: AuthKind) => {
    setKind(k);
    setError(null);
    if (k === "google") {
      setIssuer("https://accounts.google.com");
      if (!displayName) setDisplayName("Google");
    } else if (k === "github" && !displayName) {
      setDisplayName("GitHub");
    }
  };

  const dns = /^[a-z0-9]([-a-z0-9]{0,61}[a-z0-9])?$/;
  const nameOk =
    dns.test(name) && name !== "helm" && name.length <= maxProviderName && !existing.includes(name);
  const valid = nameOk && /^https?:\/\/.+/.test(issuer) && clientID !== "" && clientSecret !== "";

  // Store the clientSecret first; the provider row references the
  // returned Secret and lands in the config on the section's Save.
  const submit = async () => {
    setBusy(true);
    setError(null);
    try {
      const res = await AuthProviders.putSecret(name, { clientSecret });
      onAdd({
        name,
        kind,
        ...(displayName ? { displayName } : {}),
        enabled: true,
        issuer,
        clientID,
        configRef: res.name,
      });
      onClose();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to store the client secret");
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="space-y-3 rounded-md border border-border bg-surface/30 p-4">
      <div className="text-sm font-medium">Add identity provider</div>
      {!externalURLSet && (
        <p className="text-xs text-warning">
          Set <span className="font-mono">General → External URL</span> first — it forms the
          provider&apos;s OIDC redirect URL.
        </p>
      )}
      <div className="grid gap-3 md:grid-cols-2">
        <FieldLabel label="Kind">
          <Select
            aria-label="Provider kind"
            value={kind}
            onValueChange={(v) => applyPreset(v as AuthKind)}
            options={[
              { value: "oidc", label: "Generic OIDC" },
              { value: "google", label: "Google" },
              { value: "github", label: "GitHub (via OIDC bridge)" },
            ]}
          />
          {kind === "github" && (
            <span className="text-[11px] text-muted">
              GitHub.com has no OIDC discovery for user login — run an OIDC bridge such as
              Dex and enter its issuer URL below.
            </span>
          )}
        </FieldLabel>
        <FieldLabel label="Name">
          <Input placeholder="corp-sso" value={name} onChange={(e) => setName(e.target.value)} />
          <span className="text-[11px] text-muted">
            A short lowercase identifier (letters, digits, and dashes) used in the login
            route and the Secret name.
          </span>
        </FieldLabel>
        <FieldLabel label="Display name (login button)">
          <Input
            placeholder="Acme SSO"
            value={displayName}
            onChange={(e) => setDisplayName(e.target.value)}
          />
        </FieldLabel>
        <FieldLabel label="Issuer URL">
          <Input
            placeholder="https://idp.example.com"
            value={issuer}
            onChange={(e) => setIssuer(e.target.value)}
          />
        </FieldLabel>
        <FieldLabel label="Client ID">
          <Input value={clientID} onChange={(e) => setClientID(e.target.value)} />
        </FieldLabel>
        <FieldLabel label="Client secret">
          <Input
            type="password"
            value={clientSecret}
            onChange={(e) => setClientSecret(e.target.value)}
          />
        </FieldLabel>
      </div>
      {error && <p className="text-xs text-danger">{error}</p>}
      <div className="flex justify-end gap-2 pt-1">
        <Button variant="ghost" onClick={onClose}>Cancel</Button>
        <Button disabled={!valid || busy} onClick={() => void submit()}>
          {busy ? "Storing…" : "Add provider"}
        </Button>
      </div>
    </div>
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
        Stored as a Secret labelled <span className="font-mono">gameplane.local/backup-destination=true</span>.
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

const allNotifEvents: NotifEventType[] = [
  "server.unhealthy",
  "server.recovered",
  "backup.failed",
  "backup.succeeded",
  "restore.failed",
  "restore.succeeded",
];

// Mirrors notify.DefaultOn server-side: failures plus the paired recovery.
const defaultOnEvents: NotifEventType[] = [
  "server.unhealthy",
  "server.recovered",
  "backup.failed",
  "restore.failed",
];

const sinkIcons: Record<SinkKind, typeof Bell> = {
  discord: MessagesSquare,
  slack: Slack,
  smtp: Mail,
  webhook: Webhook,
  ntfy: BellRing,
};

function EventChip({ label }: { label: string }) {
  return (
    <span className="rounded-full bg-surface px-2 py-0.5 text-[11px] text-muted">
      {label}
    </span>
  );
}

function EventChips({ events }: { events?: NotifEventType[] }) {
  if (!events || events.length === 0) return <EventChip label="default events" />;
  return (
    <>
      {events.slice(0, 2).map((e) => (
        <EventChip key={e} label={e} />
      ))}
      {events.length > 2 && <EventChip label={`+${events.length - 2}`} />}
    </>
  );
}

// The managed-Secret name the API derives for a sink (mirrors
// sinkSecretPrefix in api/internal/handlers/notifications.go). The
// configRef must stay a DNS label (≤63), which caps the sink name.
const sinkSecretPrefix = "gameplane-notify-";
const maxSinkName = 63 - sinkSecretPrefix.length;

function AddSinkForm({
  existing,
  onAdd,
  onClose,
}: {
  existing: string[];
  onAdd: (s: NotifSink) => void;
  onClose: () => void;
}) {
  const [name, setName] = useState("");
  const [kind, setKind] = useState<SinkKind>("discord");
  const [events, setEvents] = useState<NotifEventType[]>(defaultOnEvents);
  const [creds, setCreds] = useState<Record<string, string>>({ tls: "starttls" });
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const cred = (key: string) => creds[key] ?? "";
  const setCred = (key: string) => (e: ChangeEvent<HTMLInputElement>) => {
    setError(null);
    setCreds((c) => ({ ...c, [key]: e.target.value }));
  };

  const dns = /^[a-z0-9]([-a-z0-9]{0,61}[a-z0-9])?$/;
  const nameOk = dns.test(name) && !existing.includes(name) && name.length <= maxSinkName;
  const credsOk =
    kind === "smtp"
      ? cred("host") !== "" && cred("from") !== "" && cred("to") !== ""
      : /^https?:\/\/.+/.test(cred("url"));
  const toggleEvent = (ev: NotifEventType) =>
    setEvents((cur) => (cur.includes(ev) ? cur.filter((e) => e !== ev) : [...cur, ev]));

  // Store the credential Secret first; the sink row references it via the
  // returned configRef and lands in the config on the section's Save.
  const submit = async () => {
    setBusy(true);
    setError(null);
    try {
      const body =
        kind === "smtp"
          ? {
              kind,
              host: cred("host"),
              port: cred("port"),
              username: cred("username"),
              password: cred("password"),
              from: cred("from"),
              to: cred("to"),
              tls: cred("tls"),
            }
          : kind === "ntfy"
            ? { kind, url: cred("url"), token: cred("token") }
            : { kind, url: cred("url"), authorization: cred("authorization") };
      const res = await Notifications.putSecret(name, body);
      onAdd({ name, kind, enabled: true, configRef: res.name, events });
      onClose();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to store the sink credentials");
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="space-y-3 rounded-md border border-border bg-surface/30 p-4">
      <div className="text-sm font-medium">Add sink</div>
      <div className="grid gap-3 md:grid-cols-2">
        <FieldLabel label="Name">
          <Input
            placeholder="team-alerts"
            value={name}
            onChange={(e) => setName(e.target.value)}
          />
          <span className="text-[11px] text-muted">
            A short lowercase identifier (letters, digits, and dashes) used to
            name the sink and its Secret — like <span className="font-mono">team-alerts</span>.
          </span>
        </FieldLabel>
        <FieldLabel label="Kind">
          <Select
            aria-label="Sink kind"
            value={kind}
            onValueChange={(v) => setKind(v as SinkKind)}
            options={["discord", "slack", "smtp", "webhook", "ntfy"].map((k) => ({
              value: k,
              label: k,
            }))}
          />
        </FieldLabel>
        {kind === "smtp" ? (
          <>
            <FieldLabel label="SMTP host">
              <Input placeholder="mail.example.com" value={cred("host")} onChange={setCred("host")} />
            </FieldLabel>
            <FieldLabel label="Port (default 587)">
              <Input placeholder="587" value={cred("port")} onChange={setCred("port")} />
            </FieldLabel>
            <FieldLabel label="Username (optional)">
              <Input value={cred("username")} onChange={setCred("username")} />
            </FieldLabel>
            <FieldLabel label="Password (optional)">
              <Input type="password" value={cred("password")} onChange={setCred("password")} />
            </FieldLabel>
            <FieldLabel label="From address">
              <Input placeholder="gameplane@example.com" value={cred("from")} onChange={setCred("from")} />
            </FieldLabel>
            <FieldLabel label="To (comma-separated)">
              <Input placeholder="ops@example.com" value={cred("to")} onChange={setCred("to")} />
            </FieldLabel>
            <FieldLabel label="TLS">
              <Select
                aria-label="SMTP TLS mode"
                value={cred("tls")}
                onValueChange={(v) => setCreds((c) => ({ ...c, tls: v }))}
                options={["starttls", "implicit", "none"].map((m) => ({ value: m, label: m }))}
              />
            </FieldLabel>
          </>
        ) : (
          <>
            <FieldLabel label={kind === "ntfy" ? "Topic URL" : "Webhook URL"}>
              <Input
                placeholder={
                  kind === "ntfy"
                    ? "https://ntfy.sh/my-topic"
                    : kind === "discord"
                      ? "https://discord.com/api/webhooks/…"
                      : kind === "slack"
                        ? "https://hooks.slack.com/services/…"
                        : "https://example.com/hook"
                }
                value={cred("url")}
                onChange={setCred("url")}
              />
            </FieldLabel>
            {kind === "ntfy" && (
              <FieldLabel label="Access token (optional)">
                <Input type="password" placeholder="tk_…" value={cred("token")} onChange={setCred("token")} />
              </FieldLabel>
            )}
            {kind === "webhook" && (
              <FieldLabel label="Authorization header (optional)">
                <Input type="password" placeholder="Bearer …" value={cred("authorization")} onChange={setCred("authorization")} />
              </FieldLabel>
            )}
          </>
        )}
      </div>
      <div className="space-y-1.5">
        <div className="text-xs text-muted">Events (failures and recovery are pre-selected)</div>
        <div className="grid gap-1.5 sm:grid-cols-2">
          {allNotifEvents.map((ev) => (
            <label key={ev} className="flex items-center gap-2 text-xs text-fg">
              <input
                type="checkbox"
                className="accent-primary"
                checked={events.includes(ev)}
                onChange={() => toggleEvent(ev)}
              />
              <span className="font-mono">{ev}</span>
            </label>
          ))}
        </div>
      </div>
      {error && <p className="text-xs text-danger">{error}</p>}
      <div className="flex justify-end gap-2 pt-1">
        <Button variant="ghost" onClick={onClose}>Cancel</Button>
        <Button disabled={!nameOk || !credsOk || busy} onClick={() => void submit()}>
          {busy ? "Storing…" : "Add sink"}
        </Button>
      </div>
    </div>
  );
}

function NotificationsSection({ initial }: { initial?: NotificationsCfg }) {
  const f = useSectionForm<NotificationsCfg>(initial ?? defaultNotif, "notifications");
  const [adding, setAdding] = useState(false);
  const [testResults, setTestResults] = useState<
    Record<string, { ok: boolean; message: string }>
  >({});
  const test = useMutation({ mutationFn: (name: string) => Notifications.test(name) });
  // The test endpoint fires against the *persisted* config; with unsaved
  // edits in the draft it would test something other than what's shown.
  const dirty = JSON.stringify(f.draft) !== JSON.stringify(initial ?? defaultNotif);

  const runTest = (name: string) =>
    test.mutate(name, {
      onSuccess: () =>
        setTestResults((r) => ({ ...r, [name]: { ok: true, message: "delivered" } })),
      onError: (e) =>
        setTestResults((r) => ({
          ...r,
          [name]: { ok: false, message: e instanceof Error ? e.message : "delivery failed" },
        })),
    });

  return (
    <SectionCard
      title="Notifications"
      subtitle="Deliver server health and backup/restore events to Discord, Slack, ntfy, email, or webhooks. Credentials are entered when adding a sink and stored as a labelled Secret in the control-plane namespace."
      footer={
        <>
          <SaveStatus pending={f.pending} error={f.error} saved={f.saved} />
          <Button onClick={f.save} disabled={f.pending}>Save changes</Button>
        </>
      }
    >
      {f.draft.sinks.length === 0 && !adding && (
        <div className="text-sm text-muted">No notification sinks configured.</div>
      )}
      <ul className="divide-y divide-border">
        {f.draft.sinks.map((s, idx) => {
          const Icon = sinkIcons[s.kind] ?? Bell;
          const result = testResults[s.name];
          return (
            <li key={s.name} className="flex items-center gap-3 py-3">
              <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md bg-surface">
                <Icon className="h-4 w-4 text-muted" />
              </div>
              <div className="min-w-0 flex-1">
                <div className="text-sm">{s.name}</div>
                <div className="truncate text-xs text-muted">
                  {s.kind}
                  {s.configRef ? ` · Secret: ${s.configRef}` : " · no Secret configured"}
                </div>
              </div>
              {s.configRef ? (
                <div className="hidden items-center gap-1.5 lg:flex">
                  <EventChips events={s.events} />
                </div>
              ) : (
                <span className="rounded-full bg-warning/15 px-2 py-0.5 text-[11px] font-medium text-warning">
                  Needs secret
                </span>
              )}
              {result && (
                <span
                  className={cn(
                    "max-w-48 truncate text-[11px]",
                    result.ok ? "text-success" : "text-danger",
                  )}
                >
                  {result.ok ? "✓ delivered" : result.message}
                </span>
              )}
              <Button
                variant="outline"
                size="sm"
                disabled={!s.configRef || dirty || test.isPending}
                title={dirty ? "Save changes first — tests run against the saved config" : undefined}
                onClick={() => runTest(s.name)}
              >
                Send test
              </Button>
              <Switch
                aria-label={s.enabled ? `Disable sink ${s.name}` : `Enable sink ${s.name}`}
                checked={s.enabled}
                onCheckedChange={(v) => {
                  const next = f.draft.sinks.map((x, i) =>
                    i === idx ? { ...x, enabled: v } : x,
                  );
                  f.update({ sinks: next });
                }}
              />
              <Button
                variant="ghost"
                size="icon"
                aria-label={`Delete sink ${s.name}`}
                onClick={() => {
                  // Best-effort cleanup of the API-managed Secret; the
                  // server refuses user-created Secrets, and a failure
                  // only leaves an orphaned Secret behind.
                  if (s.configRef === sinkSecretPrefix + s.name) {
                    void Notifications.deleteSecret(s.name).catch(() => undefined);
                  }
                  f.update({ sinks: f.draft.sinks.filter((_, i) => i !== idx) });
                }}
              >
                <Trash2 className="h-4 w-4" />
              </Button>
            </li>
          );
        })}
      </ul>
      {adding ? (
        <AddSinkForm
          existing={f.draft.sinks.map((s) => s.name)}
          onAdd={(s) => f.update({ sinks: [...f.draft.sinks, s] })}
          onClose={() => setAdding(false)}
        />
      ) : (
        <Button variant="outline" size="sm" onClick={() => setAdding(true)}>
          <Plus className="mr-1.5 h-4 w-4" />
          Add sink
        </Button>
      )}
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

