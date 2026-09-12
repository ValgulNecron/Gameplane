import { useState } from "react";
import { Loader2 } from "lucide-react";
import {
  Modal,
  ModalBackdrop,
  ModalContainer,
  ModalDialog,
  ModalHeader,
  ModalHeading,
  ModalBody,
  ModalFooter,
  Button,
  Input,
  Label,
  Description,
  Select,
  ListBox,
  ListBoxItem,
  Checkbox,
} from "@heroui/react";
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

  // Reset the form whenever the dialog (re)opens or the source being edited
  // changes. Adjusted directly during render (not in an effect), gated on
  // the previously-seen (open, source) pair.
  const [resetFor, setResetFor] = useState<{ open: boolean; source: ModuleSource | null }>({
    open: false,
    source,
  });
  if (open !== resetFor.open || source !== resetFor.source) {
    setResetFor({ open, source });
    if (open) {
      setError(null);
      setName(source?.metadata.name ?? "");
      setF(source ? formFrom(source) : emptyForm);
    }
  }

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
    <Modal isOpen={open} onOpenChange={onOpenChange}>
      <ModalBackdrop isDismissable={!busy} isKeyboardDismissDisabled={busy}>
        <ModalContainer>
          <ModalDialog>
          <ModalHeader>
            <ModalHeading>
              {editing ? `Edit source ${source.metadata.name}` : "Add module source"}
            </ModalHeading>
          </ModalHeader>

          <ModalBody className="gap-4 max-h-[60vh] overflow-y-auto">
            <Description className="text-sm text-muted">
              Where the operator discovers and pulls module bundles from.
            </Description>

            {!editing && (
              <div className="space-y-1">
                <Label htmlFor="source-name" className="text-xs">
                  Name
                </Label>
                <Input
                  id="source-name"
                  value={name}
                  onChange={(e) => setName(e.target.value.toLowerCase())}
                  placeholder="community"
                  className="mt-1"
                />
                <Description className="mt-1 text-xs">
                  DNS label identifying this source.
                </Description>
              </div>
            )}

            <div className="space-y-1">
              <Select
                value={f.type}
                onChange={(v) => set({ type: v as ModuleSourceType })}
                className="mt-1"
                aria-label="Type"
              >
                <Label className="text-xs">Type</Label>
                <Select.Trigger>
                  <Select.Value />
                  <Select.Indicator className="ml-auto h-4 w-4" />
                </Select.Trigger>
                <Select.Popover>
                  <ListBox aria-label="Type options">
                    {TYPE_OPTIONS.map((opt) => (
                      <ListBoxItem key={opt.value} id={opt.value}>
                        {opt.label}
                      </ListBoxItem>
                    ))}
                  </ListBox>
                </Select.Popover>
              </Select>
            </div>

            {f.type === "oci" && (
              <>
                <div className="space-y-1">
                  <Label htmlFor="oci-url" className="text-xs">
                    Registry URL
                  </Label>
                  <Input
                    id="oci-url"
                    value={f.url}
                    onChange={(e) => set({ url: e.target.value })}
                    placeholder="ghcr.io/valgulnecron/gameplane-modules"
                    className="mt-1"
                  />
                </div>

                <div className="space-y-1">
                  <Label htmlFor="oci-modules" className="text-xs">
                    Modules
                  </Label>
                  <Input
                    id="oci-modules"
                    value={f.modules}
                    onChange={(e) => set({ modules: e.target.value })}
                    placeholder="minecraft-java, valheim"
                    className="mt-1"
                  />
                  <Description className="mt-1 text-xs">
                    Comma-separated module names (registries can&apos;t be enumerated).
                  </Description>
                </div>

                <div className="space-y-1">
                  <Label htmlFor="oci-secret" className="text-xs">
                    Pull secret
                  </Label>
                  <Input
                    id="oci-secret"
                    value={f.secretName}
                    onChange={(e) => set({ secretName: e.target.value })}
                    placeholder="registry-creds"
                    className="mt-1"
                  />
                  <Description className="mt-1 text-xs">
                    dockerconfigjson Secret in the operator namespace (optional).
                  </Description>
                </div>

                <Checkbox
                  id="oci-insecure"
                  isSelected={f.insecure}
                  onChange={(isSelected) => set({ insecure: isSelected })}
                  className="flex-row items-center gap-2"
                >
                  <Checkbox.Control>
                    <Checkbox.Indicator />
                  </Checkbox.Control>
                  <Checkbox.Content className="text-xs text-muted">
                    Allow plain HTTP (local registries only)
                  </Checkbox.Content>
                </Checkbox>

                <div className="space-y-1">
                  <Select
                    value={f.verifyMode}
                    onChange={(v) => set({ verifyMode: v as VerifyMode })}
                    className="mt-1"
                    aria-label="Signature verification"
                  >
                    <Label className="text-xs">Signature verification</Label>
                    <Select.Trigger>
                      <Select.Value />
                      <Select.Indicator className="ml-auto h-4 w-4" />
                    </Select.Trigger>
                    <Select.Popover>
                      <ListBox aria-label="Signature verification options">
                        {VERIFY_OPTIONS.map((opt) => (
                          <ListBoxItem key={opt.value} id={opt.value}>
                            {opt.label}
                          </ListBoxItem>
                        ))}
                      </ListBox>
                    </Select.Popover>
                  </Select>
                  <Description className="mt-1 text-xs">
                    Require a valid cosign signature on every bundle pulled from this source.
                  </Description>
                </div>

                {f.verifyMode === "keyed" && (
                  <div className="space-y-1">
                    <Label htmlFor="verify-key-secret" className="text-xs">
                      Public key secret
                    </Label>
                    <Input
                      id="verify-key-secret"
                      value={f.verifyKeySecret}
                      onChange={(e) => set({ verifyKeySecret: e.target.value })}
                      placeholder="cosign-pub"
                      className="mt-1"
                    />
                    <Description className="mt-1 text-xs">
                      Secret holding the cosign public key under the cosign.pub data key.
                    </Description>
                  </div>
                )}

                {f.verifyMode === "keyless" && (
                  <>
                    <div className="space-y-1">
                      <Label htmlFor="verify-issuer" className="text-xs">
                        OIDC issuer
                      </Label>
                      <Input
                        id="verify-issuer"
                        value={f.verifyIssuer}
                        onChange={(e) => set({ verifyIssuer: e.target.value })}
                        placeholder="https://token.actions.githubusercontent.com"
                        className="mt-1"
                      />
                      <Description className="mt-1 text-xs">
                        Issuer embedded in the signing certificate.
                      </Description>
                    </div>

                    <div className="space-y-1">
                      <Label htmlFor="verify-identity" className="text-xs">
                        Certificate identity
                      </Label>
                      <Input
                        id="verify-identity"
                        value={f.verifyIdentity}
                        onChange={(e) => set({ verifyIdentity: e.target.value })}
                        placeholder="github.com/org/repo/.github/workflows/release.yml@refs/heads/main"
                        className="mt-1"
                      />
                      <Description className="mt-1 text-xs">
                        SAN identity that must have produced the signature.
                      </Description>
                    </div>
                  </>
                )}
              </>
            )}

            {f.type === "git" && (
              <>
                <div className="space-y-1">
                  <Label htmlFor="git-url" className="text-xs">
                    Clone URL
                  </Label>
                  <Input
                    id="git-url"
                    value={f.url}
                    onChange={(e) => set({ url: e.target.value })}
                    placeholder="https://github.com/example/gameplane-modules"
                    className="mt-1"
                  />
                </div>

                <div className="space-y-1">
                  <Label htmlFor="git-ref" className="text-xs">
                    Ref
                  </Label>
                  <Input
                    id="git-ref"
                    value={f.ref}
                    onChange={(e) => set({ ref: e.target.value })}
                    placeholder="main"
                    className="mt-1"
                  />
                  <Description className="mt-1 text-xs">
                    Branch or tag. Defaults to main.
                  </Description>
                </div>

                <div className="space-y-1">
                  <Label htmlFor="git-subpath" className="text-xs">
                    Subdirectory
                  </Label>
                  <Input
                    id="git-subpath"
                    value={f.subPath}
                    onChange={(e) => set({ subPath: e.target.value })}
                    placeholder="modules"
                    className="mt-1"
                  />
                  <Description className="mt-1 text-xs">
                    Scan only this path inside the repo (optional).
                  </Description>
                </div>

                <div className="space-y-1">
                  <Label htmlFor="git-secret" className="text-xs">
                    Credentials secret
                  </Label>
                  <Input
                    id="git-secret"
                    value={f.secretName}
                    onChange={(e) => set({ secretName: e.target.value })}
                    placeholder="gh-creds"
                    className="mt-1"
                  />
                  <Description className="mt-1 text-xs">
                    Secret with token / username+password (https) or ssh-privatekey + known_hosts (ssh). Optional.
                  </Description>
                </div>
              </>
            )}

            {f.type === "http" && (
              <>
                <div className="space-y-1">
                  <Label htmlFor="http-url" className="text-xs">
                    Archive URL
                  </Label>
                  <Input
                    id="http-url"
                    value={f.url}
                    onChange={(e) => set({ url: e.target.value })}
                    placeholder="https://example.com/modules.tar.gz"
                    className="mt-1"
                  />
                  <Description className="mt-1 text-xs">
                    A .tar.gz or .zip of module directories.
                  </Description>
                </div>

                <div className="space-y-1">
                  <Label htmlFor="http-secret" className="text-xs">
                    Credentials secret
                  </Label>
                  <Input
                    id="http-secret"
                    value={f.secretName}
                    onChange={(e) => set({ secretName: e.target.value })}
                    placeholder="archive-creds"
                    className="mt-1"
                  />
                  <Description className="mt-1 text-xs">
                    Secret with token (Bearer) or username+password. Optional.
                  </Description>
                </div>

                <Checkbox
                  id="http-insecure"
                  isSelected={f.insecure}
                  onChange={(isSelected) => set({ insecure: isSelected })}
                  className="flex-row items-center gap-2"
                >
                  <Checkbox.Control>
                    <Checkbox.Indicator />
                  </Checkbox.Control>
                  <Checkbox.Content className="text-xs text-muted">
                    Allow plain HTTP (local registries only)
                  </Checkbox.Content>
                </Checkbox>
              </>
            )}

            {f.type === "local" && (
              <div className="space-y-1">
                <Label htmlFor="local-path" className="text-xs">
                  Path
                </Label>
                <Input
                  id="local-path"
                  value={f.path}
                  onChange={(e) => set({ path: e.target.value })}
                  placeholder="bundles"
                  className="mt-1"
                />
                <Description className="mt-1 text-xs">
                  Relative to the operator&apos;s module mount (Helm: operator.localModules). Empty scans the mount root.
                </Description>
              </div>
            )}

            {f.type === "upload" && (
              <div className="rounded border border-border bg-card/40 px-3 py-2 text-xs text-muted">
                Indexes bundles uploaded through the dashboard (stored as ConfigMaps in the
                operator namespace). No further configuration.
              </div>
            )}

            <div className="space-y-1">
              <Label htmlFor="allow-list" className="text-xs">
                Allow list
              </Label>
              <Input
                id="allow-list"
                value={f.allow}
                onChange={(e) => set({ allow: e.target.value })}
                placeholder="minecraft-*"
                className="mt-1"
              />
              <Description className="mt-1 text-xs">
                Optional module name filter — exact names or globs, comma-separated.
              </Description>
            </div>

            <div className="space-y-1">
              <Label htmlFor="refresh-interval" className="text-xs">
                Refresh interval
              </Label>
              <Input
                id="refresh-interval"
                value={f.refreshInterval}
                onChange={(e) => set({ refreshInterval: e.target.value })}
                placeholder="1h"
                className="mt-1"
              />
              <Description className="mt-1 text-xs">
                How often the catalog re-indexes. Defaults to 1h.
              </Description>
            </div>

            {error && (
              <p role="alert" className="mt-2 text-xs text-danger">
                {error}
              </p>
            )}
          </ModalBody>

          <ModalFooter className="gap-2">
            <Button
              variant="secondary"
              size="sm"
              onPress={() => onOpenChange(false)}
              isDisabled={busy}
            >
              Cancel
            </Button>
            <Button
              variant="primary"
              size="sm"
              isDisabled={busy}
              onPress={submit}
            >
              {busy ? <Loader2 className="h-4 w-4 animate-spin" /> : null}
              {editing ? "Save" : "Add source"}
            </Button>
          </ModalFooter>
          </ModalDialog>
        </ModalContainer>
      </ModalBackdrop>
    </Modal>
  );
}
