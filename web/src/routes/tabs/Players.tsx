import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
  ChevronDown,
  ChevronRight,
  RotateCw,
  UserMinus,
  UserPlus,
  Ban,
  Undo2,
  ListChecks,
} from "lucide-react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { StatCard } from "@/components/ui/stat";
import { APIError } from "@/lib/api";
import { Players as PlayersAPI } from "@/lib/endpoints";
import { cn } from "@/lib/utils";

interface ModResp {
  ok: boolean;
  raw?: string;
}

type Action = "kick" | "ban";

export function PlayersTab({ name, ns }: { name: string; ns?: string }) {
  const qc = useQueryClient();
  const [pending, setPending] = useState<{ player: string; action: Action } | null>(null);
  const [reason, setReason] = useState("");
  const [showBanned, setShowBanned] = useState(false);
  const [showWhitelist, setShowWhitelist] = useState(false);
  const [wlName, setWlName] = useState("");
  const [status, setStatus] = useState<{ kind: "ok" | "err"; text: string } | null>(null);

  const { data, refetch, isFetching } = useQuery({
    queryKey: ["players", name, ns],
    queryFn: () => PlayersAPI.snapshot(name, ns),
    refetchInterval: 5_000,
  });
  const caps = data?.capabilities;

  // Banned + whitelist are fetched whenever the game supports them so the
  // summary tiles always have counts; the sections below just toggle the
  // list's visibility, not the fetch.
  const { data: banned, isFetching: bannedFetching } = useQuery({
    queryKey: ["banned", name, ns],
    queryFn: () => PlayersAPI.banned(name, ns),
    enabled: caps?.unban ?? false,
  });
  const { data: whitelist, isFetching: whitelistFetching } = useQuery({
    queryKey: ["whitelist", name, ns],
    queryFn: () => PlayersAPI.whitelist(name, ns),
    enabled: caps?.whitelist ?? false,
  });

  const moderate = useMutation<ModResp, unknown, { action: Action | "unban"; player: string; reason?: string }>({
    mutationFn: (vars) =>
      PlayersAPI.moderate(name, vars.action, {
        name: vars.player,
        ...(vars.reason ? { reason: vars.reason } : {}),
      }, ns),
    onSuccess: (resp, vars) => {
      setPending(null);
      setReason("");
      setStatus({
        kind: "ok",
        text: resp.raw ? truncate(resp.raw, 200) : `${vars.action} ${vars.player} ok`,
      });
      return Promise.all([
        qc.invalidateQueries({ queryKey: ["players", name, ns] }),
        qc.invalidateQueries({ queryKey: ["banned", name, ns] }),
      ]);
    },
    onError: (err) => setStatus({ kind: "err", text: errMsg(err) }),
  });

  const whitelistMut = useMutation<ModResp, unknown, { op: "add" | "remove"; player: string }>({
    mutationFn: (vars) =>
      vars.op === "add"
        ? PlayersAPI.whitelistAdd(name, vars.player, ns)
        : PlayersAPI.whitelistRemove(name, vars.player, ns),
    onSuccess: (resp, vars) => {
      setWlName("");
      setStatus({
        kind: "ok",
        text: resp.raw ? truncate(resp.raw, 200) : `whitelist ${vars.op} ${vars.player} ok`,
      });
      return qc.invalidateQueries({ queryKey: ["whitelist", name, ns] });
    },
    onError: (err) => setStatus({ kind: "err", text: errMsg(err) }),
  });

  const noModeration = caps && !caps.kick && !caps.ban && !caps.unban && !caps.whitelist;

  return (
    <div className="space-y-6 p-6">
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-3">
        <StatCard
          label="Online"
          value={data ? (data.max >= 0 ? `${data.online} / ${data.max}` : `${data.online}`) : "—"}
          accent="success"
        />
        {caps?.whitelist && (
          <StatCard label="Whitelisted" value={whitelist?.length ?? "—"} accent="primary" />
        )}
        {caps?.unban && (
          <StatCard label="Banned" value={banned?.length ?? "—"} accent="danger" />
        )}
      </div>

      <header className="flex items-center justify-between">
        <h2 className="text-sm text-muted">
          {data ? (data.max >= 0 ? `${data.online} / ${data.max} online` : `${data.online} online`) : "Loading…"}
        </h2>
        <Button variant="ghost" size="sm" onClick={() => refetch()} disabled={isFetching} title="Refresh">
          <RotateCw className={cn("h-3 w-3", isFetching && "animate-spin")} />
        </Button>
      </header>

      {status && (
        <div
          className={cn(
            "rounded border px-3 py-2 text-sm",
            status.kind === "ok"
              ? "border-border bg-surface/30 text-fg"
              : "border-red-500/40 bg-red-500/10 text-red-200",
          )}
        >
          <div className="flex items-start justify-between gap-3">
            <span className="font-mono">{status.text}</span>
            <button onClick={() => setStatus(null)} className="text-xs text-muted hover:text-fg">
              dismiss
            </button>
          </div>
        </div>
      )}

      {noModeration && (
        <p className="text-sm text-muted">Player moderation isn&apos;t supported for this game.</p>
      )}

      <ul className="grid gap-1 md:grid-cols-2 lg:grid-cols-3">
        {data?.players.map((p) => (
          <li
            key={p}
            className="flex items-center justify-between gap-2 rounded border border-border bg-surface/30 px-3 py-2 font-mono text-sm"
          >
            <span className="truncate">{p}</span>
            {caps && (caps.kick || caps.ban) && (
              <div className="flex items-center gap-1">
                {caps.kick && (
                  <Button
                    variant="ghost"
                    size="sm"
                    title="Kick"
                    onClick={() => {
                      setPending({ player: p, action: "kick" });
                      setReason("");
                    }}
                  >
                    <UserMinus className="h-3 w-3" />
                  </Button>
                )}
                {caps.ban && (
                  <Button
                    variant="ghost"
                    size="sm"
                    title="Ban"
                    onClick={() => {
                      setPending({ player: p, action: "ban" });
                      setReason("");
                    }}
                  >
                    <Ban className="h-3 w-3" />
                  </Button>
                )}
              </div>
            )}
          </li>
        ))}
        {data?.players.length === 0 && <p className="text-sm text-muted">Nobody online.</p>}
      </ul>

      {pending && (
        <ConfirmAction
          player={pending.player}
          action={pending.action}
          reason={reason}
          onReasonChange={setReason}
          submitting={moderate.isPending}
          onCancel={() => {
            setPending(null);
            setReason("");
          }}
          onConfirm={() =>
            moderate.mutate({
              action: pending.action,
              player: pending.player,
              reason: reason.trim() || undefined,
            })
          }
        />
      )}

      {caps?.whitelist && (
        <section>
          <button
            onClick={() => setShowWhitelist((v) => !v)}
            className="flex items-center gap-1 text-sm text-muted hover:text-fg"
          >
            {showWhitelist ? <ChevronDown className="h-3 w-3" /> : <ChevronRight className="h-3 w-3" />}
            <ListChecks className="h-3 w-3" /> Whitelist {whitelist ? `(${whitelist.length})` : ""}
          </button>
          {showWhitelist && (
            <div className="mt-2 space-y-2">
              <form
                className="flex items-center gap-2"
                onSubmit={(e) => {
                  e.preventDefault();
                  const n = wlName.trim();
                  if (n) whitelistMut.mutate({ op: "add", player: n });
                }}
              >
                <Input
                  className="flex-1"
                  placeholder="Add player to whitelist…"
                  value={wlName}
                  onChange={(e) => setWlName(e.target.value)}
                  maxLength={32}
                />
                <Button size="sm" type="submit" disabled={!wlName.trim() || whitelistMut.isPending}>
                  <UserPlus className="h-3 w-3" /> Add
                </Button>
              </form>
              {whitelistFetching && !whitelist && <p className="text-sm text-muted">Loading…</p>}
              {whitelist?.map((w) => (
                <div
                  key={w}
                  className="flex items-center justify-between gap-2 rounded border border-border bg-surface/30 px-3 py-2 font-mono text-sm"
                >
                  <span className="truncate">{w}</span>
                  <Button
                    variant="ghost"
                    size="sm"
                    title="Remove from whitelist"
                    disabled={whitelistMut.isPending}
                    onClick={() => whitelistMut.mutate({ op: "remove", player: w })}
                  >
                    <UserMinus className="h-3 w-3" />
                  </Button>
                </div>
              ))}
              {whitelist?.length === 0 && <p className="text-sm text-muted">Whitelist is empty.</p>}
            </div>
          )}
        </section>
      )}

      {caps?.unban && (
        <section>
          <button
            onClick={() => setShowBanned((v) => !v)}
            className="flex items-center gap-1 text-sm text-muted hover:text-fg"
          >
            {showBanned ? <ChevronDown className="h-3 w-3" /> : <ChevronRight className="h-3 w-3" />}
            Banned {banned ? `(${banned.length})` : ""}
          </button>
          {showBanned && (
            <div className="mt-2 space-y-1">
              {bannedFetching && !banned && <p className="text-sm text-muted">Loading…</p>}
              {banned?.map((b) => (
                <div
                  key={b.name}
                  className="flex items-center justify-between gap-2 rounded border border-border bg-surface/30 px-3 py-2 text-sm"
                >
                  <div className="min-w-0 flex-1">
                    <div className="font-mono">{b.name}</div>
                    {(b.reason || b.source) && (
                      <div className="truncate text-xs text-muted">
                        {b.source && <span>by {b.source}</span>}
                        {b.source && b.reason && <span> · </span>}
                        {b.reason && <span>{b.reason}</span>}
                      </div>
                    )}
                  </div>
                  <Button
                    variant="ghost"
                    size="sm"
                    title="Unban"
                    disabled={moderate.isPending}
                    onClick={() => moderate.mutate({ action: "unban", player: b.name })}
                  >
                    <Undo2 className="h-3 w-3" /> Unban
                  </Button>
                </div>
              ))}
              {banned?.length === 0 && <p className="text-sm text-muted">Nobody is banned.</p>}
            </div>
          )}
        </section>
      )}
    </div>
  );
}

function ConfirmAction({
  player,
  action,
  reason,
  onReasonChange,
  onConfirm,
  onCancel,
  submitting,
}: {
  player: string;
  action: Action;
  reason: string;
  onReasonChange: (v: string) => void;
  onConfirm: () => void;
  onCancel: () => void;
  submitting: boolean;
}) {
  const verb = action === "kick" ? "Kick" : "Ban";
  return (
    <div className="rounded border border-border bg-surface/50 p-4">
      <p className="text-sm text-fg">
        {verb} <span className="font-mono">{player}</span>?
      </p>
      <Input
        className="mt-3"
        placeholder="Reason (optional, max 256 chars)"
        value={reason}
        onChange={(e) => onReasonChange(e.target.value)}
        maxLength={256}
        autoFocus
      />
      <div className="mt-3 flex items-center justify-end gap-2">
        <Button variant="ghost" size="sm" onClick={onCancel} disabled={submitting}>
          Cancel
        </Button>
        <Button size="sm" onClick={onConfirm} disabled={submitting}>
          {submitting ? `${verb}ing…` : verb}
        </Button>
      </div>
    </div>
  );
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
    return err.body || `request failed (${err.status})`;
  }
  return err instanceof Error ? err.message : "action failed";
}
