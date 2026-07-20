import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import * as Dialog from "@radix-ui/react-dialog";
import {
  Backpack,
  Ban,
  Bot,
  Clock,
  Cloud,
  CloudLightning,
  CloudRain,
  Flag,
  Gamepad2,
  Gauge,
  Gift,
  Leaf,
  Lock,
  LockOpen,
  Megaphone,
  MessageSquare,
  MicOff,
  Pause,
  Play,
  RefreshCw,
  Repeat,
  RotateCcw,
  Save,
  Sun,
  UserMinus,
  UserPlus,
  Zap,
  type LucideIcon,
} from "lucide-react";

import type { ActionParamDecl, GameTemplate, ServerActionDecl } from "@/types";
import { Servers } from "@/lib/endpoints";
import { rconAvailable } from "@/lib/capabilities";
import { APIError } from "@/lib/api";
import { useMe, can } from "@/lib/auth";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { cn } from "@/lib/utils";

// Curated icon map for the lucide names modules declare. Unknown names
// fall back to a generic bolt so a typo never breaks the row.
const iconMap: Record<string, LucideIcon> = {
  megaphone: Megaphone,
  clock: Clock,
  cloud: Cloud,
  "user-plus": UserPlus,
  "user-minus": UserMinus,
  save: Save,
  ban: Ban,
  "message-square": MessageSquare,
  gauge: Gauge,
  pause: Pause,
  play: Play,
  flag: Flag,
  repeat: Repeat,
  bot: Bot,
  leaf: Leaf,
  "refresh-cw": RefreshCw,
  "mic-off": MicOff,
  gift: Gift,
  "gamepad-2": Gamepad2,
  sun: Sun,
  backpack: Backpack,
  "cloud-rain": CloudRain,
  "cloud-lightning": CloudLightning,
  "rotate-ccw": RotateCcw,
  lock: Lock,
  "lock-open": LockOpen,
};

// resolveTransport mirrors the API's transport resolution so the UI can tell
// a fire-and-forget stdin action (shows "sent") from an rcon one (shows
// output): the action's explicit transport wins, else rcon when the game has
// a live console, else stdin.
function resolveTransport(action: ServerActionDecl, hasRcon: boolean): "rcon" | "stdin" {
  if (action.transport) return action.transport;
  return hasRcon ? "rcon" : "stdin";
}

function actionIcon(name?: string): LucideIcon {
  return (name && iconMap[name]) || Zap;
}

type RunStatus = { kind: "ok" | "err"; text: string; type?: "output" | "sent" };

