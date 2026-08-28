// Network-capture surface for a GameServer's "Capture" tab. Mirrors the
// Backups tab's structure (own TanStack Query calls, own mutations, plain
// <table> for the list) per CLAUDE.md's "Add a new dashboard page" recipe.
// Design source: design-export/json/{f0s9zG,Bbnga,dBILX,xvlB6,m5kOm4,
// O08uaD,b4eaUf}.json — read directly (never design.pen; see CLAUDE.md
// rule 2). Endpoints and error-body shape follow
// specs/done_003-network-capture-sidecar/contracts/rest-api.md (plain-text
// httperr bodies, `:verb` route suffixes, capture id as a query param).
import { Fragment, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Activity,
  CircleCheckBig,
  CirclePlay,
  CircleX,
  Download,
  Eye,
  Inbox,
  Power,
  PowerOff,
  Square,
  Trash2,
  TriangleAlert,
} from "lucide-react";
import * as Dialog from "@radix-ui/react-dialog";
import { APIError, Captures, CaptureStartBody } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { Meter } from "@/components/ui/meter";
import { FieldLabel } from "@/components/ui/field";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { ErrorBanner } from "@/components/backups/ErrorBanner";
import { formatBytes, formatRelative } from "@/lib/utils";
import type { GameServer, NetworkCapture } from "@/types";

const DEFAULT_RETENTION_SECONDS = 86400; // 24h — FR-007's engineering default, not a legal mandate.

function formatDuration(totalSeconds: number): string {
  const s = Math.max(0, Math.floor(totalSeconds));
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  const sec = s % 60;
  if (h) return `${h}h ${m}m`;
  if (m) return `${m}m ${sec}s`;
  return `${sec}s`;
}

function elapsedSeconds(startedAt: string): number {
  const t = new Date(startedAt).getTime();
  if (Number.isNaN(t)) return 0;
  return Math.max(0, Math.floor((Date.now() - t) / 1000));
}

function durationBetween(startedAt: string, completedAt: string): string {
  if (!startedAt || !completedAt) return "—";
  const start = new Date(startedAt).getTime();
  const end = new Date(completedAt).getTime();
  if (Number.isNaN(start) || Number.isNaN(end)) return "—";
  return formatDuration((end - start) / 1000);
}

// Expiry badges recolor as the retention window closes in, so an admin
// scanning the table can tell "plenty of time" from "about to be
// GC'd" at a glance (see design-export/json/m5kOm4.json's per-row colors).
function expiryTone(secondsLeft: number): string {
  if (secondsLeft <= 3600) return "bg-danger/20 text-danger";
  if (secondsLeft <= 21600) return "bg-warning/20 text-warning";
  return "bg-muted/20 text-muted";
}

function expiryLabel(expiresAt: string): { text: string; tone: string } {
  const t = new Date(expiresAt).getTime();
  if (Number.isNaN(t)) return { text: "—", tone: "bg-muted/20 text-muted" };
  const secondsLeft = Math.max(0, Math.floor((t - Date.now()) / 1000));
  const h = Math.floor(secondsLeft / 3600);
  const m = Math.floor((secondsLeft % 3600) / 60);
  const text = h ? `${h}h${m ? ` ${m}m` : ""}` : `${m}m`;
  return { text, tone: expiryTone(secondsLeft) };
}

const phaseTone: Record<string, string> = {
  Pending: "bg-muted/20 text-muted",
  Running: "bg-warning/20 text-warning",
  Completed: "bg-success/20 text-success",
  Failed: "bg-danger/20 text-danger",
  Expired: "bg-muted/20 text-muted",
};

function CapturePhaseBadge({ phase }: { phase: string }) {
  return (
    <span className={`inline-flex h-5 items-center rounded px-2 text-xs font-mono ${phaseTone[phase] ?? "bg-muted/20 text-muted"}`}>
      {phase}
    </span>
  );
}

interface Props {
  name: string;
  ns?: string;
  gs?: GameServer;
}

