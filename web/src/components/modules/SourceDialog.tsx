import { useEffect, useState, type ReactNode } from "react";
import * as Dialog from "@radix-ui/react-dialog";
import { Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { APIError } from "@/lib/api";
import type { ModuleSource, ModuleSourceSpec, ModuleSourceType } from "@/types";

interface SourceDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  // Existing source to edit; null creates a new one.
  source: ModuleSource | null;
  onConfirm: (args: { name: string; spec: ModuleSourceSpec }) => Promise<void> | void;
  busy?: boolean;
}

const TYPE_OPTIONS: Array<{ value: ModuleSourceType; label: string }> = [
  { value: "oci", label: "OCI registry" },
  { value: "git", label: "Git repository" },
  { value: "http", label: "HTTP archive (.tar.gz / .zip)" },
  { value: "local", label: "Local directory (operator mount)" },
  { value: "upload", label: "Uploaded bundles" },
];

type VerifyMode = "none" | "keyed" | "keyless";

const VERIFY_OPTIONS: Array<{ value: VerifyMode; label: string }> = [
  { value: "none", label: "None" },
  { value: "keyed", label: "Keyed (cosign public key)" },
  { value: "keyless", label: "Keyless (Fulcio)" },
];

// form mirrors the spec union flattened into editable strings.
interface form {
  type: ModuleSourceType;
  url: string;
  modules: string; // comma/space separated (oci)
  secretName: string;
  insecure: boolean;
  ref: string;
  subPath: string;
  path: string;
  allow: string; // comma/space separated
  refreshInterval: string;
  // Cosign signature policy — OCI sources only (CEL-enforced on the CRD).
  verifyMode: VerifyMode;
  verifyKeySecret: string;
  verifyIssuer: string;
  verifyIdentity: string;
}

const emptyForm: form = {
  type: "oci",
  url: "",
  modules: "",
  secretName: "",
  insecure: false,
  ref: "",
  subPath: "",
  path: "",
  allow: "",
  refreshInterval: "",
  verifyMode: "none",
  verifyKeySecret: "",
  verifyIssuer: "",
  verifyIdentity: "",
};

function formFrom(source: ModuleSource): form {
  const spec = source.spec;
  const verify = spec.verify;
  return {
    ...emptyForm,
    type: spec.type ?? "oci",
    url: spec.oci?.url ?? spec.git?.url ?? spec.http?.url ?? "",
    modules: (spec.oci?.modules ?? []).map((m) => m.name).join(", "),
    secretName:
      spec.oci?.pullSecretRef?.name ?? spec.git?.secretRef?.name ?? spec.http?.secretRef?.name ?? "",
    insecure: spec.oci?.insecure ?? spec.http?.insecure ?? false,
    ref: spec.git?.ref ?? "",
    subPath: spec.git?.subPath ?? "",
    path: spec.local?.path ?? "",
    allow: (spec.allow ?? []).join(", "),
    refreshInterval: spec.refreshInterval ?? "",
    verifyMode: verify?.keyless ? "keyless" : verify?.key ? "keyed" : "none",
    verifyKeySecret: verify?.key?.name ?? "",
    verifyIssuer: verify?.keyless?.issuer ?? "",
    verifyIdentity: verify?.keyless?.identity ?? "",
  };
}

function splitList(raw: string): string[] {
  return raw
    .split(/[\s,]+/)
    .map((s) => s.trim())
    .filter(Boolean);
}

// specFrom renders the form back into the discriminated union the API
// expects — only the nested config matching the type is sent.
export function specFrom(f: form): ModuleSourceSpec {
  const spec: ModuleSourceSpec = { type: f.type };
  if (f.allow.trim()) spec.allow = splitList(f.allow);
  if (f.refreshInterval.trim()) spec.refreshInterval = f.refreshInterval.trim();
  const secretRef = f.secretName.trim() ? { name: f.secretName.trim() } : undefined;
  switch (f.type) {
    case "oci":
      spec.oci = {
        url: f.url.trim(),
        modules: splitList(f.modules).map((name) => ({ name })),
        ...(f.insecure ? { insecure: true } : {}),
        ...(secretRef ? { pullSecretRef: secretRef } : {}),
      };
      // verify is OCI-only (CEL-enforced); never emit it for other types.
      if (f.verifyMode === "keyed") {
        spec.verify = { key: { name: f.verifyKeySecret.trim() } };
      } else if (f.verifyMode === "keyless") {
        spec.verify = {
          keyless: { issuer: f.verifyIssuer.trim(), identity: f.verifyIdentity.trim() },
        };
      }
      break;
    case "git":
      spec.git = {
        url: f.url.trim(),
        ...(f.ref.trim() ? { ref: f.ref.trim() } : {}),
        ...(f.subPath.trim() ? { subPath: f.subPath.trim() } : {}),
        ...(secretRef ? { secretRef } : {}),
      };
      break;
    case "http":
      spec.http = {
        url: f.url.trim(),
        ...(f.insecure ? { insecure: true } : {}),
        ...(secretRef ? { secretRef } : {}),
      };
      break;
    case "local":
      spec.local = f.path.trim() ? { path: f.path.trim() } : {};
      break;
    case "upload":
      break;
  }
  return spec;
}