// ServerActionsCard renders the module-declared operator actions
// (spec.capabilities.actions) as buttons. Actions with parameters or a
// confirm flag open a dialog; the rest run immediately. Every run POSTs
// to /servers/{name}/actions/run, which the API gates to operator+.
export function ServerActionsCard({ name, tmpl }: { name: string; tmpl?: GameTemplate }) {
  const qc = useQueryClient();
  const { data: me } = useMe();
  const canRun = can(me, "servers:write");
  const [active, setActive] = useState<ServerActionDecl | null>(null);
  const [status, setStatus] = useState<RunStatus | null>(null);

  const actions = tmpl?.spec.capabilities?.actions ?? [];
  const hasRcon = rconAvailable(tmpl);

  const run = useMutation<
    { ok: boolean; raw?: string },
    unknown,
    { action: ServerActionDecl; params?: Record<string, string> }
  >({
    mutationFn: (vars) => Servers.runAction(name, { id: vars.action.id, params: vars.params }),
    onSuccess: (resp, vars) => {
      setActive(null);
      // Base sent-vs-output on the action's TRANSPORT, not on whether the
      // response body is empty: an rcon command legitimately returns no
      // output (cs2's `say`, project-zomboid's `servermsg`), and that must
      // still read as "ran", not the fire-and-forget "sent" chip. Only a
      // stdin action truly returns nothing because the output goes to the
      // Console stream.
      if (resolveTransport(vars.action, hasRcon) === "stdin") {
        setStatus({ kind: "ok", type: "sent", text: `${vars.action.displayName} sent` });
      } else {
        setStatus({
          kind: "ok",
          type: "output",
          text: resp.raw ? truncate(resp.raw, 200) : `${vars.action.displayName} ran`,
        });
      }
      return qc.invalidateQueries({ queryKey: ["server-status", name] });
    },
    onError: (err) => {
      setActive(null);
      setStatus({ kind: "err", text: errMsg(err) });
    },
  });

  if (actions.length === 0) return null;

  // Group actions: separate grouped and ungrouped, preserving group order
  const grouped = new Map<string, ServerActionDecl[]>();
  const ungrouped: ServerActionDecl[] = [];
  const groupOrder: string[] = [];

  for (const action of actions) {
    if (action.group) {
      if (!grouped.has(action.group)) {
        grouped.set(action.group, []);
        groupOrder.push(action.group);
      }
      grouped.get(action.group)!.push(action);
    } else {
      ungrouped.push(action);
    }
  }

  // Render action button
  const renderActionButton = (a: ServerActionDecl) => {
    const Icon = actionIcon(a.icon);
    const needsDialog = (a.params?.length ?? 0) > 0 || a.confirm;
    const disabled = !canRun || !hasRcon || run.isPending;
    return (
      <button
        key={a.id}
        disabled={disabled}
        title={!canRun ? "Requires operator role" : a.description}
        onClick={() =>
          needsDialog ? setActive(a) : run.mutate({ action: a })
        }
        className="flex w-full items-start gap-3 rounded-md px-2 py-2 text-left hover:bg-surface disabled:cursor-not-allowed disabled:opacity-50"
      >
        <span className={a.danger ? "text-danger" : "text-muted"}>
          <Icon className="h-4 w-4" />
        </span>
        <div className="flex-1">
          <div className={a.danger ? "text-sm text-danger" : "text-sm text-fg"}>
            {a.displayName}
          </div>
          {a.description && (
            <div className="pt-0.5 text-[11px] text-muted">{a.description}</div>
          )}
        </div>
      </button>
    );
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle>Quick actions</CardTitle>
      </CardHeader>
      <CardContent className="space-y-1">
        {!hasRcon && (
          <p className="px-2 pb-2 text-xs text-muted">
            Actions need a live console; this game has none.
          </p>
        )}
        {status && (
          <div
            className={cn(
              "mb-2 rounded border px-3 py-2",
              status.kind === "ok" && status.type === "sent"
                ? "inline-flex items-center gap-3 border-success/40 bg-success/10 text-success text-xs"
                : status.kind === "ok"
                  ? "border-border bg-surface/40 text-fg text-sm"
                  : "border-danger/40 bg-danger/10 text-danger text-sm",
            )}
          >
            <div className="flex items-start justify-between gap-3 w-full">
              <span
                className={cn(
                  "break-all",
                  // Mono for anything carrying raw server text — command
                  // output AND error bodies — but not the friendly "sent" chip.
                  ((status.kind === "ok" && status.type === "output") || status.kind === "err") &&
                    "font-mono",
                )}
              >
                {status.text}
              </span>
              <button
                onClick={() => setStatus(null)}
                className={cn(
                  "shrink-0",
                  status.kind === "ok" && status.type === "sent"
                    ? "text-xs text-success hover:text-success/80"
                    : "text-xs text-muted hover:text-fg",
                )}
              >
                dismiss
              </button>
            </div>
          </div>
        )}
        {groupOrder.length > 0 ? (
          <>
            {groupOrder.map((groupName) => (
              <div key={groupName}>
                <div className="text-xs uppercase tracking-wide text-muted px-2 py-2">
                  {groupName}
                </div>
                {grouped.get(groupName)!.map(renderActionButton)}
              </div>
            ))}
            {ungrouped.length > 0 && (
              <div>
                <div className="text-xs uppercase tracking-wide text-muted px-2 py-2">
                  Actions
                </div>
                {ungrouped.map(renderActionButton)}
              </div>
            )}
          </>
        ) : (
          ungrouped.map(renderActionButton)
        )}
      </CardContent>

      {active && (
        <ActionDialog
          action={active}
          pending={run.isPending}
          onCancel={() => setActive(null)}
          onRun={(params) => run.mutate({ action: active, params })}
        />
      )}
    </Card>
  );
}

