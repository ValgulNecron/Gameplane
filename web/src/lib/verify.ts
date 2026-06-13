import type { CatalogEntry, ModuleSource, ModuleVerifySpec } from "@/types";

export type VerifyMode = "keyed" | "keyless" | "none";

// verifyMode classifies a source's cosign policy, mirroring
// ModuleSource.spec.verify: keyless (Fulcio) takes precedence over a keyed
// public key if both are somehow present; an absent verify block means the
// source does not require signatures.
export function verifyMode(spec?: ModuleVerifySpec): VerifyMode {
  if (spec?.keyless) return "keyless";
  if (spec?.key) return "keyed";
  return "none";
}

// EntryVerify is the catalog-card view of a module's verification posture.
// ModuleCard's VerifyBadge maps it: enforced -> solid "verified"; mode!=none
// && !enforced && !mixed -> outline "policy" (declared, not yet checked);
// mixed || none -> no badge (suppressed, never over-claims).
export interface EntryVerify {
  // mode is the representative policy to badge.
  mode: VerifyMode;
  // enforced is true only for an installed entry whose source actually
  // verifies — i.e. the running bytes were signature-checked. It is false
  // pre-install, because nothing has been verified yet.
  enforced: boolean;
  // mixed flags a not-yet-installed entry whose candidate sources disagree on
  // policy, so a single badge would over-claim.
  mixed: boolean;
}

// verifyForEntry derives the verification posture for one catalog row by
// joining it against the live ModuleSource list (already fetched by the
// Modules page). The join lives client-side on purpose: the operator exposes
// no "verified" flag, and aggregating one server-side would both add business
// logic to the API (rule 10 — it stays a pure read/map) and erase the
// per-source distinction the UI needs. For an installed entry the
// authoritative source is the one it was pulled from; otherwise we summarise
// across the candidate sources.
export function verifyForEntry(
  entry: CatalogEntry,
  sources: ModuleSource[],
): EntryVerify {
  const byName = new Map<string, ModuleSource>();
  for (const s of sources) {
    if (s.metadata?.name) byName.set(s.metadata.name, s);
  }

  if (entry.installed && entry.installedFrom) {
    const mode = verifyMode(byName.get(entry.installedFrom)?.spec.verify);
    return { mode, enforced: mode !== "none", mixed: false };
  }

  const modes = (entry.sources ?? []).map((ref) =>
    verifyMode(byName.get(ref.name)?.spec.verify),
  );
  const mixed = modes.some((m) => m !== modes[0]);
  const mode = modes.find((m) => m !== "none") ?? "none";
  return { mode, enforced: false, mixed };
}