// SourceDialog adds or edits a ModuleSource: a type selector plus the
// fields that type needs. Admin-only on the server side.
export function SourceDialog({ open, onOpenChange, source, onConfirm, busy }: SourceDialogProps) {
  const editing = source !== null;
  const [name, setName] = useState("");
  const [f, setF] = useState<form>(emptyForm);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) return;
    setError(null);
    setName(source?.metadata.name ?? "");
    setF(source ? formFrom(source) : emptyForm);
  }, [open, source]);

  const set = (patch: Partial<form>) => setF((prev) => ({ ...prev, ...patch }));

  async function submit() {
    if (!name.trim()) {
      setError("name is required");
      return;
    }
    if (f.type !== "upload" && f.type !== "local" && !f.url.trim()) {
      setError("url is required");
      return;
    }
    if (f.type === "oci" && splitList(f.modules).length === 0) {
      setError("OCI sources need at least one module name");
      return;
    }
    if (f.type === "oci" && f.verifyMode === "keyed" && !f.verifyKeySecret.trim()) {
      setError("keyed verification needs a public key secret name");
      return;
    }
    if (
      f.type === "oci" &&
      f.verifyMode === "keyless" &&
      (!f.verifyIssuer.trim() || !f.verifyIdentity.trim())
    ) {
      setError("keyless verification needs an issuer and identity");
      return;
    }
    setError(null);
    try {
      await onConfirm({ name: name.trim(), spec: specFrom(f) });
    } catch (err) {
      setError(err instanceof APIError ? err.body || err.message : (err as Error).message);
    }
  }

  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 z-40 bg-black/60" />
        <Dialog.Content className="fixed left-1/2 top-1/2 z-50 w-[520px] max-w-[calc(100vw-2rem)] -translate-x-1/2 -translate-y-1/2 rounded-lg border border-border bg-card p-5 text-fg shadow-2xl">
          <Dialog.Title className="text-base font-semibold">
            {editing ? `Edit source ${source.metadata.name}` : "Add module source"}
          </Dialog.Title>
          <Dialog.Description className="pt-1 text-xs text-muted">
            Where the operator discovers and pulls module bundles from.
          </Dialog.Description>

          <div className="max-h-[60vh] space-y-3 overflow-y-auto pt-4">
            {!editing && (
              <Field label="Name" hint="DNS label identifying this source.">
                <Input
                  value={name}
                  onChange={(e) => setName(e.target.value.toLowerCase())}
                  placeholder="community"
                />
              </Field>
            )}
            <Field label="Type">
              <Select
                value={f.type}
                onValueChange={(v) => set({ type: v as ModuleSourceType })}
                options={TYPE_OPTIONS}
              />
            </Field>

            {f.type === "oci" && (
              <>
                <Field label="Registry URL">
                  <Input
                    value={f.url}
                    onChange={(e) => set({ url: e.target.value })}
                    placeholder="ghcr.io/valgulnecron/gameplane-modules"
                  />
                </Field>
                <Field
                  label="Modules"
                  hint="Comma-separated module names (registries can't be enumerated)."
                >
                  <Input
                    value={f.modules}
                    onChange={(e) => set({ modules: e.target.value })}
                    placeholder="minecraft-java, valheim"
                  />
                </Field>
                <Field label="Pull secret" hint="dockerconfigjson Secret in the operator namespace (optional).">
                  <Input
                    value={f.secretName}
                    onChange={(e) => set({ secretName: e.target.value })}
                    placeholder="registry-creds"
                  />
                </Field>
                <InsecureToggle checked={f.insecure} onChange={(insecure) => set({ insecure })} />
                <Field
                  label="Signature verification"
                  hint="Require a valid cosign signature on every bundle pulled from this source."
                >
                  <Select
                    value={f.verifyMode}
                    onValueChange={(v) => set({ verifyMode: v as VerifyMode })}
                    options={VERIFY_OPTIONS}
                  />
                </Field>
                {f.verifyMode === "keyed" && (
                  <Field
                    label="Public key secret"
                    hint="Secret holding the cosign public key under the cosign.pub data key."
                  >
                    <Input
                      value={f.verifyKeySecret}
                      onChange={(e) => set({ verifyKeySecret: e.target.value })}
                      placeholder="cosign-pub"
                    />
                  </Field>
                )}
                {f.verifyMode === "keyless" && (
                  <>
                    <Field label="OIDC issuer" hint="Issuer embedded in the signing certificate.">
                      <Input
                        value={f.verifyIssuer}
                        onChange={(e) => set({ verifyIssuer: e.target.value })}
                        placeholder="https://token.actions.githubusercontent.com"
                      />
                    </Field>
                    <Field
                      label="Certificate identity"
                      hint="SAN identity that must have produced the signature."
                    >
                      <Input
                        value={f.verifyIdentity}
                        onChange={(e) => set({ verifyIdentity: e.target.value })}
                        placeholder="github.com/org/repo/.github/workflows/release.yml@refs/heads/main"
                      />
                    </Field>
                  </>
                )}
              </>
            )}

            {f.type === "git" && (
              <>
                <Field label="Clone URL">
                  <Input
                    value={f.url}
                    onChange={(e) => set({ url: e.target.value })}
                    placeholder="https://github.com/example/gameplane-modules"
                  />
                </Field>
                <Field label="Ref" hint="Branch or tag. Defaults to main.">
                  <Input value={f.ref} onChange={(e) => set({ ref: e.target.value })} placeholder="main" />
                </Field>
                <Field label="Subdirectory" hint="Scan only this path inside the repo (optional).">
                  <Input
                    value={f.subPath}
                    onChange={(e) => set({ subPath: e.target.value })}
                    placeholder="modules"
                  />
                </Field>
                <Field
                  label="Credentials secret"
                  hint="Secret with token / username+password (https) or ssh-privatekey + known_hosts (ssh). Optional."
                >
                  <Input
                    value={f.secretName}
                    onChange={(e) => set({ secretName: e.target.value })}
                    placeholder="gh-creds"
                  />
                </Field>
              </>
            )}

            {f.type === "http" && (
              <>
                <Field label="Archive URL" hint="A .tar.gz or .zip of module directories.">
                  <Input
                    value={f.url}
                    onChange={(e) => set({ url: e.target.value })}
                    placeholder="https://example.com/modules.tar.gz"
                  />
                </Field>
                <Field label="Credentials secret" hint="Secret with token (Bearer) or username+password. Optional.">
                  <Input
                    value={f.secretName}
                    onChange={(e) => set({ secretName: e.target.value })}
                    placeholder="archive-creds"
                  />
                </Field>
                <InsecureToggle checked={f.insecure} onChange={(insecure) => set({ insecure })} />
              </>
            )}

            {f.type === "local" && (
              <Field
                label="Path"
                hint="Relative to the operator's module mount (Helm: operator.localModules). Empty scans the mount root."
              >
                <Input value={f.path} onChange={(e) => set({ path: e.target.value })} placeholder="bundles" />
              </Field>
            )}

            {f.type === "upload" && (
              <div className="rounded border border-border bg-card/40 px-3 py-2 text-xs text-muted">
                Indexes bundles uploaded through the dashboard (stored as ConfigMaps in the
                operator namespace). No further configuration.
              </div>
            )}

            <Field label="Allow list" hint="Optional module name filter — exact names or globs, comma-separated.">
              <Input
                value={f.allow}
                onChange={(e) => set({ allow: e.target.value })}
                placeholder="minecraft-*"
              />
            </Field>
            <Field label="Refresh interval" hint="How often the catalog re-indexes. Defaults to 1h.">
              <Input
                value={f.refreshInterval}
                onChange={(e) => set({ refreshInterval: e.target.value })}
                placeholder="1h"
              />
            </Field>
          </div>

          {error && (
            <div className="mt-3 rounded border border-danger/40 bg-danger/10 px-3 py-2 text-xs text-danger">
              {error}
            </div>
          )}

          <div className="mt-5 flex justify-end gap-2">
            <Button variant="outline" onClick={() => onOpenChange(false)} disabled={busy}>
              Cancel
            </Button>
            <Button onClick={submit} disabled={busy}>
              {busy ? <Loader2 className="h-4 w-4 animate-spin" /> : null}
              {editing ? "Save" : "Add source"}
            </Button>
          </div>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}

function InsecureToggle({ checked, onChange }: { checked: boolean; onChange: (v: boolean) => void }) {
  return (
    <label className="flex items-center gap-2 text-xs text-muted">
      <input
        type="checkbox"
        checked={checked}
        onChange={(e) => onChange(e.target.checked)}
        className="h-3.5 w-3.5 accent-primary"
      />
      Allow plain HTTP (local registries only)
    </label>
  );
}

function Field({ label, hint, children }: { label: string; hint?: string; children: ReactNode }) {
  return (
    <label className="block">
      <div className="pb-1 text-xs text-muted">{label}</div>
      {children}
      {hint && <div className="pt-1 text-[11px] text-muted">{hint}</div>}
    </label>
  );
}