// ActionDialog collects the action's declared parameters, validates them
// client-side, and hands the values back. The server re-validates and
// sanitizes — this is just to catch obvious mistakes before the round
// trip and to render the right input per type.
function ActionDialog({
  action,
  pending,
  onCancel,
  onRun,
}: {
  action: ServerActionDecl;
  pending: boolean;
  onCancel: () => void;
  onRun: (params: Record<string, string>) => void;
}) {
  const params = action.params ?? [];
  const [values, setValues] = useState<Record<string, string>>(() =>
    Object.fromEntries(params.map((p) => [p.name, p.default ?? defaultFor(p)])),
  );

  const errors = params
    .map((p) => validate(p, values[p.name] ?? ""))
    .filter((e): e is string => e !== null);
  const valid = errors.length === 0;

  return (
    <Dialog.Root open onOpenChange={(o) => !o && onCancel()}>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 z-40 bg-black/60" />
        <Dialog.Content className="fixed left-1/2 top-1/2 z-50 w-[440px] max-w-[calc(100vw-2rem)] -translate-x-1/2 -translate-y-1/2 rounded-lg border border-border bg-card p-5 text-fg shadow-2xl">
          <Dialog.Title className="text-base font-semibold">{action.displayName}</Dialog.Title>
          <Dialog.Description asChild>
            <p className={action.description ? "pt-2 text-sm text-muted" : "sr-only"}>
              {action.description ?? `Run the ${action.displayName} action.`}
            </p>
          </Dialog.Description>

          <div className="space-y-3 pt-4">
            {params.map((p) => (
              <ParamField
                key={p.name}
                param={p}
                value={values[p.name] ?? ""}
                onChange={(v) => setValues((prev) => ({ ...prev, [p.name]: v }))}
              />
            ))}
            {params.length === 0 && (
              <p className="text-sm text-muted">Run this action now?</p>
            )}
          </div>

          <div className="flex items-center justify-end gap-2 pt-5">
            <Button variant="ghost" size="sm" onClick={onCancel} disabled={pending}>
              Cancel
            </Button>
            <Button
              size="sm"
              variant={action.danger ? "danger" : "default"}
              disabled={!valid || pending}
              onClick={() => onRun(collect(params, values))}
            >
              {pending ? "Running…" : "Run"}
            </Button>
          </div>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}

function ParamField({
  param,
  value,
  onChange,
}: {
  param: ActionParamDecl;
  value: string;
  onChange: (v: string) => void;
}) {
  const label = param.displayName || param.name;
  const id = `action-param-${param.name}`;
  const err = validate(param, value);
  return (
    <div>
      <label className="block pb-1 text-xs text-muted" htmlFor={id}>
        {label}
        {param.required && <span className="text-danger"> *</span>}
      </label>
      {param.type === "enum" ? (
        <select
          id={id}
          value={value}
          onChange={(e) => onChange(e.target.value)}
          className="h-9 w-full rounded-md border border-border bg-surface px-3 text-sm text-fg focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary"
        >
          {(param.enum ?? []).map((opt) => (
            <option key={opt} value={opt}>
              {opt}
            </option>
          ))}
        </select>
      ) : param.type === "bool" ? (
        <label className="flex items-center gap-2 text-sm text-fg" htmlFor={id}>
          <input
            id={id}
            type="checkbox"
            checked={value === "true"}
            onChange={(e) => onChange(e.target.checked ? "true" : "false")}
            className="h-4 w-4 accent-primary"
          />
          {value === "true" ? "Enabled" : "Disabled"}
        </label>
      ) : (
        <Input
          id={id}
          value={value}
          inputMode={param.type === "int" ? "numeric" : undefined}
          onChange={(e) => onChange(e.target.value)}
          spellCheck={false}
        />
      )}
      {param.description && <p className="pt-1 text-[11px] text-muted">{param.description}</p>}
      {err && value !== "" && <p className="pt-1 text-xs text-danger">{err}</p>}
    </div>
  );
}

function defaultFor(p: ActionParamDecl): string {
  if (p.type === "bool") return "false";
  if (p.type === "enum") return p.enum?.[0] ?? "";
  return "";
}

// validate returns an error string or null. Mirrors the agent's
// server-side checks so the UI fails fast, but the agent remains the
// source of truth (it also rejects control characters etc.).
function validate(p: ActionParamDecl, value: string): string | null {
  const v = value.trim();
  if (p.required && v === "") return `${p.displayName || p.name} is required`;
  if (v === "") return null;
  if (p.type === "int" && !/^-?\d+$/.test(v)) return "Must be a whole number";
  if (p.type === "enum" && p.enum && !p.enum.includes(value)) return "Pick a valid option";
  // Mirror the agent's control-character rejection (notably CR/LF, which
  // could chain a second console command). Spaces are fine.
  if (hasControl(value)) return "No control characters";
  return null;
}

// hasControl reports whether s contains an ASCII control character.
// Checked by code point rather than a regex to satisfy no-control-regex.
function hasControl(s: string): boolean {
  for (const ch of s) {
    const c = ch.charCodeAt(0);
    if (c < 0x20 || c === 0x7f) return true;
  }
  return false;
}

// collect builds the params payload, dropping empty optional values so
// the agent applies its declared defaults.
function collect(
  params: ActionParamDecl[],
  values: Record<string, string>,
): Record<string, string> {
  const out: Record<string, string> = {};
  for (const p of params) {
    const v = values[p.name] ?? "";
    if (v !== "") out[p.name] = v;
  }
  return out;
}

function truncate(s: string, n: number): string {
  return s.length <= n ? s : s.slice(0, n) + "…";
}

function errMsg(err: unknown): string {
  if (err instanceof APIError) {
    try {
      const parsed = JSON.parse(err.body) as { error?: string };
      if (parsed.error) return parsed.error;
    } catch {
      // fall through
    }
    if (err.status === 403) return "Your role does not allow running actions.";
    return err.body || `request failed (${err.status})`;
  }
  return err instanceof Error ? err.message : "action failed";
}
