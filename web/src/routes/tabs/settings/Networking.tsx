import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { Button } from "@/components/ui/button";
import { X, AlertCircle, Check, Loader2 } from "lucide-react";
import { useEffect, useState } from "react";
import { Link } from "@tanstack/react-router";
import { useMutation, useQuery } from "@tanstack/react-query";
import type { Expose, GameServer, GameServerNetworking, GameServerTunnel, RemotePortMapping } from "@/types";
import { PortOverridesEditor } from "@/components/server/PortOverridesEditor";
import { Servers } from "@/lib/endpoints";
import { errorText } from "@/lib/errors";
import { Field } from "./Field";
import type { SectionProps } from "./types";

const EXPOSE_OPTIONS: { value: Expose; label: string }[] = [
  { value: "ClusterIP", label: "ClusterIP (in-cluster only)" },
  { value: "NodePort", label: "NodePort" },
  { value: "LoadBalancer", label: "LoadBalancer" },
  { value: "Hostport", label: "Hostport" },
];

const TUNNEL_PROVIDER_OPTIONS: { value: "frp" | "tailscale" | "playit"; label: string }[] = [
  { value: "frp", label: "frp" },
  { value: "tailscale", label: "Tailscale" },
  { value: "playit", label: "playit.gg" },
];

export function NetworkingSection({ draft, onChange, onValidityChange }: SectionProps) {
  const net = draft.spec.networking ?? {};
  const tunnel = net.tunnel;
  const serverName = draft.metadata.name;
  const serverNamespace = draft.metadata.namespace;

  // Credential entry state — NOT part of the spec form's dirty tracking.
  const [showCredentialInput, setShowCredentialInput] = useState(false);
  const [credentialValue, setCredentialValue] = useState("");

  // Query current tunnel credentials status.
  const { data: credentialStatus, refetch: refetchCredentials } = useQuery({
    queryKey: ["tunnel-credentials", serverName],
    queryFn: () => Servers.getTunnelCredentials(serverName, serverNamespace),
    enabled: tunnel?.enabled ?? false,
  });

  const setNet = (next: GameServerNetworking) => {
    const cleaned: GameServerNetworking = {};
    if (next.expose) cleaned.expose = next.expose;
    if (next.hostname) cleaned.hostname = next.hostname;
    if (next.serviceAnnotations && Object.keys(next.serviceAnnotations).length) {
      cleaned.serviceAnnotations = next.serviceAnnotations;
    }
    if (next.portOverrides && next.portOverrides.length) {
      cleaned.portOverrides = next.portOverrides;
    }
    if (next.sourceRanges && next.sourceRanges.length) {
      cleaned.sourceRanges = next.sourceRanges;
    }
    if (next.addressPool) cleaned.addressPool = next.addressPool;
    if (next.address) cleaned.address = next.address;
    if (next.tunnel) cleaned.tunnel = next.tunnel;
    onChange({
      ...draft,
      spec: {
        ...draft.spec,
        networking: Object.keys(cleaned).length ? cleaned : undefined,
      },
    });
  };

  // Mutation to save credentials.
  const saveCredentialMutation = useMutation({
    mutationFn: async () => {
      if (!tunnel?.provider || !credentialValue.trim()) {
        throw new Error("Provider and credential value required");
      }
      const key =
        tunnel.provider === "frp"
          ? "token"
          : tunnel.provider === "tailscale"
            ? "authKey"
            : "secretKey";
      await Servers.setTunnelCredentials(serverName, tunnel.provider, { [key]: credentialValue }, serverNamespace);
      // Refetch credential status to get the updated secretName.
      void refetchCredentials();
    },
    onSuccess: () => {
      setCredentialValue("");
      setShowCredentialInput(false);
    },
  });

  // Mutation to remove credentials.
  const removeCredentialMutation = useMutation({
    mutationFn: () => Servers.removeTunnelCredentials(serverName, serverNamespace),
    onSuccess: () => {
      // Clear the credentialsSecretRef from the spec.
      setNet({ ...net, tunnel: tunnel ? { ...tunnel, credentialsSecretRef: undefined } : undefined });
      void refetchCredentials();
    },
  });

  // Compute tunnel configuration validity.
  let computedValidityError: string | null = null;
  if (tunnel?.enabled) {
    const errors: string[] = [];

    // Credentials must be configured (either already in spec or being entered).
    const hasConfiguredCredential = tunnel.credentialsSecretRef?.name || credentialStatus?.configured;
    if (!hasConfiguredCredential && !credentialValue.trim()) {
      errors.push("Tunnel credentials are required");
    }

    // Provider-specific validation.
    if (tunnel.provider === "frp") {
      if (!tunnel.frp?.serverAddr) {
        errors.push("Server address is required for frp");
      }
      const ports = tunnel.frp?.remotePorts ?? [];
      if (ports.length === 0) {
        errors.push("At least one port mapping is required for frp");
      } else {
        // Check each port mapping is valid.
        for (let i = 0; i < ports.length; i++) {
          const p = ports[i];
          if (!p.name) {
            errors.push(`Port mapping ${i + 1}: name is required`);
          }
          if (!p.remotePort || p.remotePort < 1 || p.remotePort > 65535) {
            errors.push(`Port mapping ${i + 1}: remote port must be 1–65535`);
          }
        }
      }
    }
    // tailscale and playit have no additional required fields beyond credentials.

    computedValidityError = errors.length === 0 ? null : errors[0];
  }

  // Call validity change callback whenever computed result changes.
  useEffect(() => {
    const isValid = computedValidityError === null;
    onValidityChange?.(isValid);
  }, [computedValidityError, onValidityChange]);

  return (
    <div className="space-y-6">
      <Field
        label="Expose"
        hint="Service type fronting the game pod. Still applies when a tunnel is enabled.">
        <Select
          value={net.expose ?? "ClusterIP"}
          options={EXPOSE_OPTIONS}
          onValueChange={(v) => setNet({ ...net, expose: v as Expose })}
        />
      </Field>

      {/* Tunnel section */}
      <div className="border-t border-border pt-6">
        <div className="space-y-4">
          <div>
            <div className="flex items-center gap-2">
              <label className="flex items-center gap-2 cursor-pointer">
                <input
                  type="checkbox"
                  checked={tunnel?.enabled ?? false}
                  onChange={(e) => {
                    const newTunnel: GameServerTunnel | undefined = e.target.checked
                      ? { enabled: true, provider: tunnel?.provider ?? "frp" }
                      : undefined;
                    setNet({ ...net, tunnel: newTunnel });
                  }}
                  className="h-4 w-4 rounded border border-border bg-surface accent-primary"
                />
                <span className="text-sm font-medium">Enable tunnel</span>
              </label>
            </div>
            <div className="pt-1 text-xs text-muted">
              Route players through a relay so they can connect without port-forwarding or a public IP.
            </div>
          </div>

          {tunnel?.enabled && tunnel && (
            <div className="space-y-4 rounded-lg border border-border bg-surface/40 p-4">
              <Field label="Provider" hint="">
                <Select
                  value={tunnel.provider}
                  options={TUNNEL_PROVIDER_OPTIONS}
                  onValueChange={(v) =>
                    setNet({
                      ...net,
                      tunnel: { ...tunnel, provider: v as "frp" | "tailscale" | "playit" },
                    })
                  }
                />
              </Field>

              {tunnel.provider === "tailscale" && (
                <div className="rounded-md border-l-4 border-warning bg-warning/10 p-3 text-sm">
                  <div className="flex gap-2">
                    <AlertCircle className="h-5 w-5 shrink-0 text-warning" />
                    <div>
                      <div className="font-medium text-warning">Tailnet only — not public</div>
                      <div className="pt-0.5 text-xs text-warning/80">
                        The server becomes reachable only from your Tailscale network, never from the public internet.
                      </div>
                    </div>
                  </div>
                </div>
              )}

              <TunnelCredentialField
                configured={credentialStatus?.configured}
                secretName={credentialStatus?.secretName}
                isLoading={saveCredentialMutation.isPending || removeCredentialMutation.isPending}
                error={
                  saveCredentialMutation.error
                    ? errorText(saveCredentialMutation.error, "Failed to save the credential")
                    : removeCredentialMutation.error
                      ? errorText(removeCredentialMutation.error, "Failed to remove the credential")
                      : undefined
                }
                showInput={showCredentialInput}
                onShowInput={() => setShowCredentialInput(true)}
                provider={tunnel.provider}
                credentialValue={credentialValue}
                onCredentialChange={setCredentialValue}
                onSave={() => saveCredentialMutation.mutate()}
                onCancel={() => {
                  setShowCredentialInput(false);
                  setCredentialValue("");
                }}
                onRemove={() => removeCredentialMutation.mutate()}
                hasSecretRef={!!tunnel.credentialsSecretRef?.name}
              />

              {tunnel.provider === "frp" && (
                <>
                  <Field label="Server address" hint="frp server address.">
                    <Input
                      value={tunnel.frp?.serverAddr ?? ""}
                      onChange={(e) =>
                        setNet({
                          ...net,
                          tunnel: {
                            ...tunnel,
                            frp: {
                              serverAddr: e.target.value,
                              serverPort: tunnel.frp?.serverPort,
                              remotePorts: tunnel.frp?.remotePorts ?? [],
                            },
                          },
                        })
                      }
                      placeholder="relay.example.com"
                      spellCheck={false}
                    />
                  </Field>
                  <Field label="Server port" hint="frp server port (default 7000).">
                    <Input
                      type="number"
                      value={tunnel.frp?.serverPort ?? 7000}
                      onChange={(e) =>
                        setNet({
                          ...net,
                          tunnel: {
                            ...tunnel,
                            frp: {
                              serverAddr: tunnel.frp?.serverAddr ?? "",
                              serverPort: e.target.value ? Number(e.target.value) : undefined,
                              remotePorts: tunnel.frp?.remotePorts ?? [],
                            },
                          },
                        })
                      }
                      placeholder="7000"
                      inputMode="numeric"
                    />
                  </Field>
                  <div>
                    <div className="text-sm text-fg">Port mappings</div>
                    <div className="pt-1 text-xs text-muted">Name and remote port for each game port.</div>
                    <div className="pt-2">
                      <FrpPortMappingEditor
                        values={tunnel.frp?.remotePorts ?? []}
                        onChange={(v) =>
                          setNet({
                            ...net,
                            tunnel: {
                              ...tunnel,
                              frp: {
                                serverAddr: tunnel.frp?.serverAddr ?? "",
                                serverPort: tunnel.frp?.serverPort,
                                remotePorts: v,
                              },
                            },
                          })
                        }
                      />
                    </div>
                  </div>
                </>
              )}

              {tunnel.provider === "tailscale" && (
                <>
                  <Field label="Hostname" hint="Hostname in your tailnet (optional).">
                    <Input
                      value={tunnel.tailscale?.hostname ?? ""}
                      onChange={(e) =>
                        setNet({
                          ...net,
                          tunnel: {
                            ...tunnel,
                            tailscale: {
                              ...tunnel.tailscale,
                              hostname: e.target.value || undefined,
                              tags: tunnel.tailscale?.tags,
                            },
                          },
                        })
                      }
                      placeholder="game-server"
                      spellCheck={false}
                    />
                  </Field>
                  <Field label="Tags" hint="Comma-separated tags (optional).">
                    <Input
                      value={(tunnel.tailscale?.tags ?? []).join(", ")}
                      onChange={(e) =>
                        setNet({
                          ...net,
                          tunnel: {
                            ...tunnel,
                            tailscale: {
                              hostname: tunnel.tailscale?.hostname,
                              tags: e.target.value
                                ? e.target.value
                                    .split(",")
                                    .map((t) => t.trim())
                                    .filter(Boolean)
                                : undefined,
                            },
                          },
                        })
                      }
                      placeholder="tag:server, tag:minecraft"
                      spellCheck={false}
                    />
                  </Field>
                </>
              )}

              {tunnel.provider === "playit" && (
                <Field label="Tunnel name" hint="Name of the tunnel on playit.gg.">
                  <Input
                    value={tunnel.playit?.tunnelName ?? ""}
                    onChange={(e) =>
                      setNet({
                        ...net,
                        tunnel: {
                          ...tunnel,
                          playit: {
                            tunnelName: e.target.value || undefined,
                          },
                        },
                      })
                    }
                    placeholder="my-server"
                    spellCheck={false}
                  />
                </Field>
              )}
              {tunnel.provider === "playit" && (
                <div className="rounded-md border-l-4 border-info bg-info/10 p-3 text-sm">
                  <div className="text-xs font-medium text-info">
                    Public address assigned at runtime
                  </div>
                  <div className="pt-0.5 text-xs text-info/80">
                    The tunnel&apos;s public address will be assigned by playit.gg and displayed on the connection card.
                  </div>
                </div>
              )}
              {computedValidityError && (
                <div className="rounded-md border-l-4 border-danger bg-danger/10 p-3 text-sm">
                  <div className="flex gap-2">
                    <AlertCircle className="h-5 w-5 shrink-0 text-danger" />
                    <div className="text-xs text-danger">{computedValidityError}</div>
                  </div>
                </div>
              )}
            </div>
          )}
        </div>
      </div>

      <Field label="Hostname" hint="Optional DNS name for ingress / external-dns annotations.">
        <Input
          value={net.hostname ?? ""}
          onChange={(e) => setNet({ ...net, hostname: e.target.value || undefined })}
          placeholder="mc.example.com"
          spellCheck={false}
        />
      </Field>

      <Field
        label="Service annotations"
        hint="Merged into the Service. Useful for cloud LB or external-dns config."
      >
        <KVEditor
          values={net.serviceAnnotations ?? {}}
          onChange={(v) => setNet({ ...net, serviceAnnotations: v })}
          keyPlaceholder="service.beta.kubernetes.io/…"
          valuePlaceholder="value"
        />
      </Field>

      <Field
        label="Port overrides"
        hint="Pin a NodePort or override the Service port for a named template port."
      >
        <PortOverridesEditor
          values={net.portOverrides ?? []}
          onChange={(v) => setNet({ ...net, portOverrides: v })}
        />
      </Field>

      {net.expose === "LoadBalancer" && (
        <Field
          label="LoadBalancer IP allow-list"
          hint="One CIDR per line. Restricts which clients reach the LoadBalancer; empty allows all."
        >
          <textarea
            className="block w-full rounded-md border border-border bg-surface px-3 py-2 font-mono text-sm"
            rows={3}
            value={(net.sourceRanges ?? []).join("\n")}
            onChange={(e) =>
              setNet({
                ...net,
                sourceRanges: e.target.value
                  .split(/[\n,]/)
                  .map((s) => s.trim())
                  .filter(Boolean),
              })
            }
            placeholder={"203.0.113.0/24\n10.0.0.0/8"}
            aria-label="LoadBalancer IP allow-list"
          />
        </Field>
      )}

      <AddressAssignmentSection
        key={`${draft.metadata.namespace}/${draft.metadata.name}`}
        draft={draft}
        net={net}
        setNet={setNet}
      />
    </div>
  );
}

