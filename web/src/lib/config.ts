import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api";

// Section schemas mirror api/internal/handlers/config.go validators.
// Keep these in sync if you add or rename a field server-side.
export interface GeneralCfg {
  instanceName: string;
  externalURL: string;
  defaultNamespace: string;
}

export type AuthKind = "local" | "oidc" | "google" | "github";

// Per-role IdP group lists (mirrors auth.RoleMappings server-side). A user
// matching several gets the most privileged (admin > operator > viewer).
export interface AuthRoleMappings {
  admin?: string[];
  operator?: string[];
  viewer?: string[];
}

// Fallback when roleMappings is set but no group matches; "deny" refuses
// the login. Absent means viewer.
export type AuthDefaultRole = "viewer" | "operator" | "admin" | "deny";

export interface AuthProvider {
  name: string;
  kind: AuthKind;
  displayName?: string; // login button label; defaults to the name
  enabled: boolean;
  // Non-local kinds: issuer + client id are public identifiers and live
  // in the config row; only the clientSecret hides in the configRef
  // Secret (default gameplane-auth-<name>, written via AuthProviders.putSecret).
  issuer?: string;
  clientID?: string;
  configRef?: string;
  // Group→role mapping (non-local kinds only). scopes are extra OAuth
  // scopes beyond openid/profile/email; groupsClaim defaults to "groups"
  // server-side; roleMappings absent = roles never touched after creation.
  scopes?: string[];
  groupsClaim?: string;
  roleMappings?: AuthRoleMappings;
  defaultRole?: AuthDefaultRole;
}

export interface AuthCfg {
  providers: AuthProvider[];
}

export type SinkKind = "discord" | "slack" | "smtp" | "webhook" | "ntfy";

// The closed event set mirrors api/internal/notify/events.go. A sink with
// no explicit filter receives the failure events plus server.recovered.
export type NotifEventType =
  | "server.unhealthy"
  | "server.recovered"
  | "backup.failed"
  | "backup.succeeded"
  | "restore.failed"
  | "restore.succeeded";

export interface NotifSink {
  name: string;
  kind: SinkKind;
  enabled: boolean;
  // K8s Secret (labelled gameplane.local/notification-sink=true, in the
  // control-plane namespace) holding the sink credentials. Sinks persisted
  // before delivery existed lack it; they deliver nothing until set.
  configRef?: string;
  events?: NotifEventType[];
}

export interface NotificationsCfg {
  sinks: NotifSink[];
}

export interface TelemetryCfg {
  sendMetrics: boolean;
}

// Mod-registry providers that take an admin-configured API key (mirrors
// registry.KeyedProviders server-side) and stay hidden from the Mods
// browser until one is set. The rest (modrinth, thunderstore, hangar,
// factorio, spigot, github, umod) are keyless and always available.
export type KeyedRegistryProvider = "curseforge" | "steam" | "nexus";

// One admin-managed mod-registry provider row. It carries no secret
// material — the apiKey lives in a labelled, write-only K8s Secret managed
// via ModRegistries.putSecret/deleteSecret; configRef only records a
// non-default Secret name override (registry.DefaultKeySecretName(provider)
// is used when absent).
export interface ModRegistryEntry {
  provider: KeyedRegistryProvider;
  configRef?: string;
}

export interface ModRegistriesCfg {
  registries: ModRegistryEntry[];
}

// AllConfig is the shape of GET /admin/config — every section is optional
// because the row only exists once it's been written. Defaults are
// supplied by the section components, not by the API.
//
// Backup destinations are NOT in this config — they live as labelled
// Kubernetes Secrets, served via /backup-destinations. The former
// "updates" section is gone too: the release channel is the chart's
// informational updates.channel value, read-only on /cluster/info.
export interface AllConfig {
  general?: GeneralCfg;
  auth?: AuthCfg;
  notifications?: NotificationsCfg;
  telemetry?: TelemetryCfg;
  modRegistries?: ModRegistriesCfg;
}

const KEY = ["config"] as const;

export function useConfig() {
  return useQuery({
    queryKey: KEY,
    queryFn: () => api<AllConfig>("/admin/config"),
  });
}

// useUpdateConfigSection PUTs a single section's blob and invalidates the
// cached AllConfig so the GET re-fires on success. The mutation type is
// section-specific so callers get type-checked payloads.
export function useUpdateConfigSection<K extends keyof AllConfig>(section: K) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: AllConfig[K]) =>
      api(`/admin/config/${section}`, { method: "PUT", body }),
    onSuccess: () => qc.invalidateQueries({ queryKey: KEY }),
  });
}
