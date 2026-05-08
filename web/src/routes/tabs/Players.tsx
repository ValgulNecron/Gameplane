import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { ChevronDown, ChevronRight, RotateCw, UserMinus, Ban, Undo2 } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { APIError } from "@/lib/api";
import { Players as PlayersAPI } from "@/lib/endpoints";
import { cn } from "@/lib/utils";

interface ModResp {
  ok: boolean;
  raw?: string;
}

type Action = "kick" | "ban";

export function PlayersTab({ name }: { name: string }) {
  const qc = useQueryClient();
  const [pending, setPending] = useState<{ player: string; action: Action } | null>(null);
  const [reason, setReason] = useState("");
  const [showBanned, setShowBanned] = useState(false);
  const [status, setStatus] = useState<{ kind: "ok" | "err"; text: string } | null>(null);

  const { data, refetch, isFetching } = useQuery({
    queryKey: ["players", name],
    queryFn: () => PlayersAPI.snapshot(name),
    refetchInterval: 5_000,
  });

  const { data: banned, isFetching: bannedFetching } = useQuery({
    queryKey: ["banned", name],
    queryFn: () => PlayersAPI.banned(name),
    enabled: showBanned && (data?.capabilities.unban ?? false),
  });

  const moderate = useMutation<ModResp, unknown, { action: Action | "unban"; player: string; reason?: string }>({
    mutationFn: (vars) =>
      PlayersAPI.moderate(name, vars.action, {
        name: vars.player,
        ...(vars.reason ? { reason: vars.reason } : {}),
      }),
    onSuccess: (resp, vars) => {
      setPending(null);
      setReason("");
      setStatus({
        kind: "ok",
        text: resp.raw ? truncate(resp.raw, 200) : `${vars.action} ${vars.player} ok`,
      });
      return Promise.all([
        qc.invalidateQueries({ queryKey: ["players", name] }),
        qc.invalidateQueries({ queryKey: ["banned", name] }),
      ]);
    },
    onError: (err) => {
      setStatus({ kind: "err", text: errMsg(err) });
    },
  });

  const caps = data?.capabilities;
  const noModeration = caps && !caps.kick && !caps.ban && !caps.unban;

  return (
    <div className="space-y-6 p-6">
      <header className="flex items-center justify-between">
        <h2 className="text-sm text-muted">
          {data ? `${data.online} / ${data.max} online` : "Loading…"}
        </h2>
        <Button
          variant="ghost"
          size="sm"
          onClick={() => refetch()}
          disabled={isFetching}
          title="Refresh"
        >
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
            <button
              onClick={() => setStatus(null)}
              className="text-xs text-muted hover:text-fg"
            >
              dismiss
            </button>
          </div>
        </div>
      )}

      {noModeration && (
        <p className="text-sm text-muted">
          Player moderation isn&apos;t supported for this game.
        </p>
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
        {data?.players.length === 0 && (
          <p className="text-sm text-muted">Nobody online.</p>
        )}
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

      {caps?.unban && (
        <section>
          <button
            onClick={() => setShowBanned((v) => !v)}
            className="flex items-center gap-1 text-sm text-muted hover:text-fg"
          >
            {showBanned ? (
              <ChevronDown className="h-3 w-3" />
            ) : (
              <ChevronRight className="h-3 w-3" />
            )}
            Banned {banned ? `(${banned.length})` : ""}
          </button>
          {showBanned && (
            <div className="mt-2 space-y-1">
              {bannedFetching && !banned && (
                <p className="text-sm text-muted">Loading…</p>
              )}
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
                    onClick={() =>
                      moderate.mutate({ action: "unban", player: b.name })
                    }
                  >
                    <Undo2 className="h-3 w-3" /> Unban
                  </Button>
                </div>
              ))}
              {banned?.length === 0 && (
                <p className="text-sm text-muted">Nobody is banned.</p>
              )}
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