interface AddressAssignmentSectionProps {
  draft: GameServer;
  net: GameServerNetworking;
  setNet: (next: GameServerNetworking) => void;
}

function AddressAssignmentSection({
  draft,
  net,
  setNet,
}: AddressAssignmentSectionProps) {
  const status = draft.status;
  const condition = status?.conditions?.find(
    (c) => c.type === "AddressAssignment",
  );

  const currentAddress = draft.status?.endpoints?.[0];

  // Locally-tracked field values, seeded from the spec and re-synced whenever
  // the underlying draft changes them (e.g. switching servers, or a save
  // round-trip). Deriving these two fields from `net` alone — the way the
  // rest of this form does — breaks when a caller edits both fields back to
  // back without an intervening re-render carrying the updated draft back
  // in as props: each edit would recompute against the same stale `net`
  // snapshot and the second edit could resurrect the first one's clear.
  // Tracking them locally makes consecutive edits within one render
  // cumulative instead of independently-stale.
  // React remounts this component (via a key on the parent) when the server
  // identity changes, so useState automatically re-initializes with the new net values.
  const [addressPool, setAddressPool] = useState(net.addressPool ?? "");
  const [address, setAddress] = useState(net.address ?? "");

  const updateAddressPool = (value: string) => {
    setAddressPool(value);
    setNet({ ...net, addressPool: value || undefined, address: address || undefined });
  };

  const updateAddress = (value: string) => {
    setAddress(value);
    setNet({ ...net, addressPool: addressPool || undefined, address: value || undefined });
  };

  return (
    <div className="space-y-4 border-t border-border pt-6">
      {currentAddress && (
        <Field
          label="Current assignment"
          hint="Address and pool currently bound to this server by the operator."
        >
          <div className="rounded-md border border-border bg-surface px-3 py-2">
            <div className="flex items-center gap-2">
              <span className="text-sm text-fg">{currentAddress.host}</span>
              {currentAddress.pool && (
                <span className="text-xs text-muted">from pool &apos;{currentAddress.pool}&apos;</span>
              )}
            </div>
          </div>
        </Field>
      )}

      <Field
        label="Address pool"
        hint="Name of a load-balancer address pool configured by your cluster admin. Leave blank to let the address manager choose."
      >
        <Input
          value={addressPool}
          onChange={(e) => updateAddressPool(e.target.value)}
          placeholder="pool-name"
          spellCheck={false}
        />
      </Field>

      <Field
        label="Requested address"
        hint="A specific address to request from the pool. Must be free — the address manager rejects addresses already in use. Leave blank unless you need a specific address."
      >
        <Input
          value={address}
          onChange={(e) => updateAddress(e.target.value)}
          placeholder="e.g. 203.0.113.50"
          spellCheck={false}
        />
      </Field>

      <AddressStatusField
        condition={condition}
        serverNamespace={draft.metadata.namespace}
      />

      {condition?.reason === "IgnoredForExposureMode" && (
        <div className="rounded-md border-l-4 border-warning bg-warning/10 p-3 text-sm">
          <div className="flex gap-2">
            <AlertCircle className="h-5 w-5 shrink-0 text-warning" />
            <div>
              <div className="font-medium text-warning">Address preference ignored</div>
              <div className="pt-0.5 text-xs text-warning/80">
                Expose is set to {net.expose || "ClusterIP"}. Address pool and requested address only take effect when Expose (above) is set to LoadBalancer.
              </div>
            </div>
          </div>
        </div>
      )}

      {condition?.reason === "NoAddressManagerConfigured" && (
        <div className="rounded-md border-l-4 border-warning bg-warning/10 p-3 text-sm">
          <div className="flex gap-2">
            <AlertCircle className="h-5 w-5 shrink-0 text-warning" />
            <div>
              <div className="font-medium text-warning">No address manager configured</div>
              <div className="pt-0.5 text-xs text-warning/80">
                This cluster has no MetalLB or Cilium address manager. Your pool and address preference will be saved but never applied.
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

function AddressStatusField({
  condition,
  serverNamespace,
}: {
  condition?: { reason?: string; message?: string; status?: string };
  serverNamespace?: string;
}) {
  if (!condition) {
    return null;
  }

  const getBadgeColor = (
    reason?: string,
    status?: string,
  ): { bgColor: string; textColor: string; label: string } => {
    if (status === "True") {
      return {
        bgColor: "bg-success/20",
        textColor: "text-success",
        label: "Assigned",
      };
    }

    // For False status, determine label from reason or message
    if (reason === "AssignmentPending" || reason === "ServiceNotReady") {
      return {
        bgColor: "bg-warning/20",
        textColor: "text-warning",
        label: "Pending",
      };
    }

    return {
      bgColor: "bg-danger/20",
      textColor: "text-danger",
      label: reason === "IgnoredForExposureMode" ? "Ignored" : "Error",
    };
  };

  // Parse AddressInUse message to extract the conflicting server name.
  // Message format: "Requested address "<address>" is already in use by GameServer "<name>"."
  // The operator names conflicting servers as:
  // - "namespace/name" if from a different namespace
  // - bare "name" if from the same namespace
  const parseConflictingServer = (
    message?: string,
  ): { name: string; namespace?: string } | null => {
    if (!message || !message.includes('is already in use by GameServer')) {
      return null;
    }
    // Extract the server identifier between the last pair of quotes.
    // Pattern: GameServer "name" or GameServer "namespace/name"
    const match = message.match(/GameServer "([^"]+)"/);
    if (!match || !match[1]) {
      return null;
    }
    const identifier = match[1];
    // Check if the identifier contains a "/" separator (cross-namespace)
    if (identifier.includes('/')) {
      const parts = identifier.split('/');
      // Guard against malformed input: must be exactly 2 parts, both non-empty
      if (parts.length === 2 && parts[0] && parts[1]) {
        return { name: parts[1], namespace: parts[0] };
      }
      // Malformed cross-namespace identifier; return null to fall back to plain message
      return null;
    }
    // Same namespace: use the viewed server's namespace
    return { name: identifier, namespace: serverNamespace };
  };

  const badge = getBadgeColor(condition.reason, condition.status);
  const conflictingServer = condition.reason === "AddressInUse"
    ? parseConflictingServer(condition.message)
    : null;

  return (
    <Field
      label="Address status"
      hint="Reflects the GameServer's AddressAssignment condition."
    >
      <div className="space-y-4">
        <div className="flex items-start gap-3">
          <div
            className={`${badge.bgColor} rounded px-2 py-1 text-xs font-medium ${badge.textColor} shrink-0`}
          >
            {badge.label}
          </div>
          <div className="flex-1 pt-1 text-sm text-fg">
            {conflictingServer ? (
              <>
                Requested address {condition.message?.match(/"([^"]+)"/)?.[1]} is already in use by{" "}
                <Link
                  to="/servers/$name"
                  params={{ name: conflictingServer.name }}
                  search={
                    conflictingServer.namespace ? { ns: conflictingServer.namespace } : {}
                  }
                  className="text-primary hover:underline"
                >
                  GameServer &quot;{conflictingServer.name}&quot;
                </Link>
                .
              </>
            ) : (
              condition.message
            )}
          </div>
        </div>
      </div>
    </Field>
  );
}

interface EditingPortMapping {
  name: string;
  remotePort?: number;
}

function FrpPortMappingEditor({
  values,
  onChange,
}: {
  values: RemotePortMapping[];
  onChange: (v: RemotePortMapping[]) => void;
}) {
  const [editingValues, setEditingValues] = useState<EditingPortMapping[]>(
    values.map((v) => ({ ...v })),
  );

  const update = (idx: number, patch: Partial<EditingPortMapping>) => {
    const updated = editingValues.map((p, i) =>
      i === idx ? { ...p, ...patch } : p,
    );
    setEditingValues(updated);
    // Filter to only valid entries (both name and remotePort present).
    const valid = updated.filter(
      (p) => p.name && p.remotePort !== undefined && p.remotePort !== null,
    ) as RemotePortMapping[];
    onChange(valid);
  };

  const remove = (idx: number) => {
    const updated = editingValues.filter((_, i) => i !== idx);
    setEditingValues(updated);
    const valid = updated.filter(
      (p) => p.name && p.remotePort !== undefined && p.remotePort !== null,
    ) as RemotePortMapping[];
    onChange(valid);
  };

  const add = () => {
    setEditingValues([...editingValues, { name: "", remotePort: undefined }]);
  };

  return (
    <div className="space-y-2">
      {editingValues.length > 0 && (
        <div className="hidden gap-2 text-xs text-muted sm:grid sm:grid-cols-[1fr_120px_32px] sm:items-center">
          <span>Port name</span>
          <span>Remote port</span>
          <span />
        </div>
      )}
      {editingValues.map((p, idx) => (
        <div
          key={idx}
          className="grid grid-cols-1 gap-2 rounded border border-border bg-surface/30 p-2 sm:grid-cols-[1fr_120px_32px] sm:items-center sm:border-0 sm:bg-transparent sm:p-0"
        >
          <Input
            value={p.name}
            onChange={(e) => update(idx, { name: e.target.value })}
            placeholder="game"
          />
          <Input
            type="number"
            value={p.remotePort !== undefined ? p.remotePort.toString() : ""}
            onChange={(e) =>
              update(idx, {
                remotePort: e.target.value ? Number(e.target.value) : undefined,
              })
            }
            inputMode="numeric"
            placeholder="8080"
          />
          <Button
            variant="ghost"
            size="sm"
            className="justify-self-start sm:h-8 sm:w-8 sm:justify-self-auto sm:p-0"
            title="Remove"
            onClick={() => remove(idx)}
          >
            <X className="h-3 w-3" />
            <span className="sm:hidden">Remove mapping</span>
          </Button>
        </div>
      ))}
      <Button size="sm" variant="outline" onClick={add}>
        Add port mapping
      </Button>
    </div>
  );
}

function KVEditor({
  values,
  onChange,
  keyPlaceholder,
  valuePlaceholder,
}: {
  values: Record<string, string>;
  onChange: (v: Record<string, string>) => void;
  keyPlaceholder: string;
  valuePlaceholder: string;
}) {
  const [draftKV, setDraftKV] = useState({ key: "", value: "" });
  const add = () => {
    const key = draftKV.key.trim();
    if (!key) return;
    onChange({ ...values, [key]: draftKV.value });
    setDraftKV({ key: "", value: "" });
  };
  return (
    <div className="space-y-2">
      {Object.entries(values).map(([k, v]) => (
        <div
          key={k}
          className="flex items-center gap-2 rounded border border-border bg-surface/40 px-2 py-1"
        >
          <span className="flex-1 truncate font-mono text-xs">{k}</span>
          <span className="text-muted">=</span>
          <span className="flex-1 truncate font-mono text-xs">{v}</span>
          <Button
            variant="ghost"
            size="icon"
            className="h-6 w-6"
            title="Remove"
            onClick={() => {
              const next = { ...values };
              delete next[k];
              onChange(next);
            }}
          >
            <X className="h-3 w-3" />
          </Button>
        </div>
      ))}
      <div className="flex items-center gap-2">
        <Input
          value={draftKV.key}
          onChange={(e) => setDraftKV({ ...draftKV, key: e.target.value })}
          placeholder={keyPlaceholder}
          className="flex-1"
        />
        <Input
          value={draftKV.value}
          onChange={(e) => setDraftKV({ ...draftKV, value: e.target.value })}
          placeholder={valuePlaceholder}
          className="flex-1"
        />
        <Button size="sm" variant="outline" onClick={add} disabled={!draftKV.key.trim()}>
          Add
        </Button>
      </div>
    </div>
  );
}

interface TunnelCredentialFieldProps {
  configured?: boolean;
  secretName?: string;
  isLoading: boolean;
  error?: string;
  showInput: boolean;
  onShowInput: () => void;
  provider: "frp" | "tailscale" | "playit";
  credentialValue: string;
  onCredentialChange: (value: string) => void;
  onSave: () => void;
  onCancel: () => void;
  onRemove: () => void;
  hasSecretRef: boolean;
}

function TunnelCredentialField({
  configured,
  secretName,
  isLoading,
  error,
  showInput,
  onShowInput,
  provider,
  credentialValue,
  onCredentialChange,
  onSave,
  onCancel,
  onRemove,
  hasSecretRef,
}: TunnelCredentialFieldProps) {
  const labels: Record<"frp" | "tailscale" | "playit", string> = {
    frp: "frp token",
    tailscale: "Tailscale auth key",
    playit: "playit secret key",
  };

  if (!showInput && (configured || hasSecretRef)) {
    return (
      <div className="space-y-2">
        <div className="flex items-center justify-between">
          <span className="text-sm font-medium text-fg">Tunnel credentials</span>
        </div>
        <div className="rounded-md border border-border bg-surface/30 px-3 py-2 flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Check className="h-4 w-4 text-success" />
            <div className="text-sm">
              <div className="text-fg font-medium">Configured</div>
              {secretName && <div className="text-xs text-muted">Secret: {secretName}</div>}
            </div>
          </div>
          <div className="flex items-center gap-2">
            <Button
              size="sm"
              variant="outline"
              onClick={onShowInput}
              disabled={isLoading}
            >
              Replace
            </Button>
            <Button
              size="sm"
              variant="ghost"
              onClick={onRemove}
              disabled={isLoading}
              className="text-danger hover:text-danger"
            >
              <X className="h-4 w-4" />
            </Button>
          </div>
        </div>
        {error && (
          <div className="rounded-md border-l-4 border-danger bg-danger/10 p-3 text-sm">
            <div className="flex gap-2">
              <AlertCircle className="h-5 w-5 shrink-0 text-danger" />
              <div className="text-xs text-danger">{error}</div>
            </div>
          </div>
        )}
      </div>
    );
  }

  return (
    <div className="space-y-2">
      <div className="space-y-1.5">
        <label className="block space-y-1.5">
          <span className="text-sm font-medium text-fg">Tunnel credentials</span>
          <Input
            type="password"
            value={credentialValue}
            onChange={(e) => onCredentialChange(e.target.value)}
            placeholder={labels[provider]}
            spellCheck={false}
            disabled={isLoading}
          />
          <span className="text-[11px] text-muted">
            {labels[provider]} — stored in a Kubernetes Secret, never displayed.
          </span>
        </label>
      </div>
      {error && (
        <div className="rounded-md border-l-4 border-danger bg-danger/10 p-3 text-sm">
          <div className="flex gap-2">
            <AlertCircle className="h-5 w-5 shrink-0 text-danger" />
            <div className="text-xs text-danger">{error}</div>
          </div>
        </div>
      )}
      <div className="flex gap-2">
        <Button
          size="sm"
          onClick={onSave}
          disabled={isLoading || !credentialValue.trim()}
        >
          {isLoading ? (
            <>
              <Loader2 className="h-4 w-4 animate-spin" /> Saving...
            </>
          ) : (
            <>
              <Check className="h-4 w-4" /> Save credential
            </>
          )}
        </Button>
        <Button size="sm" variant="outline" onClick={onCancel} disabled={isLoading}>
          Cancel
        </Button>
      </div>
    </div>
  );
}