export function CaptureWidget({ name, ns, gs }: Props) {
  const qc = useQueryClient();
  const enabled = gs?.spec.capture?.enabled === true;
  const retentionSeconds = gs?.spec.capture?.retentionSeconds ?? DEFAULT_RETENTION_SECONDS;
  const retentionHours = Math.max(1, Math.round(retentionSeconds / 3600));

  const [bannerDismissed, setBannerDismissed] = useState(false);
  const [showStartModal, setShowStartModal] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<NetworkCapture | null>(null);
  const [expandedId, setExpandedId] = useState<string | null>(null);

  const { data: captures } = useQuery({
    queryKey: ["captures", name, ns],
    queryFn: () => Captures.list(name, ns),
    enabled,
    refetchInterval: 5000,
  });

  const activeCapture = (captures?.captures ?? []).find(
    (c) => c.phase === "Running" || c.phase === "Pending",
  );
  const { data: activeCaptureDetails } = useQuery({
    queryKey: ["capture", name, activeCapture?.captureId, ns],
    queryFn: () => Captures.get(name, activeCapture!.captureId, ns),
    enabled: !!activeCapture,
  });

  const enableMut = useMutation({
    mutationFn: () => Captures.enable(name, ns),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["server", name, ns] }),
  });
  const disableMut = useMutation({
    mutationFn: () => Captures.disable(name, ns),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["server", name, ns] });
      void qc.invalidateQueries({ queryKey: ["captures", name, ns] });
    },
  });
  const stopMut = useMutation({
    mutationFn: (captureId: string) => Captures.stop(name, captureId, ns),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["captures", name, ns] }),
  });
  const deleteMut = useMutation({
    mutationFn: (captureId: string) => Captures.remove(name, captureId, ns),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["captures", name, ns] });
      setDeleteTarget(null);
    },
  });
  const fileMut = useMutation({
    mutationFn: (captureId: string) => Captures.download(name, captureId, ns),
    onSuccess: (blob, captureId) => {
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `capture-${captureId}.pcapng`;
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      URL.revokeObjectURL(url);
    },
  });

  if (!enabled) {
    return (
      <div className="p-6">
        <Card className="flex flex-col items-center gap-3 p-10 text-center">
          <div className="flex h-12 w-12 items-center justify-center rounded-full bg-muted/10">
            <Activity className="h-7 w-7 text-muted" />
          </div>
          <p className="text-sm font-medium text-fg">Capture is not enabled on this server.</p>
          <p className="max-w-md text-xs text-muted">
            Network packet capture is an optional feature that records raw game protocol
            traffic. Enable it to capture packets from joining players for protocol analysis.
          </p>
          {enableMut.error && <ErrorBanner err={enableMut.error} />}
          <Button onClick={() => enableMut.mutate()} disabled={enableMut.isPending}>
            <Power className="h-4 w-4" />
            {enableMut.isPending ? "Enabling…" : "Enable Capture"}
          </Button>
        </Card>
      </div>
    );
  }

  const items = captures?.captures ?? [];
  const completed = items.filter((c) => c !== activeCapture);

  return (
    <div className="space-y-4 p-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-3">
          <span
            className={`inline-flex h-5 items-center rounded px-2 text-xs font-mono ${
              activeCapture ? "bg-warning/20 text-warning" : "bg-success/20 text-success"
            }`}
          >
            {activeCapture ? "Capturing…" : "Ready"}
          </span>
          <span className="text-xs text-muted">
            {activeCapture
              ? `Capture started ${formatDuration(elapsedSeconds(activeCapture.startedAt || activeCapture.createdAt))} ago`
              : `Captures will auto-delete after ${retentionHours} hour${retentionHours === 1 ? "" : "s"}`}
          </span>
        </div>
        {!activeCapture && (
          <div className="flex items-center gap-2">
            <Button variant="outline" size="sm" onClick={() => disableMut.mutate()} disabled={disableMut.isPending}>
              <PowerOff className="h-4 w-4" />
              Disable Capture
            </Button>
            <Button size="sm" onClick={() => setShowStartModal(true)}>
              <CirclePlay className="h-4 w-4" />
              Start Capture
            </Button>
          </div>
        )}
      </div>

      {disableMut.error && <ErrorBanner err={disableMut.error} />}

      {!bannerDismissed && (
        <div className="rounded-md border border-warning/40 bg-warning/10 p-4">
          <div className="flex items-start gap-3">
            <TriangleAlert className="mt-0.5 h-5 w-5 shrink-0 text-warning" />
            <div className="space-y-2 text-sm">
              <p className="font-medium text-fg">
                Caution: network packet captures contain real player data
              </p>
              <ul className="list-none space-y-1 text-xs text-muted">
                <li>• Player IP addresses and port numbers</li>
                <li>• Network timing and game protocol messages</li>
                <li>• For some games, in-band credentials (passwords, tokens, session keys)</li>
              </ul>
              <p className="text-xs text-muted">
                Captures are not redacted or sanitized. Access is restricted to administrators
                only. Captures are automatically deleted after the configured retention window
                (currently {retentionHours} hour{retentionHours === 1 ? "" : "s"}).
              </p>
              <button
                type="button"
                className="text-xs font-medium text-fg underline"
                onClick={() => setBannerDismissed(true)}
              >
                Dismiss
              </button>
            </div>
          </div>
        </div>
      )}

      {activeCapture ? (
        <Card className="space-y-4 p-4">
          <div className="flex items-start justify-between gap-3">
            <div className="space-y-1">
              <div className="font-mono text-sm">{activeCapture.captureId}</div>
              <div className="text-xs text-muted">Filter: {activeCapture.filter || "default"}</div>
            </div>
            <Button
              size="sm"
              variant="outline"
              onClick={() => stopMut.mutate(activeCapture.captureId)}
              disabled={stopMut.isPending}
            >
              <Square className="h-4 w-4" />
              Stop Capture
            </Button>
          </div>
          {stopMut.error && <ErrorBanner err={stopMut.error} />}
          <div className="grid gap-4 sm:grid-cols-2">
            <Meter
              label="Max duration"
              pct={
                activeCaptureDetails?.maxDurationSeconds
                  ? (elapsedSeconds(activeCaptureDetails.startedAt || activeCaptureDetails.createdAt) / activeCaptureDetails.maxDurationSeconds) * 100
                  : 0
              }
              accent="warning"
              sub={
                activeCaptureDetails?.maxDurationSeconds
                  ? `Time remaining: ~${formatDuration(
                      Math.max(0, activeCaptureDetails.maxDurationSeconds - elapsedSeconds(activeCaptureDetails.startedAt || activeCaptureDetails.createdAt)),
                    )}`
                  : undefined
              }
            />
            <Meter
              label="Max size"
              pct={activeCaptureDetails?.maxSizeBytes ? (activeCaptureDetails.bytesWritten / activeCaptureDetails.maxSizeBytes) * 100 : 0}
              accent="violet"
              sub={
                activeCaptureDetails?.maxSizeBytes
                  ? `Space remaining: ~${formatBytes(Math.max(0, activeCaptureDetails.maxSizeBytes - activeCaptureDetails.bytesWritten))}`
                  : undefined
              }
            />
          </div>
          <div className="text-xs text-muted">
            {(activeCaptureDetails?.packetsWritten ?? activeCapture.packetsWritten).toLocaleString()} packets captured
          </div>
        </Card>
      ) : items.length === 0 ? (
        <Card className="flex flex-col items-center gap-2 p-10 text-center">
          <Inbox className="h-8 w-8 text-muted" />
          <p className="text-sm text-muted">No captures yet.</p>
        </Card>
      ) : (
        <section>
          <h2 className="pb-3 text-sm text-muted">Captures</h2>
          {fileMut.error && <ErrorBanner err={fileMut.error} />}
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <caption className="sr-only">Completed and failed packet captures for this server</caption>
              <thead className="text-left text-xs uppercase text-muted">
                <tr>
                  <th scope="col" className="py-2">ID</th>
                  <th scope="col">Status</th>
                  <th scope="col">Size</th>
                  <th scope="col">Packets</th>
                  <th scope="col">Duration</th>
                  <th scope="col">Completed at</th>
                  <th scope="col">Expires in</th>
                  <th scope="col">Filter</th>
                  <th scope="col"><span className="sr-only">Actions</span></th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border">
                {completed.map((c) => {
                  const expiry = c.expiresAt ? expiryLabel(c.expiresAt) : null;
                  const downloadable = c.phase === "Completed";
                  return (
                    <Fragment key={c.captureId}>
                      <tr>
                        <td className="py-2 font-mono">{c.captureId}</td>
                        <td><CapturePhaseBadge phase={c.phase} /></td>
                        <td className="font-mono">{formatBytes(c.bytesWritten)}</td>
                        <td className="font-mono">{c.packetsWritten.toLocaleString()}</td>
                        <td className="font-mono text-muted">{durationBetween(c.startedAt, c.completedAt)}</td>
                        <td className="font-mono text-muted">{formatRelative(c.completedAt || undefined)}</td>
                        <td>
                          {expiry && (
                            <span className={`inline-flex h-5 items-center rounded px-2 text-xs font-mono ${expiry.tone}`}>
                              {expiry.text}
                            </span>
                          )}
                        </td>
                        <td className="font-mono text-muted">{c.filter || "default"}</td>
                        <td className="text-right">
                          <div className="flex justify-end gap-1">
                            <Button
                              variant="ghost"
                              size="icon"
                              aria-label={`Download capture ${c.captureId}`}
                              disabled={!downloadable || fileMut.isPending}
                              onClick={() => fileMut.mutate(c.captureId)}
                            >
                              <Download className="h-4 w-4" />
                            </Button>
                            <Button
                              variant="ghost"
                              size="icon"
                              aria-label={`View capture ${c.captureId}`}
                              onClick={() => setExpandedId(expandedId === c.captureId ? null : c.captureId)}
                              aria-expanded={expandedId === c.captureId}
                            >
                              <Eye className="h-4 w-4" />
                            </Button>
                            <Button
                              variant="ghost"
                              size="icon"
                              aria-label={`Delete capture ${c.captureId}`}
                              className="text-danger"
                              onClick={() => setDeleteTarget(c)}
                            >
                              <Trash2 className="h-4 w-4" />
                            </Button>
                          </div>
                        </td>
                      </tr>
                      {expandedId === c.captureId && (
                        <tr>
                          <td colSpan={9} className="bg-surface/30 px-4 py-3 text-xs text-muted">
                            <dl className="grid grid-cols-2 gap-x-6 gap-y-1 sm:grid-cols-4">
                              <dt className="font-medium text-fg">Created</dt>
                              <dd>{formatRelative(c.createdAt)}</dd>
                              <dt className="font-medium text-fg">Started</dt>
                              <dd>{c.startedAt ? formatRelative(c.startedAt) : "—"}</dd>
                              <dt className="font-medium text-fg">Server</dt>
                              <dd className="font-mono">{c.serverName}</dd>
                            </dl>
                          </td>
                        </tr>
                      )}
                    </Fragment>
                  );
                })}
              </tbody>
            </table>
          </div>
          <p className="pt-2 text-xs text-muted">
            Showing {completed.length} of {captures?.total ?? items.length} capture{(captures?.total ?? items.length) === 1 ? "" : "s"}
          </p>
        </section>
      )}

      <StartCaptureModal
        open={showStartModal}
        onClose={() => setShowStartModal(false)}
        onStart={(body) => Captures.start(name, body, ns)}
        onStarted={() => {
          setShowStartModal(false);
          void qc.invalidateQueries({ queryKey: ["captures", name, ns] });
        }}
      />

      <ConfirmDialog
        open={deleteTarget !== null}
        onOpenChange={(o) => {
          if (!o) setDeleteTarget(null);
        }}
        title="Delete capture?"
        description={
          <>
            This permanently deletes capture{" "}
            <span className="font-mono text-fg">{deleteTarget?.captureId}</span> and its
            recorded packets. This cannot be undone.
          </>
        }
        confirmLabel="Delete capture"
        destructive
        busy={deleteMut.isPending}
        onConfirm={() => deleteTarget && deleteMut.mutate(deleteTarget.captureId)}
      />
    </div>
  );
}

