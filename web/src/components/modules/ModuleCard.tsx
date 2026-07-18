import { Link } from "@tanstack/react-router";
import {
  ArrowUpCircle,
  Download,
  ExternalLink,
  Fingerprint,
  History,
  Loader2,
  ShieldCheck,
  ShieldQuestion,
  Trash2,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { GameIcon } from "@/components/ui/game-icon";
import { cn } from "@/lib/utils";
import type { EntryVerify } from "@/lib/verify";
import type { CatalogEntry } from "@/types";

interface ModuleCardProps {
  entry: CatalogEntry;
  // Verification posture for this entry, joined client-side from the
  // sources list by the parent (see lib/verify.verifyForEntry).
  verify?: EntryVerify;
  onInstall: (entry: CatalogEntry) => void;
  onUpgrade: (entry: CatalogEntry) => void;
  onUninstall: (entry: CatalogEntry) => void;
  onRemoveUpload?: (entry: CatalogEntry) => void;
  busy?: boolean;
}

// ModuleCard is the catalog grid tile. The right-hand action depends
// on installation state:
//   - not installed      → "Install"
//   - installed, current  → "Deploy" (navigate to /servers/new)
//   - upgrade available  → "Upgrade" (healthy: a newer version is out)
//   - VersionUnavailable → "Update to vX" (Failed but recoverable: the
//                          pinned version left the catalog, so offer the
//                          available one instead of a dead red error)
//   - other phase != Ready → render the phase + spinner (no actions)
export function ModuleCard({
  entry,
  verify,
  onInstall,
  onUpgrade,
  onUninstall,
  onRemoveUpload,
  busy,
}: ModuleCardProps) {
  const upgradeAvailable =
    entry.installed &&
    entry.installedVersion &&
    entry.latestVersion &&
    entry.latestVersion !== entry.installedVersion;

  // Whether the currently PINNED version is one the catalog actually serves.
  // Right after clicking Update the CR status is briefly stale (still Failed,
  // old error) while the operator reconciles — but spec.version already points
  // at an available version, so this guards against re-showing the "update"
  // affordance (or a stale error) for a version that's really fine now.
  const pinnedInCatalog =
    !entry.pinnedVersion || (entry.versions ?? []).includes(entry.pinnedVersion);

  // The pinned version left the catalog (e.g. a git-era version the OCI
  // source never carried). This is a Failed state, but an actionable one:
  // the catalog still has a version, so offer to update rather than dead-end
  // on a red error. Distinct from upgradeAvailable, which is the healthy
  // (Ready) "newer version out" case.
  const versionUnavailable =
    entry.installed &&
    entry.phase === "Failed" &&
    entry.reason === "VersionUnavailable" &&
    !pinnedInCatalog &&
    !!entry.latestVersion;

  const inFlight = entry.phase && entry.phase !== "Ready" && entry.installed;

  const versionLabel = entry.installed
    ? `v${entry.installedVersion ?? "?"} installed${
        upgradeAvailable ? ` · v${entry.latestVersion} available` : ""
      }`
    : entry.latestVersion
    ? `v${entry.latestVersion}`
    : "no published versions";

  // Installed entries show the digest of the running bundle; catalog
  // entries show the latest available digest.
  const digest = entry.appliedDigest ?? entry.digest;

  return (
    <Card className="flex flex-col gap-3 p-4">
      <div className="flex items-start justify-between gap-3">
        <GameIcon game={entry.game ?? entry.name} size="md" />
        <div className="flex items-center gap-1.5">
          {verify && <VerifyBadge verify={verify} />}
          <StatusPill entry={entry} />
        </div>
      </div>
      <div>
        <div className="font-medium text-fg">
          {entry.displayName ?? entry.name}
        </div>
        <div className="pt-0.5 font-mono text-[11px] text-muted">{versionLabel}</div>
        {digest && (
          <div className="flex items-center gap-1 pt-0.5 font-mono text-[10px] text-muted">
            <Fingerprint className="h-3 w-3 shrink-0" />
            <span className="truncate">{shortDigest(digest)}</span>
          </div>
        )}
        {entry.installed && entry.previousVersion && (
          <div className="flex items-center gap-1 pt-0.5 text-[10px] text-muted">
            <History className="h-3 w-3 shrink-0" />
            rollback target · v{entry.previousVersion}
          </div>
        )}
      </div>
      <p className="line-clamp-3 flex-1 text-xs text-muted">
        {entry.summary ?? "No summary."}
      </p>
      {versionUnavailable ? (
        <div className="rounded border border-primary/40 bg-primary/10 px-2 py-1 text-[11px] text-primary">
          Pinned to v{entry.pinnedVersion} — no longer in the catalog. Update to v
          {entry.latestVersion}.
        </div>
      ) : entry.phase === "Failed" &&
        entry.reason !== "VersionUnavailable" &&
        entry.lastError ? (
        // Only surface the raw error for a genuine, current failure. Gating on
        // phase===Failed hides a stale error while the operator is Pulling a
        // new version; excluding VersionUnavailable keeps the actionable
        // "update" banner (above) the sole treatment for that reason and
        // suppresses its leftover error during a re-pin transition.
        <div className="rounded border border-danger/40 bg-danger/10 px-2 py-1 text-[11px] text-danger">
          {entry.lastError}
        </div>
      ) : null}
      <div className="mt-1 flex flex-wrap items-center justify-between gap-2 text-[11px] text-muted">
        <span className="font-mono">
          {entry.sources.length === 1
            ? `${entry.sources[0].name} (${entry.sources[0].type})`
            : `${entry.sources.length} sources`}
        </span>
        <div className="flex items-center gap-1">
          {entry.installed && entry.phase === "Ready" && entry.moduleName && (
            <Button size="sm" variant="outline" asChild>
              <Link
                to="/servers/new"
                search={{ template: entry.moduleName }}
              >
                <ExternalLink className="h-3.5 w-3.5" /> Deploy
              </Link>
            </Button>
          )}
          {upgradeAvailable && entry.phase === "Ready" && (
            <Button size="sm" onClick={() => onUpgrade(entry)} disabled={busy}>
              <ArrowUpCircle className="h-3.5 w-3.5" />
              Upgrade
            </Button>
          )}
          {versionUnavailable && (
            <Button size="sm" onClick={() => onUpgrade(entry)} disabled={busy}>
              <ArrowUpCircle className="h-3.5 w-3.5" />
              Update to v{entry.latestVersion}
            </Button>
          )}
          {!entry.installed && (
            <Button size="sm" onClick={() => onInstall(entry)} disabled={busy}>
              {busy ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Download className="h-3.5 w-3.5" />}
              Install
            </Button>
          )}
          {entry.installed && (
            <Button
              size="sm"
              variant="outline"
              onClick={() => onUninstall(entry)}
              disabled={busy || !!inFlight}
            >
              Uninstall
            </Button>
          )}
          {entry.sources.some((s) => s.type === "upload") && (
            <Button
              size="sm"
              variant="outline"
              onClick={() => onRemoveUpload?.(entry)}
              disabled={busy}
            >
              <Trash2 className="h-3.5 w-3.5" />
              Remove upload
            </Button>
          )}
        </div>
      </div>
    </Card>
  );
}

// VerifyBadge surfaces a module's cosign posture in three states, taking
// care never to over-claim (absence of a badge is the neutral state — we
// never render a red "unsigned" pill, which would be noise on community
// modules):
//   - enforced   → the installed bytes were signature-checked: a solid
//                  "verified" badge.
//   - policy     → a source declares a signature policy but nothing is
//                  installed yet (enforced=false): a softer outline "policy"
//                  badge, so it reads as "declared", not "checked".
//   - mixed/none → suppressed. Mixed candidate sources disagree, so a single
//                  badge would over-claim; none is the neutral default.
// The keyed-vs-keyless distinction lives in the tooltip, not the label — the
// confident badge is about the running-bytes fact, not the policy flavor.
function VerifyBadge({ verify }: { verify: EntryVerify }) {
  if (verify.mode === "none" || verify.mixed) return null;
  const keyless = verify.mode === "keyless";
  if (verify.enforced) {
    return (
      <span
        className="inline-flex items-center gap-1 rounded-full bg-success/15 px-2 py-0.5 font-mono text-[10px] uppercase text-success"
        title={keyless ? "keyless (Fulcio) signature verified" : "signature verified"}
      >
        <ShieldCheck className="h-3 w-3" />
        verified
      </span>
    );
  }
  return (
    <span
      className="inline-flex items-center gap-1 rounded-full border border-success/40 bg-transparent px-2 py-0.5 font-mono text-[10px] uppercase text-success/80"
      title={keyless ? "keyless (Fulcio) signature policy declared" : "signature policy declared"}
    >
      <ShieldQuestion className="h-3 w-3" />
      policy
    </span>
  );
}

// shortDigest trims a "<algo>:<hash>" digest to a glanceable form, keeping
// the algorithm prefix and head/tail of the hash.
function shortDigest(d: string): string {
  const idx = d.indexOf(":");
  const algo = idx >= 0 ? d.slice(0, idx) : "";
  const hash = idx >= 0 ? d.slice(idx + 1) : d;
  const short = hash.length > 16 ? `${hash.slice(0, 8)}…${hash.slice(-4)}` : hash;
  return algo ? `${algo}:${short}` : short;
}

function StatusPill({ entry }: { entry: CatalogEntry }) {
  let label = "available";
  let cls = "bg-muted/20 text-muted";
  if (entry.installed) {
    if (entry.phase === "Ready") {
      label = "installed";
      cls = "bg-success/15 text-success";
    } else if (
      entry.phase === "Failed" &&
      entry.reason === "VersionUnavailable" &&
      !!entry.pinnedVersion &&
      !(entry.versions ?? []).includes(entry.pinnedVersion)
    ) {
      // Failed, but recoverable via an update — read as actionable, not broken.
      // Guarded on the pin genuinely being gone (not a stale status mid-re-pin).
      label = "update";
      cls = "bg-primary/15 text-primary";
    } else if (entry.phase === "Failed") {
      label = "failed";
      cls = "bg-danger/15 text-danger";
    } else {
      label = (entry.phase ?? "pending").toLowerCase();
      cls = "bg-warning/15 text-warning";
    }
  }
  return (
    <span
      className={cn(
        "rounded-full px-2 py-0.5 font-mono text-[10px] uppercase",
        cls,
      )}
    >
      {label}
    </span>
  );
}
