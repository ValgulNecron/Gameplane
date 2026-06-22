import { useEffect, useRef, useState } from "react";
import { useNavigate, useSearch } from "@tanstack/react-router";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { ArrowLeft, ArrowRight, Check, ExternalLink, Loader2, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { GameIcon } from "@/components/ui/game-icon";
import { APIError } from "@/lib/api";
import { Servers, Templates, type ServerCreate } from "@/lib/endpoints";
import {
  defaultVersionId,
  isValidK8sName,
  isValidQuantity,
  isValidVersion,
  validateConfig,
} from "@/lib/validation";
import { cn } from "@/lib/utils";
import { GAME_CATEGORIES, gameCategory, type GameCategory } from "@/lib/games";
import type { GameTemplate } from "@/types";

// Wizard steps are derived per-template: the "version" step only appears when
// the template declares a version catalog (spec.versions). Templates without
// one keep the original 4-step flow.
type StepKey = "template" | "version" | "configure" | "network" | "review";
const STEP_TITLES: Record<StepKey, string> = {
  template: "Template",
  version: "Version",
  configure: "Configure",
  network: "Network",
  review: "Review",
};

function stepsFor(template: GameTemplate | null): StepKey[] {
  const base: StepKey[] = ["template", "configure", "network", "review"];
  if ((template?.spec.versions?.length ?? 0) > 0) base.splice(1, 0, "version");
  return base;
}

interface WizardState {
  name: string;
  description: string;
  template: GameTemplate | null;
  version: string;
  config: Record<string, string>;
  cpuLimit: string;
  memoryLimit: string;
  storageSize: string;
  nodePlacement: "auto" | "pin" | "gpu";
  expose: "ClusterIP" | "NodePort" | "LoadBalancer";
  hostname: string;
  sourceRanges: string;
}

const initial: WizardState = {
  name: "", description: "",
  template: null, version: "", config: {},
  cpuLimit: "4", memoryLimit: "8",
  storageSize: "50Gi", nodePlacement: "auto",
  expose: "NodePort", hostname: "", sourceRanges: "",
};

// Split the CIDR allow-list textarea (newline- or comma-separated) into a
// clean list, dropping blanks.
export function parseSourceRanges(raw: string): string[] {
  return raw
    .split(/[\n,]/)
    .map((s) => s.trim())
    .filter(Boolean);
}

function buildCreateBody(state: WizardState): ServerCreate {
  let nodeSelector: Record<string, string> | undefined;
  if (state.nodePlacement === "pin") nodeSelector = { "gameplane.gg/pinned": "true" };
  else if (state.nodePlacement === "gpu") nodeSelector = { "gameplane.gg/gpu": "true" };
  return {
    name: state.name,
    description: state.description || undefined,
    templateRef: { name: state.template!.metadata.name },
    ...(state.version ? { version: state.version } : {}),
    config: state.config,
    storage: { size: state.storageSize },
    networking: {
      expose: state.expose,
      hostname: state.hostname || undefined,
      // Only meaningful for LoadBalancer; omit otherwise so we don't store a
      // range the operator would ignore.
      ...(state.expose === "LoadBalancer" && parseSourceRanges(state.sourceRanges).length > 0
        ? { sourceRanges: parseSourceRanges(state.sourceRanges) }
        : {}),
    },
    resources: {
      limits: { cpu: state.cpuLimit, memory: `${state.memoryLimit}Gi` },
    },
    ...(nodeSelector ? { nodeSelector } : {}),
  };
}

type StepCheck = { ok: true } | { ok: false; reason: string };

function validateStep(key: StepKey, state: WizardState): StepCheck {
  if (key === "template") {
    return state.template ? { ok: true } : { ok: false, reason: "Pick a game template to continue" };
  }
  if (key === "version") {
    return isValidVersion(state.template ?? undefined, state.version)
      ? { ok: true }
      : { ok: false, reason: "Choose a version to continue" };
  }
  if (key === "configure") {
    if (!isValidK8sName(state.name)) {
      return {
        ok: false,
        reason: state.name
          ? "Name must be lowercase letters, digits, dashes (max 63)"
          : "Server name is required",
      };
    }
    if (!isValidQuantity(state.storageSize)) {
      return { ok: false, reason: "Storage must be a Kubernetes quantity (e.g. 50Gi)" };
    }
    if (!isValidQuantity(state.cpuLimit) || !isValidQuantity(state.memoryLimit)) {
      return { ok: false, reason: "CPU and memory must be valid quantities" };
    }
    const cfgErrors = validateConfig(state.template?.spec.configSchema ?? [], state.config);
    if (cfgErrors.length > 0) {
      return { ok: false, reason: cfgErrors[0].message };
    }
    return { ok: true };
  }
  return { ok: true };
}

function errorMessage(err: unknown, name: string): { title: string; body: string } {
  if (err instanceof APIError) {
    if (err.status === 409) {
      return {
        title: `A server named ${name} already exists`,
        body: "Pick a different name, or open the existing server. Names must be unique inside the namespace.",
      };
    }
    if (err.status === 403) {
      return {
        title: "Not permitted",
        body: "Your role does not allow creating servers in this namespace.",
      };
    }
    return {
      title: `Create failed (${err.status})`,
      body: err.body.slice(0, 240) || "The API rejected the request.",
    };
  }
  return {
    title: "Create failed",
    body: err instanceof Error ? err.message : "Unknown error",
  };
}

export function CreateServerWizard() {
  const [stepIndex, setStepIndex] = useState(0);
  const [state, setState] = useState<WizardState>(initial);
  const nav = useNavigate();
  const qc = useQueryClient();

  // When arriving from the Modules catalog "Deploy" action
  // (/servers/new?template=<name>), pre-select that template once the list
  // loads. One-shot, so manual changes afterwards aren't clobbered.
  const search = useSearch({ from: "/app-layout/servers/new" });
  const { data: templates } = useQuery({ queryKey: ["templates"], queryFn: () => Templates.list() });
  const presetApplied = useRef(false);
  useEffect(() => {
    if (presetApplied.current || !search.template || !templates) return;
    const match = templates.items.find((t) => t.metadata.name === search.template);
    if (match) {
      presetApplied.current = true;
      setState((s) => ({ ...s, template: match, config: {}, version: defaultVersionId(match) ?? "" }));
    }
  }, [search.template, templates]);

  const create = useMutation({
    mutationFn: () => Servers.create(buildCreateBody(state)),
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: ["servers"] });
      await nav({ to: "/servers/$name", params: { name: state.name } });
    },
  });

  const steps = stepsFor(state.template);
  const currentKey: StepKey = steps[stepIndex] ?? "template";
  const isLast = stepIndex === steps.length - 1;
  const stepCheck = validateStep(currentKey, state);
  const finalCheck = steps
    .filter((k) => k !== "review")
    .every((k) => validateStep(k, state).ok)
    ? ({ ok: true } as const)
    : ({ ok: false } as const);

  return (
    <div className="grid min-h-full place-items-center bg-background p-6">
      <div className="w-full max-w-[960px] overflow-hidden rounded-xl border border-border bg-card shadow-2xl">
        <div className="flex items-start justify-between border-b border-border px-6 py-4">
          <div>
            <div className="text-lg font-semibold">New game server</div>
            <div className="pt-0.5 text-xs text-muted">
              Step {stepIndex + 1} of {steps.length} · {STEP_TITLES[currentKey]}
            </div>
          </div>
          <button
            onClick={() => nav({ to: "/" })}
            className="rounded p-1 text-muted hover:bg-border hover:text-fg"
            title="Close"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        <StepBar steps={steps} stepIndex={stepIndex} />

        <div className="grid gap-6 px-6 py-6 md:grid-cols-[1fr_260px]">
          <div>
            {currentKey === "template" && <PickTemplate state={state} setState={setState} />}
            {currentKey === "version" && <PickVersion state={state} setState={setState} />}
            {currentKey === "configure" && <Configure state={state} setState={setState} />}
            {currentKey === "network" && <Network state={state} setState={setState} />}
            {currentKey === "review" && (
              <Review
                state={state}
                onEdit={(key) => {
                  const idx = steps.indexOf(key);
                  if (idx >= 0) setStepIndex(idx);
                }}
              />
            )}
          </div>
          <Preview state={state} />
        </div>

        {create.isError && (
          <ErrorAlert {...errorMessage(create.error, state.name)} />
        )}

        <div className="flex items-center justify-between border-t border-border px-6 py-4">
          <a href="#" className="flex items-center gap-1 text-xs text-muted hover:text-fg">
            <ExternalLink className="h-3 w-3" /> Docs: Creating game servers
          </a>
          <div className="flex items-center gap-3">
            {!stepCheck.ok && !isLast && (
              <span className="text-[11px] text-muted" data-testid="step-reason">
                {stepCheck.reason}
              </span>
            )}
            {stepIndex === 0 ? (
              <Button variant="ghost" onClick={() => nav({ to: "/" })}>
                Cancel
              </Button>
            ) : (
              <Button
                variant="ghost"
                onClick={() => setStepIndex((i) => Math.max(0, i - 1))}
              >
                <ArrowLeft className="h-4 w-4" /> Back
              </Button>
            )}
            {!isLast ? (
              <Button
                disabled={!stepCheck.ok}
                onClick={() => setStepIndex((i) => Math.min(steps.length - 1, i + 1))}
              >
                Continue to {STEP_TITLES[steps[stepIndex + 1]]} <ArrowRight className="h-4 w-4" />
              </Button>
            ) : (
              <Button
                onClick={() => create.mutate()}
                disabled={create.isPending || !finalCheck.ok}
              >
                {create.isPending ? (
                  <>Creating… <Loader2 className="h-4 w-4 animate-spin" /></>
                ) : (
                  <>Create server <Check className="h-4 w-4" /></>
                )}
              </Button>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}

function ErrorAlert({ title, body }: { title: string; body: string }) {
  return (
    <div
      role="alert"
      className="mx-6 mb-4 rounded-md border border-danger/40 bg-danger/10 px-4 py-3 text-sm"
      data-testid="create-error"
    >
      <div className="font-medium text-danger">{title}</div>
      <div className="pt-1 text-xs text-danger/80">{body}</div>
    </div>
  );
}

function StepBar({ steps, stepIndex }: { steps: StepKey[]; stepIndex: number }) {
  return (
    <ol className="flex items-center gap-2 border-b border-border px-6 py-3 text-xs">
      {steps.map((key, i) => {
        const active = i === stepIndex;
        const done = i < stepIndex;
        return (
          <li key={key} className="flex items-center gap-2">
            <span
              className={cn(
                "flex h-5 w-5 items-center justify-center rounded-full border font-mono text-[10px]",
                active ? "border-primary bg-primary/15 text-primary"
                       : done
                       ? "border-success bg-success/15 text-success"
                       : "border-border text-muted",
              )}
            >{done ? <Check className="h-3 w-3" /> : i + 1}</span>
            <span className={cn(active ? "text-fg" : "text-muted")}>{STEP_TITLES[key]}</span>
            {i < steps.length - 1 && <span className="text-muted">·</span>}
          </li>
        );
      })}
    </ol>
  );
}

const TEMPLATE_CATEGORIES = GAME_CATEGORIES;
type TemplateCategory = GameCategory;

function PickTemplate({ state, setState }: { state: WizardState; setState: (s: WizardState) => void }) {
  const { data } = useQuery({
    queryKey: ["templates"],
    queryFn: () => Templates.list(),
  });
  const [q, setQ] = useState("");
  const [cat, setCat] = useState<TemplateCategory>("all");

  const needle = q.trim().toLowerCase();
  const filtered = (data?.items ?? []).filter((t) => {
    if (cat !== "all" && gameCategory(t.spec.game) !== cat) return false;
    if (needle) {
      const hay = `${t.spec.displayName} ${t.spec.game} ${t.spec.description ?? ""}`.toLowerCase();
      if (!hay.includes(needle)) return false;
    }
    return true;
  });

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center gap-2">
        <Input
          className="min-w-[160px] flex-1"
          placeholder="Search…"
          value={q}
          onChange={(e) => setQ(e.target.value)}
          aria-label="Search templates"
        />
        <div className="flex gap-1 rounded-md border border-border bg-surface/40 p-1">
          {TEMPLATE_CATEGORIES.map((c) => (
            <button
              key={c}
              onClick={() => setCat(c)}
              className={cn(
                "rounded px-3 py-1 text-xs font-medium",
                cat === c ? "bg-primary/15 text-primary" : "text-muted hover:text-fg",
              )}
            >
              {c === "all" ? "All" : c}
            </button>
          ))}
        </div>
      </div>
      <div className="grid gap-3 sm:grid-cols-2">
        {filtered.map((t) => {
          const active = state.template?.metadata.name === t.metadata.name;
          return (
            <button
              key={t.metadata.name}
              onClick={() => setState({ ...state, template: t, config: {}, version: defaultVersionId(t) ?? "" })}
              className={cn(
                "rounded-lg border p-4 text-left transition-colors",
                active ? "border-primary bg-primary/5"
                       : "border-border hover:bg-surface/60",
              )}
            >
              <div className="flex items-center gap-3">
                <GameIcon game={t.spec.game} size="md" />
                <div>
                  <div className="text-sm font-medium">{t.spec.displayName}</div>
                  <div className="text-[11px] text-muted">v{t.spec.version}</div>
                </div>
              </div>
              <p className="pt-2 text-xs text-muted line-clamp-2">
                {t.spec.description ?? "No description."}
              </p>
            </button>
          );
        })}
      </div>
      {filtered.length === 0 && (
        <p className="text-sm text-muted">No templates match your search.</p>
      )}
    </div>
  );
}

function PickVersion({ state, setState }: { state: WizardState; setState: (s: WizardState) => void }) {
  const versions = state.template?.spec.versions ?? [];
  return (
    <div className="space-y-3">
      <div className="text-sm font-medium">Choose a version</div>
      <p className="text-xs text-muted">
        Pick the version and loader for this server. Each loader keeps its own mods on a separate
        volume, so switching never clobbers another.
      </p>
      <div className="grid gap-3 sm:grid-cols-2">
        {versions.map((v) => {
          const active = state.version === v.id;
          return (
            <button
              key={v.id}
              onClick={() => setState({ ...state, version: v.id })}
              className={cn(
                "rounded-lg border p-4 text-left transition-colors",
                active ? "border-primary bg-primary/5"
                       : "border-border hover:bg-surface/60",
              )}
            >
              <div className="flex items-center justify-between gap-2">
                <div className="text-sm font-medium">{v.displayName}</div>
                {v.default && (
                  <span className="rounded-full bg-primary/15 px-2 py-0.5 text-[10px] text-primary">
                    Default
                  </span>
                )}
              </div>
              {v.loader && <div className="pt-1 text-[11px] text-muted">Loader: {v.loader}</div>}
            </button>
          );
        })}
      </div>
    </div>
  );
}

function Configure({ state, setState }: { state: WizardState; setState: (s: WizardState) => void }) {
  const fields = state.template?.spec.configSchema ?? [];
  return (
    <div className="space-y-4">
      <label className="block space-y-1.5">
        <span className="text-xs text-muted">Server name</span>
        <Input
          value={state.name}
          onChange={(e) => setState({ ...state, name: e.target.value })}
          placeholder="mc-hardcore"
          required
        />
        <span className="text-[11px] text-muted">
          Used as pod name and subdomain. Lowercase, dash-separated.
        </span>
      </label>
      <label className="block space-y-1.5">
        <span className="text-xs text-muted">Description</span>
        <textarea
          className="min-h-[72px] w-full rounded-md border border-border bg-surface px-3 py-2 text-sm text-fg placeholder:text-muted focus:border-primary focus:outline-none"
          value={state.description}
          onChange={(e) => setState({ ...state, description: e.target.value })}
          placeholder="Hardcore survival realm with curated mods. Invite only."
        />
      </label>

      <div className="grid gap-3 sm:grid-cols-2">
        <label className="block space-y-1.5">
          <span className="text-xs text-muted">CPU limit</span>
          <div className="relative">
            <Input
              type="number"
              min="1"
              value={state.cpuLimit}
              onChange={(e) => setState({ ...state, cpuLimit: e.target.value })}
            />
            <span className="absolute right-3 top-1/2 -translate-y-1/2 text-[11px] text-muted">cores</span>
          </div>
        </label>
        <label className="block space-y-1.5">
          <span className="text-xs text-muted">Memory limit</span>
          <div className="relative">
            <Input
              type="number"
              min="1"
              value={state.memoryLimit}
              onChange={(e) => setState({ ...state, memoryLimit: e.target.value })}
            />
            <span className="absolute right-3 top-1/2 -translate-y-1/2 text-[11px] text-muted">GiB</span>
          </div>
        </label>
      </div>

      <label className="block space-y-1.5">
        <span className="text-xs text-muted">Persistent storage</span>
        <Input
          value={state.storageSize}
          onChange={(e) => setState({ ...state, storageSize: e.target.value })}
          placeholder="50Gi"
        />
        <span className="text-[11px] text-muted">Mounted at /data (RWO PVC).</span>
      </label>

      <div className="space-y-1.5">
        <span className="text-xs text-muted">Node placement</span>
        <div className="inline-flex gap-1 rounded-md border border-border p-1">
          {(["auto", "pin", "gpu"] as const).map((p) => (
            <button
              key={p}
              onClick={() => setState({ ...state, nodePlacement: p })}
              className={cn(
                "rounded px-3 py-1.5 text-xs",
                state.nodePlacement === p
                  ? "bg-primary/15 text-primary"
                  : "text-muted hover:text-fg",
              )}
            >
              {p === "auto" ? "Auto (scheduler)" : p === "pin" ? "Pin to node" : "GPU-enabled"}
            </button>
          ))}
        </div>
        <div className="text-[11px] text-muted">
          {state.nodePlacement === "auto" && "Picks node with most free memory, tolerations or no taints."}
          {state.nodePlacement === "pin" && "Pins to a specific node (selectable after create)."}
          {state.nodePlacement === "gpu" && "Requires node with a GPU-capable device plugin."}
        </div>
      </div>

      {fields.length > 0 && (
        <div className="space-y-3 pt-3">
          <div className="text-xs uppercase tracking-wide text-muted">Template configuration</div>
          {fields.map((f) => (
            <label key={f.name} className="block space-y-1.5">
              <span className="text-xs text-muted">{f.displayName ?? f.name}</span>
              {f.type === "enum" ? (
                <select
                  className="h-9 w-full rounded-md border border-border bg-surface px-3 text-sm"
                  value={state.config[f.name] ?? f.default ?? ""}
                  onChange={(e) => setState({ ...state, config: { ...state.config, [f.name]: e.target.value } })}
                >
                  {f.enum?.map((v) => <option key={v} value={v}>{v}</option>)}
                </select>
              ) : f.type === "bool" ? (
                <select
                  className="h-9 w-full rounded-md border border-border bg-surface px-3 text-sm"
                  value={state.config[f.name] ?? f.default ?? "false"}
                  onChange={(e) => setState({ ...state, config: { ...state.config, [f.name]: e.target.value } })}
                >
                  <option value="true">true</option><option value="false">false</option>
                </select>
              ) : (
                <Input
                  type={f.type === "password" ? "password" : "text"}
                  value={state.config[f.name] ?? f.default ?? ""}
                  onChange={(e) => setState({ ...state, config: { ...state.config, [f.name]: e.target.value } })}
                />
              )}
              {f.description && <span className="text-[11px] text-muted">{f.description}</span>}
            </label>
          ))}
        </div>
      )}
    </div>
  );
}

function Network({ state, setState }: { state: WizardState; setState: (s: WizardState) => void }) {
  return (
    <div className="space-y-4">
      <div className="space-y-1.5">
        <span className="text-xs text-muted">Expose</span>
        <div className="grid gap-2 sm:grid-cols-3">
          {(["ClusterIP", "NodePort", "LoadBalancer"] as const).map((e) => (
            <button
              key={e}
              onClick={() => setState({ ...state, expose: e })}
              className={cn(
                "rounded-md border p-3 text-left text-sm",
                state.expose === e
                  ? "border-primary bg-primary/5"
                  : "border-border hover:bg-surface/60",
              )}
            >
              <div className="font-medium">{e}</div>
              <div className="pt-0.5 text-[11px] text-muted">
                {e === "ClusterIP" && "Internal to cluster only."}
                {e === "NodePort" && "Reach via any node's IP."}
                {e === "LoadBalancer" && "Provisions an external LB."}
              </div>
            </button>
          ))}
        </div>
      </div>
      <label className="block space-y-1.5">
        <span className="text-xs text-muted">Hostname (optional)</span>
        <Input
          value={state.hostname}
          onChange={(e) => setState({ ...state, hostname: e.target.value })}
          placeholder="mc.example.dev"
        />
      </label>
      {state.expose === "LoadBalancer" && (
        <label className="block space-y-1.5">
          <span className="text-xs text-muted">IP allow-list (CIDRs, optional)</span>
          <textarea
            className="block w-full rounded-md border border-border bg-surface px-3 py-2 font-mono text-sm"
            rows={3}
            value={state.sourceRanges}
            onChange={(e) => setState({ ...state, sourceRanges: e.target.value })}
            placeholder={"203.0.113.0/24\n10.0.0.0/8"}
            aria-label="IP allow-list"
          />
          <span className="text-[11px] text-muted">
            One CIDR per line. Restricts which clients reach the LoadBalancer; empty allows all.
          </span>
        </label>
      )}
    </div>
  );
}

function Review({ state, onEdit }: { state: WizardState; onEdit: (key: StepKey) => void }) {
  const hasVersions = (state.template?.spec.versions?.length ?? 0) > 0;
  const sections = [
    {
      key: "template",
      title: "Template",
      rows: [["Template", state.template?.spec.displayName ?? "—"]] as Array<[string, string]>,
    },
    ...(hasVersions
      ? [{ key: "version", title: "Version", rows: [["Version", state.version || "—"]] as Array<[string, string]> }]
      : []),
    {
      key: "configure",
      title: "Configuration",
      rows: [
        ["Name", state.name || "—"],
        ["CPU", `${state.cpuLimit} cores`],
        ["Memory", `${state.memoryLimit} GiB`],
        ["Storage", state.storageSize],
        ["Placement", state.nodePlacement],
      ] as Array<[string, string]>,
    },
    {
      key: "network",
      title: "Network",
      rows: [
        ["Expose", state.expose],
        ["Hostname", state.hostname || "—"],
        ...(state.expose === "LoadBalancer"
          ? [["IP allow-list", parseSourceRanges(state.sourceRanges).join(", ") || "all"] as [string, string]]
          : []),
      ] as Array<[string, string]>,
    },
  ] as Array<{ key: StepKey; title: string; rows: Array<[string, string]> }>;
  return (
    <div className="space-y-4 text-sm">
      {sections.map((sec) => (
        <div key={sec.key}>
          <div className="flex items-center justify-between pb-1">
            <span className="text-xs uppercase tracking-wide text-muted">{sec.title}</span>
            <button
              type="button"
              onClick={() => onEdit(sec.key)}
              className="text-xs text-primary hover:underline"
            >
              Edit
            </button>
          </div>
          {sec.rows.map(([k, v]) => (
            <div key={k} className="flex justify-between border-b border-border py-1.5">
              <span className="text-muted">{k}</span>
              <span className="font-mono">{v}</span>
            </div>
          ))}
        </div>
      ))}
      {Object.keys(state.config).length > 0 && (
        <div className="pt-1">
          <div className="pb-1 text-xs uppercase text-muted">Template config</div>
          <pre className="rounded bg-surface p-3 font-mono text-xs">
            {JSON.stringify(state.config, null, 2)}
          </pre>
        </div>
      )}
    </div>
  );
}

function Preview({ state }: { state: WizardState }) {
  const name = state.template?.spec.displayName ?? "Pick a template";
  const yaml = state.template
    ? `apiVersion: gameplane.gg/v1alpha1
kind: GameServer
metadata:
  name: ${state.name || "server-name"}
spec:
  templateRef:
    name: ${state.template.metadata.name}
${state.version ? `  version: ${state.version}\n` : ""}  resources:
    cpu: ${state.cpuLimit}
    memory: ${state.memoryLimit}Gi
    storage: ${state.storageSize}
`
    : "# Pick a template to preview the YAML.";
  return (
    <aside className="rounded-lg border border-border bg-surface/40">
      <div className="flex items-center gap-2 border-b border-border px-4 py-3">
        <GameIcon game={state.template?.spec.game} size="sm" />
        <div className="min-w-0">
          <div className="truncate text-sm font-medium">{name}</div>
          <div className="truncate text-[10px] text-muted">
            Template: {state.template?.metadata.name ?? "—"}
          </div>
        </div>
      </div>
      <pre className="max-h-72 overflow-auto px-4 py-3 font-mono text-[11px] leading-relaxed text-fg scrollbar-thin">
{yaml}
      </pre>
      <div className="border-t border-border px-4 py-3 text-[11px] text-muted">
        <span className="font-medium text-fg">Memory tip.</span>
        {" "}Factor in mod/plugin overhead; Minecraft Vanilla uses ~1.5 GB at 4 players.
      </div>
    </aside>
  );
}
