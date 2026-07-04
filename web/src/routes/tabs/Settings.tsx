import { useEffect, useMemo, useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  AlertTriangle,
  HardDrive,
  Layers,
  Network,
  Settings as SettingsIcon,
  ShieldCheck,
  Sliders,
  Variable,
} from "lucide-react";

import type { GameServer } from "@/types";
import { Button } from "@/components/ui/button";
import { APIError } from "@/lib/api";
import { Servers, Templates } from "@/lib/endpoints";
import { cn } from "@/lib/utils";

import { GeneralSection } from "./settings/General";
import { VersionSection } from "./settings/Version";
import { ResourcesSection } from "./settings/Resources";
import { NetworkingSection } from "./settings/Networking";
import { EnvVarsSection } from "./settings/EnvVars";
import { LifecycleSection } from "./settings/Lifecycle";
import { AccessSection } from "./settings/Access";
import { DangerSection } from "./settings/Danger";

type SectionKey =
  | "general"
  | "version"
  | "resources"
  | "networking"
  | "env"
  | "lifecycle"
  | "access"
  | "danger";

const SECTIONS: { key: SectionKey; label: string; icon: typeof SettingsIcon }[] = [
  { key: "general",    label: "General",       icon: SettingsIcon },
  { key: "version",    label: "Version",       icon: Layers },
  { key: "resources",  label: "Resources",     icon: HardDrive },
  { key: "networking", label: "Networking",    icon: Network },
  { key: "env",        label: "Environment",   icon: Variable },
  { key: "lifecycle",  label: "Lifecycle",     icon: Sliders },
  { key: "access",     label: "RBAC & access", icon: ShieldCheck },
  { key: "danger",     label: "Danger zone",   icon: AlertTriangle },
];

export interface SettingsTabProps {
  gs?: GameServer;
  name: string;
  onDirtyChange?: (dirty: boolean) => void;
}

export function SettingsTab({ gs, name, onDirtyChange }: SettingsTabProps) {
  const qc = useQueryClient();
  const [section, setSection] = useState<SectionKey>("general");
  const [draft, setDraft] = useState<GameServer | null>(null);
  const [conflict, setConflict] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [savedAt, setSavedAt] = useState<number | null>(null);
  const baselineRef = useRef<GameServer | null>(null);
  const lastSeenRef = useRef<GameServer | undefined>(undefined);

  // Initialize / reset the draft whenever a fresh server arrives and
  // the local form has no unsaved edits. We compare against the last gs
  // we processed (not against draft) so this effect only fires on real
  // gs changes, not on draft mutations.
  useEffect(() => {
    if (!gs) return;
    if (gs === lastSeenRef.current) return;
    lastSeenRef.current = gs;
    if (baselineRef.current && draft && isDirty(draft, baselineRef.current)) {
      // Local edits in flight — don't clobber.
      return;
    }
    const clone = structuredClone(gs);
    baselineRef.current = clone;
    setDraft(clone);
    setConflict(false);
  }, [gs, draft]);

  const dirty = useMemo(() => {
    if (!draft || !baselineRef.current) return false;
    return isDirty(draft, baselineRef.current);
  }, [draft]);

  useEffect(() => {
    onDirtyChange?.(dirty);
  }, [dirty, onDirtyChange]);

  const { data: template } = useQuery({
    queryKey: ["template", draft?.spec.templateRef.name],
    queryFn: () => Templates.get(draft!.spec.templateRef.name),
    enabled: !!draft?.spec.templateRef.name,
  });

  const save = useMutation({
    mutationFn: async (next: GameServer) => {
      // Re-fetch latest to merge edits onto the freshest copy. This
      // keeps fields the UI doesn't model (e.g. operator-managed status,
      // newly-added spec keys) from being clobbered.
      const latest = await Servers.get(name);
      const merged = mergeDraftOntoLatest(next, baselineRef.current!, latest);
      return Servers.update(name, merged);
    },
    onSuccess: (saved) => {
      const clone = structuredClone(saved);
      baselineRef.current = clone;
      setDraft(clone);
      setConflict(false);
      setError(null);
      setSavedAt(Date.now());
      return qc.invalidateQueries({ queryKey: ["server", name] });
    },
    onError: (err) => {
      if (err instanceof APIError && err.status === 409) {
        setConflict(true);
        setError(null);
      } else {
        setError(errMsg(err));
      }
    },
  });

  if (!draft) {
    return <div className="p-6 text-sm text-muted">Loading…</div>;
  }

  const reset = () => {
    if (baselineRef.current) {
      setDraft(structuredClone(baselineRef.current));
    }
    setConflict(false);
    setError(null);
  };

  const reload = async () => {
    const fresh = await Servers.get(name);
    const clone = structuredClone(fresh);
    baselineRef.current = clone;
    setDraft(clone);
    setConflict(false);
    setError(null);
    qc.setQueryData(["server", name], fresh);
  };

  const onChangeDraft = (next: GameServer) => {
    setDraft(next);
    if (savedAt) setSavedAt(null);
  };

  // The Version section only exists for templates with a version catalog.
  const sections = SECTIONS.filter(
    (s) => s.key !== "version" || (template?.spec.versions?.length ?? 0) > 0,
  );

  return (
    <div className="flex h-full">
      <nav className="w-56 shrink-0 border-r border-border bg-surface/30 p-2">
        {sections.map((s) => (
          <button
            key={s.key}
            onClick={() => setSection(s.key)}
            className={cn(
              "flex w-full items-center gap-2 rounded px-2 py-2 text-left text-sm transition-colors",
              section === s.key
                ? "bg-surface text-fg"
                : "text-muted hover:bg-surface/60 hover:text-fg",
              s.key === "danger" && section !== s.key && "text-danger/80 hover:text-danger",
              s.key === "danger" && section === s.key && "text-danger",
            )}
          >
            <s.icon className="h-4 w-4" />
            {s.label}
          </button>
        ))}
      </nav>

      <div className="flex min-w-0 flex-1 flex-col">
        <div className="flex-1 overflow-auto p-6 scrollbar-thin">
          {section === "general"    && <GeneralSection    draft={draft} onChange={onChangeDraft} template={template} />}
          {section === "version"    && <VersionSection    draft={draft} onChange={onChangeDraft} template={template} />}
          {section === "resources"  && <ResourcesSection  draft={draft} onChange={onChangeDraft} template={template} />}
          {section === "networking" && <NetworkingSection draft={draft} onChange={onChangeDraft} template={template} />}
          {section === "env"        && <EnvVarsSection    draft={draft} onChange={onChangeDraft} template={template} />}
          {section === "lifecycle"  && <LifecycleSection  draft={draft} onChange={onChangeDraft} template={template} />}
          {section === "access"     && <AccessSection     draft={draft} onChange={onChangeDraft} template={template} />}
          {section === "danger"     && <DangerSection     name={name} />}
        </div>

        {section !== "danger" && (
          <footer className="flex items-center justify-between gap-4 border-t border-border bg-surface/30 px-6 py-3">
            <div className="min-w-0 text-xs">
              {conflict && (
                <span className="text-warning">
                  Server changed since you opened this page.{" "}
                  <button onClick={reload} className="underline hover:text-fg">
                    Reload
                  </button>{" "}
                  to discard your edits and load the latest.
                </span>
              )}
              {error && !conflict && <span className="text-danger">{error}</span>}
              {!conflict && !error && dirty && (
                <span className="text-muted">Unsaved changes</span>
              )}
              {!conflict && !error && !dirty && savedAt && (
                <span className="text-muted">Saved.</span>
              )}
            </div>
            <div className="flex items-center gap-2">
              <Button
                variant="ghost"
                size="sm"
                onClick={reset}
                disabled={!dirty || save.isPending}
              >
                Discard
              </Button>
              <Button
                size="sm"
                onClick={() => save.mutate(draft)}
                disabled={!dirty || save.isPending}
              >
                {save.isPending ? "Saving…" : "Save changes"}
              </Button>
            </div>
          </footer>
        )}
      </div>
    </div>
  );
}

