import { useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { ArrowLeft, ArrowRight, Check, ExternalLink, Loader2, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { GameIcon } from "@/components/ui/game-icon";
import { APIError } from "@/lib/api";
import { Servers, Templates, type ServerCreate } from "@/lib/endpoints";
import { isValidK8sName, isValidQuantity, validateConfig } from "@/lib/validation";
import { cn } from "@/lib/utils";
import type { GameTemplate } from "@/types";

type Step = 1 | 2 | 3 | 4;
const STEP_LABELS = ["Template", "Configure", "Network", "Review"] as const;

interface WizardState {
  name: string;
  description: string;
  template: GameTemplate | null;
  config: Record<string, string>;
  cpuLimit: string;
  memoryLimit: string;
  storageSize: string;
  nodePlacement: "auto" | "pin" | "gpu";
  expose: "ClusterIP" | "NodePort" | "LoadBalancer";
  hostname: string;
}

const initial: WizardState = {
  name: "", description: "",
  template: null, config: {},
  cpuLimit: "4", memoryLimit: "8",
  storageSize: "50Gi", nodePlacement: "auto",
  expose: "NodePort", hostname: "",
};

function buildCreateBody(state: WizardState): ServerCreate {
  let nodeSelector: Record<string, string> | undefined;
  if (state.nodePlacement === "pin") nodeSelector = { "kestrel.gg/pinned": "true" };
  else if (state.nodePlacement === "gpu") nodeSelector = { "kestrel.gg/gpu": "true" };
  return {
    name: state.name,
    description: state.description || undefined,
    templateRef: { name: state.template!.metadata.name },
    config: state.config,
    storage: { size: state.storageSize },
    networking: {
      expose: state.expose,
      hostname: state.hostname || undefined,
    },
    resources: {
      limits: { cpu: state.cpuLimit, memory: `${state.memoryLimit}Gi` },
    },
    ...(nodeSelector ? { nodeSelector } : {}),
  };
}

type StepCheck = { ok: true } | { ok: false; reason: string };

function validateStep(step: Step, state: WizardState): StepCheck {
  if (step === 1) {
    return state.template ? { ok: true } : { ok: false, reason: "Pick a game template to continue" };
  }
  if (step === 2) {
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
  const [step, setStep] = useState<Step>(1);
  const [state, setState] = useState<WizardState>(initial);
  const nav = useNavigate();
  const qc = useQueryClient();

  const create = useMutation({
    mutationFn: () => Servers.create(buildCreateBody(state)),
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: ["servers"] });
      await nav({ to: "/servers/$name", params: { name: state.name } });
    },
  });

  const stepCheck = validateStep(step, state);
  const finalCheck =
    validateStep(1, state).ok && validateStep(2, state).ok ? { ok: true } as const : { ok: false } as const;

  return (
    <div className="grid min-h-full place-items-center bg-background p-6">
      <div className="w-full max-w-[960px] overflow-hidden rounded-xl border border-border bg-card shadow-2xl">
        <div className="flex items-start justify-between border-b border-border px-6 py-4">
          <div>
            <div className="text-lg font-semibold">New game server</div>
            <div className="pt-0.5 text-xs text-muted">
              Step {step} of 4 · {STEP_LABELS[step - 1]}
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

        <StepBar step={step} />

        <div className="grid gap-6 px-6 py-6 md:grid-cols-[1fr_260px]">
          <div>
            {step === 1 && <PickTemplate state={state} setState={setState} />}
            {step === 2 && <Configure    state={state} setState={setState} />}
            {step === 3 && <Network      state={state} setState={setState} />}
            {step === 4 && <Review       state={state} />}
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
            {!stepCheck.ok && step < 4 && (
              <span className="text-[11px] text-muted" data-testid="step-reason">
                {stepCheck.reason}
              </span>
            )}
            <Button
              variant="ghost"
              disabled={step === 1}
              onClick={() => setStep((s) => (s - 1) as Step)}
            >
              <ArrowLeft className="h-4 w-4" /> Back
            </Button>
            {step < 4 ? (
              <Button
                disabled={!stepCheck.ok}
                onClick={() => setStep((s) => (s + 1) as Step)}
              >
                Continue to {(STEP_LABELS as readonly string[])[step] ?? "Review"} <ArrowRight className="h-4 w-4" />
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

function StepBar({ step }: { step: Step }) {
  return (
    <ol className="flex items-center gap-2 border-b border-border px-6 py-3 text-xs">
      {STEP_LABELS.map((l, i) => {
        const idx = (i + 1) as Step;
        const active = idx === step;
        const done = idx < step;
        return (
          <li key={l} className="flex items-center gap-2">
            <span
              className={cn(
                "flex h-5 w-5 items-center justify-center rounded-full border font-mono text-[10px]",
                active ? "border-primary bg-primary/15 text-primary"
                       : done
                       ? "border-success bg-success/15 text-success"
                       : "border-border text-muted",
              )}
            >{done ? <Check className="h-3 w-3" /> : idx}</span>
            <span className={cn(active ? "text-fg" : "text-muted")}>{l}</span>
            {i < STEP_LABELS.length - 1 && <span className="text-muted">·</span>}
          </li>
        );
      })}
    </ol>
  );
}

function PickTemplate({ state, setState }: { state: WizardState; setState: (s: WizardState) => void }) {
  const { data } = useQuery({
    queryKey: ["templates"],
    queryFn: () => Templates.list(),
  });
  return (
    <div className="space-y-3">
      <div className="text-sm font-medium">Choose a game</div>
      <div className="grid gap-3 sm:grid-cols-2">
        {data?.items.map((t) => {
          const active = state.template?.metadata.name === t.metadata.name;
          return (
            <button
              key={t.metadata.name}
              onClick={() => setState({ ...state, template: t, config: {} })}
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
    </div>
  );
}

function Review({ state }: { state: WizardState }) {
  const rows: Array<[string, string]> = [
    ["Template",    state.template?.spec.displayName ?? "—"],
    ["Name",        state.name || "—"],
    ["CPU",         `${state.cpuLimit} cores`],
    ["Memory",      `${state.memoryLimit} GiB`],
    ["Storage",     state.storageSize],
    ["Placement",   state.nodePlacement],
    ["Expose",      state.expose],
    ["Hostname",    state.hostname || "—"],
  ];
  return (
    <div className="space-y-2 text-sm">
      {rows.map(([k, v]) => (
        <div key={k} className="flex justify-between border-b border-border py-1.5">
          <span className="text-muted">{k}</span>
          <span className="font-mono">{v}</span>
        </div>
      ))}
      {Object.keys(state.config).length > 0 && (
        <div className="pt-3">
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
    ? `apiVersion: kestrel.gg/v1alpha1
kind: GameServer
metadata:
  name: ${state.name || "server-name"}
spec:
  templateRef:
    name: ${state.template.metadata.name}
  resources:
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