// Isolated so the (frequent, per-keystroke) form state doesn't re-render
// the whole widget, and so the invalid-filter error state — this file's
// other design-mandated constraint alongside the dual progress bars — is
// scoped to just the field that produced it.
function StartCaptureModal({
  open,
  onClose,
  onStart,
  onStarted,
}: {
  open: boolean;
  onClose: () => void;
  onStart: (body: CaptureStartBody) => Promise<NetworkCapture>;
  onStarted: () => void;
}) {
  const [filter, setFilter] = useState("");
  const [durationValue, setDurationValue] = useState(300);
  const [durationUnit, setDurationUnit] = useState<"seconds" | "minutes">("seconds");
  const [sizeValue, setSizeValue] = useState(5120);
  const [sizeUnit, setSizeUnit] = useState<"MB" | "GB">("MB");
  const [retentionValue, setRetentionValue] = useState(24);
  const [retentionUnit, setRetentionUnit] = useState<"hours" | "days">("hours");

  const start = useMutation({
    mutationFn: () =>
      onStart({
        filter: filter.trim() || undefined,
        maxDurationSeconds: durationUnit === "minutes" ? durationValue * 60 : durationValue,
        maxSizeBytes: (sizeUnit === "GB" ? sizeValue * 1024 : sizeValue) * 1024 * 1024,
        ttlSecondsAfterFinished: retentionUnit === "days" ? retentionValue * 86400 : retentionValue * 3600,
      }),
    onSuccess: onStarted,
  });

  // The design's invalid-filter state (red border, red X, inline error,
  // greyed Start button) is driven by the API's filter-validation error
  // (FR-003) rather than client-side BPF parsing — pcap-filter syntax isn't
  // re-implemented in the browser. Any edit to the filter clears the stale
  // error so the field doesn't keep flagging text the user has since fixed.
  const filterError =
    start.error instanceof APIError && /filter/i.test(start.error.body) ? start.error.body : null;

  function handleFilterChange(v: string) {
    setFilter(v);
    if (start.isError) start.reset();
  }

  return (
    <Dialog.Root open={open} onOpenChange={(o) => !o && onClose()}>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 z-40 bg-black/60" />
        <Dialog.Content className="fixed left-1/2 top-1/2 z-50 w-[520px] max-w-[calc(100vw-2rem)] -translate-x-1/2 -translate-y-1/2 rounded-lg border border-border bg-card p-5 text-fg shadow-2xl">
          <Dialog.Title className="text-base font-semibold">Start Capture</Dialog.Title>
          <Dialog.Description className="pt-1 text-sm text-muted">
            Records raw network traffic on this server&rsquo;s advertised ports (or a custom
            filter) for later download.
          </Dialog.Description>

          <form
            className="space-y-4 pt-4"
            onSubmit={(e) => {
              e.preventDefault();
              start.mutate();
            }}
          >
            <FieldLabel label="Packet Filter">
              <div className="flex items-center gap-2">
                <Input
                  aria-label="Packet Filter"
                  value={filter}
                  onChange={(e) => handleFilterChange(e.target.value)}
                  placeholder="tcp port 25565"
                  spellCheck={false}
                  className={filterError ? "border-danger focus:border-danger focus:ring-danger" : undefined}
                />
                {filter &&
                  (filterError ? (
                    <CircleX className="h-[18px] w-[18px] shrink-0 text-danger" aria-hidden="true" />
                  ) : (
                    <CircleCheckBig className="h-[18px] w-[18px] shrink-0 text-success" aria-hidden="true" />
                  ))}
              </div>
              <p className="text-[11px] text-muted">
                Optional. Use pcap-filter syntax (e.g. &quot;tcp port 8080&quot;, &quot;host
                192.168.1.5&quot;). Leave blank to capture only on this server&rsquo;s game
                ports.
              </p>
              {filterError && <p className="text-[11px] text-danger">{filterError}</p>}
            </FieldLabel>

            <FieldLabel label="Max Duration *">
              <div className="flex items-center gap-2">
                <Input
                  type="number"
                  min={1}
                  max={durationUnit === "minutes" ? 60 : 3600}
                  value={durationValue}
                  onChange={(e) => setDurationValue(Number(e.target.value))}
                  className="w-[140px]"
                  aria-label="Max duration value"
                  required
                />
                <Select
                  className="w-[140px]"
                  aria-label="Max duration unit"
                  value={durationUnit}
                  onValueChange={(v) => setDurationUnit(v as "seconds" | "minutes")}
                  options={[
                    { value: "seconds", label: "seconds" },
                    { value: "minutes", label: "minutes" },
                  ]}
                />
              </div>
              <p className="text-[11px] text-muted">
                How long the capture runs before auto-stopping. Range 1–3600 seconds.
              </p>
            </FieldLabel>

            <FieldLabel label="Max Size *">
              <div className="flex items-center gap-2">
                <Input
                  type="number"
                  min={1}
                  value={sizeValue}
                  onChange={(e) => setSizeValue(Number(e.target.value))}
                  className="w-[140px]"
                  aria-label="Max size value"
                  required
                />
                <Select
                  className="w-[140px]"
                  aria-label="Max size unit"
                  value={sizeUnit}
                  onValueChange={(v) => setSizeUnit(v as "MB" | "GB")}
                  options={[
                    { value: "MB", label: "MB" },
                    { value: "GB", label: "GB" },
                  ]}
                />
              </div>
              <p className="text-[11px] text-muted">
                Maximum file size. Capture stops when reached.
              </p>
            </FieldLabel>

            <FieldLabel label="Retention">
              <div className="flex items-center gap-2">
                <Input
                  type="number"
                  min={1}
                  value={retentionValue}
                  onChange={(e) => setRetentionValue(Number(e.target.value))}
                  className="w-[140px]"
                  aria-label="Retention value"
                />
                <Select
                  className="w-[140px]"
                  aria-label="Retention unit"
                  value={retentionUnit}
                  onValueChange={(v) => setRetentionUnit(v as "hours" | "days")}
                  options={[
                    { value: "hours", label: "hours" },
                    { value: "days", label: "days" },
                  ]}
                />
              </div>
              <p className="text-[11px] text-muted">
                How long the capture is retained before auto-delete.
              </p>
            </FieldLabel>

            {start.error && !filterError && <ErrorBanner err={start.error} />}

            <div className="flex justify-end gap-2 pt-1">
              <Button type="button" variant="ghost" onClick={onClose} disabled={start.isPending}>
                Cancel
              </Button>
              <Button
                type="submit"
                disabled={start.isPending || !!filterError || durationValue < 1 || sizeValue < 1}
              >
                {start.isPending ? "Starting…" : "Start Capture"}
              </Button>
            </div>
          </form>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