function isDirty(a: GameServer, b: GameServer): boolean {
  return JSON.stringify(serializeForDiff(a)) !== JSON.stringify(serializeForDiff(b));
}

// Strip status + resourceVersion before diffing so server-side updates
// (heartbeats, conditions) don't mark the form as dirty.
function serializeForDiff(gs: GameServer) {
  const meta = { ...gs.metadata };
  delete meta.resourceVersion;
  return { metadata: meta, spec: gs.spec };
}

// mergeDraftOntoLatest applies the user's edits (the diff between
// `draft` and the originally-loaded `baseline`) onto the freshest
// server-side object. Fields the UI doesn't touch are preserved.
function mergeDraftOntoLatest(
  draft: GameServer,
  baseline: GameServer,
  latest: GameServer,
): GameServer {
  // Start from the latest server-side object to keep its resourceVersion
  // and any unknown fields.
  const out = structuredClone(latest);

  // Apply spec wholesale from draft — every spec field surfaced in the
  // form is owned by the user. Anything we don't model in the draft we
  // also don't render, so adopting draft.spec is the desired behavior.
  out.spec = structuredClone(draft.spec);

  // Apply user-editable metadata (labels + our annotations) without
  // clobbering operator-managed annotations. We compute the diff between
  // baseline and draft and apply it onto latest.
  out.metadata = {
    ...latest.metadata,
    labels: structuredClone(draft.metadata.labels),
    annotations: mergeAnnotations(
      baseline.metadata.annotations ?? {},
      draft.metadata.annotations ?? {},
      latest.metadata.annotations ?? {},
    ),
  };

  return out;
}

function mergeAnnotations(
  baseline: Record<string, string>,
  draft: Record<string, string>,
  latest: Record<string, string>,
): Record<string, string> | undefined {
  const out: Record<string, string> = { ...latest };

  // Keys the user added or changed: copy draft value over.
  for (const [k, v] of Object.entries(draft)) {
    if (baseline[k] !== v) out[k] = v;
  }
  // Keys the user removed: drop them from the merged set.
  for (const k of Object.keys(baseline)) {
    if (!(k in draft)) delete out[k];
  }

  return Object.keys(out).length ? out : undefined;
}

function errMsg(err: unknown): string {
  if (err instanceof APIError) {
    try {
      const parsed = JSON.parse(err.body) as { error?: string; message?: string };
      if (parsed.error) return parsed.error;
      if (parsed.message) return parsed.message;
    } catch {
      // fall through
    }
    return err.body || `request failed (${err.status})`;
  }
  return err instanceof Error ? err.message : "save failed";
}
